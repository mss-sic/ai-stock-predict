package service

import (
	"encoding/json"
	"fmt"
	"log"
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
func (s *LiveTradingService) RunDaily(tradeDate string, mode string) (*DailyRunResult, error) {
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

	// 1. Find all active strategy runs
	var runs []model.StrategyRun
	if err := db.MySQL.Where("status = ?", "active").Find(&runs).Error; err != nil {
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
		// Persist logs every 5 entries
		if len(allLogs)%5 == 0 {
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

	// Find all active strategy runs
	var runs []model.StrategyRun
	if err := db.MySQL.Where("status = ?", "active").Find(&runs).Error; err != nil {
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
		if scanned%500 == 0 || candidates > 0 || signals > 0 {
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
		return 0, nil, fmt.Errorf("load strategy %d: %w", run.StrategyID, err)
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

	// Update run's last run date + persist logs as JSON
	logsJSON, _ := json.Marshal(result.Logs)
	db.MySQL.Model(run).Updates(map[string]interface{}{
		"last_run_date": tradeDate,
		"current_equity": s.calcTotalEquity(&alloc, &positions),
		"last_run_log": string(logsJSON),
	})

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

	// Get existing position info
	var existingQty int
	var existingAvgCost float64
	var buyDate string
	for _, p := range *positions {
		if p.StockCode == sig.StockCode && p.Quantity > 0 {
			existingQty = int(p.Quantity)
			existingAvgCost = p.AvgCost
			buyDate = p.FirstBuyDate
			break
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
		len(*positions), execCfg,
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
			Quantity: result.NewQuantity, AvgCost: result.NewAvgCost,
			CurrentPrice: result.ExecPrice,
			FirstBuyDate: tradeDate, LastTradeDate: tradeDate,
			StopLossPrice: calcStopLoss(strategy, result.ExecPrice),
			StopProfitPrice: calcStopProfit(strategy, result.ExecPrice),
		}
		db.MySQL.Create(&livePos)
		*positions = append(*positions, livePos)
		s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, 0, 0, result.Reason)
		s.syncHoldingToAccount(run, sig.StockCode, tradeDate, result.NewQuantity, result.NewAvgCost)

	case "add":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				p.Quantity = result.NewQuantity
				p.AvgCost = result.NewAvgCost
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, 0, 0, result.Reason)
				s.syncHoldingToAccount(run, sig.StockCode, tradeDate, p.Quantity, p.AvgCost)
				break
			}
		}

	case "sell", "stop":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				p.RealizedPnl += result.Pnl
				p.Quantity = 0
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, result.Pnl, result.PnlPct, result.Reason)
				s.syncHoldingToAccount(run, sig.StockCode, tradeDate, 0, 0)
				break
			}
		}

	case "reduce":
		for j := range *positions {
			if (*positions)[j].StockCode == sig.StockCode {
				p := &(*positions)[j]
				p.RealizedPnl += result.Pnl
				p.Quantity = result.NewQuantity
				p.LastTradeDate = tradeDate
				db.MySQL.Save(p)
				s.recordTrade(run, alloc, sig, result.ExecPrice, result.ExecQty, result.ExecAmount, result.Pnl, result.PnlPct, result.Reason)
				if p.Quantity <= 0 {
					s.syncHoldingToAccount(run, sig.StockCode, tradeDate, 0, 0)
				} else {
					s.syncHoldingToAccount(run, sig.StockCode, tradeDate, p.Quantity, p.AvgCost)
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

// ExecuteSignalByIDWithPrice executes a signal with actual trade price/qty.
// If actualPrice=0, uses the signal's planned price.
func (s *LiveTradingService) ExecuteSignalByIDWithPrice(signalID uint, userID uint, actualPrice float64, actualQty int) error {
	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ? AND user_id = ?", signalID, userID).First(&sig).Error; err != nil {
		return fmt.Errorf("signal not found: %w", err)
	}
	// Use actual price/qty if provided, fall back to planned
	if actualPrice > 0 {
		sig.PlannedPrice = actualPrice
	}
	if actualQty > 0 {
		sig.PlannedQty = actualQty
		sig.PlannedAmount = actualPrice * float64(actualQty)
	}
	var run model.StrategyRun
	if err := db.MySQL.Where("strategy_id = ? AND user_id = ? AND status = ?", sig.StrategyID, userID, "active").First(&run).Error; err != nil {
		return fmt.Errorf("active run not found for strategy %d: %w", sig.StrategyID, err)
	}
	var strategy model.Strategy
	if err := db.MySQL.First(&strategy, run.StrategyID).Error; err != nil {
		return fmt.Errorf("strategy not found: %w", err)
	}
	var alloc model.StrategyFundAllocation
	if err := db.MySQL.Where("strategy_run_id = ? AND status = ?", run.ID, "active").First(&alloc).Error; err != nil {
		return fmt.Errorf("allocation not found: %w", err)
	}
	var positions []model.LivePosition
	db.MySQL.Where("strategy_run_id = ?", run.ID).Find(&positions)
	return s.executeSignal(&run, &strategy, &alloc, &positions, &sig, sig.ExecDate)
}

// syncHoldingToAccount keeps the holdings table in sync with live_positions.
func (s *LiveTradingService) syncHoldingToAccount(run *model.StrategyRun, stockCode, tradeDate string, quantity int, avgCost float64) {
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
			"quantity":   quantity,
			"cost_price": avgCost,
			"total_cost": avgCost * float64(quantity),
			"buy_date":   tradeDate,
		})
	} else {
		// Create new
		newH := model.Holding{
			UserID: ownerUserID, AccountID: alloc.AccountID,
			StockCode: stockCode,
			CostPrice: avgCost, Quantity: quantity,
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
		sig.StrategyID, sig.RunID, sig.StockCode, sig.ExecDate, sig.ActionType, []string{"pending", "confirmed"}).
		First(&existing).Error
	if err == nil {
		// Update existing pending signal with fresh data
		db.MySQL.Model(&existing).Updates(map[string]interface{}{
			"planned_price":  sig.PlannedPrice,
			"planned_qty":    sig.PlannedQty,
			"planned_amount": sig.PlannedAmount,
			"reason":         sig.Reason,
			"signal_date":    sig.SignalDate,
			"status":         "pending", // reset to pending
		})
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
			strategyID, runID, stockCode, execDate, actionType, []string{"pending", "confirmed", "skipped", "rejected"}).
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
