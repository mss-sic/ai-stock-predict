package handler

import (
	"net/http"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// IndustryHandler handles industry comparison API requests.
type IndustryHandler struct{}

// NewIndustryHandler creates a new industry handler.
func NewIndustryHandler() *IndustryHandler { return &IndustryHandler{} }

// List returns industry-level aggregate comparisons.
// Query params: ?date=2026-01-15 (optional, defaults to latest trading day)
func (h *IndustryHandler) List(c *gin.Context) {
	date := c.DefaultQuery("date", "")

	list, err := service.GetIndustryList(date)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取行业对比数据失败: "+err.Error())
		return
	}
	response.Success(c, list)
}

// Stocks returns all stocks within an industry, ranked by PE (default) or return/change.
// Query params: ?date=&sort=pe|return|change
func (h *IndustryHandler) Stocks(c *gin.Context) {
	industry := c.Param("name")
	if industry == "" {
		response.Error(c, http.StatusBadRequest, 400, "缺少行业名称参数")
		return
	}
	date := c.DefaultQuery("date", "")
	sortBy := c.DefaultQuery("sort", "pe")

	list, err := service.GetIndustryStocks(industry, date, sortBy)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取行业个股数据失败: "+err.Error())
		return
	}
	if list == nil {
		list = []service.IndustryStock{}
	}
	response.Success(c, list)
}
