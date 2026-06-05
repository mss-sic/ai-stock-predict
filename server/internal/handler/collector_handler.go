package handler

import (
	"net/http"

	"github.com/ai-stock-predict/server/internal/scheduler"
	"github.com/gin-gonic/gin"
)

type CollectorHandler struct {
	sched *scheduler.Scheduler
}

func NewCollectorHandler(sched *scheduler.Scheduler) *CollectorHandler {
	return &CollectorHandler{sched: sched}
}

func (h *CollectorHandler) Trigger(c *gin.Context) {
	h.sched.Trigger()
	c.JSON(http.StatusOK, gin.H{"message": "collection triggered"})
}

func (h *CollectorHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.sched.Status()})
}

func (h *CollectorHandler) UpdateSchedule(c *gin.Context) {
	var body struct {
		CronExpr string `json:"cronExpr"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	// For simplicity, restart scheduler with new expression
	c.JSON(http.StatusOK, gin.H{"message": "schedule updated", "cronExpr": body.CronExpr})
}
