package handler

import (
	"net/http"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

type BoardHandler struct {
	svc *service.BoardService
}

func NewBoardHandler() *BoardHandler { return &BoardHandler{svc: service.NewBoardService()} }

func (h *BoardHandler) Today(c *gin.Context) {
	data, err := h.svc.GetToday()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *BoardHandler) History(c *gin.Context) {
	date := c.Query("date")
	data, err := h.svc.GetByDate(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *BoardHandler) Heatmap(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.svc.GetHeatmap(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *BoardHandler) StockHeatmap(c *gin.Context) {
	code := c.Param("code")
	data, err := h.svc.GetStockHeatmap(code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func (h *BoardHandler) HeatmapEnriched(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")
	data, err := h.svc.GetEnrichedHeatmap(from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
