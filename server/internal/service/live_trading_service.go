package service

import (
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"math"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// LiveTradingService orchestrates daily live trading execution.
// End-of-day pipeline: Load → Evaluate → Signal → Execute → Snapshot
type LiveTradingService struct {
	positionSvc *PositionService
	signalSvc   *SignalService
	backtestSvc *BacktestService
}

// NewLiveTradingService creates a new live trading service.
func NewLiveTradingService() *LiveTradingService {
	return &LiveTradingService{
		positionSvc: NewPositionService(),
		signalSvc:   NewSignalService(),
		backtestSvc: NewBacktestService(),
	}
}

// ── Daily Execution Pipeline ──

// DailyRunResult holds the summary of one day's execution.
type DailyRunResult struct {
	Date             string   `json:"date"`
	StrategiesRan    int      `json:"strategiesRan"`
	SignalsGenerated int      `json:"signalsGenerated"`
	TradesExecuted   int      `json:"tradesExecuted"`
	Errors           []string `json:"errors"`
	Logs             []string `json:"logs"`
}

// RunDaily evaluates yesterday's data to generate signals for today.
// Uses tradeDate as evaluation date; if empty, defaults to yesterday.
// exec_date = next trading day from eval_date (today when called after close, tomorrow when called intraday).
func (s *LiveTradingService) RunDaily(tradeDate string, mode string, runID uint) (*DailyRunResult, error) {
	// Default tradeDate based on mode
	if tradeDate == "" {
		if mode == "after_close" {
			// After market close: use today's data to generate T+1 signals
			tradeDate = time.Now().Format("2006-01-02")
		} else {
			// pre_market / intraday: use latest available data (yesterday)
			tradeDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		}
	}
	result := &DailyRunResult{Date: tradeDate}

	addSystemLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		result.Logs = append(result.Logs, msg)
		log.Printf("[live] %s", msg)
	}
	addSystemLog("═══ 执行模式: %s | 交易日: %s ═══", mode, tradeDate)

	// 1. Find strategy runs — scoped by runID (mandatory)
	var runs []model.StrategyRun
	query := db.MySQL.Where("status = ?", "active")
	if runID > 0 {
		query = query.Where("id = ?", runID)
	} else {
		return nil, fmt.Errorf("runID is required")
	}
	if err := query.Find(&runs).Error; err != nil {
		return nil, fmt.Errorf("query active runs: %w", err)
	}
	result.StrategiesRan = len(runs)
	log.Printf("[live] RunDaily %s: %d active runs", tradeDate, len(runs))

	// 2. For each active run, execute daily pipeline
	for _, run := range runs {
		sigCount, logs, err := s.runStrategyDaily(&run, tradeDate, mode, nil)
		if err != nil {
			errMsg := fmt.Sprintf("run %d: %v", run.ID, err)
			result.Errors = append(result.Errors, errMsg)
			log.Printf("[live] run %d failed on %s: %v", run.ID, tradeDate, err)
			continue
		}
		result.SignalsGenerated += sigCount
		result.Logs = append(result.Logs, logs...)
	}

	log.Printf("[live] RunDaily %s complete: %d signals, %d trades, %d errors",
		tradeDate, result.SignalsGenerated, result.TradesExecuted, len(result.Errors))

	return result, nil
}

// RunDailyWithTask runs the daily pipeline asynchronously, updating a DailyRunTask in real-time.
func (s *LiveTradingService) RunDailyWithTask(task *model.DailyRunTask) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[live-async] PANIC in RunDailyWithTask: %v", r)
			task.Status = "failed"
			task.Error = fmt.Sprintf("panic: %v", r)
			db.MySQL.Save(task)
		}
	}()
	task.Status = "running"
	db.MySQL.Save(task)

	tradeDate := task.TradeDate
	mode := task.Mode
	if tradeDate == "" {
		if mode == "after_close" {
			tradeDate = time.Now().Format("2006-01-02")
		} else {
			tradeDate = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		}
		task.TradeDate = tradeDate
	}

	var allLogs []string
	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		allLogs = append(allLogs, msg)
		log.Printf("[live-async] %s", msg)
		// Persist logs every entry for real-time frontend updates
		if len(allLogs)%2 == 0 {
			logsJSON, _ := json.Marshal(allLogs)
			db.MySQL.Model(task).Updates(map[string]interface{}{
				"logs": string(logsJSON),
				"scanned_stocks": task.ScannedStocks,
				"candidate_count": task.CandidateCount,
				"signal_count": task.SignalCount,
			})
		}
	}

	addLog("═══ 执行模式: %s | 交易日: %s ═══", mode, tradeDate)

	// Find strategy runs — always scoped by task.RunID (mandatory)
	var runs []model.StrategyRun
	query := db.MySQL.Where("status = ?", "active")
	if task.RunID > 0 {
		query = query.Where("id = ?", task.RunID)
		addLog("目标运行: ID=%d", task.RunID)
	} else {
		task.Status = "failed"
		task.Error = "runID is required but not provided"
		logsJSON, _ := json.Marshal(allLogs)
		task.Logs = string(logsJSON)
		db.MySQL.Save(task)
		return
	}
	if err := query.Find(&runs).Error; err != nil {
		task.Status = "failed"
		task.Error = fmt.Sprintf("查询运行失败: %v", err)
		logsJSON, _ := json.Marshal(allLogs)
		task.Logs = string(logsJSON)
		db.MySQL.Save(task)
		return
	}
	addLog("活跃策略运行: %d个", len(runs))

	totalSignals := 0
	progressFn := func(scanned, candidates, signals int) {
		if scanned > 0 { task.ScannedStocks = scanned }
		if candidates > 0 { task.CandidateCount = candidates }
		if signals > 0 { task.SignalCount = signals }
		{ // always save on progress update
			logsJSON, _ := json.Marshal(allLogs)
			db.MySQL.Model(task).Updates(map[string]interface{}{
				"scanned_stocks":  task.ScannedStocks,
				"candidate_count": task.CandidateCount,
				"signal_count":    task.SignalCount,
				"logs":            string(logsJSON),
			})
		}
	}
	for _, run := range runs {
		sigCount, logs, err := s.runStrategyDaily(&run, tradeDate, mode, progressFn)
		if err != nil {
			addLog("❌ run %d 执行失败: %v", run.ID, err)
		}
		totalSignals += sigCount
		allLogs = append(allLogs, logs...)
	}

	task.Status = "completed"
	task.SignalCount = totalSignals
	logsJSON, _ := json.Marshal(allLogs)
	task.Logs = string(logsJSON)
	db.MySQL.Save(task)

	log.Printf("[live-async] RunDaily %s complete: %d signals", tradeDate, totalSignals)
}

// runStrategyDaily processes one strategy run for one trading day.
// Returns: signalsGenerated, logs, error
func (s *LiveTradingService) runStrategyDaily(run *model.StrategyRun, tradeDate string, mode string, progressFn func(scanned, candidates, signals int)) (int, []string, error) {
	// Load strategy config
	var strategy model.Strategy
	if err := db.MySQL.First(&strategy, run.StrategyID).Error; err != nil {
		return 0, nil, fmt.Errorf("策略模板(ID=%d)不存在或已删除，请检查实盘运行 %d 的策略配置", run.StrategyID, run.ID)
	}

	// Load fund allocation
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
		return 0, nil, fmt.Errorf("load allocation for run %d: %w", run.ID, err)
	}

	// Load current positions
	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&positions)

	// Load strategy conditions
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ? AND enabled = true", run.StrategyID).Find(&conds)

	result := DailyRunResult{}
	// Step 1+2: Evaluate conditions + generate T+1 signals (stop-profit/loss handled by SignalEngine)
	newSignals, evalLogs, evalErr := s.evaluateAndGenerateSignals(run, &strategy, &alloc, &positions, tradeDate, mode, conds, progressFn)
	if evalErr != nil {
		log.Printf("[live] run %d evaluate failed: %v", run.ID, evalErr)
	}
	result.Logs = append(result.Logs, evalLogs...)
	result.SignalsGenerated += newSignals

	// Step 4: Snapshot end-of-day portfolio
	if err := s.snapshotPortfolio(run, &alloc, &positions, tradeDate); err != nil {
		log.Printf("[live] run %d snapshot failed: %v", run.ID, err)
	}

	// Update run's last run date + equity
	db.MySQL.Model(run).Updates(map[string]interface{}{
		"last_run_date": tradeDate,
		"current_equity": s.calcTotalEquity(&alloc, &positions),
	})

	// Persist logs to run_execution_logs table (per-day, per-type)
	if len(result.Logs) > 0 {
		logSvc := NewExecutionLogService()
		logSvc.SaveRunLogs(run.ID, tradeDate, "strategy", result.Logs)
		// Also keep backward-compat last_run_log
		logsJSON, _ := json.Marshal(result.Logs)
		db.MySQL.Model(run).Update("last_run_log", string(logsJSON))
	}

	return int(result.SignalsGenerated), result.Logs, nil
}

// ── Step 1: Execute Pending Signals ──

func (s *LiveTradingService) executePendingSignals(
	run *model.StrategyRun, strategy *model.Strategy,
	alloc *model.StrategyFundAllocation, positions *[]model.LivePosition,
	tradeDate string, conds []model.StrategyCondition,
) (int, error) {
	// Find pending signals with execDate = today
	var pendingSignals []model.BacktestSignal
	prevDate := previousTradeDate(tradeDate)
	if prevDate != "" {
		db.MySQL.Where("strategy_id = ? AND run_id = ? AND exec_date = ? AND status = ?",
			run.StrategyID, run.ID, tradeDate, "pending").Find(&pendingSignals)
	}

	tradesExecuted := 0
	for i := range pendingSignals {
		sig := &pendingSignals[i]
		if err := s.executeSignal(run, strategy, alloc, positions, sig, tradeDate); err != nil {
			log.Printf("[live] signal %d exec failed: %v", sig.ID, err)
			continue
		}
		tradesExecuted++
	}

	return tradesExecuted, nil
}

func (s *LiveTradingService) executeSignal(
	run *model.StrategyRun, strategy *model.Strategy,
	alloc *model.StrategyFundAllocation, positions *[]model.LivePosition,
	sig *model.BacktestSignal, tradeDate string,
) error {
	execEng := NewExecutionEngine()
	execPrice := sig.PlannedPrice

	// Use account-level commission/tax for live trading
	commissionRate, minCommission, stampTaxRate := s.getAccountCommission(alloc)

	// Get existing position info (from live_positions — strategy view)
	var existingQty int
	var existingAvgCost float64
	var existingTodayBuyQty int
	var buyDate string
	for _, p := range *positions {
		if p.StockCode == sig.StockCode && p.Quantity > 0 {
			existingQty = int(p.Quantity)
			existingAvgCost = p.AvgCost
			existingTodayBuyQty = p.TodayBuyQty
			buyDate = p.FirstBuyDate
			break
		}
	}

	// Also read account-level today_buy_qty from holdings (covers manual trades)
	var h model.Holding
	if db.MySQL.Where("account_id = ? AND stock_code = ?", alloc.AccountID, sig.StockCode).First(&h).Error == nil {
		if h.TodayBuyQty > existingTodayBuyQty {
			existingTodayBuyQty = h.TodayBuyQty // Use max: strategy buy + manual buy
		}
	}

	// Get daily change for limit-down check (live mode may not have this)
	var dailyChg float64
	db.PG.Raw("SELECT COALESCE(pct_chg, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", sig.StockCode, tradeDate).Scan(&dailyChg)

	execCfg := ExecutionConfig{
		CommissionRate: commissionRate,
		MinCommission:  minCommission,
		StampTaxRate:   stampTaxRate,
		MaxHoldings:    strategy.MaxHoldings,
		SlippagePct:    0, // live trading: no slippage
	}

	cash := alloc.CurrentCash
	result := execEng.Execute(
		sig.ActionType, sig.StockCode, sig.StockName,
		buyDate, tradeDate,
		sig.PlannedQty, sig.PlannedAmount,
		execPrice, dailyChg,
		&cash, existingQty, existingAvgCost,
		len(*positions), existingTodayBuyQty, execCfg,
	)

	if !result.Executed {
		sig.Status = "skipped"
		sig.SkipReason = result.SkipReason
		db.MySQL.Save(sig)
		return nil
	}

	sig.Status = "executed"
	sig.ExecPrice = result.ExecPrice
	sig.ExecQty = result.ExecQty
	sig.ExecAmount = result.ExecAmount
	sig.Pnl = result.Pnl
	sig.PnlPct = result.PnlPct
	alloc.CurrentCash = cash

	// Update positions + record trade
	switch result.ActionType {
	case "buy":
		livePos := model.LivePosition{
			UserID: run.UserID, StrategyRunID: run.ID, AllocationID: alloc.ID,
			StockCode: sig.StockCode, StockName: sig.StockName,
			Quantity: result.NewQuantity, TodayBuyQty: result.NewQuantity, AvailSellQty: 0,
			AvgCost: result.NewAvgCost,
			CurrentPrice: result.ExecPrice,
			FirstBuyDate: tradeDate, LastTradeDate: tradeDate, HoldDays: 1,
			StopLossPrice: calcStopLoss(strategy, result.ExecPrice),
			StopProfitPrice: calcStopProfit(strategy, result.ExecPrice),
		}
		db.MySQL.Create(&livePos)
		*positions = append(*positions, livePos)
		s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, 0, 0, result.Reason)
		s.syncHoldingToAccount(run, sig.StockCode, sig.StockName, tradeDate, result.NewQuantity, result.NewAvgCost, result.NewQuantity, result.ExecPrice)

	case "add":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				addQty := result.NewQuantity - p.Quantity
				p.Quantity = result.NewQuantity
				p.TodayBuyQty += addQty
				p.AvailSellQty = p.Quantity - p.TodayBuyQty
				p.AvgCost = result.NewAvgCost
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, 0, 0, result.Reason)
				s.syncHoldingToAccount(run, sig.StockCode, sig.StockName, tradeDate, p.Quantity, p.AvgCost, p.TodayBuyQty, result.ExecPrice)
				break
			}
		}

	case "sell", "stop":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				p.RealizedPnl += result.Pnl
				p.Quantity = result.NewQuantity
				p.TodayBuyQty = 0
				p.AvailSellQty = 0
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, result.Pnl, result.PnlPct, result.Reason)
				s.syncHoldingToAccount(run, sig.StockCode, sig.StockName, tradeDate, 0, 0, 0, 0)
				break
			}
		}

	case "reduce":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				p.RealizedPnl += result.Pnl
				p.Quantity = result.NewQuantity
				p.AvailSellQty = p.Quantity - p.TodayBuyQty
				if p.AvailSellQty < 0 {
					p.AvailSellQty = 0
				}
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, result.Pnl, result.PnlPct, result.Reason)
				if p.Quantity <= 0 {
					s.syncHoldingToAccount(run, sig.StockCode, sig.StockName, tradeDate, 0, 0, 0, 0)
				} else {
					s.syncHoldingToAccount(run, sig.StockCode, sig.StockName, tradeDate, p.Quantity, p.AvgCost, p.TodayBuyQty, result.ExecPrice)
				}
				break
			}
		}

	case "hold":
		sig.SkipReason = result.Reason
	}

	db.MySQL.Save(sig)
	db.MySQL.Save(alloc)

	// Sync trading account available_cash
	db.MySQL.Model(&model.TradingAccount{}).Where("id = ?", alloc.AccountID).
		Update("available_cash", alloc.CurrentCash)

	return nil
}

// getAccountCommission returns commission/tax rates from the trading account.
// Falls back to strategy defaults if account has no custom settings.
func (s *LiveTradingService) getAccountCommission(alloc *model.StrategyFundAllocation) (commRate, minComm, stampTax float64) {
	commRate = 0.00025
	minComm = 5.0
	stampTax = 0.0005

	var account model.TradingAccount
	if err := db.MySQL.Where("id = ?", alloc.AccountID).First(&account).Error; err != nil {
		return
	}
	// Account-level rates can be extended later with custom fields
	return
}

// ── Step 2: Stop Conditions ──

func (s *LiveTradingService) checkStopConditions(run *model.StrategyRun, positions *[]model.LivePosition, tradeDate string) int {
	signalsGenerated := 0
	for i := range *positions {
		p := &(*positions)[i]
		if p.Quantity <= 0 {
			continue
		}

		// Get today's close (simplified — in production, use KlineCache)
		todayClose := p.CurrentPrice

		// Update current price
		p.CurrentPrice = todayClose

		// Check stop profit
		if p.StopProfitPrice > 0 && todayClose >= p.StopProfitPrice {
			signal := model.BacktestSignal{
				StrategyID: run.StrategyID, RunID: run.ID, UserID: run.UserID,
				SignalDate: tradeDate, ExecDate: nextTradeDate(tradeDate),
				StockCode: p.StockCode, StockName: p.StockName,
				ActionType: "stop", PlannedPrice: todayClose,
				PlannedQty: p.Quantity, PlannedAmount: todayClose * float64(p.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止盈触发: %.2f >= %.2f", todayClose, p.StopProfitPrice),
			}
			db.MySQL.Create(&signal)
			signalsGenerated++
			continue
		}

		// Check stop loss
		if p.StopLossPrice > 0 && todayClose <= p.StopLossPrice {
			signal := model.BacktestSignal{
				StrategyID: run.StrategyID, RunID: run.ID, UserID: run.UserID,
				SignalDate: tradeDate, ExecDate: nextTradeDate(tradeDate),
				StockCode: p.StockCode, StockName: p.StockName,
				ActionType: "stop", PlannedPrice: todayClose,
				PlannedQty: p.Quantity, PlannedAmount: todayClose * float64(p.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止损触发: %.2f <= %.2f", todayClose, p.StopLossPrice),
			}
			db.MySQL.Create(&signal)
			signalsGenerated++
		}
	}

	return signalsGenerated
}

// ── Step 3: Evaluate Conditions + Generate Signals ──

func (s *LiveTradingService) evaluateAndGenerateSignals(
	run *model.StrategyRun, strategy *model.Strategy,
	alloc *model.StrategyFundAllocation, positions *[]model.LivePosition,
	tradeDate string, mode string, conds []model.StrategyCondition,
	progressFn func(scanned, candidates, signals int),
) (int, []string, error) {
	var logs []string
	addLog := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		logs = append(logs, msg)
		log.Printf("[live] %s", msg)
	}

	buyConds := filterByType(conds, "buy")
	sellConds := filterByType(conds, "sell")
	addConds := filterByType(conds, "add")
	reduceConds := filterByType(conds, "reduce")

	nextDate := nextTradeDate(tradeDate)
	if nextDate == "" {
		addLog("⚠ 无法计算下一交易日，跳过信号生成")
		return 0, logs, nil
	}

	addLog("═══ 策略 [%s] 条件评估开始 ═══", strategy.Name)
	addLog("交易日: %s → 执行日: %s", tradeDate, nextDate)
	addLog("持仓: %d/%d | 可用资金: ¥%.0f", countActivePositions(*positions), strategy.MaxHoldings, alloc.CurrentCash)
	addLog("条件: 买入%d条 卖出%d条 加仓%d条 减仓%d条", len(buyConds), len(sellConds), len(addConds), len(reduceConds))
	addLog("─── 参数生效检查 ───")
	addLog("💰 资金: 初始资本¥%.0f | 当前现金¥%.0f | 分配比例%.0f%%",
		alloc.AllocatedCapital, alloc.CurrentCash, alloc.PctOfAccount)
	addLog("📊 仓位基础: 最大持股%d | 买入%.0f%% | 加仓%.0f%% | 减仓%.0f%%",
		strategy.MaxHoldings, strategy.BuyPositionPct, strategy.AddPositionPct, strategy.ReducePositionPct)
	addLog("🛡 风控: 固定止损%.0f%% | 固定止盈%.0f%% | 单票集中度%.0f%% | 日亏损熔断%.0f%%",
		strategy.StopLoss, strategy.StopProfit, strategy.PositionConcentrationLimit*100, strategy.MaxDailyLoss)
	addLog("📈 动态仓位: %v | 手动总上限%.0f%% | 手动单日上限%.0f%% | 单行业≤%.0f%% | 最少行业%d",
		strategy.EnableDynamicSizing, strategy.MaxTotalPosition, strategy.DailyBuyLimit,
		strategy.MaxSingleIndustry, strategy.MinIndustryCount)
	if strategy.EnableTrailingStop {
		addLog("🎯 止盈模式: 移动止盈 (激活%.0f%% 回撤%.0f%%)",
			strategy.TrailingStopActivation, strategy.TrailingStopDrawdown)
	} else {
		addLog("🎯 止盈模式: 固定止盈%.0f%%", strategy.StopProfit)
	}
	sc := strategy.ScoringConfig
	if len(sc.Dimensions) > 0 {
		addLog("⭐ 评分配置: %d维 | 最低入选%.0f分", len(sc.Dimensions), sc.MinScore*100)
		for _, d := range sc.Dimensions {
			addLog("   %s(%s) 权重%.0f%% %s", d.Name, d.Indicator, d.Weight*100,
				map[string]string{"asc":"越小越好","desc":"越大越好"}[d.Direction])
		}
	} else {
		addLog("⭐ 评分配置: 使用默认(趋势40%% 动量30%% 量能20%% 波动10%%)")
	}
	addLog("🎚 风控强度: 进攻阈值≥%.1f | 防御阈值≥%.1f | 市场最低≥%.1f | 仓位乘数%.1f | 策略模式=%s",
		strategy.AggressiveThreshold, strategy.DefensiveThreshold,
		strategy.MarketCompositeMin, strategy.MarketPositionBias, strategy.PolicyMode)
	addLog("─── 参数检查完毕 ───")

	// ── Convert conditions to engine format ──
	engineBuyConds := ConvertToConditionDefs(conds, "buy")
	engineAddConds := ConvertToConditionDefs(conds, "add")
	engineSellConds := ConvertToConditionDefs(conds, "sell")
	engineReduceConds := ConvertToConditionDefs(conds, "reduce")

	// ── Convert positions to engine format ──
	sigPositions := make(map[string]*SignalPosition)
	for i := range *positions {
		pos := &(*positions)[i]
		if pos.Quantity > 0 {
			sigPositions[pos.StockCode] = &SignalPosition{
				Code: pos.StockCode, Name: pos.StockName,
				Quantity: pos.Quantity, BuyPrice: pos.AvgCost, BuyDate: pos.FirstBuyDate,
			}
		}
	}

	// ── Build stock universe ──
	var universe []StockInfo
	if run.StockPool != "" {
		codes := parseStockPool(run.StockPool)
		addLog("📋 使用策略股池: %d只", len(codes))
		for _, code := range codes {
			var name string; var price float64
			db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", code).Scan(&name)
			db.PG.Raw("SELECT COALESCE(close,0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&price)
			if name != "" && price > 0 {
				universe = append(universe, StockInfo{Code: code, Name: name})
			}
		}
	} else {
		addLog("📋 未设置股池，扫描全市场（排除ST/北交所）")
		type Row struct { Code string; Name string }
		var rows []Row
		db.PG.Raw(`SELECT s.code, s.name
			FROM stocks_basic s
			JOIN LATERAL (SELECT amount FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
			WHERE s.code NOT LIKE '%ST%'
			  AND s.code NOT LIKE '8%'
			ORDER BY k.amount DESC`).Scan(&rows)
		for _, r := range rows {
			universe = append(universe, StockInfo{Code: r.Code, Name: r.Name})
		}
		addLog("📋 候选股票: %d只", len(universe))
	}

	// ── Position Sizing ──
	posSizingEngine := NewPositionSizingEngine()
	runDays := int(time.Since(run.CreatedAt).Hours()/24) + 1
	if runDays < 1 { runDays = 1 }
	cumulativePnl := alloc.CurrentCash - alloc.AllocatedCapital // simplified
	riskAlerts := 0 // TODO: query actual risk alerts
	budget := posSizingEngine.CalculateWithStrategy(tradeDate, alloc.CurrentCash, runDays, cumulativePnl, riskAlerts, strategy)
	addLog("🎚 风控判定: %s | 综合分%.1f | 仓位乘数%.1f", budget.RegimeReason, budget.CompositeScore, budget.PositionBias)

	// ── 日亏损熔断检查 ──
	if budget.DailyLossLimit < 0 {
		dailyLossPct := cumulativePnl / alloc.AllocatedCapital * 100
		if dailyLossPct <= budget.DailyLossLimit {
			addLog("🛑 日亏损熔断触发: 累计亏损%.1f%% ≤ 熔断线%.1f%%，禁止开仓", dailyLossPct, budget.DailyLossLimit)
			budget.TotalPositionPct = 0
			budget.DailyBuyPct = 0
			budget.MaxBuyCash = 0
		}
	}
	addLog("💰 仓位预算: 总≤%.0f%% 单日≤%.0f%% 单票≤%.0f%% 熔断%.0f%% (¥%.0f) — %s", budget.TotalPositionPct, budget.DailyBuyPct, budget.SinglePositionLimit, budget.DailyLossLimit, budget.MaxBuyCash, budget.Reason)

	// ── Run shared SignalEngine ──
	sigEngine := NewSignalEngine()
	maxBuyPct := budget.DailyBuyPct
	scoringCfg := strategy.ScoringConfig
	if len(scoringCfg.Dimensions) == 0 {
		scoringCfg = model.DefaultScoringConfig()
	}
	sigCfg := SignalEngineConfig{
		MaxHoldings:       strategy.MaxHoldings,
		MaxTotalBuyPct:    maxBuyPct,
		BuyPositionPct:    strategy.BuyPositionPct,
		AddPositionPct:    strategy.AddPositionPct,
		ReducePositionPct: strategy.ReducePositionPct,
		StopLoss:              strategy.StopLoss,
		StopProfit:            strategy.StopProfit,
		EnableTrailingStop:    strategy.EnableTrailingStop,
		TrailingStopActivation: strategy.TrailingStopActivation,
		TrailingStopDrawdown:  strategy.TrailingStopDrawdown,
		ScoringConfig:         scoringCfg,
	}
	if sigCfg.BuyPositionPct <= 0 { sigCfg.BuyPositionPct = 10 }
	if sigCfg.MaxHoldings <= 0 { sigCfg.MaxHoldings = 10 }
	addLog("⚙ SignalEngine: 持股上限%d | 单日买入≤%.0f%% | 单票%.0f%% | 移动止盈%v",
		sigCfg.MaxHoldings, sigCfg.MaxTotalBuyPct, sigCfg.BuyPositionPct, sigCfg.EnableTrailingStop)

	dp := NewPGDataProvider(nil, make(map[string]int)) // live mode: single-date, fallback to nextTradeDate

	sharedSignals := sigEngine.GenerateSignals(tradeDate, alloc.CurrentCash, sigPositions, universe,
		engineBuyConds, engineAddConds, engineSellConds, engineReduceConds, dp, sigCfg, &budget, func(format string, args ...interface{}) {
			msg := fmt.Sprintf(format, args...)
			addLog(msg)
			// Update task progress when scanning
			if progressFn != nil && strings.Contains(msg, "扫描进度") {
				var scanned, total int
				fmt.Sscanf(msg, "🔍 扫描进度: %d/%d", &scanned, &total)
				if scanned > 0 {
					progressFn(scanned, 0, 0)
				}
			}
		})

	// ── Convert to BacktestSignal and persist ──
	signalsGenerated := 0
	for _, ss := range sharedSignals {
		// Skip signals for stocks that already have a pending/confirmed signal for same date+action
		if s.signalExists(run.StrategyID, run.ID, ss.StockCode, nextDate, ss.ActionType) {
			if mode != "after_close" {
				addLog("🔄 刷新 %s(%s) — 更新已有待执行信号", ss.StockName, ss.StockCode)
				// Update existing signal reason
				db.MySQL.Model(&model.BacktestSignal{}).Where(
					"strategy_id = ? AND run_id = ? AND stock_code = ? AND exec_date = ? AND action_type = ? AND status IN ?",
					run.StrategyID, run.ID, ss.StockCode, nextDate, ss.ActionType, []string{"pending", "confirmed"},
				).Updates(map[string]interface{}{
					"planned_price":  ss.PlannedPrice,
					"planned_qty":    ss.PlannedQty,
					"planned_amount": ss.PlannedAmount,
					"reason":         ss.Reason,
					"signal_date":    tradeDate,
					"status":         "pending",
				})
			}
			continue
		}

		signal := model.BacktestSignal{
			StrategyID: run.StrategyID, RunID: run.ID, UserID: run.UserID,
			SignalDate: tradeDate, ExecDate: nextDate,
			StockCode: ss.StockCode, StockName: ss.StockName,
			ActionType: ss.ActionType, PlannedPrice: ss.PlannedPrice,
			PlannedQty: ss.PlannedQty, PlannedAmount: ss.PlannedAmount,
			Status: "pending", Reason: ss.Reason,
		}
		if s.upsertSignal(&signal) {
			signalsGenerated++
		}
	}

	addLog("═══ 评估完成: 生成 %d 个信号 ═══", signalsGenerated)
	return signalsGenerated, logs, nil
}

// countActivePositions counts positions with quantity > 0.
func countActivePositions(positions []model.LivePosition) int {
	n := 0
	for _, p := range positions {
		if p.Quantity > 0 { n++ }
	}
	return n
}

// parseStockPool parses comma/space separated stock codes.
func parseStockPool(pool string) []string {
	if pool == "" {
		return nil
	}
	var codes []string
	for _, part := range strings.Split(pool, ",") {
		part = strings.TrimSpace(part)
		if len(part) == 6 {
			codes = append(codes, part)
		}
	}
	return codes
}

// ── Step 4: Portfolio Snapshot ──


// TakeAllDailySnapshots takes snapshot for all active strategy runs (scheduler wrapper).
func (s *LiveTradingService) TakeAllDailySnapshots(tradeDate string) {
	var runs []model.StrategyRun
	if err := db.MySQL.Where("status = ?", "active").Find(&runs).Error; err != nil {
		log.Printf("[live] TakeAllDailySnapshots: query runs error: %v", err)
		return
	}
	for _, run := range runs {
		var alloc model.StrategyFundAllocation
		if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
			continue
		}
		var positions []model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&positions)
		if err := s.snapshotPortfolio(&run, &alloc, &positions, tradeDate); err != nil {
			log.Printf("[live] run %d snapshot failed: %v", run.ID, err)
		}
		s.updateRunStats(run.ID)
	}
}

func (s *LiveTradingService) snapshotPortfolio(
	run *model.StrategyRun, alloc *model.StrategyFundAllocation,
	positions *[]model.LivePosition, tradeDate string,
) error {
	// Build SnapshotPosition map
	snapPositions := make(map[string]SnapshotPosition)
	for _, p := range *positions {
		if p.Quantity <= 0 { continue }
		marketPrice := p.CurrentPrice
		if marketPrice <= 0 {
			db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", p.StockCode).Scan(&marketPrice)
		}
		snapPositions[p.StockCode] = SnapshotPosition{
			Code: p.StockCode, Name: p.StockName,
			Quantity: int(p.Quantity), BuyPrice: p.AvgCost,
			MarketPrice: marketPrice,
			MarketValue: marketPrice * float64(p.Quantity),
			ProfitPct: (marketPrice - p.AvgCost) / p.AvgCost * 100,
		}
	}

	// Use SnapshotEngine for consistent metrics
	snapEng := NewSnapshotEngine(run.InitialCapital)
	snapResult := snapEng.TakeSnapshot(tradeDate, alloc.CurrentCash, snapPositions)

	posJSON, _ := json.Marshal(*positions)

	// Upsert snapshot
	var existing model.DailyPortfolioSnapshot
	err := db.MySQL.Where("strategy_run_id = ? AND snapshot_date = ?", run.ID, tradeDate).
		First(&existing).Error
	if err == nil {
		return db.MySQL.Model(&existing).Updates(map[string]interface{}{
			"cash":              snapResult.Cash,
			"position_value":    snapResult.PositionValue,
			"total_equity":      snapResult.TotalEquity,
			"daily_return_pct":  snapResult.DailyReturn,
			"cumulative_return": snapResult.CumulativeReturn,
			"max_drawdown_pct":  snapResult.MaxDrawdownPct,
			"position_count":    snapResult.PositionCount,
			"positions_json":    string(posJSON),
		}).Error
	}

	snapshot := model.DailyPortfolioSnapshot{
		UserID: run.UserID, StrategyRunID: run.ID, AllocationID: alloc.ID,
		SnapshotDate: tradeDate,
		Cash: snapResult.Cash, PositionValue: snapResult.PositionValue,
		TotalEquity: snapResult.TotalEquity,
		DailyReturnPct: snapResult.DailyReturn,
		CumulativeReturn: snapResult.CumulativeReturn,
		MaxDrawdownPct: snapResult.MaxDrawdownPct,
		PositionCount: snapResult.PositionCount,
		PositionsJSON: string(posJSON),
	}

	return db.MySQL.Create(&snapshot).Error
}

// ── Helpers ──

func (s *LiveTradingService) calcTotalEquity(alloc *model.StrategyFundAllocation, positions *[]model.LivePosition) float64 {
	equity := alloc.CurrentCash
	for _, p := range *positions {
		if p.Quantity > 0 {
			equity += p.CurrentPrice * float64(p.Quantity)
		}
	}
	return equity
}

// updateRunStats recalculates and persists strategy_runs summary fields from current allocations + positions.
func (s *LiveTradingService) updateRunStats(runID uint) {
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ?", runID).First(&run).Error; err != nil {
		return
	}
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", runID, "active").First(&alloc).Error; err != nil {
		return
	}
	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ? AND quantity > 0", runID).Find(&positions)

	equity := s.calcTotalEquity(&alloc, &positions)
	var totalReturn float64
	if run.InitialCapital > 0 {
		totalReturn = (equity - run.InitialCapital) / run.InitialCapital * 100
	}
	var tradeCount int64
	db.MySQL.Model(&model.LiveTrade{}).Where("strategy_run_id = ? AND signal_id IS NOT NULL", runID).Count(&tradeCount)

	db.MySQL.Model(&run).Updates(map[string]interface{}{
		"current_equity": equity,
		"total_return":   math.Round(totalReturn*100) / 100,
		"trade_count":    tradeCount,
	})

	// Also update linked trading account stats
	if run.AccountID > 0 {
		var account model.TradingAccount
		if err := db.MySQL.Where("id = ?", run.AccountID).First(&account).Error; err == nil {
			posValue := 0.0
			for _, p := range positions {
				if p.Quantity > 0 {
					posValue += p.CurrentPrice * float64(p.Quantity)
				}
			}
			totalProfit := equity - account.InitialCapital
			nav := 1.0
			if account.InitialCapital > 0 {
				nav = equity / account.InitialCapital
			}
			db.MySQL.Model(&account).Updates(map[string]interface{}{
				"total_assets":        equity,
				"total_market_value":  math.Round(posValue*100) / 100,
				"total_profit":        math.Round(totalProfit*100) / 100,
				"nav":                 math.Round(nav*10000) / 10000,
			})
		}
	}
}

func (s *LiveTradingService) recordTrade(
	run *model.StrategyRun, alloc *model.StrategyFundAllocation,
	sig *model.BacktestSignal, price float64, qty int, amount, pnl, pnlPct float64, reason string,
) {
	trade := model.LiveTrade{
		UserID: run.UserID, StrategyRunID: run.ID, AllocationID: alloc.ID,
		SignalID: &sig.ID, TradeDate: sig.ExecDate,
		StockCode: sig.StockCode, StockName: sig.StockName,
		ActionType: sig.ActionType, Price: price, Quantity: qty, Amount: amount,
		Reason: reason, ExecutionMode: "auto",
	}
	if pnl != 0 {
		trade.Pnl = math.Round(pnl*100) / 100
		trade.PnlPct = math.Round(pnlPct*100) / 100
	}
	db.MySQL.Create(&trade)
}

func countActive(positions []model.LivePosition) int {
	n := 0
	for _, p := range positions {
		if p.Quantity > 0 {
			n++
		}
	}
	return n
}

func calcStopLoss(strategy *model.Strategy, buyPrice float64) float64 {
	if strategy.StopLoss < 0 {
		return buyPrice * (1 + strategy.StopLoss/100)
	}
	return 0
}

func calcStopProfit(strategy *model.Strategy, buyPrice float64) float64 {
	if strategy.StopProfit > 0 {
		return buyPrice * (1 + strategy.StopProfit/100)
	}
	return 0
}

// ExecuteSignalByID executes a single signal by its ID (legacy, uses planned price).
func (s *LiveTradingService) ExecuteSignalByID(signalID uint, userID uint) error {
	return s.ExecuteSignalByIDWithPrice(signalID, userID, 0, 0)
}

// FinalizeSignalExecution is the unified entry point for completing a trade.
// It takes runID, signalID, actual execution price and quantity, and performs:
//   - signal status update to "executed"
//   - trade record creation
//   - position update (create/add/reduce/close)
//   - fund allocation cash update
//   - holding sync to account
// All order paths (manual, broker sync, lobster auto, API) should call this.
func (s *LiveTradingService) FinalizeSignalExecution(runID uint, signalID uint, execPrice float64, execQty int) error {
	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ?", signalID).First(&sig).Error; err != nil {
		return fmt.Errorf("signal %d not found: %w", signalID, err)
	}

	// Set actual execution values
	if execPrice > 0 {
		sig.PlannedPrice = execPrice
	}
	if execQty > 0 {
		sig.PlannedQty = execQty
		sig.PlannedAmount = execPrice * float64(execQty)
	}

	// Load run by runID directly
	var run model.StrategyRun
	if err := db.MySQL.Where("id = ?", runID).First(&run).Error; err != nil {
		return fmt.Errorf("run %d not found: %w", runID, err)
	}

	// Verify signal belongs to this run
	if sig.RunID != runID {
		return fmt.Errorf("signal %d does not belong to run %d (sig.RunID=%d)", signalID, runID, sig.RunID)
	}

	var strategy model.Strategy
	if err := db.MySQL.First(&strategy, run.StrategyID).Error; err != nil {
		return fmt.Errorf("strategy %d not found: %w", run.StrategyID, err)
	}

	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
		return fmt.Errorf("allocation for run %d not found: %w", runID, err)
	}

	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ?", run.ID).Find(&positions)

	if err := s.executeSignal(&run, &strategy, &alloc, &positions, &sig, sig.ExecDate); err != nil {
		return err
	}

	// Step 6: Write back strategy_runs statistics
	s.updateRunStats(run.ID)

	return nil
}

// ExecuteSignalByIDWithPrice executes a signal with actual trade price/qty.
// Delegates to FinalizeSignalExecution for unified trade completion logic.
func (s *LiveTradingService) ExecuteSignalByIDWithPrice(signalID uint, userID uint, actualPrice float64, actualQty int) error {
	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", signalID, userID).First(&sig).Error; err != nil {
		return fmt.Errorf("signal not found: %w", err)
	}
	return s.FinalizeSignalExecution(sig.RunID, sig.ID, actualPrice, actualQty)
}

// syncHoldingToAccount keeps the holdings table in sync with live_positions.
func (s *LiveTradingService) syncHoldingToAccount(run *model.StrategyRun, stockCode, stockName, tradeDate string, quantity int, avgCost float64, todayBuyQty int, currentPrice float64) {
	// Find the account linked to this run's allocation, then get the account owner (user_id)
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
		return
	}
	var account model.TradingAccount
	ownerUserID := run.UserID // fallback
	if err := db.MySQL.First(&account, alloc.AccountID).Error; err == nil {
		ownerUserID = account.UserID
	}

	var holding model.Holding
	err := db.MySQL.Where("user_id = ? AND account_id = ? AND stock_code = ?", ownerUserID, alloc.AccountID, stockCode).First(&holding).Error

	if quantity <= 0 {
		// Position closed → delete holding
		if err == nil {
			db.MySQL.Delete(&holding)
		}
		return
	}

	if err == nil {
		// Update existing
		db.MySQL.Model(&holding).Updates(map[string]interface{}{
			"quantity":       quantity,
			"cost_price":     avgCost,
			"total_cost":     avgCost * float64(quantity),
			"buy_date":       tradeDate,
			"today_buy_qty":  todayBuyQty,
			"avail_sell_qty": quantity - todayBuyQty,
			"stock_name":     stockName,
			"current_price":  currentPrice,
		})
	} else {
		// Create new
		newH := model.Holding{
			UserID: ownerUserID, AccountID: alloc.AccountID,
			StockCode: stockCode, StockName: stockName,
			CostPrice: avgCost, Quantity: quantity,
			TodayBuyQty: todayBuyQty, AvailSellQty: quantity - todayBuyQty,
			CurrentPrice: currentPrice,
			TotalCost: avgCost * float64(quantity), BuyDate: tradeDate,
		}
		db.MySQL.Create(&newH)
	}
}

// 
// upsertSignal updates an existing pending signal, or creates a new one.
// Returns true if a new signal was created.
func (s *LiveTradingService) upsertSignal(sig *model.BacktestSignal) bool {
	var existing model.BacktestSignal
	err := db.MySQL.Where("strategy_id = ? AND run_id = ? AND stock_code = ? AND exec_date = ? AND action_type = ? AND status IN ?",
		sig.StrategyID, sig.RunID, sig.StockCode, sig.ExecDate, sig.ActionType,
		[]string{"pending", "confirmed", "pending_order", "pending_manual", "pending_auto", "partial_filled"}).
		First(&existing).Error
	if err == nil {
		// Update existing signal — only reset status if currently pending/confirmed
		updates := map[string]interface{}{
			"planned_price":  sig.PlannedPrice,
			"planned_qty":    sig.PlannedQty,
			"planned_amount": sig.PlannedAmount,
			"reason":         sig.Reason,
			"signal_date":    sig.SignalDate,
		}
		if existing.Status == "pending" || existing.Status == "confirmed" {
			updates["status"] = "pending"
		}
		// Keep pending_order/pending_manual/partial_filled status unchanged
		db.MySQL.Model(&existing).Updates(updates)
		return false
	}
	db.MySQL.Create(sig)
	return true
}

// signalExists checks if a pending/confirmed signal already exists for the same stock+date.
func (s *LiveTradingService) signalExists(strategyID, runID uint, stockCode, execDate, actionType string) bool {
	var count int64
	db.MySQL.Model(&model.BacktestSignal{}).
		Where("strategy_id = ? AND run_id = ? AND stock_code = ? AND exec_date = ? AND action_type = ? AND status IN ?",
			strategyID, runID, stockCode, execDate, actionType,
			[]string{"pending", "confirmed", "pending_order", "pending_manual", "pending_auto", "partial_filled", "skipped", "rejected"}).
		Count(&count)
	return count > 0
}

func filterByType(conds []model.StrategyCondition, condType string) []model.StrategyCondition {
	var result []model.StrategyCondition
	for _, c := range conds {
		if c.CondType == condType {
			result = append(result, c)
		}
	}
	return result
}

func previousTradeDate(date string) string {
	t, _ := time.Parse("2006-01-02", date)
	prev := t.AddDate(0, 0, -1)
	// Skip weekends (Saturday=6, Sunday=0)
	for prev.Weekday() == time.Saturday || prev.Weekday() == time.Sunday {
		prev = prev.AddDate(0, 0, -1)
	}
	return prev.Format("2006-01-02")
}

func nextTradeDate(date string) string {
	t, _ := time.Parse("2006-01-02", date)
	next := t.AddDate(0, 0, 1)
	// Skip weekends (Saturday=6, Sunday=0)
	for next.Weekday() == time.Saturday || next.Weekday() == time.Sunday {
		next = next.AddDate(0, 0, 1)
	}
	return next.Format("2006-01-02")
}

// RefreshLivePositions updates current_price and unrealized_pnl for all active positions,
// then writes back strategy_runs statistics. Called by scheduler during market hours.
func (s *LiveTradingService) RefreshLivePositions() error {
	var runs []model.StrategyRun
	if err := db.MySQL.Where("status = ?", "active").Find(&runs).Error; err != nil {
		return fmt.Errorf("query runs: %w", err)
	}

	for _, run := range runs {
		var alloc model.StrategyFundAllocation
		if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
			continue
		}

		var positions []model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND quantity > 0", run.ID).Find(&positions)

		for j := range positions {
			p := &positions[j]
			var latestPrice float64
			db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", p.StockCode).Scan(&latestPrice)
			if latestPrice > 0 {
				p.CurrentPrice = latestPrice
				p.UnrealizedPnl = math.Round((latestPrice-p.AvgCost)*float64(p.Quantity)*100) / 100
				if p.AvgCost > 0 {
					p.UnrealizedPnlPct = math.Round((latestPrice-p.AvgCost)/p.AvgCost*10000) / 100
				}
				if p.FirstBuyDate != "" {
					buyDate, _ := time.Parse("2006-01-02", p.FirstBuyDate)
					p.HoldDays = int(time.Since(buyDate).Hours()/24) + 1
				}
				db.MySQL.Save(p)
			}
		}

		s.updateRunStats(run.ID)
		s.snapshotPortfolio(&run, &alloc, &positions, time.Now().Format("2006-01-02"))
	}

	return nil
}

// ResetDailyBuyLock moves today_buy_qty into avail_sell_qty at start of new trading day.
// All shares bought yesterday become available to sell today.
func (s *LiveTradingService) ResetDailyBuyLock() error {
	return db.MySQL.Exec("UPDATE live_positions SET avail_sell_qty = quantity, today_buy_qty = 0 WHERE today_buy_qty > 0").Error
}

// ImportBrokerPositionsToRun imports existing broker positions into a new strategy run.
// Unlike ReconcileFromBroker, this does NOT delete existing positions first — it only adds
// positions that don't already exist in the run.
func (s *LiveTradingService) ImportBrokerPositionsToRun(strategyRunID uint, accountID uint, userID uint) error {
	bs := NewBrokerService()
	portfolio, err := bs.SyncPositionsFromBroker(accountID, userID)
	if err != nil {
		return fmt.Errorf("sync broker positions: %w", err)
	}

	// Get balance for cash + total equity update
	balance, err := bs.GetBrokerBalance(accountID, userID)
	if err != nil {
		return fmt.Errorf("get broker balance: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	for _, bp := range portfolio.Positions {
		// Skip if this position already exists for this run
		var existing model.LivePosition
		if err := db.MySQL.Where("strategy_run_id = ? AND stock_code = ?", strategyRunID, bp.SecCode).First(&existing).Error; err == nil {
			continue
		}

		pos := model.LivePosition{
			UserID:        userID,
			StrategyRunID: strategyRunID,
			StockCode:     bp.SecCode,
			StockName:     bp.SecName,
			Quantity:      bp.Count,
			AvailSellQty:  bp.AvailCount,
			TodayBuyQty:   bp.Count - bp.AvailCount,
			AvgCost:       bp.CostPrice,
			CurrentPrice:  bp.Price,
			UnrealizedPnl: math.Round((bp.Price-bp.CostPrice)*float64(bp.Count)*100) / 100,
		}
		if bp.CostPrice > 0 {
			pos.UnrealizedPnlPct = math.Round((bp.Price-bp.CostPrice)/bp.CostPrice*10000) / 100
		}
		pos.FirstBuyDate = today
		pos.LastTradeDate = today
		db.MySQL.Create(&pos)

		// Sync holdings
		s.syncHoldingToAccountFromReconcile(accountID, bp.SecCode, bp.SecName, bp.Count, bp.CostPrice, bp.Count-bp.AvailCount, bp.Price, today)
	}

	// Update fund allocation with actual cash and equity from broker
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", strategyRunID, "active").First(&alloc).Error; err == nil {
		db.MySQL.Model(&alloc).Updates(map[string]interface{}{
			"current_cash":     balance.AvailBalance,
			"allocated_capital": portfolio.TotalAssets,
		})
	}

	// Update strategy_runs with actual portfolio state
	var run model.StrategyRun
	if err := db.MySQL.First(&run, strategyRunID).Error; err == nil {
		db.MySQL.Model(&run).Updates(map[string]interface{}{
			"current_equity": portfolio.TotalAssets,
			"initial_capital": portfolio.TotalAssets,
		})
	}

	log.Printf("[import] run=%d account=%d: imported %d positions, equity=%.2f cash=%.2f",
		strategyRunID, accountID, portfolio.PosCount, portfolio.TotalAssets, balance.AvailBalance)
	return nil
}

// ReconcileFromBroker rebuilds live_positions, holdings, and strategy_runs
// from the broker's actual portfolio snapshot. Used as a data repair mechanism
// when local records diverge from broker state.
func (s *LiveTradingService) ReconcileFromBroker(accountID uint, userID uint, strategyRunID uint) error {
	bs := NewBrokerService()
	portfolio, err := bs.SyncPositionsFromBroker(accountID, userID)
	if err != nil {
		return fmt.Errorf("sync broker positions: %w", err)
	}

	// Get balance for cash update
	balance, err := bs.GetBrokerBalance(accountID, userID)
	if err != nil {
		return fmt.Errorf("get broker balance: %w", err)
	}

	today := time.Now().Format("2006-01-02")

	// Step 1: Upsert live_positions from broker (preserve strategy cost basis)
	brokerCodes := make(map[string]bool)
	for _, bp := range portfolio.Positions {
		brokerCodes[bp.SecCode] = true

		var existing model.LivePosition
		err := db.MySQL.Where("strategy_run_id = ? AND stock_code = ?", strategyRunID, bp.SecCode).First(&existing).Error
		if err == nil {
			// Strategy already tracks this → refresh quantity/price, KEEP strategy cost
			db.MySQL.Model(&existing).Updates(map[string]interface{}{
				"current_price":   bp.Price,
				"unrealized_pnl":  math.Round((bp.Price-existing.AvgCost)*float64(bp.Count)*100) / 100,
				"quantity":        bp.Count,
				"avail_sell_qty":  bp.AvailCount,
				"last_trade_date": today,
				"updated_at":      time.Now(),
			})
			if existing.AvgCost > 0 {
				unrealizedPnlPct := math.Round((bp.Price-existing.AvgCost)/existing.AvgCost*10000) / 100
				db.MySQL.Model(&existing).Update("unrealized_pnl_pct", unrealizedPnlPct)
			}
		} else {
			// New position from broker — strategy doesn't track it (manual trade)
			pos := model.LivePosition{
				UserID:        userID,
				StrategyRunID: strategyRunID,
				StockCode:     bp.SecCode,
				StockName:     bp.SecName,
				Quantity:      bp.Count,
				AvailSellQty:  bp.AvailCount,
				TodayBuyQty:   bp.Count - bp.AvailCount,
				AvgCost:       bp.CostPrice,
				CurrentPrice:  bp.Price,
				UnrealizedPnl: math.Round((bp.Price-bp.CostPrice)*float64(bp.Count)*100) / 100,
				ActionPlan:    "manual_import",
			}
			if bp.CostPrice > 0 {
				pos.UnrealizedPnlPct = math.Round((bp.Price-bp.CostPrice)/bp.CostPrice*10000) / 100
			}
			pos.FirstBuyDate = today
			pos.LastTradeDate = today
			db.MySQL.Create(&pos)
		}

		// Sync holdings
		s.syncHoldingToAccountFromReconcile(accountID, bp.SecCode, bp.SecName, bp.Count, bp.CostPrice, bp.Count-bp.AvailCount, bp.Price, today)
	}

	// Mark strategy positions that broker no longer has (strategy sold, broker synced)
	db.MySQL.Model(&model.LivePosition{}).
		Where("strategy_run_id = ? AND quantity > 0 AND stock_code NOT IN ?", strategyRunID, maps.Keys(brokerCodes)).
		Update("action_plan", "broker_cleared")

	// Step 2: Update fund allocation cash
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", strategyRunID, "active").First(&alloc).Error; err == nil {
		db.MySQL.Model(&alloc).Update("current_cash", balance.AvailBalance)
	}

	// Step 3: Update strategy_runs
	s.updateRunStats(strategyRunID)

	log.Printf("[reconcile] account=%d run=%d: synced %d positions, cash=%.2f, total=%.2f",
		accountID, strategyRunID, portfolio.PosCount, balance.AvailBalance, portfolio.TotalAssets)
	return nil
}

// syncHoldingToAccountFromReconcile is a variant that takes accountID directly.
func (s *LiveTradingService) syncHoldingToAccountFromReconcile(accountID uint, stockCode, stockName string, quantity int, costPrice float64, todayBuyQty int, currentPrice float64, tradeDate string) {
	if db.MySQL == nil {
		return
	}
	totalCost := costPrice * float64(quantity)
	availQty := quantity - todayBuyQty
	if availQty < 0 { availQty = 0 }
	db.MySQL.Exec("INSERT INTO holdings (user_id, account_id, stock_code, stock_name, cost_price, quantity, today_buy_qty, avail_sell_qty, current_price, total_cost, buy_date, created_at, updated_at) "+
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) "+
		"ON DUPLICATE KEY UPDATE stock_name = VALUES(stock_name), cost_price = VALUES(cost_price), quantity = VALUES(quantity), today_buy_qty = VALUES(today_buy_qty), avail_sell_qty = VALUES(avail_sell_qty), current_price = VALUES(current_price), total_cost = VALUES(total_cost), updated_at = NOW()",
		0, accountID, stockCode, stockName, costPrice, quantity, todayBuyQty, availQty, currentPrice, totalCost, tradeDate, time.Now(), time.Now())
}

