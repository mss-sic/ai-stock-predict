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

	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type IndexData struct {
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	Val    float64 `json:"val"`
	Chg    float64 `json:"chg"`
	ChgPct float64 `json:"chgPct"`
}

var (
	idxCache      []IndexData
	idxCacheLock  sync.RWMutex
	idxCacheTime  time.Time
	idxCacheShortTTL = 5 * time.Second    // 盘中 5 秒刷新
	idxCacheLongTTL  = 5 * time.Minute   // 盘后 5 分钟
)

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
	// 使用腾讯 qt 行情接口拉取大盘指数
	// 格式: v_sh000001="1~上证指数~000001~现价~昨收~今开~..."
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
		// parts[1]=名称, parts[3]=现价, parts[4]=昨收
		val, _ := strconv.ParseFloat(parts[3], 64)
		prevClose, _ := strconv.ParseFloat(parts[4], 64)
		chg := val - prevClose
		chgPct := 0.0
		if prevClose != 0 {
			chgPct = (chg / prevClose) * 100
		}
		// 保留两位小数
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
