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
