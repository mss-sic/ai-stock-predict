package handler

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/ai-stock-predict/server/internal/collector"
	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/scheduler"
	"github.com/gin-gonic/gin"
	"github.com/ai-stock-predict/server/pkg/response"
)

type CollectorHandler struct {
	sched *scheduler.Scheduler
}

func NewCollectorHandler(sched *scheduler.Scheduler) *CollectorHandler {
	return &CollectorHandler{sched: sched}
}

func (h *CollectorHandler) Trigger(c *gin.Context) {
	var body struct {
		Phases []string `json:"phases"`
	}
	c.ShouldBindJSON(&body)
	h.sched.Trigger(body.Phases)

	c.JSON(http.StatusOK, gin.H{
		"message": "采集已触发",
		"data":    collector.GetProgress(),
	})
}

func (h *CollectorHandler) Status(c *gin.Context) {
	prog := collector.GetProgress()
	prog.LastRun = h.sched.Status()["lastRun"]
	response.Success(c, prog)
}

func (h *CollectorHandler) UpdateSchedule(c *gin.Context) {
	var body struct {
		CronExpr string `json:"cronExpr"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "schedule updated", "cronExpr": body.CronExpr})
}

func (h *CollectorHandler) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	collector.RegisterSSEWriter(c.Writer)
	defer collector.UnregisterSSEWriter(c.Writer)
	fmt.Fprintf(c.Writer, "data: {\"type\":\"connected\"}\n\n")
	c.Writer.Flush()
	ctx := c.Request.Context()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(c.Writer, ": heartbeat\n\n")
			c.Writer.Flush()
		}
	}
}

func (h *CollectorHandler) History(c *gin.Context) {
	var logs []model.CollectionLog
	db.MySQL.Order("started_at DESC").Limit(50).Find(&logs)
	response.Success(c, logs)
}

// ClearHistory deletes stuck or old collection logs (type=stuck|errors|all, default stuck)
func (h *CollectorHandler) ClearHistory(c *gin.Context) {
	clearType := c.DefaultQuery("type", "stuck")

	var runningDeleted, errorDeleted int64

	if clearType == "stuck" || clearType == "all" {
		cutoff := time.Now().Add(-30 * time.Minute)
		result := db.MySQL.Where("status = ? AND started_at < ?", "running", cutoff).Delete(&model.CollectionLog{})
		runningDeleted = result.RowsAffected
	}

	if clearType == "errors" || clearType == "all" {
		cutoff := time.Now().Add(-24 * time.Hour)
		result := db.MySQL.Where("status IN ? AND started_at < ?", []string{"error", "failed"}, cutoff).Delete(&model.CollectionLog{})
		errorDeleted = result.RowsAffected
	}

	totalDeleted := runningDeleted + errorDeleted
	msg := fmt.Sprintf("已清除 %d 条记录", totalDeleted)
	if runningDeleted > 0 && errorDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条卡住记录 + %d 条错误记录", runningDeleted, errorDeleted)
	} else if runningDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条卡住的采集记录", runningDeleted)
	} else if errorDeleted > 0 {
		msg = fmt.Sprintf("已清除 %d 条错误采集记录", errorDeleted)
	}

	response.Success(c, map[string]interface{}{
		"deleted":      totalDeleted,
		"runningDeleted": runningDeleted,
		"errorDeleted":  errorDeleted,
		"message":      msg,
	})
}

func (h *CollectorHandler) StockCollect(c *gin.Context) {
	code := c.Param("code")
	var body struct {
		Phases []string `json:"phases"`
	}
	c.ShouldBindJSON(&body)
	if len(body.Phases) == 0 {
		body.Phases = []string{"shareholder", "financial", "news"}
	}
	go func() {
		for _, phase := range body.Phases {
			collector.RunStockCollection(phase, code)
		}
	}()
	response.Success(c, gin.H{"message": "单股采集已触发", "stockCode": code, "phases": body.Phases})
}

// CollectReports triggers per-stock report collection via Python script


func (h *CollectorHandler) CollectReports(c *gin.Context) {
	code := c.Param("code")
	if code == "" {
		c.JSON(400, gin.H{"error": "missing stock code"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()
	emit := func(typ, msg, level string) {
		data, _ := json.Marshal(map[string]string{"type": typ, "message": msg, "level": level})
		select {
		case <-ctx.Done():
			return
		default:
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	}

	emit("log", "开始拉取 "+code+" 研报数据...", "info")

	cmd := exec.CommandContext(ctx, "python3", "-u", "scripts/collector/report_collect.py", "2024-01-01", "2026-12-31", code)
	cmd.Dir = "/Users/admin/Documents/ai-stock-predict"
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		emit("error", "启动采集失败: "+err.Error(), "error")
		return
	}

	if err := cmd.Start(); err != nil {
		emit("error", "启动采集进程失败: "+err.Error(), "error")
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			emit("log", line, "info")
		}
	}

	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			errMsg := stderr.String()
			if len(errMsg) > 500 {
				errMsg = errMsg[:500] + "..."
			}
			emit("error", "采集异常: "+errMsg, "error")
		}
	}

	var count int64
	db.PG.Raw("SELECT count(*) FROM stock_reports WHERE stock_code = ?", code).Scan(&count)
	if count > 0 {
		emit("complete", fmt.Sprintf("成功拉取 %d 篇研报", count), "success")
	} else {
		emit("complete", "该股票暂无机构研报（近2年无券商覆盖）", "warn")
	}
}
