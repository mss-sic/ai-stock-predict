package handler

import (
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type CapitalFlowHandler struct{}

func (h *CapitalFlowHandler) GetSummary(c *gin.Context) {
	data, err := service.GetCapitalSummary()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取资金面概览失败: "+err.Error())
		return
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetFundFlowTop(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	direction := c.DefaultQuery("direction", "in")
	data, err := service.GetFundFlowTop(limit, direction)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取资金流向排名失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.FundFlowRank{}
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetNorthboundTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	data, err := service.GetNorthboundTrend(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取北向趋势失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.NorthboundTrend{}
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetFundFlowDaily(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "60"))
	data, err := service.GetFundFlowDaily(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取资金流向趋势失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.FundFlowDaily{}
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetMarginTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "60"))
	data, err := service.GetMarginTrend(days)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取融资融券趋势失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.MarginTrend{}
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetMarginTop(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	marginType := c.DefaultQuery("type", "rz")
	data, err := service.GetMarginTop(limit, marginType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取融资融券排名失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.MarginTop{}
	}
	response.Success(c, data)
}

func (h *CapitalFlowHandler) GetStockCapitalRank(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	sortBy := c.DefaultQuery("sort", "netFlow"); order := c.DefaultQuery("order", "desc")
	data, err := service.GetStockCapitalRank(limit, sortBy, order)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取资金排名失败: "+err.Error())
		return
	}
	if data == nil {
		data = []service.StockCapitalRank{}
	}
	response.Success(c, data)
}
