package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	schedv2 "github.com/ai-stock-predict/server/internal/scheduler/v2"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

// LiveTradingHandler provides HTTP handlers for live trading operations.
type LiveTradingHandler struct {
	liveSvc   *service.LiveTradingService
	brokerSvc *service.BrokerService
}

// NewLiveTradingHandler creates a new live trading handler.
func NewLiveTradingHandler() *LiveTradingHandler {
	return &LiveTradingHandler{
		liveSvc:   service.NewLiveTradingService(),
		brokerSvc: service.NewBrokerService(),
	}
}

// ── Account Management (Multi-Account) ──

// ListAccounts returns all trading accounts for the user.
func (h *LiveTradingHandler) ListAccounts(c *gin.Context) {
	uid := getUID(c)
	var accounts []model.TradingAccount
	db.MySQL.Where("user_id = ? AND status = ?", uid, "active").Order("created_at ASC").Find(&accounts)
	response.Success(c, accounts)
}

// CreateAccount creates a new trading account.
func (h *LiveTradingHandler) CreateAccount(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Name          string  `json:"name"`
		Broker        string  `json:"broker"`
		AccountType   string  `json:"accountType"`
		AccountNumber string  `json:"accountNumber"`
		InitialCapital float64 `json:"initialCapital"`
		MxAPIKey      string  `json:"mxApiKey"`
		MxAccountID   string  `json:"mxAccountId"`
		BrokerMode    string  `json:"brokerMode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if body.Name == "" {
		body.Name = "交易账户"
	}
	if body.AccountType == "" {
		body.AccountType = "simulated"
	}
	if body.InitialCapital <= 0 {
		body.InitialCapital = 100000
	}

	account := model.TradingAccount{
		UserID: uid, Name: body.Name, Broker: body.Broker,
		AccountType: body.AccountType, AccountNumber: body.AccountNumber,
		InitialCapital: body.InitialCapital, AvailableCash: body.InitialCapital,
		TotalDeposit: body.InitialCapital,
		MxAPIKey: body.MxAPIKey, MxAccountID: body.MxAccountID, BrokerMode: body.BrokerMode,
	}
	if err := db.MySQL.Create(&account).Error; err != nil {
		response.InternalError(c, "创建账户失败")
		return
	}
	response.Created(c, account)
}

// UpdateAccount updates an existing trading account.
func (h *LiveTradingHandler) UpdateAccount(c *gin.Context) {
	uid := getUID(c)
	aid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Name          *string  `json:"name"`
		Broker        *string  `json:"broker"`
		AccountType   *string  `json:"accountType"`
		AccountNumber *string  `json:"accountNumber"`
		InitialCapital *float64 `json:"initialCapital"`
		AvailableCash  *float64 `json:"availableCash"`
		Status        *string  `json:"status"`
		MxAPIKey      *string  `json:"mxApiKey"`
		MxAccountID   *string  `json:"mxAccountId"`
		BrokerMode    *string  `json:"brokerMode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if body.Name != nil { updates["name"] = *body.Name }
	if body.Broker != nil { updates["broker"] = *body.Broker }
	if body.AccountType != nil { updates["account_type"] = *body.AccountType }
	if body.AccountNumber != nil { updates["account_number"] = *body.AccountNumber }
	if body.InitialCapital != nil { updates["initial_capital"] = *body.InitialCapital }
	if body.AvailableCash != nil { updates["available_cash"] = *body.AvailableCash }
	if body.Status != nil { updates["status"] = *body.Status }
	if body.MxAPIKey != nil { updates["mx_api_key"] = *body.MxAPIKey }
	if body.MxAccountID != nil { updates["mx_account_id"] = *body.MxAccountID }
	if body.BrokerMode != nil { updates["broker_mode"] = *body.BrokerMode }

	if err := db.MySQL.Model(&model.TradingAccount{}).Where("id = ? AND user_id = ?", aid, uid).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新账户失败")
		return
	}
	response.Success(c, map[string]string{"status": "ok"})
}

// DeleteAccount soft-deletes (archives) an account.
func (h *LiveTradingHandler) DeleteAccount(c *gin.Context) {
	uid := getUID(c)
	aid, _ := strconv.Atoi(c.Param("id"))
	if err := db.MySQL.Model(&model.TradingAccount{}).Where("id = ? AND user_id = ?", aid, uid).
		Update("status", "archived").Error; err != nil {
		response.InternalError(c, "删除账户失败")
		return
	}
	response.Success(c, map[string]string{"status": "archived"})
}

// ── Strategy Run Management ──

// CreateRun creates a new live strategy run.
func (h *LiveTradingHandler) CreateRun(c *gin.Context) {
	uid := getUID(c)
	type NotifyConfigInput struct {
		Channel    string `json:"channel"`    // dingtalk_bot / feishu_bot / wecom_bot
		Name       string `json:"name"`       // display name
		WebhookURL string `json:"webhookUrl"` // webhook URL
	}
	var body struct {
		StrategyID        uint                 `json:"strategyId"`
		AccountID         uint                 `json:"accountId"`
		Name              string               `json:"name"`
		InitialCapital    float64              `json:"initialCapital"`
		PctOfAccount      float64              `json:"pctOfAccount"`
		StockPool         string               `json:"stockPool"`
		StartDate         string               `json:"startDate"`
		AutoDailyCron     string               `json:"autoDailyCron"`
		AutoTradeExecCron string               `json:"autoTradeExecCron"`
		NotifyEnabled     bool                 `json:"notifyEnabled"`
		NotifyConfigs     []NotifyConfigInput  `json:"notifyConfigs"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if body.Name == "" {
		body.Name = "实盘运行"
	}
	if body.InitialCapital <= 0 {
		body.InitialCapital = 100000
	}

	// Validate or create trading account
	var account model.TradingAccount
	if body.AccountID > 0 {
		if err := db.MySQL.Where("id = ? AND user_id = ?", body.AccountID, uid).First(&account).Error; err != nil {
			response.BadRequest(c, "交易账户不存在")
			return
		}
		// Prevent same account being used by multiple active strategy runs
		var conflictRun model.StrategyRun
		if err := db.MySQL.Where("account_id = ? AND status IN ?", body.AccountID, []string{"active", "paused"}).First(&conflictRun).Error; err == nil {
			response.BadRequest(c, fmt.Sprintf("该资金账户已被实盘「%s」使用，请先停用或归档后再创建新实盘", conflictRun.Name))
			return
		}
	} else {
		// Auto-create default account
		db.MySQL.Where("user_id = ? AND status = ?", uid, "active").First(&account)
		if account.ID == 0 {
			account = model.TradingAccount{
				UserID: uid, Name: "默认账户", AccountType: "simulated",
				InitialCapital: body.InitialCapital, AvailableCash: body.InitialCapital,
				TotalDeposit: body.InitialCapital,
			}
			db.MySQL.Create(&account)
		}
	}

	// Create strategy run
	run := model.StrategyRun{
		UserID:            uid,
		StrategyID:        body.StrategyID,
		AccountID:         account.ID,
		Name:              body.Name,
		Status:            "active",
		StockPool:         body.StockPool,
		StartDate:         body.StartDate,
		InitialCapital:    body.InitialCapital,
		CurrentEquity:     body.InitialCapital,
		AutoDailyCron:     body.AutoDailyCron,
		AutoTradeExecCron: body.AutoTradeExecCron,
	}
	if err := db.MySQL.Create(&run).Error; err != nil {
		response.InternalError(c, "创建运行实例失败")
		return
	}

	// Create fund allocation
	if body.PctOfAccount <= 0 {
		body.PctOfAccount = 100
	}
	alloc := model.StrategyFundAllocation{
		UserID: uid, AccountID: account.ID, StrategyRunID: run.ID,
		AllocatedCapital: body.InitialCapital, CurrentCash: body.InitialCapital,
		PctOfAccount: body.PctOfAccount, Status: "active",
	}
	db.MySQL.Create(&alloc)

	// Create notification configs and associate with run
	var notifyChannelIDs []uint
	if body.NotifyEnabled && len(body.NotifyConfigs) > 0 {
		for _, nc := range body.NotifyConfigs {
			if nc.Channel == "" || nc.WebhookURL == "" {
				continue
			}
			if nc.Name == "" {
				nc.Name = nc.Channel
			}
			cfg := model.NotificationConfig{
				UserID:  uid,
				Channel: nc.Channel,
				Name:    nc.Name,
				Config:  model.JSONMap{"webhook_url": nc.WebhookURL},
				Enabled: true,
			}
			if err := db.MySQL.Create(&cfg).Error; err != nil {
				log.Printf("[live] create notification config failed: %v", err)
				continue
			}
			notifyChannelIDs = append(notifyChannelIDs, cfg.ID)
		}
		// Update run with notification settings
		channelsJSON, _ := json.Marshal(notifyChannelIDs)
		db.MySQL.Model(&run).Updates(map[string]interface{}{
			"notify_enabled":  true,
			"notify_channels": string(channelsJSON),
		})
	}

	// Import existing positions from broker if account already has holdings
	if account.Broker != "" && account.Broker != "manual" {
		go func() {
			svc := service.NewLiveTradingService()
			if err := svc.ImportBrokerPositionsToRun(run.ID, account.ID, uid); err != nil {
				log.Printf("[live] CreateRun: import positions for run %d failed: %v", run.ID, err)
			} else {
				log.Printf("[live] CreateRun: imported broker positions for run %d from account %d", run.ID, account.ID)
			}
		}()
	}

	// Register v2 scheduler tasks for this strategy run
	if sched := schedv2.GetGlobal(); sched != nil {
		sched.RegisterStrategyRunTasks(run.ID, uid)
	}

	response.Created(c, map[string]interface{}{
		"runId":        run.ID,
		"allocationId": alloc.ID,
		"accountId":    account.ID,
	})
}

// ListRuns returns all strategy runs for the user.
func (h *LiveTradingHandler) ListRuns(c *gin.Context) {
	uid := getUID(c)
	var runs []model.StrategyRun
	query := db.MySQL.Where("user_id = ?", uid)
	if sid := c.Query("strategy_id"); sid != "" {
		query = query.Where("strategy_id = ?", sid)
	}
	query.Order("created_at DESC").Find(&runs)
	response.Success(c, runs)
}

// GetRun returns a single strategy run with positions and allocation.
func (h *LiveTradingHandler) GetRun(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND user_id = ?", rid, uid).First(&run).Error; err != nil {
		response.NotFound(c, "运行实例不存在")
		return
	}

	// Linked trading account
	var linkedAccount model.TradingAccount
	if run.AccountID > 0 {
		db.MySQL.Where("id = ?", run.AccountID).First(&linkedAccount)
	}
	if linkedAccount.ID == 0 {
		db.MySQL.Where("user_id = ? AND status = ?", uid, "active").
			Order("id ASC").First(&linkedAccount)
	}

	var alloc model.StrategyFundAllocation
	db.MySQL.Where("strategy_run_id = ? AND status = ?", rid, "active").First(&alloc)

	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ? AND quantity > 0", rid).Find(&positions)

	var trades []model.LiveTrade
	db.MySQL.Where("strategy_run_id = ?", rid).Order("trade_date DESC").Limit(50).Find(&trades)

	// Strategy config
	var strategy model.Strategy
	db.MySQL.First(&strategy, run.StrategyID)

	// Strategy conditions
	var conditions []model.StrategyCondition
	db.MySQL.Where("strategy_id = ? AND enabled = true", run.StrategyID).Find(&conditions)

	// All signals for this run (pending + historical executed/skipped/rejected)
	var signals []model.BacktestSignal
	db.MySQL.Where("strategy_id = ? AND user_id = ? AND run_id = ?", run.StrategyID, uid, run.ID).
		Order("exec_date DESC, id DESC").Limit(200).Find(&signals)

	// All pre-market decisions for this run
	var decisions []model.PreMarketDecision
	db.MySQL.Where("user_id = ? AND run_id = ?", uid, rid).
		Order("trade_date DESC, id DESC").Limit(100).Find(&decisions)

	// Parse persisted logs
	var persistedLogs []string
	if run.LastRunLog != "" {
		json.Unmarshal([]byte(run.LastRunLog), &persistedLogs)
	}

	response.Success(c, map[string]interface{}{
		"run":           run,
		"strategy":      strategy,
		"account":       linkedAccount,
		"allocation":    alloc,
		"positions":     positions,
		"trades":        trades,
		"signals":       signals,
		"conditions":    conditions,
		"decisions":     decisions,
		"persistedLogs": persistedLogs,
	})
}

// UpdateRunConfig updates scheduling and notification settings for a run.
func (h *LiveTradingHandler) UpdateRunConfig(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		AutoDailyCron     *string `json:"autoDailyCron"`
		AutoTradeExecCron *string `json:"autoTradeExecCron"`
		AiReviewEnabled   *bool   `json:"aiReviewEnabled"`
		ExecutionMode     *string `json:"executionMode"`
		NotifyEnabled     *bool   `json:"notifyEnabled"`
		NotifyChannels    *string `json:"notifyChannels"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if body.AutoDailyCron != nil     { updates["auto_daily_cron"] = *body.AutoDailyCron }
	if body.AutoTradeExecCron != nil { updates["auto_trade_exec_cron"] = *body.AutoTradeExecCron }
	if body.AiReviewEnabled != nil   { updates["ai_review_enabled"] = *body.AiReviewEnabled }
	if body.ExecutionMode != nil     { updates["execution_mode"] = *body.ExecutionMode }
	if body.NotifyEnabled != nil     { updates["notify_enabled"] = *body.NotifyEnabled }
	if body.NotifyChannels != nil    { updates["notify_channels"] = *body.NotifyChannels }

	// Validate execution mode: if switching to auto/mx, verify at least one account supports it
	if body.ExecutionMode != nil && *body.ExecutionMode != "manual" {
		var count int64
		db.MySQL.Model(&model.TradingAccount{}).
			Where("user_id = ? AND status = ? AND mx_api_key != ''", uid, "active").
			Count(&count)
		if count == 0 {
			response.Error(c, 400, 400, "切换为自动交易需要先在账户设置中配置妙想API Key")
			return
		}
	}

	if len(updates) == 0 {
		response.BadRequest(c, "无更新内容")
		return
	}

	if err := db.MySQL.Model(&model.StrategyRun{}).Where("id = ? AND user_id = ?", rid, uid).
		Updates(updates).Error; err != nil {
		response.InternalError(c, "更新配置失败")
		return
	}

	// Sync cron changes to v2 scheduler TaskInstances
	if body.AutoTradeExecCron != nil || body.AutoDailyCron != nil {
		sched := schedv2.GetGlobal()
		if sched != nil {
			daily := ""
			preMkt := ""
			if body.AutoDailyCron != nil { daily = *body.AutoDailyCron }
			if body.AutoTradeExecCron != nil { preMkt = *body.AutoTradeExecCron }
			sched.SyncRunCron(uint(rid), daily, preMkt)
		}
	}

	response.Success(c, map[string]string{"status": "ok"})
}

// ExecuteTrade manually triggers trade execution for a run.
func (h *LiveTradingHandler) ExecuteTrade(c *gin.Context) {
	uid := getUID(c)
	rid, err := strconv.Atoi(c.Param("id"))
	if err != nil || rid <= 0 {
		response.BadRequest(c, "无效的实盘运行ID")
		return
	}

	var body struct {
		TradeDate string `json:"tradeDate"`
		SkipAI    *bool  `json:"skipAi"`
		Force     bool   `json:"force"`
	}
	_ = c.ShouldBindJSON(&body)

	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND user_id = ?", rid, uid).First(&run).Error; err != nil {
		response.Error(c, 404, 404, "实盘运行不存在")
		return
	}

	tradeDate := time.Now().Format("2006-01-02")
	if body.TradeDate != "" {
		tradeDate = body.TradeDate
	} else if dateQ := c.Query("date"); dateQ != "" {
		tradeDate = dateQ
	}

	// Honor skipAi — temporarily override run's AiReviewEnabled
	if body.SkipAI != nil && *body.SkipAI {
		run.AiReviewEnabled = false
	}

	svc := service.NewTradeExecService(service.NewAIService())
	result, err := svc.ExecuteForRun(tradeDate, uint(rid), body.Force)
	if err != nil {
		response.InternalError(c, "交易执行失败: "+err.Error())
		return
	}

	response.Success(c, result)
}

// ExecuteSingleSignal manually executes a single signal (user override).
func (h *LiveTradingHandler) ExecuteSingleSignal(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("signalId"))
	rid, _ := strconv.Atoi(c.Param("id"))

	var body struct {
		Price    *float64 `json:"price"`
		Quantity *int     `json:"quantity"`
		Action   *string  `json:"action"` // "execute" / "skip" / "manual"
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND run_id = ? AND user_id = ?", sid, rid, uid).First(&sig).Error; err != nil {
		response.Error(c, 404, 404, "信号不存在")
		return
	}

	if body.Action != nil && *body.Action == "skip" {
		sig.Status = "skipped"
		sig.SkipReason = "用户手动跳过"
		db.MySQL.Save(&sig)
		response.Success(c, map[string]string{"status": "skipped"})
		return
	}

	if body.Action != nil && *body.Action == "manual" {
		sig.Status = "pending_manual"
		sig.SkipReason = "用户标记为手动下单"
		if body.Price != nil { sig.OrderPrice = *body.Price }
		if body.Quantity != nil { sig.SuggestedQty = *body.Quantity }
		db.MySQL.Save(&sig)
		response.Success(c, map[string]string{"status": "pending_manual"})
		return
	}

	// Execute: find account and dispatch
	var account model.TradingAccount
	db.MySQL.Where("user_id = ? AND status = ?", uid, "active").Order("id ASC").First(&account)

	_ = service.NewTradeExecService(service.NewAIService())
	// Direct execution (manual dispatch regardless of broker mode)
	sig.Status = "pending_manual"
	sig.SkipReason = "用户手动触发，请在前端确认下单"
	if body.Price != nil { sig.OrderPrice = *body.Price }
	if body.Quantity != nil { sig.SuggestedQty = *body.Quantity }
	db.MySQL.Save(&sig)
	_ = account // may use for context later

	response.Success(c, map[string]interface{}{
		"status": "pending_manual",
		"signal": sig,
	})
}

// UpdateRunStatus pauses/resumes/stops a strategy run.
func (h *LiveTradingHandler) UpdateRunStatus(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	if err := db.MySQL.Model(&model.StrategyRun{}).Where("id = ? AND user_id = ?", rid, uid).
		Update("status", body.Status).Error; err != nil {
		response.InternalError(c, "更新状态失败")
		return
	}

	// Sync scheduler tasks
	if sched := schedv2.GetGlobal(); sched != nil {
		switch body.Status {
		case "active":
			sched.RegisterStrategyRunTasks(uint(rid), uid)
		case "paused", "stopped":
			sched.DisableStrategyRunTasks(uint(rid))
		}
	}

	response.Success(c, map[string]string{"status": body.Status})
}

// ── Daily Execution ──

// RunDaily creates an async task for the daily signal-generation pipeline.
// mode: "after_close" (default, generates T+1 signals), "pre_market" or "intraday" (refresh pending only)
func (h *LiveTradingHandler) RunDaily(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		TradeDate string `json:"tradeDate"`
		Mode      string `json:"mode"`   // "after_close", "pre_market", "intraday"
		RunID     uint   `json:"runId"`  // required: scope to a specific run
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.RunID == 0 {
		response.BadRequest(c, "runId is required")
		return
	}
	if body.Mode == "" {
		body.Mode = "after_close"
	}
	tradeDate := body.TradeDate
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}

	// Check for existing running task (scoped to same runId if provided)
	var existing model.DailyRunTask
	checkQuery := db.MySQL.Where("trade_date = ? AND status IN ?",
		tradeDate, []string{"pending", "running"})
	checkQuery = checkQuery.Where("run_id = ?", body.RunID)
	if err := checkQuery.First(&existing).Error; err == nil {
		response.Success(c, map[string]interface{}{
			"taskId":  existing.ID,
			"status":  existing.Status,
			"message": "已有进行中的任务",
		})
		return
	}

	// Create async task
	task := model.DailyRunTask{
		UserID:    uid,
		TradeDate: tradeDate,
		Mode:      body.Mode,
		Status:    "pending",
		RunID:     body.RunID,
	}
	if err := db.MySQL.Create(&task).Error; err != nil {
		response.InternalError(c, "创建任务失败")
		return
	}

	// Start async execution
	go h.liveSvc.RunDailyWithTask(&task)

	response.Created(c, map[string]interface{}{
		"taskId":  task.ID,
		"status":  "pending",
		"message": "任务已创建，正在异步执行",
	})
}

// GetDailyRunTask returns the status of an async daily-run task.
func (h *LiveTradingHandler) GetDailyRunTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "无效的任务ID")
		return
	}

	var task model.DailyRunTask
	if err := db.MySQL.First(&task, taskID).Error; err != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	var logs []string
	if task.Logs != "" {
		json.Unmarshal([]byte(task.Logs), &logs)
	}

	progress := 0
	if task.TotalStocks > 0 {
		progress = task.ScannedStocks * 100 / task.TotalStocks
	}

	response.Success(c, map[string]interface{}{
		"id":             task.ID,
		"tradeDate":      task.TradeDate,
		"runId":          task.RunID,
		"mode":           task.Mode,
		"status":         task.Status,
		"totalStocks":    task.TotalStocks,
		"scannedStocks":  task.ScannedStocks,
		"candidateCount": task.CandidateCount,
		"signalCount":    task.SignalCount,
		"progress":       progress,
		"logs":           logs,
		"error":          task.Error,
	})
}

// GetLatestDailyRunTask returns the most recent daily-run task for today.
func (h *LiveTradingHandler) GetLatestDailyRunTask(c *gin.Context) {
	tradeDate := c.Query("tradeDate")
	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}

	var task model.DailyRunTask
	query := db.MySQL.Where("trade_date = ?", tradeDate)
	if runID := c.Query("runId"); runID != "" {
		query = query.Where("run_id = ?", runID)
	}
	if err := query.Order("id DESC").First(&task).Error; err != nil {
		response.Success(c, map[string]interface{}{
			"tradeDate": tradeDate,
			"status":    "none",
		})
		return
	}

	var logs []string
	if task.Logs != "" {
		json.Unmarshal([]byte(task.Logs), &logs)
	}

	progress := 0
	if task.TotalStocks > 0 {
		progress = task.ScannedStocks * 100 / task.TotalStocks
	}

	response.Success(c, map[string]interface{}{
		"id":             task.ID,
		"tradeDate":      task.TradeDate,
		"runId":          task.RunID,
		"mode":           task.Mode,
		"status":         task.Status,
		"totalStocks":    task.TotalStocks,
		"scannedStocks":  task.ScannedStocks,
		"candidateCount": task.CandidateCount,
		"signalCount":    task.SignalCount,
		"progress":       progress,
		"logs":           logs,
		"error":          task.Error,
	})
}

// ── Signal Execution ──

// UpdateSignal allows manual editing of a pending signal.
func (h *LiveTradingHandler) UpdateSignal(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&sig).Error; err != nil {
		response.NotFound(c, "信号不存在")
		return
	}
	if sig.Status != "pending" && sig.Status != "confirmed" {
		response.BadRequest(c, "只能修改待执行/已确认的信号")
		return
	}

	var body struct {
		PlannedPrice  *float64 `json:"plannedPrice"`
		PlannedQty    *int     `json:"plannedQty"`
		Reason        *string  `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if body.PlannedPrice != nil { updates["planned_price"] = *body.PlannedPrice; sig.PlannedPrice = *body.PlannedPrice }
	if body.PlannedQty != nil { updates["planned_qty"] = *body.PlannedQty; sig.PlannedQty = *body.PlannedQty }
	if body.Reason != nil { updates["reason"] = *body.Reason }

	if len(updates) == 0 {
		response.BadRequest(c, "无更新内容")
		return
	}

	// Recalculate planned amount
	if body.PlannedPrice != nil || body.PlannedQty != nil {
		updates["planned_amount"] = sig.PlannedPrice * float64(sig.PlannedQty)
	}

	if err := db.MySQL.Model(&sig).Updates(updates).Error; err != nil {
		response.InternalError(c, "更新信号失败")
		return
	}
	response.Success(c, sig)
}

// DeleteSignal removes a pending signal.
func (h *LiveTradingHandler) DeleteSignal(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&sig).Error; err != nil {
		response.NotFound(c, "信号不存在")
		return
	}
	if sig.Status != "pending" {
		response.BadRequest(c, "只能删除待执行的信号")
		return
	}

	if err := db.MySQL.Delete(&sig).Error; err != nil {
		response.InternalError(c, "删除信号失败")
		return
	}
	response.Success(c, map[string]string{"status": "deleted"})
}

// ExecuteSignal executes or abandons a signal with actual trade details.
// ExecuteSignal executes or abandons a signal with actual trade details.
func (h *LiveTradingHandler) ExecuteSignal(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))

	var body struct {
		Action      string  `json:"action"`      // "execute" or "abandon"
		ActualPrice float64 `json:"actualPrice"` // actual execution price
		ActualQty   int     `json:"actualQty"`   // actual execution quantity
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		// Legacy: allow empty body for backward compat
		body.Action = "execute"
	}

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&sig).Error; err != nil {
		response.NotFound(c, "信号不存在")
		return
	}
	if sig.Status != "confirmed" && sig.Status != "pending" {
		response.BadRequest(c, "信号状态不允许执行: "+sig.Status)
		return
	}

	if body.Action == "abandon" {
		sig.Status = "skipped"
		sig.SkipReason = "用户主动放弃"
		db.MySQL.Save(&sig)
		response.Success(c, map[string]string{"status": "abandoned"})
		return
	}

	if err := h.liveSvc.ExecuteSignalByIDWithPrice(uint(sid), uid, body.ActualPrice, body.ActualQty); err != nil {
		log.Printf("[live] ExecuteSignal %d failed: %v", sid, err)
		response.InternalError(c, "信号执行失败: "+err.Error())
		return
	}

	// If the account has mx_moni broker mode, also place order via broker
	go func() {
		var run model.StrategyRun
		if err := db.MySQL.Where("strategy_id = ? AND user_id = ? AND status = ?",
			sig.StrategyID, uid, "active").First(&run).Error; err != nil {
			return
		}
		var alloc model.StrategyFundAllocation
		if err := db.MySQL.Where("strategy_run_id = ? AND status = ?",
			run.ID, "active").First(&alloc).Error; err != nil {
			return
		}
		var account model.TradingAccount
		if err := db.MySQL.Where("id = ? AND user_id = ?",
			alloc.AccountID, uid).First(&account).Error; err != nil {
			return
		}
		if account.BrokerMode != "mx_moni" {
			return
		}

		orderType := sig.ActionType
		useMarketPrice := false
		price := sig.PlannedPrice
		if body.ActualPrice > 0 {
			price = body.ActualPrice
		}
		if price <= 0 {
			useMarketPrice = true
		}
		qty := sig.PlannedQty
		if body.ActualQty > 0 {
			qty = body.ActualQty
		}

		req := &service.BrokerOrderRequest{
			StockCode:      sig.StockCode,
			OrderType:      orderType,
			Price:          price,
			Quantity:       qty,
			UseMarketPrice: useMarketPrice,
		}
		if _, err := h.brokerSvc.PlaceBrokerOrder(account.ID, uid, req); err != nil {
			log.Printf("[live] Broker order failed for signal %d: %v", sid, err)
		}
	}()

	response.Success(c, map[string]string{"status": "executed"})
}

// ── Position & Trade Queries ──

func (h *LiveTradingHandler) GetPositions(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ? AND user_id = ? AND quantity > 0", rid, uid).Find(&positions)
	response.Success(c, positions)
}

func (h *LiveTradingHandler) GetTrades(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	var trades []model.LiveTrade
	db.MySQL.Where("strategy_run_id = ? AND user_id = ?", rid, uid).
		Order("trade_date DESC").Limit(200).Find(&trades)
	response.Success(c, trades)
}

func (h *LiveTradingHandler) GetDailySnapshots(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	// Return only the latest snapshot per day to avoid duplicate dates
	var snapshots []model.DailyPortfolioSnapshot
	db.MySQL.Raw(`
		SELECT * FROM daily_portfolio_snapshots t1
		WHERE strategy_run_id = ? AND user_id = ?
		AND id = (
			SELECT t2.id FROM daily_portfolio_snapshots t2
			WHERE t2.strategy_run_id = t1.strategy_run_id
			AND t2.snapshot_date = t1.snapshot_date
			ORDER BY t2.created_at DESC LIMIT 1
		)
		ORDER BY t1.snapshot_date ASC
		LIMIT 90
	`, rid, uid).Scan(&snapshots)
	response.Success(c, snapshots)
}

// ── Account Summary (backward-compat) ──

// GetAccount returns all active accounts for the user.
func (h *LiveTradingHandler) GetAccount(c *gin.Context) {
	uid := getUID(c)
	var accounts []model.TradingAccount
	db.MySQL.Where("user_id = ? AND status = ?", uid, "active").Find(&accounts)
	// Auto-create if none
	if len(accounts) == 0 {
		acct := model.TradingAccount{
			UserID: uid, Name: "默认账户", AccountType: "simulated",
			InitialCapital: 100000, AvailableCash: 100000, TotalDeposit: 100000,
		}
		db.MySQL.Create(&acct)
		accounts = append(accounts, acct)
	}
	response.Success(c, accounts)
}

// ── Route Registration ──

// SendNotification sends the decision report for a run via configured notification channels.
func (h *LiveTradingHandler) SendNotification(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	// Load run
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND user_id = ?", rid, uid).First(&run).Error; err != nil {
		response.NotFound(c, "运行实例不存在")
		return
	}

	log.Printf("[live] SendNotification run=%d strategy=%d user=%d", rid, run.StrategyID, uid)

	// Read requested trade date from body (default to latest)
	var reqBody struct { TradeDate string `json:"tradeDate"` }
	c.ShouldBindJSON(&reqBody)
	reqDate := reqBody.TradeDate
	log.Printf("[live] SendNotification reqDate=%q", reqDate)

	// Try to load completed pre-market task for the requested date
	displayDate := reqDate
	if displayDate == "" {
		displayDate = time.Now().Format("2006-01-02")
	}
	title := fmt.Sprintf("%s · %s 决策报告", run.Name, displayDate)

	var result map[string]interface{}
	reportMarkdown := ""

	var task model.PreMarketTask
	query := db.MySQL.Where("run_id = ? AND status = ?", run.ID, "completed")
	if reqDate != "" {
		query = query.Where("trade_date = ?", reqDate)
	}
	if err := query.Order("id DESC").First(&task).Error; err == nil && task.ResultJSON != "" {
		// Use stored PreMarketTask data
		json.Unmarshal([]byte(task.ResultJSON), &result)
		displayDate = task.TradeDate
		if displayDate == "" {
			displayDate = time.Now().Format("2006-01-02")
		}
		title = fmt.Sprintf("%s · %s 决策报告", run.Name, displayDate)
		if rpt, ok := result["notifyMarkdown"].(string); ok {
			reportMarkdown = rpt
		} else if rpt, ok := result["report"].(string); ok {
			reportMarkdown = rpt
		}
		log.Printf("[live] SendNotification: using stored PreMarketTask id=%d tradeDate=%s", task.ID, task.TradeDate)
	} else {
		// Fallback: build report from live signals + decisions for the requested date
		log.Printf("[live] SendNotification: no PreMarketTask for %s, building from live data", reqDate)
		result = h.buildReportFromLiveData(run, reqDate)
		if rpt, ok := result["notifyMarkdown"].(string); ok {
			reportMarkdown = rpt
		}
		displayDate = reqDate
		if displayDate == "" {
			displayDate = time.Now().Format("2006-01-02")
		}
		title = fmt.Sprintf("%s · %s 决策报告", run.Name, displayDate)
		result["tradeDate"] = displayDate
	}

	// Build Feishu card + text fallback from task data
	cardJSON, textBody := buildCardFromReport(run.Name, displayDate, reportMarkdown, result, run.ExecutionMode)
	envelope, _ := json.Marshal(map[string]string{
		"card": cardJSON,
		"text": textBody,
	})
	body := string(envelope)

	// Send via notification service
	notifSvc := service.NewNotificationService()
	if err := notifSvc.SendToUser(uid, title, body); err != nil {
		log.Printf("[live] notify run %d failed: %v", run.ID, err)
		response.InternalError(c, "发送通知失败: "+err.Error())
		return
	}

	response.Success(c, map[string]interface{}{
		"status":  "sent",
		"channel": "all",
		"message": "通知已发送至所有已配置渠道",
	})
}

// ── Broker Integration ──

// SyncFromBroker syncs positions/holdings from the account's configured broker.
func (h *LiveTradingHandler) SyncFromBroker(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	portfolio, err := h.brokerSvc.SyncPositionsFromBroker(uint(id), uid)
	if err != nil {
		log.Printf("[live] SyncFromBroker %d failed: %v", id, err)
		response.InternalError(c, "同步失败: "+err.Error())
		return
	}

	// Detect drift between live_positions and holdings for active runs
	go func() {
		var activeRuns []model.StrategyRun
		db.MySQL.Where("account_id = ? AND status IN ?", id, []string{"active", "paused"}).Find(&activeRuns)
		for _, run := range activeRuns {
			var lpCount, hCount int64
			db.MySQL.Model(&model.LivePosition{}).Where("strategy_run_id = ? AND quantity > 0", run.ID).Count(&lpCount)
			db.MySQL.Model(&model.Holding{}).Where("account_id = ? AND quantity > 0", id).Count(&hCount)
			if lpCount != hCount {
				log.Printf("[reconcile] run=%d account=%d: live_positions=%d holdings=%d (drift detected — possible manual trades or pending settlement)", run.ID, id, lpCount, hCount)
			} else {
				// Same count, check per-stock quantity match
				rows, _ := db.MySQL.Raw(`
					SELECT lp.stock_code, lp.quantity AS lq, COALESCE(h.quantity, 0) AS hq
					FROM live_positions lp
					LEFT JOIN holdings h ON h.account_id = ? AND h.stock_code = lp.stock_code
					WHERE lp.strategy_run_id = ? AND lp.quantity > 0 AND lp.quantity != COALESCE(h.quantity, 0)
				`, id, run.ID).Rows()
				if rows != nil {
					defer rows.Close()
					for rows.Next() {
						var code string
						var lq, hq int
						rows.Scan(&code, &lq, &hq)
						log.Printf("[reconcile] run=%d stock=%s: live=%d holdings=%d (quantity drift)", run.ID, code, lq, hq)
					}
				}
			}
		}
	}()

	response.Success(c, portfolio)
}

// GetBrokerBalance fetches balance from the account's configured broker.
func (h *LiveTradingHandler) GetBrokerBalance(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	balance, err := h.brokerSvc.GetBrokerBalance(uint(id), uid)
	if err != nil {
		log.Printf("[live] GetBrokerBalance %d failed: %v", id, err)
		response.InternalError(c, "查询余额失败: "+err.Error())
		return
	}

	response.Success(c, balance)
}

// GetBrokerOrders fetches recent orders from the account's configured broker.
func (h *LiveTradingHandler) GetBrokerOrders(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	orders, err := h.brokerSvc.GetBrokerOrders(uint(id), uid)
	if err != nil {
		log.Printf("[live] GetBrokerOrders %d failed: %v", id, err)
		response.InternalError(c, "查询委托失败: "+err.Error())
		return
	}

	response.Success(c, orders)
}

// PlaceBrokerOrder places a trade order through the account's configured broker.
func (h *LiveTradingHandler) PlaceBrokerOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	var body struct {
		StockCode      string  `json:"stockCode"`
		OrderType      string  `json:"orderType"`
		Price          float64 `json:"price"`
		Quantity       int     `json:"quantity"`
		UseMarketPrice bool    `json:"useMarketPrice"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, 400, 400, "参数错误: "+err.Error())
		return
	}

	if body.StockCode == "" || body.OrderType == "" || body.Quantity <= 0 {
		response.Error(c, 400, 400, "缺少必要参数: stockCode, orderType, quantity")
		return
	}

	req := &service.BrokerOrderRequest{
		StockCode:      body.StockCode,
		OrderType:      body.OrderType,
		Price:          body.Price,
		Quantity:       body.Quantity,
		UseMarketPrice: body.UseMarketPrice,
	}

	result, err := h.brokerSvc.PlaceBrokerOrder(uint(id), uid, req)
	if err != nil {
		log.Printf("[live] PlaceBrokerOrder %d failed: %v", id, err)
		response.InternalError(c, "下单失败: "+err.Error())
		return
	}

	response.Success(c, result)
}

// CancelBrokerOrder cancels an order through the account's configured broker.
func (h *LiveTradingHandler) CancelBrokerOrder(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	var body struct {
		OrderID   string `json:"orderId"`
		StockCode string `json:"stockCode"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Error(c, 400, 400, "参数错误: "+err.Error())
		return
	}

	if err := h.brokerSvc.CancelBrokerOrder(uint(id), uid, body.OrderID, body.StockCode); err != nil {
		log.Printf("[live] CancelBrokerOrder %d failed: %v", id, err)
		response.InternalError(c, "撤单失败: "+err.Error())
		return
	}

	response.SuccessMsg(c, "撤单成功")
}

// BrokerStatus checks connectivity with the account's configured broker.
func (h *LiveTradingHandler) BrokerStatus(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id == 0 {
		response.Error(c, 400, 400, "缺少账户ID")
		return
	}
	uid := getUID(c)

	_, err = h.brokerSvc.GetBrokerBalance(uint(id), uid)
	if err != nil {
		log.Printf("[live] BrokerStatus %d failed: %v", id, err)
		response.Success(c, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
		})
		return
	}

	// Refetch the account to get all updated fields
	var acct model.TradingAccount
	db.MySQL.First(&acct, id)
	response.Success(c, map[string]interface{}{
		"connected":        true,
		"totalAssets":      acct.TotalAssets,
		"availBalance":     acct.AvailableCash,
		"frozenFunds":      acct.FrozenCash,
		"totalMarketValue": acct.TotalMarketValue,
		"totalProfit":      acct.TotalProfit,
		"nav":              acct.Nav,
		"initialCapital":   acct.InitialCapital,
	})
}

// ClearSignals removes all pending signals for a run on a specific date.
func (h *LiveTradingHandler) ClearSignals(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))
	date := c.Query("date")
	if date == "" {
		response.BadRequest(c, "缺少日期参数 date")
		return
	}

	// Verify run ownership
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND user_id = ?", rid, uid).First(&run).Error; err != nil {
		response.NotFound(c, "运行不存在")
		return
	}

	result := db.MySQL.Where("run_id = ? AND user_id = ? AND exec_date = ? AND status IN ?",
		rid, uid, date, []string{"pending"}).Delete(&model.BacktestSignal{})

	response.Success(c, map[string]interface{}{
		"deleted": result.RowsAffected,
		"date":    date,
	})
}

func RegisterLiveTradingRoutes(r *gin.RouterGroup, h *LiveTradingHandler) {
	// Account CRUD (multi-account)
	r.GET("/accounts", h.ListAccounts)
	r.POST("/accounts", h.CreateAccount)
	r.PUT("/accounts/:id", h.UpdateAccount)
	r.DELETE("/accounts/:id", h.DeleteAccount)
	r.GET("/account", h.GetAccount) // backward compat — returns array

	// Strategy runs
	r.POST("/runs", h.CreateRun)
	r.GET("/runs", h.ListRuns)
	r.GET("/runs/:id", h.GetRun)
	r.PUT("/runs/:id/status", h.UpdateRunStatus)
	r.PUT("/runs/:id/config", h.UpdateRunConfig)

	// Daily execution (async)
	r.POST("/daily-run", h.RunDaily)
	r.GET("/daily-run/tasks/:id", h.GetDailyRunTask)
	r.GET("/daily-run/tasks/latest", h.GetLatestDailyRunTask)

	// Trade execution (new pipeline)
	r.POST("/runs/:id/trade-exec", h.ExecuteTrade)
	r.POST("/runs/:id/signals/:signalId/execute", h.ExecuteSingleSignal)

	// Signal execution
	r.PUT("/signals/:id", h.UpdateSignal)
	r.DELETE("/signals/:id", h.DeleteSignal)
	r.POST("/signals/:id/execute", h.ExecuteSignal)
	r.DELETE("/runs/:id/signals", h.ClearSignals)

	// Positions & trades
	r.GET("/runs/:id/positions", h.GetPositions)
	r.GET("/runs/:id/trades", h.GetTrades)
	r.GET("/runs/:id/snapshots", h.GetDailySnapshots)
	r.POST("/runs/:id/notify", h.SendNotification)

	// Broker integration
	r.POST("/accounts/:id/sync", h.SyncFromBroker)
	r.GET("/accounts/:id/broker-status", h.BrokerStatus)
	r.GET("/accounts/:id/broker-orders", h.GetBrokerOrders)
	r.POST("/accounts/:id/broker-order", h.PlaceBrokerOrder)
	r.POST("/accounts/:id/broker-cancel", h.CancelBrokerOrder)

	// Order sync
	r.POST("/order-sync", h.SyncOrders)
	r.POST("/reconcile", h.ReconcileFromBroker)
	r.GET("/runs/:id/reconciliation", h.GetReconciliation)
	r.POST("/signals/:id/sync-order", h.SyncSignalOrder)
	r.POST("/signals/:id/cancel-order", h.CancelSignalOrder)
	r.POST("/runs/:id/signals/cancel-all-orders", h.CancelAllSignalOrders)

	// Execution logs
	r.GET("/runs/:id/logs", h.GetRunLogs)
}

// CancelSignalOrder cancels a pending_order signal's broker order and resets it to pending.
func (h *LiveTradingHandler) CancelSignalOrder(c *gin.Context) {
	sid, _ := strconv.Atoi(c.Param("id"))
	if sid <= 0 {
		response.BadRequest(c, "无效的信号ID")
		return
	}
	uid := getUID(c)

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&sig).Error; err != nil {
		response.NotFound(c, "信号不存在")
		return
	}
	if sig.Status != "pending_order" && sig.Status != "partial_filled" {
		response.BadRequest(c, "只有委托中或部分成交的信号才能撤单")
		return
	}

	// Get the run to find the account
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ?", sig.RunID).First(&run).Error; err != nil {
		response.InternalError(c, "找不到关联策略运行")
		return
	}

	// Cancel broker order if broker_order_id exists
	if sig.BrokerOrderID != "" && run.AccountID > 0 {
		if err := h.brokerSvc.CancelBrokerOrder(run.AccountID, uid, sig.BrokerOrderID, sig.StockCode); err != nil {
			log.Printf("[live] CancelSignalOrder signal=%d order=%s failed: %v", sid, sig.BrokerOrderID, err)
			response.InternalError(c, "券商撤单失败: "+err.Error())
			return
		}
	}

	// Reset signal status to pending for re-execution
	db.MySQL.Model(&sig).Updates(map[string]interface{}{
		"status":          "pending",
		"broker_order_id": "",
		"reason":          sig.Reason + " | 已撤单，等待重新执行",
	})

	log.Printf("[live] CancelSignalOrder signal=%d stock=%s order=%s → reset to pending", sid, sig.StockCode, sig.BrokerOrderID)
	response.SuccessMsg(c, "撤单成功，信号已重置为待执行")
}

// CancelAllSignalOrders batch-cancels all pending_order signals for a run on the current date.
func (h *LiveTradingHandler) CancelAllSignalOrders(c *gin.Context) {
	rid, _ := strconv.Atoi(c.Param("id"))
	if rid <= 0 {
		response.BadRequest(c, "无效的运行ID")
		return
	}
	uid := getUID(c)
	today := time.Now().Format("2006-01-02")

	var sigs []model.BacktestSignal
	db.MySQL.Where("run_id = ? AND user_id = ? AND exec_date = ? AND status IN ?",
		rid, uid, today, []string{"pending_order", "partial_filled"}).Find(&sigs)

	if len(sigs) == 0 {
		response.Success(c, map[string]interface{}{"cancelled": 0, "message": "当日无委托中的订单"})
		return
	}

	// Get the run to find the account
	var run model.StrategyRun
	db.MySQL.Where("id = ?", rid).First(&run)

	successCount := 0
	failCount := 0
	for _, sig := range sigs {
		if sig.BrokerOrderID != "" && run.AccountID > 0 {
			if err := h.brokerSvc.CancelBrokerOrder(run.AccountID, uid, sig.BrokerOrderID, sig.StockCode); err != nil {
				log.Printf("[live] CancelAllSignalOrders signal=%d order=%s failed: %v", sig.ID, sig.BrokerOrderID, err)
				failCount++
				continue
			}
		}
		db.MySQL.Model(&sig).Updates(map[string]interface{}{
			"status":          "pending",
			"broker_order_id": "",
			"reason":          sig.Reason + " | 批量撤单，等待重新执行",
		})
		successCount++
	}

	log.Printf("[live] CancelAllSignalOrders run=%d: cancelled=%d failed=%d", rid, successCount, failCount)
	response.Success(c, map[string]interface{}{
		"cancelled": successCount,
		"failed":    failCount,
		"message":   fmt.Sprintf("成功撤单 %d 笔，失败 %d 笔", successCount, failCount),
	})
}

// GetRunLogs returns execution logs for a run, grouped by log_type.
// Query params: ?date=YYYY-MM-DD (defaults to latest available)
func (h *LiveTradingHandler) GetRunLogs(c *gin.Context) {
	rid, _ := strconv.Atoi(c.Param("id"))
	if rid <= 0 {
		response.BadRequest(c, "无效的ID")
		return
	}
	date := c.Query("date")
	logSvc := service.NewExecutionLogService()

	if date == "" {
		dates, _ := logSvc.GetAvailableDates(uint(rid))
		if len(dates) > 0 {
			date = dates[0]
		}
	}

	if date == "" {
		response.Success(c, map[string]interface{}{"strategy": []string{}, "trade_exec": []string{}})
		return
	}

	logs, err := logSvc.LoadRunLogsJSON(uint(rid), date)
	if err != nil {
		response.InternalError(c, "加载日志失败")
		return
	}

	strategyTime := logSvc.LastExecutionTime(uint(rid), date, "strategy")
	tradeExecTime := logSvc.LastExecutionTime(uint(rid), date, "trade_exec")

	dates, _ := logSvc.GetAvailableDates(uint(rid))
	response.Success(c, map[string]interface{}{
		"logs":            logs,
		"date":            date,
		"availableDates":  dates,
		"strategyTime":    strategyTime,
		"tradeExecTime":   tradeExecTime,
	})
}

// ReconcileFromBroker rebuilds positions/funds/trades from broker's actual state.
// Use for data repair when local records are out of sync.
func (h *LiveTradingHandler) ReconcileFromBroker(c *gin.Context) {
	uid := getUID(c)
	runID, _ := strconv.Atoi(c.DefaultQuery("runId", "0"))
	accountID, _ := strconv.Atoi(c.DefaultQuery("accountId", "0"))

	if runID == 0 || accountID == 0 {
		// Try to resolve from run
		var run model.StrategyRun
		if err := db.MySQL.Where("id = ? AND user_id = ?", runID, uid).First(&run).Error; err == nil {
			if accountID == 0 {
				accountID = int(run.AccountID)
			}
		} else {
			response.BadRequest(c, "请提供 runId 和 accountId")
			return
		}
	}

	svc := service.NewLiveTradingService()
	if err := svc.ReconcileFromBroker(uint(accountID), uid, uint(runID)); err != nil {
		response.InternalError(c, "数据修复失败: "+err.Error())
		return
	}
	response.Success(c, map[string]string{"status": "ok"})
}

// GetReconciliation compares live_positions (strategy view) vs holdings (broker view).
func (h *LiveTradingHandler) GetReconciliation(c *gin.Context) {
	uid := getUID(c)
	rid, _ := strconv.Atoi(c.Param("id"))

	// Load run
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND user_id = ?", rid, uid).First(&run).Error; err != nil {
		response.NotFound(c, "实盘运行不存在")
		return
	}

	// Load live_positions (strategy view)
	var livePositions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ? AND quantity > 0", rid).Find(&livePositions)

	// Load holdings (broker view)
	var holdings []model.Holding
	db.MySQL.Where("account_id = ? AND quantity > 0", run.AccountID).Find(&holdings)

	// Build lookup maps
	liveMap := make(map[string]model.LivePosition)
	for _, p := range livePositions { liveMap[p.StockCode] = p }
	holdingMap := make(map[string]model.Holding)
	for _, h := range holdings { holdingMap[h.StockCode] = h }

	type ReconItem struct {
		StockCode  string      `json:"stockCode"`
		Live       interface{} `json:"live,omitempty"`
		Holding    interface{} `json:"holding,omitempty"`
		Status     string      `json:"status"`
		DiffCause  string      `json:"diffCause,omitempty"`
	}
	var matched, manualOnly, strategyOnly, priceDiff []ReconItem

	for code, lp := range liveMap {
		if h, ok := holdingMap[code]; ok {
			diff := lp.AvgCost - h.CostPrice
			if diff < 0 { diff = -diff }
			if diff > 0.01 {
				priceDiff = append(priceDiff, ReconItem{
					StockCode: code,
					Live:      map[string]interface{}{"quantity": lp.Quantity, "avgCost": lp.AvgCost, "todayBuyQty": lp.TodayBuyQty},
					Holding:   map[string]interface{}{"quantity": h.Quantity, "costPrice": h.CostPrice, "todayBuyQty": h.TodayBuyQty},
					Status:    "price_diff",
					DiffCause: fmt.Sprintf("成本差异 %.2f%%", (lp.AvgCost-h.CostPrice)/h.CostPrice*100),
				})
			} else {
				matched = append(matched, ReconItem{
					StockCode: code,
					Live:      map[string]interface{}{"quantity": lp.Quantity, "avgCost": lp.AvgCost, "todayBuyQty": lp.TodayBuyQty},
					Holding:   map[string]interface{}{"quantity": h.Quantity, "costPrice": h.CostPrice, "todayBuyQty": h.TodayBuyQty},
					Status:    "matched",
				})
			}
		} else {
			strategyOnly = append(strategyOnly, ReconItem{
				StockCode: code,
				Live:      map[string]interface{}{"quantity": lp.Quantity, "avgCost": lp.AvgCost},
				Status:    "strategy_only",
			})
		}
	}
	for code, h := range holdingMap {
		if _, ok := liveMap[code]; !ok {
			manualOnly = append(manualOnly, ReconItem{
				StockCode: code,
				Holding:   map[string]interface{}{"quantity": h.Quantity, "costPrice": h.CostPrice, "stockName": h.StockName},
				Status:    "manual_only",
				DiffCause: "手动交易或券商同步",
			})
		}
	}

	response.Success(c, map[string]interface{}{
		"runId":       rid,
		"accountId":   run.AccountID,
		"matched":     matched,
		"manualOnly":  manualOnly,
		"strategyOnly": strategyOnly,
		"priceDiff":   priceDiff,
		"summary": map[string]interface{}{
			"totalMatched":     len(matched),
			"totalManualOnly":  len(manualOnly),
			"totalStrategyOnly": len(strategyOnly),
			"totalPriceDiff":   len(priceDiff),
			"isClean":          len(manualOnly) == 0 && len(strategyOnly) == 0 && len(priceDiff) == 0,
		},
	})
}

// SyncOrders manually triggers order status synchronization from brokers.
// Optional query param: ?runId=N to sync only signals for a specific run.
func (h *LiveTradingHandler) SyncOrders(c *gin.Context) {
	svc := service.NewOrderSyncService()
	runID, _ := strconv.Atoi(c.DefaultQuery("runId", "0"))
	result, err := svc.SyncAllPendingOrders(uint(runID))
	if err != nil {
		response.InternalError(c, "订单同步失败: "+err.Error())
		return
	}
	response.Success(c, result)
}

// SyncSignalOrder syncs a single signal's order status from the broker.
func (h *LiveTradingHandler) SyncSignalOrder(c *gin.Context) {
	sid, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	svc := service.NewOrderSyncService()
	newStatus, err := svc.SyncOrderForSignal(uint(sid))
	if err != nil {
		response.InternalError(c, "订单同步失败: "+err.Error())
		return
	}
	response.Success(c, map[string]string{"status": newStatus})
}

// ── Feishu Card Builder ──

// buildCardFromReport builds a Feishu card JSON from pre-market task result data.
func buildCardFromReport(runName, displayDate, markdownReport string, result map[string]interface{}, executionMode string) (cardJSON, textBody string) {
	// ── Extract structured data from result ──
	buyCount, _ := result["buyCount"].(float64)
	sellCount, _ := result["sellCount"].(float64)
	totalSignals, _ := result["totalSignals"].(float64)
	availableCash, _ := result["availableCash"].(float64)
	totalBuyAmount, _ := result["totalBuyAmount"].(float64)
	totalSellAmount, _ := result["totalSellAmount"].(float64)

	// ── Text body (DingTalk/WeCom) ──
	tl := []string{}
	tl = append(tl, fmt.Sprintf("📊 %s · %s", runName, displayDate))
	tl = append(tl, fmt.Sprintf("买入 %d 笔 (¥%.0f) | 卖出 %d 笔 (¥%.0f) | 可用 ¥%.0f",
		int(buyCount), totalBuyAmount, int(sellCount), totalSellAmount, availableCash))
	textBody = strings.Join(tl, "\n")

	// ── Feishu Card ──
	el := []map[string]interface{}{}

	// Mode tag
	isAuto := executionMode == "auto"
	modeTag := "🔧 手动执行模式"
	if isAuto {
		modeTag = "🤖 自动执行模式"
	}

	// --- Fund Summary Row ---
	el = append(el, map[string]interface{}{
		"tag": "column_set", "flex_mode": "stretch", "horizontal_spacing": "8px", "margin": "0px 0px 12px 0px",
		"columns": []map[string]interface{}{
			fcFundCard("blue", fmt.Sprintf("¥%.0f", availableCash), "💰 可用资金"),
			fcFundCard("blue", fmt.Sprintf("¥%.0f", totalBuyAmount), "📈 计划买入"),
			fcFundCard("red", fmt.Sprintf("¥%.0f", totalSellAmount), "📉 计划卖出"),
		},
	})

	// --- Position summary (if exists) ---
	if positions, ok := result["positions"].([]interface{}); ok && len(positions) > 0 {
		totalMV := 0.0
		totalPnL := 0.0
		for _, p := range positions {
			if pm, ok := p.(map[string]interface{}); ok {
				if mv, ok := pm["marketValue"].(float64); ok { totalMV += mv }
				if pnl, ok := pm["unrealizedPnl"].(float64); ok { totalPnL += pnl }
			}
		}
		pnlColor := "green"
		pnlSign := "+"
		if totalPnL < 0 { pnlColor = "red"; pnlSign = "" }
		el = append(el, fcMd(fmt.Sprintf("📦 **当前持仓**  %d 只 · 市值 ¥%.0f · 浮动 <font color='%s'>%s¥%.0f</font>",
			len(positions), totalMV, pnlColor, pnlSign, totalPnL), "0px 0px 12px 0px"))
	}

	// --- Signal Cards (2-column grid) ---
	signals, _ := result["signals"].([]interface{})
	if len(signals) > 0 {
		el = append(el, fcMd(fmt.Sprintf("**📡 交易信号 (%d 笔)**", len(signals)), "0px 0px 8px 0px"))

		// Build 2-column rows
		for i := 0; i < len(signals); i += 2 {
			row := []map[string]interface{}{}
			for j := i; j < i+2 && j < len(signals); j++ {
				row = append(row, fcSignalCard(signals[j].(map[string]interface{})))
			}
			// If odd count, add empty placeholder
			if len(row) == 1 {
				row = append(row, map[string]interface{}{
					"tag": "column", "width": "weighted", "weight": 1,
					"elements": []map[string]interface{}{},
				})
			}
			el = append(el, map[string]interface{}{
				"tag": "column_set", "flex_mode": "stretch", "horizontal_spacing": "8px", "margin": "0px 0px 8px 0px",
				"columns": row,
			})
		}
	}

	// --- Mode instruction ---
	el = append(el, fcHr())
	if isAuto {
		el = append(el, fcMd("🤖 **自动执行** — 系统将在开盘后按挂单价自动下单，请确保账户资金充足。", "0px 0px 4px 0px"))
	} else {
		el = append(el, fcMd("🔧 **手动执行** — 请登录交易终端，参考以上价格和数量进行挂单操作。", "0px 0px 4px 0px"))
		el = append(el, fcMd(fmt.Sprintf("> 📋 共 **%d 笔**交易指令，建议在开盘前完成挂单", int(totalSignals)), "0px"))
	}

	// --- Footer ---
	el = append(el, fcMd("<font color='#86909C' size='12'>⚠️ 本报告由 AI 多智能体系统自动生成，仅供参考，不构成投资建议。市场有风险，投资需谨慎。</font>", "8px 0px 0px 0px"))

	card := map[string]interface{}{
		"schema": "2.0", "config": map[string]interface{}{"update_multi": true},
		"header": map[string]interface{}{
			"template": "blue", "padding": "10px 10px 10px 10px",
			"icon":  map[string]string{"tag": "standard_icon", "color": "blue", "token": "chart-line"},
			"title": map[string]string{"tag": "plain_text", "content": "智策投研 · 决策报告"},
			"text_tag_list": []map[string]interface{}{
				{"tag": "text_tag", "color": "blue", "text": map[string]string{"tag": "plain_text", "content": runName}},
				{"tag": "text_tag", "color": "neutral", "text": map[string]string{"tag": "plain_text", "content": modeTag}},
				{"tag": "text_tag", "color": "neutral", "text": map[string]string{"tag": "plain_text", "content": displayDate}},
			},
		},
		"body": map[string]interface{}{"direction": "vertical", "elements": el},
	}
	b, _ := json.Marshal(card)
	cardJSON = string(b)
	return
}

// fcFundCard builds a fund summary card (small blue box with value + label).
func fcFundCard(color, value, label string) map[string]interface{} {
	return map[string]interface{}{
		"tag": "column", "width": "weighted", "weight": 1,
		"background_style": "blue-50", "padding": "8px", "vertical_spacing": "2px",
		"elements": []map[string]interface{}{
			{"tag": "markdown", "content": fmt.Sprintf("## <font color='%s'>%s</font>", color, value), "text_align": "center"},
			{"tag": "markdown", "content": fmt.Sprintf("<font color='grey' size='12'>%s</font>", label), "text_align": "center", "text_size": "normal"},
		},
	}
}

// fcSignalCard builds a single signal card for the 2-column grid.
func fcSignalCard(sig map[string]interface{}) map[string]interface{} {
	stockName, _ := sig["stockName"].(string)
	stockCode, _ := sig["stockCode"].(string)
	action, _ := sig["action"].(string)
	status, _ := sig["status"].(string)
	price, _ := sig["price"].(float64)
	qty, _ := sig["qty"].(float64)
	amount, _ := sig["amount"].(float64)
	confidence, _ := sig["confidence"].(float64)
	premium, _ := sig["premium"].(float64)

	actColor := "blue"
	actLabel := "买入"
	if action == "sell" || action == "reduce" || action == "stop" {
		actColor = "red"
		actLabel = "卖出"
	}

	// Map status to Chinese label
	statusLabel := ""
	switch status {
	case "pending":
		statusLabel = "待确认"
	case "confirmed":
		statusLabel = "已确认"
	case "executed":
		statusLabel = "已执行"
	case "pending_order":
		statusLabel = "委托中"
	case "pending_manual":
		statusLabel = "待手动"
	case "pending_auto":
		statusLabel = "待自动"
	case "partial_filled":
		statusLabel = "部分成交"
	case "rejected":
		statusLabel = "已驳回"
	case "skipped":
		statusLabel = "已跳过"
	}

	actionText := actLabel
	if statusLabel != "" {
		actionText = fmt.Sprintf("%s（%s）", actLabel, statusLabel)
	}

	header := fmt.Sprintf("**<font color='%s'>%s %s</font>**", actColor, stockName, stockCode)
	details := fmt.Sprintf("• 操作: <font color='%s'>**%s**</font>\n", actColor, actionText)
	if premium != 0 {
		details += fmt.Sprintf("• 价格: ¥%.2f (+%.1f%%)\n", price, premium)
	} else {
		details += fmt.Sprintf("• 价格: ¥%.2f\n", price)
	}
	details += fmt.Sprintf("• 数量: %.0f 股\n• 金额: ¥%.0f", qty, amount)
	if confidence > 0 {
		details += fmt.Sprintf("\n• AI置信度: %.0f%%", confidence)
	}

	return map[string]interface{}{
		"tag": "column", "width": "weighted", "weight": 1,
		"background_style": "blue-50", "padding": "8px", "vertical_spacing": "2px",
		"elements": []map[string]interface{}{
			{"tag": "markdown", "content": header},
			{"tag": "markdown", "content": details},
		},
	}
}

func fcHr() map[string]interface{} {
	return map[string]interface{}{"tag": "hr", "margin": "8px 0px"}
}

func fcMd(content, margin string) map[string]interface{} {
	return map[string]interface{}{"tag": "markdown", "content": content, "margin": margin}
}

// buildReportFromLiveData builds a notification report from live signals and decisions
// when no PreMarketTask exists for the requested date.
func (h *LiveTradingHandler) buildReportFromLiveData(run model.StrategyRun, tradeDate string) map[string]interface{} {
	result := make(map[string]interface{})

	if tradeDate == "" {
		tradeDate = time.Now().Format("2006-01-02")
	}

	// Load signals
	var signals []model.BacktestSignal
	db.MySQL.Where("exec_date = ? AND run_id = ?", tradeDate, run.ID).
		Order("id ASC").Find(&signals)

	// Load decisions
	var decisions []model.PreMarketDecision
	db.MySQL.Where("trade_date = ? AND run_id = ?", tradeDate, run.ID).
		Order("id ASC").Find(&decisions)

	// Fallback: if no signals on exec_date, try signal_date
	if len(signals) == 0 {
		db.MySQL.Where("signal_date = ? AND run_id = ?", tradeDate, run.ID).
			Order("id ASC").Find(&signals)
	}

	// Load account info for fund data
	var availableCash, totalBuyAmount, totalSellAmount float64
	if run.AccountID > 0 {
		var acc model.TradingAccount
		if err := db.MySQL.Where("id = ?", run.AccountID).First(&acc).Error; err == nil {
			availableCash = acc.AvailableCash
		}
	}
	// Fallback: use run equity
	if availableCash == 0 {
		availableCash = run.CurrentEquity
	}

	// Load positions for portfolio summary
	type posSummary struct {
		StockCode      string  `json:"stockCode"`
		StockName      string  `json:"stockName"`
		Quantity       int     `json:"quantity"`
		CurrentPrice   float64 `json:"currentPrice"`
		MarketValue    float64 `json:"marketValue"`
		UnrealizedPnl  float64 `json:"unrealizedPnl"`
		UnrealizedPnlPct float64 `json:"unrealizedPnlPct"`
	}
	var positions []interface{}
	var rawPositions []struct {
		StockCode       string
		StockName       string
		Quantity        int
		CurrentPrice    float64
		UnrealizedPnl   float64
		UnrealizedPnlPct float64
	}
	db.MySQL.Table("live_positions").Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&rawPositions)
	for _, p := range rawPositions {
		positions = append(positions, map[string]interface{}{
			"stockCode": p.StockCode, "stockName": p.StockName,
			"quantity": p.Quantity, "currentPrice": p.CurrentPrice,
			"marketValue": float64(p.Quantity) * p.CurrentPrice,
			"unrealizedPnl": p.UnrealizedPnl, "unrealizedPnlPct": p.UnrealizedPnlPct,
		})
	}

	buyCount, sellCount, holdCount := 0, 0, 0
	for _, s := range signals {
		switch s.ActionType {
		case "buy", "add":
			buyCount++
		case "sell", "reduce", "stop":
			sellCount++
		default:
			holdCount++
		}
	}

	// Build decision map
	decMap := make(map[uint]model.PreMarketDecision)
	for _, d := range decisions {
		decMap[d.SignalID] = d
	}

	// Build structured signals array for card rendering
	var sigCards []interface{}
	for _, sig := range signals {
		dec, hasDec := decMap[sig.ID]
		action := sig.ActionType
		qty := sig.PlannedQty
		price := sig.PlannedPrice
		amount := sig.PlannedAmount
		confidence := 0.0
		premium := sig.SuggestedPremium

		if hasDec {
			if dec.FinalAction != "" { action = dec.FinalAction }
			confidence = dec.Confidence
			if dec.SuggestedPremium != 0 { premium = dec.SuggestedPremium }

			// Sanity-check decision values before overriding signal originals
			// AI sometimes generates absurd amounts (e.g., ¥0.1) or zero quantities
			if dec.SuggestedQty > 0 && dec.SuggestedQty < sig.PlannedQty*10 {
				qty = dec.SuggestedQty
			}
			if dec.FinalPrice > 0 && dec.FinalPrice < sig.PlannedPrice*2 {
				price = dec.FinalPrice
			}
			if dec.FinalAmount > 100 && dec.FinalAmount < sig.PlannedAmount*10 {
				amount = dec.FinalAmount
			} else if qty > 0 && price > 0 {
				// Recalculate amount from qty*price if decision amount is garbage
				amount = float64(qty) * price
			}
		}

		if action == "buy" || action == "add" {
			totalBuyAmount += amount
		} else {
			totalSellAmount += amount
		}

		sigCards = append(sigCards, map[string]interface{}{
			"stockName": sig.StockName, "stockCode": sig.StockCode,
			"action": action, "status": sig.Status, "price": price, "qty": float64(qty),
			"amount": amount, "confidence": confidence, "premium": premium,
		})
	}

	// Build markdown report (for text-based channels)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# 交易执行报告\n\n> %s · %s  ·  信号 `%d` 条\n\n---\n\n",
		run.Name, tradeDate, len(signals)))
	sb.WriteString(fmt.Sprintf("## 💰 资金概览\n\n> 可用资金: ¥%.0f | 计划买入: ¥%.0f | 计划卖出: ¥%.0f\n\n",
		availableCash, totalBuyAmount, totalSellAmount))
	sb.WriteString(fmt.Sprintf("## 📋 交易执行指令\n\n> %s\n\n> 买入 %d · 卖出 %d · 持有 %d\n\n",
		tradeDate, buyCount, sellCount, holdCount))
	for i, sig := range signals {
		dec, hasDec := decMap[sig.ID]
		action := sig.ActionType
		qty := sig.PlannedQty
		price := sig.PlannedPrice
		amount := sig.PlannedAmount
		confidence := 0.0
		if hasDec {
			if dec.FinalAction != "" { action = dec.FinalAction }
			if dec.SuggestedQty > 0 { qty = dec.SuggestedQty }
			if dec.FinalPrice > 0 { price = dec.FinalPrice }
			if dec.FinalAmount > 0 { amount = dec.FinalAmount }
			confidence = dec.Confidence
		}
		actionEmoji := "📈"
		if action == "sell" || action == "reduce" || action == "stop" {
			actionEmoji = "📉"
		}
		sb.WriteString(fmt.Sprintf("**%d.** %s %s %s %s \\\n", i+1, actionEmoji, sig.StockName, sig.StockCode, action))
		sb.WriteString(fmt.Sprintf("> 价格: ¥%.2f | 数量: %d 股 | 金额: ¥%.0f", price, qty, amount))
		if confidence > 0 { sb.WriteString(fmt.Sprintf(" | AI %.0f%%", confidence)) }
		sb.WriteString("\n\n")
	}
	if len(signals) == 0 { sb.WriteString("> ⚠️ 当日无确认执行的信号\n\n") }
	sb.WriteString("---\n\n> ⚠️ **免责声明**：本报告由 AI 多智能体系统自动生成，仅供参考，不构成投资建议。市场有风险，投资需谨慎。")

	// Populate result
	result["total"] = len(signals)
	result["confirmed"] = buyCount + sellCount
	result["rejected"] = 0
	result["modified"] = 0
	result["notifyMarkdown"] = sb.String()
	result["buyCount"] = float64(buyCount)
	result["sellCount"] = float64(sellCount)
	result["holdCount"] = float64(holdCount)
	result["totalSignals"] = float64(len(signals))
	result["tradeDate"] = tradeDate
	result["availableCash"] = availableCash
	result["totalBuyAmount"] = totalBuyAmount
	result["totalSellAmount"] = totalSellAmount
	result["signals"] = sigCards
	result["positions"] = positions

	return result
}
