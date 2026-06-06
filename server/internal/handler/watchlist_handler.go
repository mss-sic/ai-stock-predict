package handler

import (
	"net/http"
	"strconv"

	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

type WatchlistHandler struct {
	svc *service.WatchlistService
}

func NewWatchlistHandler() *WatchlistHandler { return &WatchlistHandler{svc: service.NewWatchlistService()} }

func (h *WatchlistHandler) List(c *gin.Context) {
	userID, _ := strconv.Atoi(c.DefaultQuery("userId", "1"))
	items, err := h.svc.List(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *WatchlistHandler) Add(c *gin.Context) {
	var body struct {
		UserId    uint   `json:"userId"`
		StockCode string `json:"stockCode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.UserId == 0 { body.UserId = 1 }
	if err := h.svc.Add(body.UserId, body.StockCode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "added"})
}

func (h *WatchlistHandler) Remove(c *gin.Context) {
	code := c.Param("code")
	userID, _ := strconv.Atoi(c.DefaultQuery("userId", "1"))
	if err := h.svc.Remove(uint(userID), code); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}
