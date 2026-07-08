package handler

import (
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// SchedulerLogHandler provides task execution history APIs.
type SchedulerLogHandler struct{}

func NewSchedulerLogHandler() *SchedulerLogHandler { return &SchedulerLogHandler{} }

// ListLogs returns paginated task execution logs with filters.
func (h *SchedulerLogHandler) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	status := c.Query("status")
	phase := c.Query("phase")
	dateFrom := c.Query("dateFrom")
	dateTo := c.Query("dateTo")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var logs []model.TaskLog
	var total int64

	query := db.MySQL.Model(&model.TaskLog{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if phase != "" {
		query = query.Where("phase = ?", phase)
	}
	if dateFrom != "" {
		query = query.Where("started_at >= ?", dateFrom)
	}
	if dateTo != "" {
		query = query.Where("started_at <= ?", dateTo+" 23:59:59")
	}

	query.Count(&total)
	query.Order("started_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)

	response.Success(c, map[string]interface{}{
		"items":    logs,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// GetLog returns a single task log with full detail.
func (h *SchedulerLogHandler) GetLog(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的日志ID")
		return
	}

	var log model.TaskLog
	if err := db.MySQL.First(&log, id).Error; err != nil {
		response.NotFound(c, "日志不存在")
		return
	}

	response.Success(c, log)
}

// GetStats returns 24h task execution statistics.
func (h *SchedulerLogHandler) GetStats(c *gin.Context) {
	since := time.Now().Add(-24 * time.Hour)

	var total int64
	var failed int64
	var running int64
	var slowCount int64

	db.MySQL.Model(&model.TaskLog{}).
		Where("started_at >= ?", since).
		Count(&total)

	db.MySQL.Model(&model.TaskLog{}).
		Where("started_at >= ? AND status = ?", since, "failed").
		Count(&failed)

	db.MySQL.Model(&model.TaskLog{}).
		Where("started_at >= ? AND status = ?", since, "running").
		Count(&running)

	db.MySQL.Model(&model.TaskLog{}).
		Where("started_at >= ? AND duration_ms > ?", since, 300000).
		Count(&slowCount)

	successRate := 0.0
	if total > 0 {
		successRate = float64(total-failed-running) / float64(total) * 100
	}

	response.Success(c, map[string]interface{}{
		"total24h":     total,
		"failed24h":    failed,
		"running24h":   running,
		"slowTasks24h": slowCount,
		"successRate":  mathRound(successRate, 1),
	})
}

func mathRound(v float64, decimals int) float64 {
	pow := 1.0
	for i := 0; i < decimals; i++ {
		pow *= 10
	}
	return float64(int(v*pow+0.5)) / pow
}
