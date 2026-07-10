package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/ws"
)

// TradeExecService handles the trade execution pipeline:
// AI review → broker dispatch → position update → trade recording.
type TradeExecService struct {
	aiSvc     *AIService
	preMktSvc *PreMarketService
	brokerSvc *BrokerService
	hub       *ws.Hub
}

// NewTradeExecService creates a new trade execution service.
func NewTradeExecService(aiSvc *AIService) *TradeExecService {
	return &TradeExecService{
		aiSvc:     aiSvc,
		preMktSvc: NewPreMarketService(aiSvc),
		brokerSvc: NewBrokerService(),
	}
}

// SetHub sets the WebSocket hub for agent signal broadcasting.
func (s *TradeExecService) SetHub(hub *ws.Hub) {
	s.hub = hub
}

// TradeExecResult summarizes the outcome of a trade execution run.
type TradeExecResult struct {
	TradeDate   string `json:"tradeDate"`
	RunID       uint   `json:"runId"`
	TotalSignals int   `json:"totalSignals"`
	AIReviewed   bool  `json:"aiReviewed"`
	Confirmed    int   `json:"confirmed"`
	Rejected     int   `json:"rejected"`
	Executed     int   `json:"executed"`     // successfully placed
	Pending      int   `json:"pending"`      // pending manual/auto order
	Failed       int   `json:"failed"`       // order failed
	Logs         []string `json:"logs"`
	ReportMarkdown string `json:"reportMarkdown"`
}

// ExecuteForRun runs the full trade execution pipeline for a specific strategy run.
func (s *TradeExecService) ExecuteForRun(tradeDate string, runID uint, force bool) (*TradeExecResult, error) {
	result := &TradeExecResult{TradeDate: tradeDate, RunID: runID}

	// Load run config
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ?", runID).First(&run).Error; err != nil {
		return nil, fmt.Errorf("strategy run %d not found: %v", runID, err)
	}

	// Load the trading account — use the run's linked account
	var account model.TradingAccount
	if run.AccountID > 0 {
		if err := db.MySQL.Where("id = ? AND user_id = ? AND status = ?", run.AccountID, run.UserID, "active").
			First(&account).Error; err != nil {
			log.Printf("[trade_exec] linked account %d not found for run %d", run.AccountID, runID)
		}
	}
	if account.ID == 0 {
		// Fallback to any active account for this user
		db.MySQL.Where("user_id = ? AND status = ?", run.UserID, "active").
			Order("id ASC").First(&account)
	}
	if account.ID == 0 {
		account.BrokerMode = "manual"
		log.Printf("[trade_exec] no active account for user %d, defaulting to manual", run.UserID)
	}

	// Resolve effective execution mode with priority
	effectiveMode := resolveExecMode(&run, &account)
	// If mode requires broker but account lacks API key, fall back to manual
	if effectiveMode == "mx_moni" && account.MxAPIKey == "" {
		result.Logs = append(result.Logs, "⚠️ 账户未配置妙想API Key，降级为手动执行")
		effectiveMode = "manual"
	}

	result.Logs = append(result.Logs, fmt.Sprintf("交易执行 run=%d account=%d mode=%s (run=%s account=%s)",
		runID, account.ID, effectiveMode, run.ExecutionMode, account.BrokerMode))

	// 1. Load pending + pending_manual + pending_auto signals (retry on mode switch)
	var signals []model.BacktestSignal
	db.MySQL.Where("exec_date = ? AND status IN ? AND run_id = ?", tradeDate, []string{"pending", "pending_manual", "pending_auto"}, runID).
		Order("id ASC").Find(&signals)
	result.TotalSignals = len(signals)

	if len(signals) == 0 {
		result.Logs = append(result.Logs, "无待执行信号")
		result.ReportMarkdown = s.buildExecReport(result, run, account, nil)
		return result, nil
	}

	// 2. AI Review (if enabled)
	var decisions []model.PreMarketDecision
	if run.AiReviewEnabled {
		result.AIReviewed = true
		result.Logs = append(result.Logs, "AI审查已开启，启动多智能体分析...")
		pmResult, err := s.preMktSvc.FinalizePreMarketForRun(tradeDate, runID)
		if err != nil {
			result.Logs = append(result.Logs, fmt.Sprintf("AI审查出错: %v", err))
		} else {
			decisions = pmResult.Decisions
			result.Confirmed = pmResult.Confirmed
			result.Rejected = pmResult.Rejected
			result.Logs = append(result.Logs, pmResult.Logs...)
		}
	} else {
		// No AI review — confirm all signals
		result.Logs = append(result.Logs, "AI审查未开启，所有信号直接进入交易执行")
		for i := range signals {
			sig := &signals[i]
			decision := model.PreMarketDecision{
				SignalID:   sig.ID,
				RunID:      sig.RunID,
				UserID:     sig.UserID,
				StockCode:  sig.StockCode,
				StockName:  sig.StockName,
				Status:     "confirmed",
				Confidence: 85,
				Reason:     "AI审查未开启，信号直接通过",
				TradeDate:  tradeDate,
				FinalAction:  sig.ActionType,
				FinalPrice:   sig.PlannedPrice,
				FinalQty:     sig.PlannedQty,
				FinalAmount:  sig.PlannedAmount,
				SuggestedQty:     sig.PlannedQty,
				SuggestedPremium: 1.5,
				OrderPrice:       sig.PlannedPrice * 1.015,
				OrderPriceLimit:  sig.PlannedPrice * 1.03,
				CreatedAt:    time.Now(),
			}
			decisions = append(decisions, decision)
			result.Confirmed++
		}
	}

	// 3. Execute confirmed signals
	var executedSignals []model.BacktestSignal
	for i := range decisions {
		dec := &decisions[i]
		if dec.Status != "confirmed" {
			continue
		}
		// Find matching signal
		var sig *model.BacktestSignal
		for j := range signals {
			if signals[j].ID == dec.SignalID {
				sig = &signals[j]
				break
			}
		}
		if sig == nil {
			continue
		}

		execStatus := s.executeSignal(sig, &account, &run, tradeDate, force)
		priceStr := fmt.Sprintf("%.2f", sig.OrderPrice)
		if sig.OrderPrice <= 0 {
			priceStr = "市价"
		}
		switch execStatus {
		case "submitted":
			result.Executed++
			executedSignals = append(executedSignals, *sig)
			result.Logs = append(result.Logs, fmt.Sprintf("📤 %s %s | %s %d股@%s | %s",
				sig.StockCode, sig.StockName, sig.ActionType, sig.ExecQty, priceStr, sig.SkipReason))
		case "pending":
			result.Pending++
			result.Logs = append(result.Logs, fmt.Sprintf("⏳ %s %s | %s %d股@%s | %s",
				sig.StockCode, sig.StockName, sig.ActionType, sig.ExecQty, priceStr, sig.SkipReason))
		case "failed":
			result.Failed++
			result.Logs = append(result.Logs, fmt.Sprintf("❌ %s %s | %s %d股@%s | %s",
				sig.StockCode, sig.StockName, sig.ActionType, sig.ExecQty, priceStr, sig.SkipReason))
		}
	}

	result.Logs = append(result.Logs, fmt.Sprintf("执行完成: 执行%d 挂单%d 失败%d",
		result.Executed, result.Pending, result.Failed))

	result.ReportMarkdown = s.buildExecReport(result, run, account, executedSignals)
	return result, nil
}

// resolveExecMode determines the execution mode with priority:
// StrategyRun.ExecutionMode > TradingAccount.BrokerMode > "manual"
func resolveExecMode(run *model.StrategyRun, account *model.TradingAccount) string {
	mode := run.ExecutionMode
	if mode == "" || mode == "auto" {
		// "auto" means use account's broker mode if available
		if account.BrokerMode != "" && account.BrokerMode != "manual" {
			mode = account.BrokerMode
		} else {
			mode = "manual"
		}
	}
	// Normalize:
	// "mx" → "mx_moni"
	if mode == "mx" {
		mode = "mx_moni"
	}
	if mode == "" {
		mode = "manual"
	}
	return mode
}

// executeSignal executes a single confirmed signal.
// Returns: "executed" / "pending" / "failed"
func (s *TradeExecService) executeSignal(sig *model.BacktestSignal, account *model.TradingAccount, run *model.StrategyRun, tradeDate string, force bool) string {
	brokerMode := resolveExecMode(run, account)

	switch brokerMode {
	case "manual":
		sig.Status = "pending_manual"
		sig.SkipReason = "手动执行模式，请在前端确认下单"
		db.MySQL.Save(sig)
		return "pending"

	case "mx_moni":
		return s.executeViaMx(sig, account, run, tradeDate, force)

	case "lobster":
		sig.Status = "pending_auto"
		sig.SkipReason = "龙虾自动模式，等待自动下单"
		db.MySQL.Save(sig)

		// Broadcast to connected agent via WebSocket
		if s.hub != nil {
			sigData := map[string]interface{}{
				"signalId":   sig.ID,
				"stockCode":  sig.StockCode,
				"stockName":  sig.StockName,
				"actionType": sig.ActionType,
				"price":      sig.OrderPrice,
				"quantity":   sig.PlannedQty,
				"amount":     sig.PlannedAmount,
				"execDate":   sig.ExecDate,
				"reason":     sig.Reason,
			}
			s.hub.BroadcastSignal(account.ID, sigData)
			log.Printf("[trade_exec] broadcast signal %d to account %d via WS hub", sig.ID, account.ID)
		}
		return "pending"

	default:
		sig.Status = "pending_manual"
		sig.SkipReason = fmt.Sprintf("未知执行模式: %s", brokerMode)
		db.MySQL.Save(sig)
		return "pending"
	}
}

// executeViaMx places an order via 妙想 broker API.
func (s *TradeExecService) executeViaMx(sig *model.BacktestSignal, account *model.TradingAccount, run *model.StrategyRun, tradeDate string, force bool) string {
	// Check if market is open
	if !isMarketOpenNow() && !force {
		sig.Status = "pending_order"
		sig.SkipReason = "非交易时间(9:30-11:30,13:00-15:00)，订单已挂起，将在开盘时自动提交"
		db.MySQL.Save(sig)
		log.Printf("[trade_exec] mx order pending (market closed): %s %s", sig.StockCode, sig.ActionType)
		return "pending"
	}
	if force && !isMarketOpenNow() {
		log.Printf("[trade_exec] ⚡ force=true, bypassing market hours check for %s", sig.StockCode)
	}

	// Build order request — qty priority: suggested_qty → planned_qty → planned_amount/price → board lot
	qty := sig.SuggestedQty
	if qty <= 0 {
		qty = sig.PlannedQty
	}
	if qty <= 0 {
		refPrice := sig.OrderPrice
		if refPrice <= 0 {
			refPrice = sig.PlannedPrice
		}
		if refPrice > 0 {
			qty = int(sig.PlannedAmount / refPrice)
		}
	}
	// Apply board lot rounding
	lot := BoardLotSize(sig.StockCode)
	qty = ((qty + lot - 1) / lot) * lot
	if qty < lot {
		qty = lot
	}

	// Fallback: set order price from planned price if not specified
	if sig.OrderPrice <= 0 && sig.PlannedPrice > 0 {
		sig.OrderPrice = sig.PlannedPrice
	}

	useMarket := false
	priceType := "限价"
	if sig.OrderPrice <= 0 {
		useMarket = true
		priceType = "市价"
	}
	log.Printf("[trade_exec] 📤 准备下单: %s %s %s %d股 %s accountID=%d plannedAmount=%.0f",
		sig.StockCode, sig.StockName, sig.ActionType, qty, priceType, account.ID, sig.PlannedAmount)

	req := BrokerOrderRequest{
		StockCode:      sig.StockCode,
		OrderType:      sig.ActionType,
		Price:          sig.OrderPrice,
		Quantity:       qty,
		UseMarketPrice: useMarket,
	}

	log.Printf("[trade_exec] 🔄 调用妙想下单 API: %s %s %d股", req.OrderType, req.StockCode, req.Quantity)
	orderResult, err := s.brokerSvc.PlaceBrokerOrder(account.ID, account.UserID, &req)
	if err != nil {
		log.Printf("[trade_exec] ❌ 妙想下单失败 %s %s: %v", sig.StockCode, sig.ActionType, err)
		sig.Status = "order_failed"
		sig.SkipReason = truncateStr(fmt.Sprintf("妙想下单失败: %v", err), 200)
		db.MySQL.Save(sig)
		return "failed"
	}

	// Update signal — order placed, waiting for broker confirmation
	sig.Status = "pending_order"
	sig.BrokerOrderID = orderResult.OrderID
	sig.ExecPrice = sig.OrderPrice
	sig.ExecQty = qty
	sig.SkipReason = truncateStr(fmt.Sprintf("委托已提交 orderID=%s status=%s", orderResult.OrderID, orderResult.Status), 200)
	db.MySQL.Save(sig)
	log.Printf("[trade_exec] ✅ 妙想委托已提交: %s %s %d股@%.2f orderID=%s status=%s",
		sig.StockCode, sig.ActionType, qty, sig.OrderPrice, orderResult.OrderID, orderResult.Status)

	log.Printf("[trade_exec] 📝 委托已记录, 成交后自动创建交易记录: run=%d code=%s %s %d@%.2f orderID=%s",
		run.ID, sig.StockCode, sig.ActionType, qty, sig.OrderPrice, orderResult.OrderID)
	return "submitted"
}

// isMarketOpenNow checks if the market is currently in trading hours (9:30-11:30, 13:00-15:00).
func isMarketOpenNow() bool {
	now := time.Now()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return false
	}
	t := now.Hour()*100 + now.Minute()
	return (t >= 930 && t <= 1130) || (t >= 1300 && t <= 1500)
}

// buildExecReport generates a markdown report for the trade execution.
func (s *TradeExecService) buildExecReport(result *TradeExecResult, run model.StrategyRun, account model.TradingAccount, executedSignals []model.BacktestSignal) string {
	report := fmt.Sprintf("# 交易执行报告\n\n")
	report += fmt.Sprintf("> %s  ·  %s\n\n", run.Name, result.TradeDate)
	report += fmt.Sprintf("---\n\n")
	report += fmt.Sprintf("## 📊 执行概览\n\n")
	report += fmt.Sprintf("| 项目 | 数值 |\n")
	report += fmt.Sprintf("|------|------|\n")
	report += fmt.Sprintf("| 信号总数 | %d |\n", result.TotalSignals)
	if result.AIReviewed {
		report += fmt.Sprintf("| AI审查 | %s |\n", "✅ 已审查")
	}
	report += fmt.Sprintf("| 确认执行 | %d |\n", result.Confirmed)
	report += fmt.Sprintf("| 驳回 | %d |\n", result.Rejected)
	report += fmt.Sprintf("| 已执行 | %d |\n", result.Executed)
	report += fmt.Sprintf("| 挂单中 | %d |\n", result.Pending)
	report += fmt.Sprintf("| 失败 | %d |\n", result.Failed)
	report += fmt.Sprintf("| 执行账户 | %s (%s) |\n", account.Name, account.BrokerMode)
	report += fmt.Sprintf("\n---\n\n")

	if len(executedSignals) > 0 {
		report += fmt.Sprintf("## 📋 已执行交易\n\n")
		for _, sig := range executedSignals {
			report += fmt.Sprintf("- **%s**(%s) %s — 价格 ¥%.2f × %d 股 ≈ ¥%.0f\n",
				sig.StockName, sig.StockCode, sig.ActionType,
				sig.OrderPrice, sig.SuggestedQty, sig.OrderPrice*float64(sig.SuggestedQty))
		}
	} else if result.Confirmed > 0 {
		report += fmt.Sprintf("## 📋 待执行\n\n")
		report += fmt.Sprintf("> %d 笔信号已确认，等待执行（%s模式）\n", result.Confirmed, account.BrokerMode)
	} else {
		report += fmt.Sprintf("## ⚠️ 当日无执行交易\n\n")
	}

	report += fmt.Sprintf("\n---\n\n")
	report += fmt.Sprintf("> ⚠️ **免责声明**：本报告由系统自动生成，仅供参考，不构成投资建议。\n")

	// Persist logs to run_execution_logs table (per-day, per-type)
	if len(result.Logs) > 0 {
		logSvc := NewExecutionLogService()
		logSvc.SaveRunLogs(run.ID, result.TradeDate, "trade_exec", result.Logs)
		// Also keep backward-compat
		logJSON, _ := json.Marshal(result.Logs)
		run.LastRunLog = string(logJSON)
		run.LastRunDate = result.TradeDate
		db.MySQL.Save(&run)
	}

	return report
}



