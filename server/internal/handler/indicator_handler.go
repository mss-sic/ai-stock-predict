package handler

import (
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// IndicatorHandler handles technical indicator API requests.
type IndicatorHandler struct{}

// GetIndicators returns full technical indicators for a stock.
func (h *IndicatorHandler) GetIndicators(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	daysStr := c.DefaultQuery("days", "120")
	days, _ := strconv.Atoi(daysStr)
	if days < 20 {
		days = 20
	}
	if days > 500 {
		days = 500
	}

	data, err := service.ComputeIndicators(code, days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "计算技术指标失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.IndicatorRow{}
	}
	response.Success(c, data)
}

// ScanSignals scans all stocks for technical signals.
func (h *IndicatorHandler) ScanSignals(c *gin.Context) {
	signals, err := service.ScanGoldenCross(0)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "扫描信号失败: "+err.Error())
		return
	}
	if signals == nil {
		signals = []service.SignalResult{}
	}
	response.Success(c, signals)
}

type indicatorItem struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Category string  `json:"category"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Desc     string  `json:"desc"`
	Zero     bool    `json:"zero"`
	NoData   bool    `json:"noData"`
}

// GetAllIndicators returns all 84 indicators from the JSONB cache for a stock.
func (h *IndicatorHandler) GetAllIndicators(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	date := c.DefaultQuery("date", "")
	cacheSvc := service.NewIndicatorCacheService()

	var allIndicators map[string]float64

	latest := cacheSvc.LatestDateForStock(code)
	if date != "" {
		allIndicators, _ = cacheSvc.GetBatch(code, date)
		if allIndicators == nil && latest != "" {
			date = latest
			allIndicators, _ = cacheSvc.GetBatch(code, latest)
		}
	} else {
		date = latest
		if latest != "" {
			allIndicators, _ = cacheSvc.GetBatch(code, latest)
		}
	}

	if allIndicators == nil {
		allIndicators = make(map[string]float64)
	}

	metaList := AllIndicators("")
	items := make([]indicatorItem, 0, len(metaList))
	for _, m := range metaList {
		v, ok := allIndicators[m.Key]
		item := indicatorItem{
			Key:      m.Key,
			Label:    m.Label,
			Category: m.Category,
			Value:    v,
			Unit:     m.Unit,
			Desc:     m.Desc,
			Zero:     ok && v == 0,
			NoData:   !ok,
		}
		items = append(items, item)
	}

	response.Success(c, map[string]interface{}{
		"indicators": items,
		"date":       date,
		"count":      len(items),
	})
}

// GetIndicatorDates returns available dates with cached indicators for a stock.
func (h *IndicatorHandler) GetIndicatorDates(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	cacheSvc := service.NewIndicatorCacheService()
	dates := cacheSvc.AvailableDates(code, 30)
	if dates == nil {
		dates = []string{}
	}
	response.Success(c, dates)
}
