package handler

import (
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
	idxCache     []IndexData
	idxCacheLock sync.RWMutex
	idxCacheTime time.Time
)

func isTradingTime() bool {
	now := time.Now().In(time.FixedZone("CST", 8*3600))
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	t930 := time.Date(now.Year(), now.Month(), now.Day(), 9, 30, 0, 0, now.Location())
	t1500 := time.Date(now.Year(), now.Month(), now.Day(), 15, 0, 0, 0, now.Location())
	return now.After(t930) && now.Before(t1500)
}

func fetchFromSina() []IndexData {
	req, err := http.NewRequest("GET", "https://hq.sinajs.cn/list=s_sh000001,s_sz399001,s_sz399006", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	log.Printf("[indices] HTTP status: %d", resp.StatusCode)
	if err != nil {
		log.Printf("[indices] fetch error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		log.Printf("[indices] read body error: %v, len=%d", err, len(bodyBytes))
		return nil
	}
	body := string(bodyBytes)
	log.Printf("[indices] body len=%d, first 200: %s", len(bodyBytes), body[:min(len(body), 200)])

	configs := []struct {
		prefix string
		name   string
		code   string
	}{
		{"s_sh000001", "上证指数", "000001"},
		{"s_sz399001", "深证成指", "399001"},
		{"s_sz399006", "创业板指", "399006"},
	}

	var indices []IndexData
	for _, cfg := range configs {
		idx := strings.Index(body, cfg.prefix)
		if idx < 0 {
			continue
		}
		start := strings.Index(body[idx:], `"`)
		if start < 0 {
			continue
		}
		end := strings.Index(body[idx+start+1:], `"`)
		if end < 0 {
			continue
		}
		val := body[idx+start+1 : idx+start+1+end]
		parts := strings.Split(val, ",")
		if len(parts) < 4 {
			continue
		}
		v, _ := strconv.ParseFloat(parts[1], 64)
		c, _ := strconv.ParseFloat(parts[2], 64)
		p, _ := strconv.ParseFloat(parts[3], 64)
		indices = append(indices, IndexData{
			Name: cfg.name, Code: cfg.code,
			Val: v, Chg: c, ChgPct: p,
		})
	}
	return indices
}

func GetIndices(c *gin.Context) {
	idxCacheLock.RLock()
	age := time.Since(idxCacheTime)
	idxCacheLock.RUnlock()

	maxAge := 30 * time.Minute
	if isTradingTime() {
		maxAge = 3 * time.Second
	}

	if len(idxCache) > 0 && age < maxAge {
		response.Success(c, gin.H{
			"indices": idxCache,
			"cached":  true,
			"trading": isTradingTime(),
		})
		return
	}

	indices := fetchFromSina()
	if len(indices) == 0 {
		if len(idxCache) > 0 {
			response.Success(c, gin.H{
				"indices": idxCache,
				"cached":  true,
				"stale":   true,
				"trading": isTradingTime(),
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

	response.Success(c, gin.H{
		"indices": indices,
		"cached":  false,
		"trading": isTradingTime(),
	})
}
