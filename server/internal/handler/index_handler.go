package handler

import (
	"encoding/json"
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
	idxCacheTTL   = 30 * time.Second
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
	url := "https://web.ifzq.gtimg.cn/appstock/app/indexlist/get?market=hs&type=index"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
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

	var result struct {
		Code int `json:"code"`
		Data map[string]struct {
			Name   string  `json:"name"`
			Last   float64 `json:"last"`
			Chg    float64 `json:"chg"`
			ChgPct string  `json:"chgPct"`
		} `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		log.Printf("[indices] json error: %v, body=%s", err, string(bodyBytes[:min(len(bodyBytes), 300)]))
		return nil
	}

	indexMap := map[string]IndexData{
		"sh000001": {Name: "上证指数", Code: "000001"},
		"sz399001": {Name: "深证成指", Code: "399001"},
		"sz399006": {Name: "创业板指", Code: "399006"},
	}

	var indices []IndexData
	for key, idx := range indexMap {
		if data, ok := result.Data[key]; ok {
			chgPct, _ := strconv.ParseFloat(strings.TrimSuffix(data.ChgPct, "%"), 64)
			indices = append(indices, IndexData{
				Name: idx.Name, Code: idx.Code,
				Val: data.Last, Chg: data.Chg, ChgPct: chgPct,
			})
		}
	}
	return indices
}

func GetIndices(c *gin.Context) {
	idxCacheLock.RLock()
	if len(idxCache) > 0 && time.Since(idxCacheTime) < idxCacheTTL {
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
