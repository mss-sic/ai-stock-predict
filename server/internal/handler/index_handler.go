package handler

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

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
	if err != nil {
		log.Printf("[indices] fetch error: %v", err)
		return nil
	}
	defer resp.Body.Close()

	buf := make([]byte, 2048)
	n, _ := resp.Body.Read(buf)
	body := string(buf[:n])

	// sina format: var hq_str_s_sh000001="上证指数,4027.74,-30.04,-0.74,..."
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
		// Find the quoted value
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
		c.JSON(http.StatusOK, gin.H{"data": idxCache, "cached": true, "trading": isTradingTime()})
		return
	}

	indices := fetchFromSina()
	if len(indices) == 0 {
		if len(idxCache) > 0 {
			c.JSON(http.StatusOK, gin.H{"data": idxCache, "cached": true, "stale": true, "trading": isTradingTime()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": []IndexData{}, "error": "fetch failed"})
		return
	}

	idxCacheLock.Lock()
	idxCache = indices
	idxCacheTime = time.Now()
	idxCacheLock.Unlock()

	c.JSON(http.StatusOK, gin.H{"data": indices, "cached": false, "trading": isTradingTime()})
}
