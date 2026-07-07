package handler

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

// PreMarketHandler handles pre-market finalization and notification APIs.
type PreMarketHandler struct {
	preMarketSvc *service.PreMarketService
}

// NewPreMarketHandler creates a new pre-market handler.
func NewPreMarketHandler(aiSvc *service.AIService) *PreMarketHandler {
	return &PreMarketHandler{
		preMarketSvc: service.NewPreMarketService(aiSvc),
	}
}

// ── Pre-Market Finalization (Async) ──

// FinalizePreMarket starts an async pre-market decision pipeline and returns a task ID.
func (h *PreMarketHandler) FinalizePreMarket(c *gin.Context) {
	var body struct {
		TradeDate string `json:"tradeDate"`
		RunID     uint   `json:"runId"`
		SkipAI    bool   `json:"skipAi"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if body.TradeDate == "" {
		body.TradeDate = time.Now().Format("2006-01-02")
	}
	uid := getUID(c)

	// Check if there's already a running task for this date
	var existing model.PreMarketTask
	if err := db.MySQL.Where("user_id = ? AND trade_date = ? AND status IN ?",
		uid, body.TradeDate, []string{"pending", "running"}).First(&existing).Error; err == nil {
		response.Success(c, map[string]interface{}{
			"taskId": existing.ID,
			"status": existing.Status,
			"message": "已有进行中的任务",
		})
		return
	}

	// Create task
	task := model.PreMarketTask{
		UserID:    uid,
		RunID:     body.RunID,
		TradeDate: body.TradeDate,
		Status:    "pending",
		SkipAI:    body.SkipAI,
	}
	if err := db.MySQL.Create(&task).Error; err != nil {
		response.InternalError(c, "创建任务失败")
		return
	}

	// Start async execution (route based on AI toggle)
	if task.SkipAI {
		go h.preMarketSvc.RunAsyncNoAI(&task)
	} else {
		go h.preMarketSvc.RunAsync(&task)
	}

	response.Created(c, map[string]interface{}{
		"taskId": task.ID,
		"status": "pending",
		"message": "任务已创建，正在异步执行",
	})
}

// GetTaskStatus returns the current status of an async pre-market task.
func (h *PreMarketHandler) GetTaskStatus(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	var task model.PreMarketTask
	if err := db.MySQL.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Parse logs
	var logs []string
	if task.Logs != "" {
		json.Unmarshal([]byte(task.Logs), &logs)
	}

	// Parse result
	var result interface{}
	if task.ResultJSON != "" {
		json.Unmarshal([]byte(task.ResultJSON), &result)
	}

	progress := 0
	if task.TotalSignals > 0 {
		progress = task.CompletedSignals * 100 / task.TotalSignals
	}

	// Parse stage details
	var stageDetails interface{}
	if task.StageDetails != "" {
		json.Unmarshal([]byte(task.StageDetails), &stageDetails)
	}
	// Parse position patrol
	var positionPatrol interface{}
	if task.PositionPatrol != "" {
		json.Unmarshal([]byte(task.PositionPatrol), &positionPatrol)
	}

	response.Success(c, map[string]interface{}{
		"id":               task.ID,
		"runId":           task.RunID,
		"tradeDate":        task.TradeDate,
		"status":           task.Status,
		"totalSignals":     task.TotalSignals,
		"completedSignals": task.CompletedSignals,
		"currentStage":     task.CurrentStage,
		"currentCode":      task.CurrentCode,
		"progress":         progress,
		"logs":             logs,
		"stageDetails":     stageDetails,
		"positionPatrol":   positionPatrol,
		"skipAi":          task.SkipAI,
		"result":           result,
		"error":            task.Error,
	})
}

// GetLatestTask returns the most recent task for today.
func (h *PreMarketHandler) GetLatestTask(c *gin.Context) {
	uid := getUID(c)
	tradeDate := c.Query("tradeDate")
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}

	var task model.PreMarketTask
	if err := db.MySQL.Where("user_id = ? AND trade_date = ?", uid, tradeDate).
		Order("id DESC").First(&task).Error; err != nil {
		response.NotFound(c, "无任务记录")
		return
	}

	// Parse logs
	var logs []string
	if task.Logs != "" {
		json.Unmarshal([]byte(task.Logs), &logs)
	}

	// Parse result
	var result interface{}
	if task.ResultJSON != "" {
		json.Unmarshal([]byte(task.ResultJSON), &result)
	}

	progress := 0
	if task.TotalSignals > 0 {
		progress = task.CompletedSignals * 100 / task.TotalSignals
	}

	var stageDetails interface{}
	if task.StageDetails != "" {
		json.Unmarshal([]byte(task.StageDetails), &stageDetails)
	}
	var positionPatrol interface{}
	if task.PositionPatrol != "" {
		json.Unmarshal([]byte(task.PositionPatrol), &positionPatrol)
	}

	response.Success(c, map[string]interface{}{
		"id":               task.ID,
		"runId":           task.RunID,
		"tradeDate":        task.TradeDate,
		"status":           task.Status,
		"totalSignals":     task.TotalSignals,
		"completedSignals": task.CompletedSignals,
		"currentStage":     task.CurrentStage,
		"currentCode":      task.CurrentCode,
		"progress":         progress,
		"logs":             logs,
		"stageDetails":     stageDetails,
		"positionPatrol":   positionPatrol,
		"skipAi":          task.SkipAI,
		"result":           result,
		"error":            task.Error,
	})
}

// GetPreMarketDecisions returns pre-market decisions for a date.
func (h *PreMarketHandler) GetPreMarketDecisions(c *gin.Context) {
	uid := getUID(c)
	tradeDate := c.Query("tradeDate")

	var decisions []model.PreMarketDecision
	q := db.MySQL.Where("user_id = ?", uid)
	if tradeDate != "" {
		q = q.Where("trade_date = ?", tradeDate)
	}
	q.Order("created_at DESC").Limit(100).Find(&decisions)

	type DecisionWithSignal struct {
		model.PreMarketDecision
		SignalStatus string `json:"signalStatus"`
	}
	result := make([]DecisionWithSignal, 0, len(decisions))
	for _, d := range decisions {
		item := DecisionWithSignal{PreMarketDecision: d}
		var sig model.BacktestSignal
		if err := db.MySQL.Where("id = ?", d.SignalID).Select("status").First(&sig).Error; err == nil {
			item.SignalStatus = sig.Status
		}
		result = append(result, item)
	}

	response.Success(c, result)
}

// ── Notification Configuration ──

func (h *PreMarketHandler) ListNotificationConfigs(c *gin.Context) {
	uid := getUID(c)
	var configs []model.NotificationConfig
	db.MySQL.Where("user_id = ?", uid).Order("created_at ASC").Find(&configs)
	response.Success(c, configs)
}

func (h *PreMarketHandler) CreateNotificationConfig(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Channel string                 `json:"channel"`
		Name    string                 `json:"name"`
		Config  map[string]interface{} `json:"config"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Channel == "" {
		response.BadRequest(c, "参数错误: 请提供 channel, name, config")
		return
	}

	cfg := model.NotificationConfig{
		UserID: uid, Channel: body.Channel, Name: body.Name,
		Config: model.JSONMap(body.Config), Enabled: true,
	}
	if err := db.MySQL.Create(&cfg).Error; err != nil {
		response.InternalError(c, "创建通知配置失败")
		return
	}
	response.Created(c, cfg)
}

func (h *PreMarketHandler) UpdateNotificationConfig(c *gin.Context) {
	uid := getUID(c)
	cid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Name    *string                 `json:"name"`
		Config  *map[string]interface{} `json:"config"`
		Enabled *bool                   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if body.Name != nil { updates["name"] = *body.Name }
	if body.Config != nil { updates["config"] = model.JSONMap(*body.Config) }
	if body.Enabled != nil { updates["enabled"] = *body.Enabled }

	if err := db.MySQL.Model(&model.NotificationConfig{}).Where("id = ? AND user_id = ?", cid, uid).
		Updates(updates).Error; err != nil {
		response.InternalError(c, "更新通知配置失败")
		return
	}
	response.Success(c, map[string]string{"status": "ok"})
}

func (h *PreMarketHandler) DeleteNotificationConfig(c *gin.Context) {
	uid := getUID(c)
	cid, _ := strconv.Atoi(c.Param("id"))
	db.MySQL.Where("id = ? AND user_id = ?", cid, uid).Delete(&model.NotificationConfig{})
	response.Success(c, map[string]string{"status": "deleted"})
}

