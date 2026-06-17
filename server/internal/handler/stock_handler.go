package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/repository"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/pkg/response"
)

type StockHandler struct {
	svc *service.StockService
}

func NewStockHandler() *StockHandler { return &StockHandler{svc: service.NewStockService()} }

func (h *StockHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	industry := c.Query("industry")
	keyword := c.Query("keyword")

	stocks, total, err := h.svc.List(industry, keyword, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stocks, "total": total, "page": page, "pageSize": pageSize})
}

func (h *StockHandler) GetDetail(c *gin.Context) {
	code := c.Param("code")
	stock, err := h.svc.GetDetail(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "stock not found"})
		return
	}
	response.Success(c, stock)
}

func (h *StockHandler) GetKLine(c *gin.Context) {
	code := c.Param("code")
	from := c.Query("from")
	to := c.Query("to")
	klines, err := h.svc.GetKLine(code, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	response.Success(c, klines)
}

func (h *StockHandler) GetIndicator(c *gin.Context) {
	code := c.Param("code")
	ind, err := h.svc.GetIndicator(code)
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, ind)
}

func (h *StockHandler) GetSignal(c *gin.Context) {
	code := c.Param("code")
	signal, err := h.svc.GetSignal(code)
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, signal)
}



func (h *StockHandler) GetFinancials(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetFinancials(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetShareholders(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetShareholders(code)
	if err != nil {
		log.Printf("[Shareholders] query error for %s: %v", code, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	log.Printf("[Shareholders] got %d rows for %s", len(data), code)
	response.Success(c, data)
}

func (h *StockHandler) GetNews(c *gin.Context) {
	code := c.Param("code")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetNews(code, limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetReports(c *gin.Context) {
	code := c.Param("code")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetReports(code, limit)
	if err != nil {
		log.Printf("[Reports] query error for %s: %v", code, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func (h *StockHandler) GetIndustryReports(c *gin.Context) {
	industry := c.Query("industry")
	if industry == "" {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	data, err := h.svc.GetIndustryReports(industry, limit)
	if err != nil {
		log.Printf("[IndustryReports] query error for %s: %v", industry, err)
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	response.Success(c, data)
}

func GetDataStats(c *gin.Context) {
	stats := repository.GetDataStats()
	response.Success(c, stats)
}

func GetDataDetail(c *gin.Context) {
	typ := c.Param("type")
	results := repository.GetDataDetail(typ)
	response.Success(c, results)
}

// RepairKLine triggers full data repair for a stock (async).
func (h *StockHandler) RepairKLine(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		response.BadRequest(c, "缺少股票代码")
		return
	}
	// Run repair in background (can take ~1-5 seconds)
	go func() {
		log.Printf("[repair] starting for %s", code)
		if err := collector.RepairStock(code); err != nil {
			log.Printf("[repair] failed for %s: %v", code, err)
		} else {
			log.Printf("[repair] completed for %s", code)
		}
	}()
	response.Success(c, gin.H{"message": "数据修复已触发", "stockCode": code})
}
