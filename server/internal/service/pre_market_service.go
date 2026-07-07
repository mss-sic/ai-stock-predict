package service

import (
	"strings"
	"sync"
	"time"

	"encoding/json"
	"fmt"
	"log"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// PreMarketService handles the pre-market finalization pipeline.
// Runs before market open (T+1 ~9:10): validates all pending signals with
// TradingAgents multi-agent analysis, confirms/rejects each, and sends notifications.
type PreMarketService struct {
	taOrch *TradingAgentOrchestrator
	notifier *NotificationService
	maxSignalsPerDay int
}

// NewPreMarketService creates a new pre-market service.
func NewPreMarketService(aiSvc *AIService) *PreMarketService {
	return &PreMarketService{
		taOrch:           NewTradingAgentOrchestrator(aiSvc),
		notifier:         NewNotificationService(),
		maxSignalsPerDay: 30, // safety cap to prevent runaway token costs
	}
}

// RunAsync executes the pre-market pipeline asynchronously, updating task progress.

// RunAsync executes the pre-market pipeline concurrently, updating per-signal stage progress.
func (s *PreMarketService) RunAsync(task *model.PreMarketTask) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[pre_market] task %d panic: %v", task.ID, r)
			task.Status = "failed"
			task.Error = fmt.Sprintf("panic: %v", r)
			db.MySQL.Save(task)
		}
	}()

	task.Status = "running"
	db.MySQL.Save(task)

	tradeDate := task.TradeDate
	logs := []string{}
	var logsMu sync.Mutex
	var stageMu sync.Mutex
	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logsMu.Lock()
		logs = append(logs, msg)
		logsJSON, _ := json.Marshal(logs)
		task.Logs = string(logsJSON)
		logsMu.Unlock()
	}

	addLog("═══ 交易执行开始 ═══")
	addLog("交易日: %s", tradeDate)

	// 1. Load pending signals (scoped by runId if provided)
	var signals []model.BacktestSignal
	q2 := db.MySQL.Where("exec_date = ? AND status = ?", tradeDate, "pending")
	if task.RunID > 0 {
		q2 = q2.Where("run_id = ?", task.RunID)
	}
	q2.Order("id ASC").Find(&signals)

	// 1b. Load holdings and generate virtual "hold" signals for positions not covered by pending signals
	holdSignalIDs := s.injectHoldingsForAnalysis(&signals, task, tradeDate, addLog)
	addLog("持仓分析: 生成 %d 条持有信号纳入AI分析", len(holdSignalIDs))

	total := len(signals)
	if total > s.maxSignalsPerDay {
		addLog("⚠ 信号数%d超过上限%d，截断处理", total, s.maxSignalsPerDay)
		signals = signals[:s.maxSignalsPerDay]
	}
	task.TotalSignals = total
	task.CurrentStage = "init"
	db.MySQL.Save(task)

	if total == 0 {
		addLog("═══ 交易执行: 无待验证信号 ═══")
		// Still run position patrol
		patrolResult := s.runPositionPatrol(tradeDate)
		patrolJSON, _ := json.Marshal(patrolResult)
		task.PositionPatrol = string(patrolJSON)

		task.Status = "completed"
		task.CompletedSignals = 0
		task.CurrentStage = "done"
		resultData := map[string]interface{}{
			"confirmed": 0, "rejected": 0, "modified": 0, "total": 0,
			"positionPatrol": patrolResult,
		}
		resultJSON, _ := json.Marshal(resultData)
		task.ResultJSON = string(resultJSON)
		logsJSON, _ := json.Marshal(logs)
		task.Logs = string(logsJSON)
		db.MySQL.Save(task)
		return
	}

	addLog("待验证信号: %d个 (并发处理)", total)

	// 2. Initialize per-signal stage details
	type signalStage struct {
		SignalID uint   `json:"signalId"`
		Code     string `json:"code"`
		Name     string `json:"name"`
		Action   string `json:"action"`
		Stage    string `json:"stage"`
		Logs     []string `json:"logs"`
	}
	stageDetails := make([]signalStage, total)
	for i, sig := range signals {
		stageDetails[i] = signalStage{
			SignalID: sig.ID, Code: sig.StockCode, Name: sig.StockName,
			Action: sig.ActionType, Stage: "waiting", Logs: []string{},
		}
	}

	// Helper: persist stage details
	persistStages := func() {
		sdJSON, _ := json.Marshal(stageDetails)
		task.StageDetails = string(sdJSON)
		logsJSON, _ := json.Marshal(logs)
		task.Logs = string(logsJSON)
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"stage_details": task.StageDetails,
			"completed_signals": task.CompletedSignals,
			"current_stage": task.CurrentStage,
			"logs": task.Logs,
		})
	}
	persistStages()

	// 3. Concurrent workers
	type workerResult struct {
		Idx      int
		Decision *model.PreMarketDecision
		Err      error
	}
	results := make(chan workerResult, total)

	for i := range signals {
		go func(idx int, sig model.BacktestSignal) {
			// Init stage
			func() {
				stageMu.Lock()
				defer stageMu.Unlock()
				sd := &stageDetails[idx]
				sd.Stage = "init"
			}()
			persistStages()

			// Phase callback for TA pipeline progress
			progressCB := func(stockCode, phase string) {
				stageMu.Lock()
				sd := &stageDetails[idx]
				switch phase {
				case "analysts":
					sd.Stage = "analysts"
					sd.Logs = append(sd.Logs, "📊 分析师并行报告...")
				case "debate":
					sd.Stage = "debate"
					sd.Logs = append(sd.Logs, "🐂🐻 牛熊辩论...")
				case "trader":
					sd.Stage = "trader"
					sd.Logs = append(sd.Logs, "💰 交易员决策...")
				case "risk":
					sd.Stage = "risk"
					sd.Logs = append(sd.Logs, "🛡 风控+PM审核...")
				}
				stageMu.Unlock()
				persistStages()
			}

			decision, err := s.validateSignalWithProgress(&sig, tradeDate, progressCB)

			// Matrix stage
			func() {
				stageMu.Lock()
				defer stageMu.Unlock()
				sd := &stageDetails[idx]
				sd.Stage = "matrix"
				sd.Logs = append(sd.Logs, "🔢 决策矩阵优化...")
			}()
			persistStages()

			if err != nil {
				log.Printf("[pre_market] signal %d TA failed: %v", sig.ID, err)
				decision = s.fallbackDecision(&sig, tradeDate, err.Error())
			}

			// Save signal execution metadata (don't overwrite status)
			// Status stays pending until actually executed; rejected signals become skipped
			if decision.Status == "rejected" {
				sig.Status = "skipped"
				// Truncate reason to fit VARCHAR(200)
				reason := decision.Reason
				runes := []rune(reason)
				if len(runes) > 190 {
					reason = string(runes[:190]) + "..."
				}
				sig.SkipReason = reason
			}
			sig.SuggestedPremium = decision.SuggestedPremium
			sig.OrderPrice = decision.OrderPrice
			sig.OrderPriceLimit = decision.OrderPriceLimit
			sig.SuggestedQty = decision.SuggestedQty
			sig.OpenPrice = decision.OpenPrice
			sig.OpenDeviation = decision.OpenDeviation
			sig.DecisionRule = decision.DecisionRule
			// Use targeted Updates to avoid GORM Save issues with concurrent access
			if err := db.MySQL.Model(&sig).Where("id = ?", sig.ID).Updates(map[string]interface{}{
				"status":             sig.Status,
				"skip_reason":        sig.SkipReason,
				"suggested_premium":  sig.SuggestedPremium,
				"order_price":        sig.OrderPrice,
				"order_price_limit":  sig.OrderPriceLimit,
				"suggested_qty":      sig.SuggestedQty,
				"open_price":         sig.OpenPrice,
				"open_deviation":     sig.OpenDeviation,
				"decision_rule":      sig.DecisionRule,
			}).Error; err != nil {
				log.Printf("[pre_market] failed to save signal %d: %v", sig.ID, err)
			}

			results <- workerResult{Idx: idx, Decision: decision, Err: err}
		}(i, signals[i])
	}

	// 4. Collect results
	var confirmed, rejected, modified, errors int
	allDecisions := make([]workerResult, total)
	for range signals {
		r := <-results
		allDecisions[r.Idx] = r
		if r.Err != nil {
			errors++
		}

		// Update stage detail
		func() {
			stageMu.Lock()
			defer stageMu.Unlock()
			sd := &stageDetails[r.Idx]
			switch r.Decision.Status {
		case "confirmed":
			confirmed++
			sd.Stage = "done"
			sd.Logs = append(sd.Logs, fmt.Sprintf("✅ 确认 — 置信度%.0f%%", r.Decision.Confidence))
		case "rejected":
			rejected++
			sd.Stage = "done"
			sd.Logs = append(sd.Logs, fmt.Sprintf("❌ 驳回 — %s", truncateStr(r.Decision.Reason, 60)))
		case "modified":
			modified++
			sd.Stage = "done"
			sd.Logs = append(sd.Logs, fmt.Sprintf("🔄 修正 — 置信度%.0f%%", r.Decision.Confidence))
		}

		task.CompletedSignals++
		addLog("── 信号#%d: %s(%s) → %s (置信度%.0f%%)",
			r.Decision.SignalID, r.Decision.StockCode, r.Decision.StockName,
			r.Decision.Status, r.Decision.Confidence)
		}()
		persistStages()
	}

	addLog("═══ 信号决策完成: 确认%d 驳回%d 修正%d 错误%d ═══", confirmed, rejected, modified, errors)

	// 5. Position patrol
	addLog("🔍 持仓巡检...")
	task.CurrentStage = "patrol"
	patrolResult := s.runPositionPatrol(tradeDate)
	patrolJSON, _ := json.Marshal(patrolResult)
	task.PositionPatrol = string(patrolJSON)
	if len(patrolResult) > 0 {
		addLog("持仓巡检: 生成 %d 条新信号", len(patrolResult))
		for _, p := range patrolResult {
			addLog("  ⚡ %s(%s) → %s", p["name"], p["code"], p["action"])
		}
	} else {
		addLog("持仓巡检: 无需操作")
	}
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"position_patrol": task.PositionPatrol,
		"current_stage": task.CurrentStage,
		"logs": task.Logs,
	})

	// 6. Build report & send notifications
	decisions := make([]model.PreMarketDecision, 0)
	for _, r := range allDecisions {
		if r.Decision != nil {
			decisions = append(decisions, *r.Decision)
		}
	}
	report := s.buildDecisionReport(tradeDate, signals, decisions, patrolResult, false)
	notified := 0
	if len(decisions) > 0 {
		notified = s.sendPreMarketNotifications(tradeDate, report, signals, decisions)
	}
	addLog("📤 通知已发送: %d条", notified)

	// 7. Mark complete
	task.Status = "completed"
	task.CompletedSignals = total
	task.CurrentStage = "done"
	task.CurrentCode = ""

	resultData := map[string]interface{}{
		"confirmed": confirmed, "rejected": rejected, "modified": modified,
		"errors": errors, "total": total, "notificationsSent": notified,
		"positionPatrol": patrolResult,
		"stageDetails": stageDetails,
		"notifyMarkdown": report,
	}
	resultJSON, _ := json.Marshal(resultData)
	task.ResultJSON = string(resultJSON)
	logsJSON, _ := json.Marshal(logs)
	task.Logs = string(logsJSON)
	db.MySQL.Save(task)

	log.Printf("[pre_market] task %d complete: %d confirmed, %d rejected", task.ID, confirmed, rejected)
}

// runPositionPatrol checks all active holdings for stop-loss/profit triggers.
// injectHoldingsForAnalysis loads current holdings, generates virtual "hold" signals
// for positions not covered by existing pending signals, and appends them to signals.
// Returns the IDs of the newly created signals.
func (s *PreMarketService) injectHoldingsForAnalysis(signals *[]model.BacktestSignal, task *model.PreMarketTask, tradeDate string, addLog func(string, ...interface{})) []uint {
	var ids []uint

	// Find active runs
	var runs []model.StrategyRun
	db.MySQL.Where("status = ?", "active").Find(&runs)
	if len(runs) == 0 {
		return ids
	}

	// Collect already-covered stock codes from existing signals
	covered := make(map[string]bool)
	for _, sig := range *signals {
		covered[sig.StockCode] = true
	}

	for _, run := range runs {
		var strategy model.Strategy
		if err := db.MySQL.First(&strategy, run.StrategyID).Error; err != nil {
			continue
		}

		var positions []model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&positions)

		for _, pos := range positions {
			if covered[pos.StockCode] {
				continue // Already has a pending signal in current batch
			}
			// Also check DB for existing pending/confirmed signals for same stock+date
			var existCount int64
			db.MySQL.Model(&model.BacktestSignal{}).
				Where("stock_code = ? AND exec_date = ? AND action_type = 'hold' AND status IN ?",
					pos.StockCode, nextTradeDate(tradeDate), []string{"pending", "confirmed"}).
				Count(&existCount)
			if existCount > 0 {
				covered[pos.StockCode] = true
				continue // Already has a hold signal in DB
			}

			// Get current price from PG
			var currentPrice float64
			db.PG.Raw(`SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1`,
				pos.StockCode).Scan(&currentPrice)
			if currentPrice <= 0 {
				currentPrice = pos.AvgCost
			}

			marketValue := currentPrice * float64(pos.Quantity)
			pnlPct := (currentPrice - pos.AvgCost) / pos.AvgCost * 100
			if pos.AvgCost <= 0 {
				pnlPct = 0
			}

			reason := fmt.Sprintf("持仓巡检: 成本¥%.2f 现价¥%.2f 盈亏%.1f%% 数量%d 市值¥%.0f",
				pos.AvgCost, currentPrice, pnlPct, pos.Quantity, marketValue)

			holdSig := model.BacktestSignal{
				StrategyID:  run.StrategyID,
				RunID:       run.ID,
				UserID:      run.UserID,
				SignalDate:  tradeDate,
				ExecDate:    nextTradeDate(tradeDate),
				StockCode:   pos.StockCode,
				StockName:   pos.StockName,
				ActionType:  "hold",
				PlannedPrice: currentPrice,
				PlannedQty:   int(pos.Quantity),
				PlannedAmount: marketValue,
				Status:      "pending",
				Reason:      reason,
			}
			db.MySQL.Create(&holdSig)
			*signals = append(*signals, holdSig)
			ids = append(ids, holdSig.ID)
			covered[pos.StockCode] = true
		}
	}

	return ids
}

func (s *PreMarketService) runPositionPatrol(tradeDate string) []map[string]interface{} {
	results := make([]map[string]interface{}, 0)

	// Find all active runs
	var runs []model.StrategyRun
	db.MySQL.Where("status = ?", "active").Find(&runs)

	for _, run := range runs {
		var strategy model.Strategy
		if err := db.MySQL.First(&strategy, run.StrategyID).Error; err != nil {
			continue
		}

		var positions []model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&positions)

		for _, pos := range positions {
			// Get current price
			var currentPrice float64
			db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1",
				pos.StockCode).Scan(&currentPrice)
			if currentPrice <= 0 {
				continue
			}

			// Update position current price
			pos.CurrentPrice = currentPrice
			db.MySQL.Save(&pos)

			// Check stop loss
			if pos.StopLossPrice > 0 && currentPrice <= pos.StopLossPrice {
				// Check if stop signal already exists
				var stopExist int64
				db.MySQL.Model(&model.BacktestSignal{}).
					Where("stock_code = ? AND exec_date = ? AND action_type = 'stop' AND status IN ?",
						pos.StockCode, nextTradeDate(tradeDate), []string{"pending", "confirmed"}).
					Count(&stopExist)
				if stopExist > 0 { continue }

				signal := model.BacktestSignal{
					StrategyID: run.StrategyID, RunID: run.ID, UserID: run.UserID,
					SignalDate: tradeDate, ExecDate: nextTradeDate(tradeDate),
					StockCode: pos.StockCode, StockName: pos.StockName,
					ActionType: "stop", PlannedPrice: currentPrice,
					PlannedQty: int(pos.Quantity), PlannedAmount: currentPrice * float64(pos.Quantity),
					Status: "pending",
					Reason: fmt.Sprintf("止损触发: ¥%.2f ≤ ¥%.2f (成本¥%.2f)", currentPrice, pos.StopLossPrice, pos.AvgCost),
				}
				db.MySQL.Create(&signal)
				results = append(results, map[string]interface{}{
					"code": pos.StockCode, "name": pos.StockName, "action": "stop",
					"reason": signal.Reason, "price": currentPrice,
				})
			}

			// Check stop profit
			if pos.StopProfitPrice > 0 && currentPrice >= pos.StopProfitPrice {
				// Check if sell signal already exists
				var sellExist int64
				db.MySQL.Model(&model.BacktestSignal{}).
					Where("stock_code = ? AND exec_date = ? AND action_type IN ? AND status IN ?",
						pos.StockCode, nextTradeDate(tradeDate), []string{"sell", "reduce"}, []string{"pending", "confirmed"}).
					Count(&sellExist)
				if sellExist > 0 { continue }

				signal := model.BacktestSignal{
					StrategyID: run.StrategyID, RunID: run.ID, UserID: run.UserID,
					SignalDate: tradeDate, ExecDate: nextTradeDate(tradeDate),
					StockCode: pos.StockCode, StockName: pos.StockName,
					ActionType: "sell", PlannedPrice: currentPrice,
					PlannedQty: int(pos.Quantity), PlannedAmount: currentPrice * float64(pos.Quantity),
					Status: "pending",
					Reason: fmt.Sprintf("止盈触发: ¥%.2f ≥ ¥%.2f (成本¥%.2f)", currentPrice, pos.StopProfitPrice, pos.AvgCost),
				}
				db.MySQL.Create(&signal)
				results = append(results, map[string]interface{}{
					"code": pos.StockCode, "name": pos.StockName, "action": "sell",
					"reason": signal.Reason, "price": currentPrice,
				})
			}
		}
	}

	return results
}

// PreMarketResult holds the summary of pre-market finalization.
type PreMarketResult struct {
	TradeDate        string                    `json:"tradeDate"`
	TotalSignals     int                       `json:"totalSignals"`
	Confirmed        int                       `json:"confirmed"`
	Rejected         int                       `json:"rejected"`
	Modified         int                       `json:"modified"`
	Errors           int                       `json:"errors"`
	Decisions        []model.PreMarketDecision `json:"decisions"`
	NotificationsSent int                      `json:"notificationsSent"`
	Logs             []string                  `json:"logs"`
}

// FinalizePreMarket runs the pre-market pipeline for all pending signals with today's exec date.

// PositionPatrol runs the position patrol and returns alerts (public wrapper for scheduler).
func (s *PreMarketService) PositionPatrol(tradeDate string) []map[string]interface{} {
	return s.runPositionPatrol(tradeDate)
}

// FinalizePreMarketForRun runs the pre-market pipeline only for signals belonging to a specific run.
// This is the per-run entry point called by the v2 scheduler.
func (s *PreMarketService) FinalizePreMarketForRun(tradeDate string, runID uint) (*PreMarketResult, error) {
	if runID == 0 {
		// Fallback: process all pending signals
		return s.FinalizePreMarket(tradeDate)
	}
	result := &PreMarketResult{TradeDate: tradeDate}

	// 1. Load pending signals for this run only
	var signals []model.BacktestSignal
	db.MySQL.Where("exec_date = ? AND status = ? AND run_id = ?", tradeDate, "pending", runID).
		Order("id ASC").Find(&signals)

	total := len(signals)
	if total > s.maxSignalsPerDay {
		log.Printf("[pre_market] WARNING: %d signals exceeds daily cap %d for run %d, truncating", total, s.maxSignalsPerDay, runID)
		signals = signals[:s.maxSignalsPerDay]
	}
	result.TotalSignals = len(signals)
	log.Printf("[pre_market] FinalizePreMarketForRun run=%d date=%s: %d signals", runID, tradeDate, len(signals))

	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		result.Logs = append(result.Logs, msg)
		log.Printf("[pre_market] %s", msg)
	}

	if len(signals) == 0 {
		addLog("═══ 交易执行 run=%d: 无待验证信号 ═══", runID)
		return result, nil
	}

	addLog("═══ 交易执行开始 run=%d ═══", runID)
	addLog("交易日: %s | 待验证信号: %d个", tradeDate, total)

	// 2. For each signal, run TA validation
	type sigResult struct {
		idx      int
		decision *model.PreMarketDecision
		err      error
	}
	results := make(chan sigResult, len(signals))

	for i := range signals {
		go func(idx int, sig *model.BacktestSignal) {
			addLog("── 信号#%d: %s(%s) %s ¥%.0f ──", sig.ID, sig.StockName, sig.StockCode, sig.ActionType, sig.PlannedAmount)
			decision, err := s.validateSignal(sig, tradeDate)
			if err != nil {
				log.Printf("[pre_market] signal %d (%s) TA validation failed: %v", sig.ID, sig.StockCode, err)
				decision = s.fallbackDecision(sig, tradeDate, err.Error())
			}
			results <- sigResult{idx: idx, decision: decision, err: err}
		}(i, &signals[i])
	}

	orderedDecisions := make([]*model.PreMarketDecision, len(signals))
	for range signals {
		r := <-results
		orderedDecisions[r.idx] = r.decision
		if r.err != nil {
			result.Errors++
		}
	}

	for i, decision := range orderedDecisions {
		sig := &signals[i]
		result.Decisions = append(result.Decisions, *decision)

		switch decision.Status {
		case "confirmed":
			result.Confirmed++
			addLog("  ✅ 确认 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		case "rejected":
			result.Rejected++
			addLog("  ❌ 驳回 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		case "modified":
			result.Modified++
			addLog("  🔄 修正 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		default:
			addLog("  ⚠ 未知状态: %s", decision.Status)
		}

		if decision.Status == "rejected" {
			sig.Status = "skipped"
			sig.SkipReason = decision.Reason
		}
		sig.SuggestedPremium = decision.SuggestedPremium
		sig.OrderPrice = decision.OrderPrice
		sig.OrderPriceLimit = decision.OrderPriceLimit
		sig.SuggestedQty = decision.SuggestedQty
		sig.OpenPrice = decision.OpenPrice
		sig.OpenDeviation = decision.OpenDeviation
		sig.DecisionRule = decision.DecisionRule
		db.MySQL.Save(sig)
	}

	// 3. Send notifications
	if len(result.Decisions) > 0 {
		reportBody := s.buildDecisionReport(tradeDate, signals, result.Decisions, nil, false)
		notified := s.sendPreMarketNotifications(tradeDate, reportBody, signals, result.Decisions)
		result.NotificationsSent = notified
	}

	addLog("═══ 交易执行完成 run=%d: 确认%d 驳回%d 修正%d 错误%d ═══",
		runID, result.Confirmed, result.Rejected, result.Modified, result.Errors)
	log.Printf("[pre_market] FinalizePreMarketForRun run=%d date=%s complete: %d confirmed, %d rejected",
		runID, tradeDate, result.Confirmed, result.Rejected)

	return result, nil
}

func (s *PreMarketService) FinalizePreMarket(tradeDate string) (*PreMarketResult, error) {
	result := &PreMarketResult{TradeDate: tradeDate}

	// 1. Load all pending signals for today's execution date
	var signals []model.BacktestSignal
	db.MySQL.Where("exec_date = ? AND status = ?", tradeDate, "pending").
		Order("id ASC").Find(&signals)

	total := len(signals)
	if total > s.maxSignalsPerDay {
		log.Printf("[pre_market] WARNING: %d signals exceeds daily cap %d, truncating", total, s.maxSignalsPerDay)
		signals = signals[:s.maxSignalsPerDay]
	}
	result.TotalSignals = len(signals)
	log.Printf("[pre_market] FinalizePreMarket %s: %d signals to validate", tradeDate, len(signals))

	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		result.Logs = append(result.Logs, msg)
		log.Printf("[pre_market] %s", msg)
	}

	if len(signals) == 0 {
		addLog("═══ 交易执行: 无待验证信号 ═══")
		return result, nil
	}

	addLog("═══ 交易执行开始 ═══")
	addLog("交易日: %s | 待验证信号: %d个", tradeDate, total)

	// 2. For each signal, run TA validation + decision matrix in parallel
	type sigResult struct {
		idx      int
		decision *model.PreMarketDecision
		err      error
	}
	results := make(chan sigResult, len(signals))

	for i := range signals {
		go func(idx int, sig *model.BacktestSignal) {
			addLog("── 信号#%d: %s(%s) %s ¥%.0f ──", sig.ID, sig.StockName, sig.StockCode, sig.ActionType, sig.PlannedAmount)
			decision, err := s.validateSignal(sig, tradeDate)
			if err != nil {
				log.Printf("[pre_market] signal %d (%s) TA validation failed: %v", sig.ID, sig.StockCode, err)
				decision = s.fallbackDecision(sig, tradeDate, err.Error())
			}
			results <- sigResult{idx: idx, decision: decision, err: err}
		}(i, &signals[i])
	}

	// Collect results in order
	orderedDecisions := make([]*model.PreMarketDecision, len(signals))
	for range signals {
		r := <-results
		orderedDecisions[r.idx] = r.decision
		if r.err != nil {
			result.Errors++
		}
	}

	for i, decision := range orderedDecisions {
		sig := &signals[i]
		result.Decisions = append(result.Decisions, *decision)

		switch decision.Status {
		case "confirmed":
			result.Confirmed++
			addLog("  ✅ 确认 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		case "rejected":
			result.Rejected++
			addLog("  ❌ 驳回 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		case "modified":
			result.Modified++
			addLog("  🔄 修正 — 置信度%.0f%% | %s", decision.Confidence, truncateStr(decision.Reason, 100))
		default:
			addLog("  ⚠ 未知状态: %s", decision.Status)
		}

		// Update signal execution metadata (don't overwrite status)
		if decision.Status == "rejected" {
			sig.Status = "skipped"
			sig.SkipReason = decision.Reason
		}
		sig.SuggestedPremium = decision.SuggestedPremium
		sig.OrderPrice = decision.OrderPrice
		sig.OrderPriceLimit = decision.OrderPriceLimit
		sig.SuggestedQty = decision.SuggestedQty
		sig.OpenPrice = decision.OpenPrice
		sig.OpenDeviation = decision.OpenDeviation
		sig.DecisionRule = decision.DecisionRule
		db.MySQL.Save(sig)
	}

	// 3. Send notifications for confirmed/modified signals
	if len(result.Decisions) > 0 {
		reportBody := s.buildDecisionReport(tradeDate, signals, result.Decisions, nil, false)
		notified := s.sendPreMarketNotifications(tradeDate, reportBody, signals, result.Decisions)
		result.NotificationsSent = notified
	}

	addLog("═══ 交易执行完成: 确认%d 驳回%d 修正%d 错误%d ═══",
		result.Confirmed, result.Rejected, result.Modified, result.Errors)

	log.Printf("[pre_market] FinalizePreMarket %s complete: %d confirmed, %d rejected, %d errors",
		tradeDate, result.Confirmed, result.Rejected, result.Errors)

	return result, nil
}

// RunAsyncNoAI generates decisions directly from signals without AI analysis.
// Creates PreMarketDecision records with status "confirmed" and signal-based data,
// and generates a markdown execution report.
func (s *PreMarketService) RunAsyncNoAI(task *model.PreMarketTask) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[pre_market] task %d panic: %v", task.ID, r)
			task.Status = "failed"
			task.Error = fmt.Sprintf("panic: %v", r)
			db.MySQL.Save(task)
		}
	}()

	task.Status = "running"
	db.MySQL.Save(task)

	tradeDate := task.TradeDate
	logs := []string{}
	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logs = append(logs, msg)
		logsJSON, _ := json.Marshal(logs)
		task.Logs = string(logsJSON)
		db.MySQL.Save(task)
	}

	addLog("═══ 信号决策报告生成（无AI模式） ═══")
	addLog("交易日: %s", tradeDate)

	// 1. Load pending signals (scoped by runId if provided)
	var signals []model.BacktestSignal
	q := db.MySQL.Where("exec_date = ? AND status IN ?", tradeDate, []string{"pending", "confirmed"})
	if task.RunID > 0 {
		q = q.Where("run_id = ?", task.RunID)
	}
	q.Order("id ASC").Find(&signals)

	total := len(signals)
	task.TotalSignals = total
	task.CurrentStage = "generating"
	db.MySQL.Save(task)

	if total == 0 {
		addLog("无待执行信号")
		task.Status = "completed"
		task.CompletedSignals = 0
		task.CurrentStage = "done"
		resultData := map[string]interface{}{
			"confirmed": 0, "rejected": 0, "total": 0,
			"notifyMarkdown": fmt.Sprintf("# 交易执行报告\n\n> %s  ·  信号 `0` 条  ·  AI 分析 `0` 条\n\n---\n\n## \U0001f4cb 交易执行指令\n\n> \u26a0\ufe0f 当日无确认执行的信号\n\n## \U0001f4e1 信号与决策概览\n\n> \u26a0\ufe0f 当日未生成信号\n\n## \U0001f916 AI 详细分析\n\n> \U0001f6ab **未启用 AI 决策引擎**\n> 信号基于策略条件直接生成，未经多智能体辩论分析。可在策略设置中开启 **AI 代理**。\n\n## \U0001f50d 持仓巡检\n\n> \u2705 当前持仓无需操作\n\n---\n\n> \u26a0\ufe0f **免责声明**：本报告由 AI 多智能体系统自动生成，仅供参考，不构成投资建议。市场有风险，投资需谨慎。", tradeDate),
		}
		resultJSON, _ := json.Marshal(resultData)
		task.ResultJSON = string(resultJSON)
		db.MySQL.Save(task)
		return
	}

	addLog("待执行信号: %d条", total)

	// 2. Create decisions directly from signals
	var decisions []model.PreMarketDecision
	buySignals := []model.BacktestSignal{}
	sellSignals := []model.BacktestSignal{}
	holdSignals := []model.BacktestSignal{}

	for _, sig := range signals {
		action := strings.ToLower(sig.ActionType)
		decision := model.PreMarketDecision{
			UserID:    task.UserID,
			TradeDate: tradeDate,
			SignalID:  sig.ID,
			StockCode: sig.StockCode,
			StockName: sig.StockName,
			FinalAction:  sig.ActionType,
			FinalPrice:   sig.PlannedPrice,
			FinalQty:     sig.PlannedQty,
			FinalAmount:  sig.PlannedAmount,
			SuggestedPremium: 1.5,
			OrderPrice:       sig.PlannedPrice * 1.015,
			OrderPriceLimit:  sig.PlannedPrice * 1.03,
			SuggestedQty:     sig.PlannedQty,
			Confidence: 100,
			Status:     "confirmed",
			Source:     "rule_based",
			Reason:     fmt.Sprintf("[信号执行] %s | 未启用AI决策，直接按信号执行", sig.Reason),
		}
		if err := s.upsertDecision(&decision); err != nil {
			addLog("❌ 保存决策失败 %s: %v", sig.StockCode, err)
			continue
		}
		decisions = append(decisions, decision)

		switch action {
		case "buy", "add":
			buySignals = append(buySignals, sig)
		case "sell", "stop", "reduce":
			sellSignals = append(sellSignals, sig)
		case "hold":
			holdSignals = append(holdSignals, sig)
		}
	}
	task.CompletedSignals = len(decisions)

	// 3. Build unified decision report
	report := s.buildDecisionReport(tradeDate, signals, decisions, nil, true)
	addLog("✅ 决策报告生成完成: %d条信号", len(decisions))

	task.Status = "completed"
	task.CurrentStage = "done"
	resultData := map[string]interface{}{
		"confirmed": len(decisions),
		"rejected":  0,
		"total":     len(decisions),
		"notifyMarkdown": report,
	}
	resultJSON, _ := json.Marshal(resultData)
	task.ResultJSON = string(resultJSON)
	db.MySQL.Save(task)
}


// buildDecisionReport generates a unified markdown decision report from signals and AI decisions.
// Fixed template with clean professional formatting. Empty data shows placeholder text.
func (s *PreMarketService) buildDecisionReport(
	tradeDate string,
	signals []model.BacktestSignal,
	decisions []model.PreMarketDecision,
	patrolResult []map[string]interface{},
	skipAI bool,
) string {
	var b strings.Builder

	// Index decisions by signalId for lookup
	decBySignal := make(map[uint]model.PreMarketDecision)
	for _, d := range decisions {
		decBySignal[d.SignalID] = d
	}

	// Stats
	confirmed, rejected, modified := 0, 0, 0
	for _, d := range decisions {
		switch d.Status {
		case "confirmed": confirmed++
		case "rejected": rejected++
		case "modified": modified++
		}
	}
	buySig, sellSig, addSig, reduceSig := 0, 0, 0, 0
	totalBuyAmt, totalSellAmt := 0.0, 0.0
	for _, sig := range signals {
		switch strings.ToLower(sig.ActionType) {
		case "buy": buySig++; totalBuyAmt += sig.PlannedAmount
		case "sell", "stop": sellSig++; totalSellAmt += sig.PlannedAmount
		case "add": addSig++; totalBuyAmt += sig.PlannedAmount
		case "reduce": reduceSig++; totalSellAmt += sig.PlannedAmount
		}
	}

	// ── Header ──
	b.WriteString("# \u76d8\u524d\u51b3\u7b56\u62a5\u544a\n\n")
	b.WriteString(fmt.Sprintf("> %s  ·  \u4fe1\u53f7 `%d` \u6761  ·  AI\u5206\u6790 `%d` \u6761  ·  %s\n\n",
		tradeDate, len(signals), len(decisions), time.Now().Format("15:04")))
	b.WriteString("---\n\n")

	// ── Section 1: Execution Plan (most important, on top) ──
	b.WriteString("## \U0001f4cb \u4ea4\u6613\u6267\u884c\u6307\u4ee4\n\n")

	execList := []model.PreMarketDecision{}
	for _, d := range decisions {
		if d.Status == "confirmed" || d.Status == "modified" {
			execList = append(execList, d)
		}
	}
	if len(execList) > 0 {
		b.WriteString("| # | \u80a1\u7968 | \u64cd\u4f5c | \u4ef7\u683c | \u6570\u91cf | \u91d1\u989d | \u4fe1\u5fc3 |\n")
		b.WriteString("|---|------|------|------|------|------|------|\n")
		for i, d := range execList {
			actTag := d.FinalAction
			confColor := "\U0001f7e2"
			if d.Confidence < 60 { confColor = "\U0001f7e1" }
			if d.Confidence < 40 { confColor = "\U0001f534" }
			b.WriteString(fmt.Sprintf("| %d | **%s** `%s` | %s | \u00a5%.2f | %d\u80a1 | \u00a5%.0f | %s %.0f%% |\n",
				i+1, d.StockName, d.StockCode, actTag,
				d.FinalPrice, d.FinalQty, d.FinalAmount, confColor, d.Confidence))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("> \u26a0\ufe0f \u5f53\u65e5\u65e0\u786e\u8ba4\u6267\u884c\u7684\u4fe1\u53f7\n\n")
	}

	// ── Section 2: Signal Overview Table ──
	b.WriteString("## \U0001f4e1 \u4fe1\u53f7\u4e0e\u51b3\u7b56\u6982\u89c8\n\n")

	if len(signals) > 0 {
		b.WriteString("| \u80a1\u7968 | \u4fe1\u53f7 | \u89e6\u53d1\u6761\u4ef6 | AI\u51b3\u7b56 | \u4fe1\u5fc3 | \u91d1\u989d |\n")
		b.WriteString("|------|------|----------|--------|------|------|\n")
		for _, sig := range signals {
			// Signal action badge
			actBadge := sig.ActionType
			switch strings.ToLower(sig.ActionType) {
			case "buy": actBadge = "\U0001f7e2 \u4e70\u5165"
			case "sell": actBadge = "\U0001f534 \u5356\u51fa"
			case "add": actBadge = "\U0001f7e1 \u52a0\u4ed3"
			case "reduce": actBadge = "\U0001f7e1 \u51cf\u4ed3"
			case "stop": actBadge = "\U0001f534 \u6b62\u635f"
			}

			// Trigger reason
			reason := sig.Reason
			runes := []rune(reason)
			if len(runes) > 40 { reason = string(runes[:40]) + "..." }

			// AI decision for this signal
			dec, hasDec := decBySignal[sig.ID]
			decBadge := "\u26aa \u5f85\u9a8c\u8bc1"
			confStr := "—"
			if hasDec {
				switch dec.Status {
				case "confirmed": decBadge = "\U0001f7e2 \u786e\u8ba4"
				case "rejected": decBadge = "\U0001f534 \u9a73\u56de"
				case "modified": decBadge = "\U0001f7e1 \u4fee\u6b63"
				}
				confStr = fmt.Sprintf("%.0f%%", dec.Confidence)
			}

			b.WriteString(fmt.Sprintf("| **%s** `%s` | %s | %s | %s | %s | \u00a5%.0f |\n",
				sig.StockName, sig.StockCode, actBadge, reason, decBadge, confStr, sig.PlannedAmount))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("> \u26a0\ufe0f \u5f53\u65e5\u672a\u751f\u6210\u4fe1\u53f7\n\n")
	}

	// ── Section 3: AI Detailed Analysis ──
	b.WriteString("## \U0001f916 AI\u8be6\u7ec6\u5206\u6790\n\n")

	if skipAI {
		b.WriteString("> \U0001f6ab **\u672a\u542f\u7528 AI \u51b3\u7b56\u5f15\u64ce**\n")
		b.WriteString("> \u4fe1\u53f7\u57fa\u4e8e\u7b56\u7565\u6761\u4ef6\u76f4\u63a5\u751f\u6210\uff0c\u672a\u7ecf\u591a\u667a\u80fd\u4f53\u8fa9\u8bba\u5206\u6790\u3002\u53ef\u5728\u7b56\u7565\u8bbe\u7f6e\u4e2d\u5f00\u542f **AI \u4ee3\u7406** \u3002\n\n")
	} else if len(decisions) > 0 {
		for _, d := range decisions {
			// Status badge
			statusIcon := ""
			switch d.Status {
			case "confirmed": statusIcon = "\u2705"
			case "rejected": statusIcon = "\u274c"
			case "modified": statusIcon = "\U0001f504"
			}

			b.WriteString(fmt.Sprintf("**%s %s** `%s`  |  %s  |  \u7f6e\u4fe1\u5ea6 %.0f%%\n\n",
				statusIcon, d.StockName, d.StockCode, d.FinalAction, d.Confidence))
			b.WriteString(fmt.Sprintf("> \u4ef7\u683c \u00a5%.2f  |  %d\u80a1  |  \u00a5%.0f\n\n",
				d.FinalPrice, d.FinalQty, d.FinalAmount))

			// AI reasoning as blockquote
			if d.TAReasoning != "" {
				reasoning := d.TAReasoning
				runes := []rune(reasoning)
				if len(runes) > 600 { reasoning = string(runes[:600]) + "..." }
				b.WriteString(fmt.Sprintf("> %s\n\n", reasoning))
			}

			// Inline params
			if d.SuggestedPremium != 0 {
				b.WriteString(fmt.Sprintf("- \u5efa\u8bae\u6ea2\u4ef7 %+.1f%%  |  \u6302\u5355\u4ef7 \u00a5%.2f\n", d.SuggestedPremium, d.OrderPrice))
			}
			if d.DecisionRule != "" && d.DecisionRule != "TA_AGENT" {
				b.WriteString(fmt.Sprintf("- \u89c4\u5219 `%s`\n", d.DecisionRule))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("> \u26a0\ufe0f AI \u51b3\u7b56\u5c1a\u672a\u6267\u884c\uff0c\u70b9\u51fb\u201c\u76d8\u524d\u51b3\u7b56\u201d\u89e6\u53d1\u5206\u6790\n\n")
	}

	// ── Section 4: Position Patrol ──
	b.WriteString("## \U0001f50d \u6301\u4ed3\u5de1\u68c0\n\n")
	if len(patrolResult) > 0 {
		b.WriteString("| \u80a1\u7968 | \u64cd\u4f5c | \u539f\u56e0 |\n")
		b.WriteString("|------|------|------|\n")
		for _, p := range patrolResult {
			b.WriteString(fmt.Sprintf("| **%v** `%v` | %v | %v |\n",
				p["name"], p["code"], p["action"], p["reason"]))
		}
		b.WriteString("\n")
	} else {
		b.WriteString("> \u2705 \u5f53\u524d\u6301\u4ed3\u65e0\u9700\u64cd\u4f5c\n\n")
	}

	// ── Section 5: Disclaimer ──
	b.WriteString("---\n\n")
	b.WriteString("> \u26a0\ufe0f **\u514d\u8d23\u58f0\u660e**\uff1a\u672c\u62a5\u544a\u7531 AI \u591a\u667a\u80fd\u4f53\u7cfb\u7edf\u81ea\u52a8\u751f\u6210\uff0c\u4ec5\u4f9b\u53c2\u8003\uff0c\u4e0d\u6784\u6210\u6295\u8d44\u5efa\u8bae\u3002\u5e02\u573a\u6709\u98ce\u9669\uff0c\u6295\u8d44\u9700\u8c28\u614e\u3002\n")

	return b.String()
}


// validateSignal runs the full TradingAgents pipeline on a single signal.
// validateSignalWithProgress runs TA pipeline with per-phase progress updates.

// confidenceThresholds returns adaptive confirm/modify thresholds based on strategy style.
// Aggressive strategies accept lower confidence (prioritize capturing opportunities).
// Conservative strategies require higher confidence (prioritize capital preservation).
func confidenceThresholds(style string) (confirm, modify float64) {
	switch style {
	case "momentum_chaser", "dip_buyer":
		return 40, 20 // aggressive: accept lower conviction
	case "swing_trader", "grid_trader":
		return 50, 30 // moderate-aggressive
	case "trend_follower":
		return 55, 30 // moderate
	case "value_hunter":
		return 65, 40 // conservative: need high conviction
	default:
		return 60, 30 // balanced default
	}
}

// loadStrategyForSignal loads the Strategy model for a given signal.
func loadStrategyForSignal(sig *model.BacktestSignal) *model.Strategy {
	var strategy model.Strategy
	if err := db.MySQL.First(&strategy, sig.StrategyID).Error; err != nil {
		return nil
	}
	return &strategy
}

func (s *PreMarketService) validateSignalWithProgress(sig *model.BacktestSignal, tradeDate string, progressCB func(string, string)) (*model.PreMarketDecision, error) {
	strategy := loadStrategyForSignal(sig)
	ctx, err := s.buildTAContext(sig, tradeDate, strategy)
	if err != nil {
		return nil, fmt.Errorf("build TA context: %w", err)
	}

	var uid uint
	db.MySQL.Model(&model.BacktestSignal{}).Where("id = ?", sig.ID).Select("user_id").Scan(&uid)
	if uid == 0 {
		uid = 1
	}

	result, err := s.taOrch.Run(TAOrchestratorConfig{
		UserID:           uid,
		ProgressCallback: progressCB,
	}, ctx)
	if err != nil {
		return nil, fmt.Errorf("TA pipeline: %w", err)
	}

	decision := &model.PreMarketDecision{
		UserID:     sig.UserID,
		RunID:      sig.RunID,
		SignalID:   sig.ID,
		TradeDate:  tradeDate,
		StockCode:  sig.StockCode,
		StockName:  sig.StockName,
		Source:     "ta_agent",
		Confidence: result.FinalDecision.Confidence,
		Reason:     fmt.Sprintf("TA: %s", truncateStr(result.FinalDecision.Reasoning, 900)),
	}

	fd := result.FinalDecision
	aiAct := strings.ToLower(fd.Action)
	sigAct := strings.ToLower(sig.ActionType)
	// For "hold" signals: AI recommending add/reduce/sell means position adjustment needed
	// "hold" matches "hold" (continue), "add"/"buy" means 加仓, "reduce"/"sell" means 减仓/卖出
	actionMatch := aiAct == sigAct ||
		(aiAct == "add" && sigAct == "buy") || (aiAct == "reduce" && sigAct == "sell") ||
		(sigAct == "hold" && (aiAct == "hold" || aiAct == "add" || aiAct == "reduce" || aiAct == "sell"))

	// AI can confirm or reject, but never change the signal's action
	if !actionMatch {
		// AI disagrees → reject, show AI's actual confidence and suggested action
		decision.Status = "rejected"
		decision.FinalAction = sig.ActionType
		decision.FinalPrice = sig.PlannedPrice
		decision.FinalAmount = sig.PlannedAmount
		decision.FinalQty = sig.PlannedQty
		// Don't cap confidence — show AI's true conviction even when disagreeing
		if decision.Reason == "" { decision.Reason = fmt.Sprintf("TA: %s", truncateStr(result.FinalDecision.Reasoning, 200)) }
		decision.Reason = fmt.Sprintf("[AI\u5efa\u8bae: %s \u7f6e\u4fe1\u5ea6%.0f%%] %s | [\u4fe1\u53f7: %s] \u88ab\u9a73\u56de",
			fd.Action, fd.Confidence, decision.Reason, sig.ActionType)
	} else {
		confirmTh, modifyTh := confidenceThresholds(strategy.StrategyStyle)
		if fd.Confidence >= confirmTh {
		decision.Status = "confirmed"
		decision.FinalAction = fd.Action
		decision.FinalPrice = fd.Price
		if fd.Amount > 0 {
			decision.FinalAmount = fd.Amount
		} else {
			decision.FinalAmount = sig.PlannedAmount
		}
		decision.FinalQty = sig.PlannedQty
	} else if fd.Confidence >= modifyTh {
		decision.Status = "modified"
		decision.FinalAction = sig.ActionType
		decision.FinalPrice = sig.PlannedPrice
		decision.FinalAmount = sig.PlannedAmount
		decision.FinalQty = sig.PlannedQty
	} else {
		decision.Status = "rejected"
		decision.FinalAction = sig.ActionType
		decision.FinalPrice = sig.PlannedPrice
		decision.FinalAmount = sig.PlannedAmount
		decision.FinalQty = sig.PlannedQty
	}
	}

	decision.TAReasoning = fd.Reasoning
	if debateBytes, err := json.Marshal(result.AnalystReports); err == nil {
		decision.TADebateJSON = string(debateBytes)
	}

	// Set auxiliary fields from AI output (FinalAction/FinalPrice etc already set above)
	decision.OrderPrice = fd.Price
	decision.DecisionRule = "TA_AGENT"
	decision.SuggestedPremium = fd.SuggestedPremium
	decision.OrderPriceLimit = fd.OrderPriceLimit
	decision.SuggestedQty = fd.SuggestedQty
	decision.OpenDeviation = fd.OpenDeviation
	decision.Reason = fmt.Sprintf("[AI] %s", truncateStr(decision.Reason, 900))
	runes := []rune(decision.Reason)
	if len(runes) > 900 {
		decision.Reason = string(runes[:900])
	}

	// For hold signals where AI recommends non-hold action, generate new pending signal
	if sigAct == "hold" && (aiAct == "add" || aiAct == "buy" || aiAct == "reduce" || aiAct == "sell") {
		newAction := aiAct
		if aiAct == "add" { newAction = "add" }
		if aiAct == "sell" || aiAct == "reduce" { newAction = "reduce" }
		newSignal := model.BacktestSignal{
			StrategyID: sig.StrategyID, UserID: sig.UserID,
			SignalDate: tradeDate, ExecDate: nextTradeDate(tradeDate),
			StockCode: sig.StockCode, StockName: sig.StockName,
			ActionType: newAction, PlannedPrice: fd.Price,
			PlannedQty: decision.SuggestedQty, PlannedAmount: fd.Amount,
			Status: "pending",
			Reason: fmt.Sprintf("AI持仓建议: %s → %s (置信度%.0f%%)", sigAct, newAction, fd.Confidence),
			SuggestedPremium: fd.SuggestedPremium, OrderPrice: fd.Price,
			OrderPriceLimit: fd.OrderPriceLimit, SuggestedQty: decision.SuggestedQty,
		}
		if newSignal.PlannedPrice <= 0 { newSignal.PlannedPrice = sig.PlannedPrice }
		if newSignal.PlannedQty <= 0 { newSignal.PlannedQty = sig.PlannedQty }
		if newSignal.PlannedAmount <= 0 { newSignal.PlannedAmount = sig.PlannedAmount }
		db.MySQL.Create(&newSignal)
		log.Printf("[pre_market] hold → %s for %s(%s), new signal #%d", newAction, sig.StockCode, sig.StockName, newSignal.ID)
	}

	if err := s.upsertDecision(decision); err != nil {
		log.Printf("[pre_market] failed to store decision for signal %d: %v", sig.ID, err)
	}

	return decision, nil
}

func (s *PreMarketService) validateSignal(sig *model.BacktestSignal, tradeDate string) (*model.PreMarketDecision, error) {
	// Build TradingAgents context from signal + stored data
	strategy := loadStrategyForSignal(sig)
	ctx, err := s.buildTAContext(sig, tradeDate, strategy)
	if err != nil {
		return nil, fmt.Errorf("build TA context: %w", err)
	}

	// Find user context (any user with this signal)
	var uid uint
	db.MySQL.Model(&model.BacktestSignal{}).Where("id = ?", sig.ID).Select("user_id").Scan(&uid)
	if uid == 0 {
		uid = 1 // fallback
	}

	// Run TA pipeline
	result, err := s.taOrch.Run(TAOrchestratorConfig{
		UserID:          uid,
	}, ctx)
	if err != nil {
		return nil, fmt.Errorf("TA pipeline: %w", err)
	}

	// Build PreMarketDecision from TA result
	decision := &model.PreMarketDecision{
		UserID:     sig.UserID,
		RunID:      sig.RunID,
		SignalID:   sig.ID,
		TradeDate:  tradeDate,
		StockCode:  sig.StockCode,
		StockName:  sig.StockName,
		Source:     "ta_agent",
		Confidence: result.FinalDecision.Confidence,
		Reason:     fmt.Sprintf("TA: %s", truncateStr(result.FinalDecision.Reasoning, 900)),
	}

	// Convert TA decision to final action
	fd := result.FinalDecision
	aiAct := strings.ToLower(fd.Action)
	sigAct := strings.ToLower(sig.ActionType)
	// For "hold" signals: AI recommending add/reduce/sell means position adjustment needed
	// "hold" matches "hold" (continue), "add"/"buy" means 加仓, "reduce"/"sell" means 减仓/卖出
	actionMatch := aiAct == sigAct ||
		(aiAct == "add" && sigAct == "buy") || (aiAct == "reduce" && sigAct == "sell") ||
		(sigAct == "hold" && (aiAct == "hold" || aiAct == "add" || aiAct == "reduce" || aiAct == "sell"))

	if !actionMatch {
		// AI disagrees → reject, show AI's actual confidence and suggested action
		decision.Status = "rejected"
		decision.FinalAction = sig.ActionType
		decision.FinalPrice = sig.PlannedPrice
		decision.FinalQty = sig.PlannedQty
		decision.FinalAmount = sig.PlannedAmount
		// Don't cap confidence — show AI's true conviction even when disagreeing
		if decision.Reason == "" { decision.Reason = fmt.Sprintf("TA: %s", truncateStr(result.FinalDecision.Reasoning, 200)) }
		decision.Reason = fmt.Sprintf("[AI\u5efa\u8bae: %s \u7f6e\u4fe1\u5ea6%.0f%%] %s | [\u4fe1\u53f7: %s] \u88ab\u9a73\u56de",
			fd.Action, fd.Confidence, decision.Reason, sig.ActionType)
	} else {
		confirmTh, modifyTh := confidenceThresholds(strategy.StrategyStyle)
		if fd.Confidence >= confirmTh {
		decision.Status = "confirmed"
		decision.FinalAction = fd.Action
		decision.FinalPrice = fd.Price
		decision.FinalAmount = fd.Amount
	} else if fd.Confidence >= modifyTh {
		decision.Status = "modified"
		decision.FinalAction = sig.ActionType
		decision.FinalPrice = sig.PlannedPrice
		decision.FinalQty = sig.PlannedQty
		decision.FinalAmount = sig.PlannedAmount
	} else {
		decision.Status = "rejected"
		decision.FinalAction = sig.ActionType
	}
	}

	// Store TA reasoning (debate JSON omitted for brevity, stored in TAReasoning)
	decision.TAReasoning = fd.Reasoning
	if debateBytes, err := json.Marshal(result.AnalystReports); err == nil {
		decision.TADebateJSON = string(debateBytes)
	}

	// Set auxiliary fields from AI output (FinalAction/FinalPrice etc already set above)
	decision.OrderPrice = fd.Price
	decision.DecisionRule = "TA_AGENT"
	decision.SuggestedPremium = fd.SuggestedPremium
	decision.OrderPriceLimit = fd.OrderPriceLimit
	decision.SuggestedQty = fd.SuggestedQty
	decision.OpenDeviation = fd.OpenDeviation
	decision.Reason = fmt.Sprintf("[AI] %s", truncateStr(decision.Reason, 900))
	runes := []rune(decision.Reason)
	if len(runes) > 900 {
		decision.Reason = string(runes[:900])
	}

	if err := s.upsertDecision(decision); err != nil {
		log.Printf("[pre_market] failed to store decision for signal %d: %v", sig.ID, err)
	}

	return decision, nil
}

// upsertDecision saves or updates a decision for the same signal+date (dedup).
func (s *PreMarketService) upsertDecision(decision *model.PreMarketDecision) error {
	var existing model.PreMarketDecision
	err := db.MySQL.Where("signal_id = ? AND trade_date = ?", decision.SignalID, decision.TradeDate).
		First(&existing).Error
	if err == nil {
		// Update existing
		decision.ID = existing.ID
		return db.MySQL.Model(&existing).Updates(map[string]interface{}{
			"status":             decision.Status,
			"final_action":       decision.FinalAction,
			"final_price":        decision.FinalPrice,
			"final_qty":          decision.FinalQty,
			"final_amount":       decision.FinalAmount,
			"confidence":         decision.Confidence,
			"suggested_premium":  decision.SuggestedPremium,
			"order_price":        decision.OrderPrice,
			"order_price_limit":  decision.OrderPriceLimit,
			"suggested_qty":      decision.SuggestedQty,
			"open_price":         decision.OpenPrice,
			"open_deviation":     decision.OpenDeviation,
			"decision_rule":      decision.DecisionRule,
			"reason":             decision.Reason,
			"ta_reasoning":       decision.TAReasoning,
			"ta_debate_json":     decision.TADebateJSON,
			"source":             decision.Source,
			"run_id":             decision.RunID,
		}).Error
	}
	// Create new
	return db.MySQL.Create(decision).Error
}


// fallbackDecision creates a rule-based decision when TA fails.



func (s *PreMarketService) fallbackDecision(sig *model.BacktestSignal, tradeDate, errMsg string) *model.PreMarketDecision {
	decision := &model.PreMarketDecision{
		UserID:     sig.UserID,
		SignalID:   sig.ID,
		TradeDate:  tradeDate,
		StockCode:  sig.StockCode,
		StockName:  sig.StockName,
		Status:     "confirmed", // rule-based: confirm by default
		Source:     "rule_based",
		Confidence: 50,
		FinalAction: sig.ActionType,
		FinalPrice:  sig.PlannedPrice,
		FinalQty:    sig.PlannedQty,
		FinalAmount: sig.PlannedAmount,
		Reason:      fmt.Sprintf("TA validation failed (%s), auto-confirmed via rule engine", truncateStr(errMsg, 100)),
	}
	s.upsertDecision(decision)
	return decision
}

// buildTAContext constructs TradingAgentContext from signal + stored data.
func (s *PreMarketService) buildTAContext(sig *model.BacktestSignal, tradeDate string, strategy *model.Strategy) (TradingAgentContext, error) {
	ctx := TradingAgentContext{
		StockCode:    sig.StockCode,
		StockName:    sig.StockName,
		TradeDate:    tradeDate,
		CurrentPrice: sig.PlannedPrice,
	}

	// Load recent price data for technical analysis
	type KlineRow struct {
		TradeDate string  `json:"trade_date"`
		Open      float64 `json:"open"`
		High      float64 `json:"high"`
		Low       float64 `json:"low"`
		Close     float64 `json:"close"`
		Volume    float64 `json:"volume"`
	}
	var klines []KlineRow
	db.PG.Raw(`SELECT trade_date, open, high, low, close, volume
		FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 30`, sig.StockCode).Scan(&klines)

	for i := len(klines) - 1; i >= 0; i-- {
		k := klines[i]
		ctx.RecentPrices = append(ctx.RecentPrices, PricePoint{
			Date: k.TradeDate, Open: k.Open, High: k.High, Low: k.Low, Close: k.Close, Volume: k.Volume,
		})
	}

	// Load market-level sentiment scores
	type SentRow struct {
		CompositeScore  float64 `json:"composite_score"`
		BreadthScore    float64 `json:"breadth_score"`
	}
	var sent SentRow
	db.PG.Raw(`SELECT COALESCE(composite_score, 0) as composite_score, COALESCE(breadth_score, 0) as breadth_score
		FROM market_sentiment WHERE trade_date = ? LIMIT 1`, tradeDate).Scan(&sent)
	ctx.SocialSentiment = sent.CompositeScore / 100.0  // normalize to -1~+1
	ctx.NewsSentiment = sent.BreadthScore / 100.0

	// Load fundamental indicators from stocks_daily_indicator
	ctx.Indicators = make(map[string]float64)
	type IndRow struct {
		PE               float64 `json:"pe"`
		PB               float64 `json:"pb"`
		PS               float64 `json:"ps"`
		TotalMarketCap   float64 `json:"total_market_cap"`
		TurnoverRate     float64 `json:"turnover_rate"`
		VolumeRatio      float64 `json:"volume_ratio"`
	}
	var ind IndRow
	db.PG.Raw(`SELECT COALESCE(pe,0) as pe, COALESCE(pb,0) as pb, COALESCE(ps,0) as ps, COALESCE(total_market_cap,0) as total_market_cap, COALESCE(turnover_rate,0) as turnover_rate, COALESCE(volume_ratio,0) as volume_ratio
		FROM stocks_daily_indicator WHERE code = ? ORDER BY trade_date DESC LIMIT 1`, sig.StockCode).Scan(&ind)
	ctx.PE = ind.PE
	ctx.PB = ind.PB
	ctx.PS = ind.PS
	ctx.MarketCap = ind.TotalMarketCap
	ctx.Indicators["turnover_rate"] = ind.TurnoverRate
	ctx.Indicators["volume_ratio"] = ind.VolumeRatio

	// Load recent news headlines
	type NewsRow struct {
		Title string `json:"title"`
	}
	var news []NewsRow
	db.PG.Raw(`SELECT COALESCE(title, '') as title FROM stock_news WHERE code = ? ORDER BY publish_date DESC LIMIT 5`, sig.StockCode).Scan(&news)
	for _, n := range news {
		if n.Title != "" {
			ctx.NewsHeadlines = append(ctx.NewsHeadlines, n.Title)
		}
	}

	// For hold signals: populate position data so AI knows cost, quantity, and P&L
	if strings.ToLower(sig.ActionType) == "hold" {
		var positions []model.LivePosition
		var runs []model.StrategyRun
		db.MySQL.Where("status = ?", "active").Find(&runs)
		for _, run := range runs {
			var ps []model.LivePosition
			db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&ps)
			positions = append(positions, ps...)
		}
		for _, pos := range positions {
			ctx.CurrentPositions = append(ctx.CurrentPositions, PositionSnapshot{
				Code:        pos.StockCode,
				Name:        pos.StockName,
				Quantity:    int(pos.Quantity),
				BuyPrice:    pos.AvgCost,
				MarketPrice: pos.CurrentPrice,
				MarketValue: pos.CurrentPrice * float64(pos.Quantity),
				ProfitPct:   (pos.CurrentPrice - pos.AvgCost) / pos.AvgCost * 100,
			})
		}
		// Cash/equity will be set from allocation elsewhere; skip for hold signal context
	}


	// ── Strategy Profile for AI decision context ──
	if strategy != nil {
		ctx.Strategy = StrategyProfile{
			Name:           strategy.Name,
			Style:          strategy.StrategyStyle,
			HoldDays:       strategy.ExpectedHoldDays,
			RiskProfile:    strategy.RiskProfile,
			Thesis:         strategy.StrategyThesis,
			StopLoss:       strategy.StopLoss,
			StopProfit:     strategy.StopProfit,
			PositionSizing: strategy.PositionSizing,
			BuyPositionPct: strategy.BuyPositionPct,
			MaxHoldings:    strategy.MaxHoldings,
		}
		if ctx.Strategy.HoldDays <= 0 {
			ctx.Strategy.HoldDays = 5
		}
		if ctx.Strategy.RiskProfile == "" {
			ctx.Strategy.RiskProfile = "balanced"
		}
	}

	return ctx, nil
}
func (s *PreMarketService) sendPreMarketNotifications(tradeDate, reportMarkdown string, signals []model.BacktestSignal, decisions []model.PreMarketDecision) int {
	sent := 0
	if len(decisions) == 0 {
		return sent
	}

	// Group decisions by run_id → signals
	type runGroup struct {
		RunID     uint
		RunName   string
		UserID    uint
		Signals   []model.BacktestSignal
		Decisions []model.PreMarketDecision
	}
	groups := make(map[uint]*runGroup)

	for i, dec := range decisions {
		rid := dec.RunID
		if g, ok := groups[rid]; ok {
			g.Decisions = append(g.Decisions, dec)
			if i < len(signals) {
				g.Signals = append(g.Signals, signals[i])
			}
		} else {
			g := &runGroup{RunID: rid, UserID: dec.UserID}
			g.Decisions = append(g.Decisions, dec)
			if i < len(signals) {
				g.Signals = append(g.Signals, signals[i])
			}
			groups[rid] = g
		}
	}

	// Look up run names
	for rid, g := range groups {
		var run model.StrategyRun
		if err := db.MySQL.Where("id = ?", rid).First(&run).Error; err == nil {
			g.RunName = run.Name
		}
		if g.RunName == "" {
			g.RunName = fmt.Sprintf("实盘运行 #%d", rid)
		}
	}

	for _, g := range groups {
		// Dedup per-run: check if notification already sent for this run+date
		var alreadySent int64
		db.MySQL.Model(&model.NotificationLog{}).
			Where("event_type = ? AND event_date = ? AND run_id = ?", "trade_exec_report", tradeDate, g.RunID).
			Count(&alreadySent)
		if alreadySent > 0 {
			log.Printf("[pre_market] notification already sent for run %d (%s) on %s, skipping", g.RunID, g.RunName, tradeDate)
			continue
		}

		// Count results
		confirmed, rejected, modified := 0, 0, 0
		for _, d := range g.Decisions {
			switch d.Status {
			case "confirmed": confirmed++
			case "rejected": rejected++
			case "modified": modified++
			}
		}

		// Build Feishu card
		card := s.buildFeishuCard(g.RunName, tradeDate, confirmed, rejected, modified, reportMarkdown)
		textBody := fmt.Sprintf("**%s** 交易执行 · %s\n确认 %d 笔 | 驳回 %d 笔 | 修正 %d 笔", g.RunName, tradeDate, confirmed, rejected, modified)

		envelope := map[string]string{"card": card, "text": textBody}
		envJSON, _ := json.Marshal(envelope)
		title := fmt.Sprintf("%s · %s 交易执行报告", g.RunName, tradeDate)

		if err := s.notifier.SendToUser(g.UserID, title, string(envJSON)); err != nil {
			log.Printf("[pre_market] notification failed for run %d user %d: %v", g.RunID, g.UserID, err)
			continue
		}

		db.MySQL.Create(&model.NotificationLog{
			UserID:    g.UserID,
			RunID:     g.RunID,
			EventType: "trade_exec_report",
			EventDate: tradeDate,
			Title:     title,
			SentAt:    time.Now(),
		})
		sent++
	}

	return sent
}

// buildFeishuCard builds a structured Feishu interactive card (schema 2.0) for pre-market decisions.
func (s *PreMarketService) buildFeishuCard(runName, date string, confirmed, rejected, modified int, markdown string) string {
	// Limit markdown detail to avoid card size limits
	detail := markdown
	if len(detail) > 6000 {
		detail = detail[:6000] + "\n...\n*(内容过长已截断，完整内容请在系统中查看)*"
	}

	// Summary line
	summary := "> 当日无确认执行信号"
	if confirmed > 0 {
		summary = fmt.Sprintf("> **%d 笔确认执行**，建议开盘前挂单。", confirmed)
	}
	if rejected > 0 {
		summary += fmt.Sprintf(" %d 笔被 AI 驳回。", rejected)
	}

	el := []map[string]interface{}{
		// Stats
		{
			"tag": "column_set", "flex_mode": "stretch", "horizontal_spacing": "8px", "margin": "0px 0px 12px 0px",
			"columns": []map[string]interface{}{
				{"tag": "column", "width": "weighted", "weight": 1,
					"elements": []map[string]interface{}{
						{"tag": "markdown", "content": fmt.Sprintf("<font color='#00B42A'>✅ 确认</font> **%d** 笔", confirmed), "text_align": "center"},
					}},
				{"tag": "column", "width": "weighted", "weight": 1,
					"elements": []map[string]interface{}{
						{"tag": "markdown", "content": fmt.Sprintf("<font color='#F53F3F'>❌ 驳回</font> **%d** 笔", rejected), "text_align": "center"},
					}},
				{"tag": "column", "width": "weighted", "weight": 1,
					"elements": []map[string]interface{}{
						{"tag": "markdown", "content": fmt.Sprintf("<font color='#FF7D00'>🔄 修正</font> **%d** 笔", modified), "text_align": "center"},
					}},
			},
		},
		// Separator
		{"tag": "hr", "margin": "8px 0px"},
		// Detail content
		{"tag": "markdown", "content": detail},
		// Divider
		{"tag": "hr", "margin": "12px 0px 4px 0px"},
		// Footer
		{"tag": "markdown", "content": summary, "margin": "4px 0px 0px 0px"},
		{"tag": "note", "elements": []map[string]interface{}{
			{"tag": "plain_text", "content": "⚠️ 本报告由 AI 多智能体系统自动生成，仅供参考，不构成投资建议。"},
		}},
	}

	card := map[string]interface{}{
		"schema": "2.0",
		"config": map[string]interface{}{"update_multi": true},
		"header": map[string]interface{}{
			"template": "blue",
			"padding":  "16px 16px 14px 16px",
			"icon":     map[string]string{"tag": "standard_icon", "token": "chart-line"},
			"title":    map[string]string{"tag": "plain_text", "content": "智策投研"},
			"subtitle": map[string]string{"tag": "plain_text", "content": runName},
		},
		"body": map[string]interface{}{
			"direction": "vertical",
			"padding":   "12px 16px 8px 16px",
			"elements":  el,
		},
	}
	b, _ := json.Marshal(card)
	return string(b)
}
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
