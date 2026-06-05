package handler

import (
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
