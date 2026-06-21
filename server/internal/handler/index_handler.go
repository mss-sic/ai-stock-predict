package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type IndexData struct {
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	Val    float64 `json:"val"`
	Chg    float64 `json:"chg"`
	ChgPct float64 `json:"chgPct"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Open   float64 `json:"open"`
	Amount float64 `json:"amount"` // 亿元
}

var (
	idxCache      []IndexData
	idxCacheLock  sync.RWMutex
	idxCacheTime  time.Time
	idxCacheShortTTL = 5 * time.Second    // 盘中 5 秒刷新
	idxCacheLongTTL  = 5 * time.Minute   // 盘后 5 分钟
)

// mapping from Tencent key to stocks_daily_k code
var idxCodeMap = map[string]string{
	"sh000001": "IDX000001",
	"sz399001": "IDX399001",
	"sz399006": "IDX399006",
}

func isTradingHour() bool {
	now := time.Now()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	t930 := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, now.Location())
	t1500 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())
	return now.After(t930) && now.Before(t1500)
}

func fetchFromTencent() []IndexData {
	indexKeys := []string{"sh000001", "sz399001", "sz399006"}
	indexNames := map[string]string{
		"sh000001": "上证指数", "sz399001": "深证成指", "sz399006": "创业板指",
	}

	url := "http://qt.gtimg.cn/q=" + strings.Join(indexKeys, ",")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("[indices] fetch error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		log.Printf("[indices] read error: %v, len=%d", err, len(bodyBytes))
		return nil
	}

	var indices []IndexData
	for _, key := range indexKeys {
		prefix := "v_" + key + "=\""
		idx := strings.Index(string(bodyBytes), prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		end := strings.Index(string(bodyBytes[start:]), "\"")
		if end < 0 {
			continue
		}
		raw := string(bodyBytes[start : start+end])
		parts := strings.Split(raw, "~")
		if len(parts) < 5 {
			continue
		}
		val, _ := strconv.ParseFloat(parts[3], 64)
		prevClose, _ := strconv.ParseFloat(parts[4], 64)
		chg := val - prevClose
		chgPct := 0.0
		if prevClose != 0 {
			chgPct = (chg / prevClose) * 100
		}
		val, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", val), 64)
		chg, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", chg), 64)
		chgPct, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", chgPct), 64)
		code := strings.TrimPrefix(key, "sh")
		code = strings.TrimPrefix(code, "sz")
		indices = append(indices, IndexData{
			Name: indexNames[key], Code: code,
			Val: val, Chg: chg, ChgPct: chgPct,
		})
	}
	return indices
}

// enrichIndices fills High/Low/Open/Amount from stocks_daily_k for each index.
func enrichIndices(indices []IndexData) {
	if len(indices) == 0 {
		return
	}

	// Find latest trade date
	var latestDate time.Time
	db.PG.Raw("SELECT MAX(trade_date) FROM stocks_daily_k WHERE code LIKE 'IDX%'").Scan(&latestDate)
	if latestDate.IsZero() {
		return
	}

	type KRow struct {
		Code   string  `gorm:"column:code"`
		Open   float64 `gorm:"column:open"`
		High   float64 `gorm:"column:high"`
		Low    float64 `gorm:"column:low"`
		Amount float64 `gorm:"column:amount"`
	}

	// Build the list of codes to query
	codes := make([]string, 0, len(indices))
	for _, idx := range indices {
		if dbCode, ok := idxCodeMap["sh"+idx.Code]; ok {
			codes = append(codes, dbCode)
		} else if dbCode, ok := idxCodeMap["sz"+idx.Code]; ok {
			codes = append(codes, dbCode)
		}
	}

	if len(codes) == 0 {
		return
	}

	var rows []KRow
	db.PG.Raw("SELECT code, open, high, low, amount FROM stocks_daily_k WHERE code IN ? AND trade_date = ?", codes, latestDate).Scan(&rows)

	rowMap := make(map[string]KRow)
	for _, r := range rows {
		rowMap[r.Code] = r
	}

	for i := range indices {
		key := "sh" + indices[i].Code
		if _, ok := idxCodeMap[key]; !ok {
			key = "sz" + indices[i].Code
		}
		dbCode := idxCodeMap[key]
		if r, ok := rowMap[dbCode]; ok {
			indices[i].Open = r.Open
			indices[i].High = r.High
			indices[i].Low = r.Low
			indices[i].Amount = r.Amount / 1e8 // Convert to 亿
		}
	}
}

func GetIndices(c *gin.Context) {
	idxCacheLock.RLock()
	ttl := idxCacheLongTTL
	if isTradingHour() {
		ttl = idxCacheShortTTL
	}
	if len(idxCache) > 0 && time.Since(idxCacheTime) < ttl {
		cached := idxCache
		idxCacheLock.RUnlock()
		response.Success(c, map[string]interface{}{
			"indices": cached,
			"cached":  true,
			"trading": isTradingHour(),
		})
		return
	}
	idxCacheLock.RUnlock()

	indices := fetchFromTencent()
	if len(indices) == 0 {
		idxCacheLock.RLock()
		fallback := idxCache
		idxCacheLock.RUnlock()
		if len(fallback) > 0 {
			response.Success(c, map[string]interface{}{
				"indices": fallback,
				"cached":  true,
				"trading": false,
			})
			return
		}
		response.InternalError(c, "获取大盘指数失败")
		return
	}

	// Enrich with High/Low/Open/Amount from K-line DB
	enrichIndices(indices)

	idxCacheLock.Lock()
	idxCache = indices
	idxCacheTime = time.Now()
	idxCacheLock.Unlock()

	response.Success(c, map[string]interface{}{
		"indices": indices,
		"cached":  false,
		"trading": isTradingHour(),
	})
}
