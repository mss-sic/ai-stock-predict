package handler

import (
	"log"
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
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
	c.JSON(http.StatusOK, gin.H{"data": stock})
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
	c.JSON(http.StatusOK, gin.H{"data": klines})
}

func (h *StockHandler) GetIndicator(c *gin.Context) {
	code := c.Param("code")
	ind, err := h.svc.GetIndicator(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ind})
}

func (h *StockHandler) GetSignal(c *gin.Context) {
	code := c.Param("code")
	signal, err := h.svc.GetSignal(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "signal not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": signal})
}

func (h *StockHandler) GetQuote(c *gin.Context) {
	code := c.Param("code")
	q, err := h.svc.GetQuote(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": q})
}

func (h *StockHandler) GetFinancials(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetFinancials(code)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
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
	c.JSON(http.StatusOK, gin.H{"data": data})
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
	c.JSON(http.StatusOK, gin.H{"data": data})
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
	c.JSON(http.StatusOK, gin.H{"data": data})
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
	c.JSON(http.StatusOK, gin.H{"data": data})
}
