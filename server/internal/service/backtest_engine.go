package service

import (
	"fmt"
	"math"

	"github.com/ai-stock-predict/server/internal/model"
)

// BacktestEngine executes a complete backtest simulation with the given configuration
// and data providers. Pure business logic — no direct database access.
// DB persistence is handled through the Persister interface.
type BacktestEngine struct {
	positionSvc    *PositionService
	signalSvc      *SignalService
	scoringSvc     *ScoringService
	posSizingEngine *PositionSizingEngine
}

// NewBacktestEngine creates a new BacktestEngine.
func NewBacktestEngine() *BacktestEngine {
	return &BacktestEngine{
		positionSvc:    NewPositionService(),
		signalSvc:      NewSignalService(),
		scoringSvc:     NewScoringService(),
		posSizingEngine: NewPositionSizingEngine(),
	}
}

// ── Configuration ──

// BacktestConfig holds all parameters for a backtest run.
type BacktestConfig struct {
	InitialCapital       float64
	MaxHoldings          int
	BuyPositionPct       float64
	AddPositionPct       float64
	ReducePositionPct    float64
	StopProfit           float64
	StopLoss             float64
	CommissionRate       float64 // default 0.00025
	MinCommission        float64 // default 5.0
	StampTaxRate         float64 // default 0.0005
	EnableDipBuy         bool
	DipBuyThreshold      float64
	DipBuyAmountPct      float64
	DipTargetReturn      float64
	DipMaxHoldDays       int
	SlippagePct          float64 // 滑点百分比, e.g. 0.1 = 0.1%
	DipCooldownDays      int
	EnableGrid           bool
	GridLevels           int
	GridLotPct           float64
	GridTriggerSqueeze   float64
	InvestmentType       string
	RegularAmount        float64
	RegularInterval      string
	EnableDynamicSizing  bool
	MaxTotalPosition    float64
	DailyBuyLimit       float64
	PositionConcentrationLimit float64
	AggressiveThreshold  float64
	DefensiveThreshold   float64
	MarketCompositeMin   float64
	MarketPositionBias   float64
	ScoringConfig        model.ScoringConfig
}

// DefaultBacktestConfig returns sensible A-share market defaults.
func DefaultBacktestConfig() BacktestConfig {
	return BacktestConfig{
		InitialCapital:   100000,
		MaxHoldings:      20,
		BuyPositionPct:   15,
		AddPositionPct:   10,
		ReducePositionPct: 50,
		StopProfit:       0,
		StopLoss:         0,
		CommissionRate:   0.00025,
		MinCommission:    5.0,
		StampTaxRate:     0.0005,
	}
}

// ── Data Provider Interface ──

// BacktestDataProvider is an alias of DataProvider for backward compatibility.
type BacktestDataProvider = DataProvider

// ── Persister Interface ──

// BacktestPersister abstracts DB persistence during backtest.
type BacktestPersister interface {
	SaveSignal(sig *BacktestSignalRecord) error
	SaveSnapshot(snap *BacktestSnapshot) error
	SaveLog(entry *BacktestLogEntry) error
	UpdateProgress(day, total int, phase string)
	IsCancelled() bool
}

// ── Signal / Trade Types ──

// BacktestSignalRecord is a pending signal to be executed at T+1.
type BacktestSignalRecord struct {
	SignalDate    string
	ExecDate      string
	StockCode     string
	StockName     string
	ActionType    string // buy / add / sell / stop / reduce / dip_buy / dip_sell / grid_buy / grid_sell
	PlannedPrice  float64
	PlannedQty    int
	PlannedAmount float64
	ExecPrice     float64
	ExecQty       int
	ExecAmount    float64
	Status        string // pending / executed / skipped
	SkipReason    string
	Reason        string
	Pnl           float64
	PnlPct        float64
}

// BacktestSnapshot is a daily portfolio snapshot.
type BacktestSnapshot struct {
	Date             string
	DayIndex         int
	Cash             float64
	TotalEquity      float64
	DailyReturn      float64
	CumulativeReturn float64
	MaxDrawdown      float64
	PositionCount    int
	Positions        []PositionSnapshot
}

// PositionSnapshot is a single position in a daily snapshot.
type PositionSnapshot struct {
	Code        string
	Name        string
	Quantity    int
	BuyPrice    float64
	MarketPrice float64
	MarketValue float64
	ProfitPct   float64
}

// BacktestLogEntry is a single log entry during backtest.
type BacktestLogEntry struct {
	Date      string
	Seq       int
	LogType   string // system / signal / trade
	Level     string // info / warn
	StockCode string
	StockName string
	Message   string
	Detail    interface{}
}

// ── Main Execution ──

// BacktestResult contains all results from a completed backtest.
type BacktestResult struct {
	FinalEquity      float64
	TotalReturn      float64
	TotalReturnPct   float64
	SharpeRatio      float64
	MaxDrawdown      float64
	MaxDrawdownPct   float64
	WinRate          float64
	TradeCount       int
	Trades           []BacktestTrade
	EquityCurve      []EquityPoint
	DailyReturns     []float64
}

// Run executes a complete backtest simulation.
func (e *BacktestEngine) Run(
	cfg BacktestConfig,
	universe []StockInfo,
	buyConds, addConds, sellConds, reduceConds []ConditionDef,
	data BacktestDataProvider,
	persister BacktestPersister,
) (*BacktestResult, error) {
	// Apply defaults
	if cfg.InitialCapital <= 0 {
		cfg.InitialCapital = 100000
	}
	if cfg.MaxHoldings <= 0 {
		cfg.MaxHoldings = 20
	}
	if cfg.CommissionRate <= 0 {
		cfg.CommissionRate = 0.00025
	}
	if cfg.MinCommission <= 0 {
		cfg.MinCommission = 5.0
	}
	if cfg.StampTaxRate <= 0 {
		cfg.StampTaxRate = 0.0005
	}

	allDates := data.Dates()
	if len(allDates) == 0 {
		return nil, fmt.Errorf("no trading dates available")
	}

	capital := cfg.InitialCapital
	remainingCash := capital
	positions := make(map[string]*Position)
	var allTrades []BacktestTrade
	var equityCurve []EquityPoint
	var dailyReturns []float64
	prevDayEquity := capital
	var pendingSignals []BacktestSignalRecord

	for di, date := range allDates {
		isLastDay := di == len(allDates)-1

		// Check cancellation
		if persister.IsCancelled() {
			return nil, fmt.Errorf("cancelled")
		}

		// ── Phase 1: Execute pending signals (T+1 open) ──
		logSeq := 100
		for i := range pendingSignals {
			sig := &pendingSignals[i]
			if sig.ExecDate != date || sig.Status != "pending" {
				continue
			}

			// Last day: skip buy/add signals
			if isLastDay && (sig.ActionType == "buy" || sig.ActionType == "add") {
				sig.Status = "skipped"
				sig.SkipReason = "最后交易日跳过买入"
				persister.SaveSignal(sig)
				persister.SaveLog(&BacktestLogEntry{
					Date: date, Seq: logSeq, LogType: "signal", Level: "warn",
					StockCode: sig.StockCode, StockName: sig.StockName,
					Message: fmt.Sprintf("信号跳过: %s %s (最后交易日)", sig.ActionType, sig.StockCode),
				})
				logSeq++
				continue
			}

			openPrice := data.GetOpen(sig.StockCode, date)
			trade := e.executeSignal(sig, openPrice, &remainingCash, positions, cfg)

			persister.SaveSignal(sig)

			if trade != nil {
				allTrades = append(allTrades, *trade)
				persister.SaveLog(&BacktestLogEntry{
					Date: date, Seq: logSeq, LogType: "trade", Level: "info",
					StockCode: sig.StockCode, StockName: sig.StockName,
					Message: fmt.Sprintf("[%s] %s %s %d股 @¥%.2f %s",
						trade.Action, trade.Code, trade.Name, trade.Quantity, trade.Price, trade.Reason),
					Detail: map[string]interface{}{
						"action": trade.Action, "price": trade.Price,
						"quantity": trade.Quantity, "pnl": trade.Pnl, "pnlPct": trade.PnlPct,
					},
				})
				logSeq++
			} else if sig.Status == "skipped" {
				persister.SaveLog(&BacktestLogEntry{
					Date: date, Seq: logSeq, LogType: "signal", Level: "warn",
					StockCode: sig.StockCode, StockName: sig.StockName,
					Message: fmt.Sprintf("信号跳过: %s %s — %s", sig.ActionType, sig.StockCode, sig.SkipReason),
				})
				logSeq++
			}
		}

		// Clean executed/skipped signals
		pendingSignals = e.cleanPending(pendingSignals)

		persister.UpdateProgress(di+1, len(allDates),
			fmt.Sprintf("生成信号: 第%d天 %s (扫描%d只)", di+1, date, len(universe)))

		// ── Phase 2: Last day force liquidate ──
		if isLastDay {
			e.forceLiquidate(date, positions, data, &remainingCash, cfg, &allTrades)
		}

		// ── Phase 3: Generate new signals ──
		if !isLastDay {
			newSignals := e.generateSignals(date, remainingCash, positions,
				universe, buyConds, addConds, sellConds, reduceConds,
				data, cfg)
			pendingSignals = append(pendingSignals, newSignals...)
		}

		// ── Phase 4: Daily snapshot ──
		totalEquity := e.positionSvc.CalculatePortfolioValue(remainingCash, positions,
			func(code string) float64 { return data.GetClose(code, date) })

		dailyRet := 0.0
		if prevDayEquity > 0 {
			dailyRet = (totalEquity - prevDayEquity) / prevDayEquity
		}
		dailyReturns = append(dailyReturns, dailyRet)

		cumRet := 0.0
		if capital > 0 {
			cumRet = (totalEquity - capital) / capital * 100
		}

		posSnapshots := e.buildPositionSnapshots(positions, data, date)
		persister.SaveSnapshot(&BacktestSnapshot{
			Date: date, DayIndex: di + 1,
			Cash: math.Round(remainingCash*100) / 100,
			TotalEquity: math.Round(totalEquity*100) / 100,
			DailyReturn: math.Round(dailyRet*10000) / 100,
			CumulativeReturn: math.Round(cumRet*100) / 100,
			PositionCount: len(positions),
			Positions: posSnapshots,
		})

		equityCurve = append(equityCurve, EquityPoint{
			Date: date, Equity: totalEquity, Return: cumRet,
		})

		prevDayEquity = totalEquity
	}

	// ── Calculate final metrics ──
	return e.calculateResult(capital, allTrades, dailyReturns, equityCurve), nil
}

// ConditionDef is a simplified condition definition for the engine.
type ConditionDef struct {
	Indicator string
	Operator  string
	Value     float64
	LogicGroup int
}

// ── Signal Generation ──

func (e *BacktestEngine) generateSignals(
	date string,
	cash float64,
	positions map[string]*Position,
	universe []StockInfo,
	buyConds, addConds, sellConds, reduceConds []ConditionDef,
	data BacktestDataProvider,
	cfg BacktestConfig,
) []BacktestSignalRecord {
	// Delegate to shared SignalEngine
	sigEngine := NewSignalEngine()

	// Convert positions map
	sigPositions := make(map[string]*SignalPosition)
	for _, pos := range positions {
		sigPositions[pos.Code] = &SignalPosition{
			Code: pos.Code, Name: pos.Name,
			Quantity: pos.Quantity, BuyPrice: pos.BuyPrice, BuyDate: pos.BuyDate,
		}
	}

	// Convert config
	scoringCfg := cfg.ScoringConfig
	if len(scoringCfg.Dimensions) == 0 {
		scoringCfg = model.DefaultScoringConfig()
	}

	// ── Position Budget for backtest ──
	var budget *PositionBudget
	if cfg.EnableDynamicSizing {
		b := e.posSizingEngine.Calculate(date, cash, 30, 0, 0)
		// Apply strategy overrides
		if cfg.MaxTotalPosition > 0 && cfg.MaxTotalPosition < b.TotalPositionPct {
			b.TotalPositionPct = cfg.MaxTotalPosition
		}
		if cfg.DailyBuyLimit > 0 && cfg.DailyBuyLimit < b.DailyBuyPct {
			b.DailyBuyPct = cfg.DailyBuyLimit
		}
		if cfg.PositionConcentrationLimit > 0 {
			b.SinglePositionLimit = cfg.PositionConcentrationLimit * 100
		}
		b.MaxBuyCash = cash * b.DailyBuyPct / 100
		budget = &b
	} else {
		// Static budget: use configured limits or defaults
		dailyBuyPct := cfg.BuyPositionPct * float64(cfg.MaxHoldings)
		if dailyBuyPct > 100 { dailyBuyPct = 100 }
		if dailyBuyPct <= 0 { dailyBuyPct = 60 }
		budget = &PositionBudget{
			TotalPositionPct: 100,
			DailyBuyPct:      dailyBuyPct,
			MaxBuyCash:       cash * dailyBuyPct / 100,
			SinglePositionLimit: cfg.BuyPositionPct,
			Reason:           "回测静态预算",
		}
	}

	maxTotalBuyPct := budget.DailyBuyPct
	sigCfg := SignalEngineConfig{
		MaxHoldings:        cfg.MaxHoldings,
		MaxTotalBuyPct:     maxTotalBuyPct,
		BuyPositionPct:     cfg.BuyPositionPct,
		AddPositionPct:     cfg.AddPositionPct,
		ReducePositionPct:  cfg.ReducePositionPct,
		StopLoss:           cfg.StopLoss,
		StopProfit:         cfg.StopProfit,
		CommissionRate:     cfg.CommissionRate,
		MinCommission:      cfg.MinCommission,
		StampTaxRate:       cfg.StampTaxRate,
		ScoringConfig:      scoringCfg,
	}

	// Convert DataProvider — BacktestDataProvider satisfies DataProvider interface
	var dp DataProvider = data

	sharedSignals := sigEngine.GenerateSignals(date, cash, sigPositions, universe, buyConds, addConds, sellConds, reduceConds, dp, sigCfg, budget, nil)

	// Convert back
	result := make([]BacktestSignalRecord, len(sharedSignals))
	for i, s := range sharedSignals {
		result[i] = BacktestSignalRecord{
			SignalDate: s.SignalDate, ExecDate: s.ExecDate,
			StockCode: s.StockCode, StockName: s.StockName,
			ActionType: s.ActionType, PlannedPrice: s.PlannedPrice,
			PlannedQty: s.PlannedQty, PlannedAmount: s.PlannedAmount,
			Status: s.Status, Reason: s.Reason,
		}
	}
	return result
}

// ── Signal Execution ──

func (e *BacktestEngine) executeSignal(
	sig *BacktestSignalRecord,
	openPrice float64,
	cash *float64,
	positions map[string]*Position,
	cfg BacktestConfig,
) *BacktestTrade {
	execEng := NewExecutionEngine()

	// Get existing position info
	var existingQty int
	var existingAvgCost float64
	var buyDate string
	var dailyChg float64
	if pos, ok := positions[sig.StockCode]; ok {
		existingQty = pos.Quantity
		existingAvgCost = pos.BuyPrice
		buyDate = pos.BuyDate
	}

	execCfg := ExecutionConfig{
		CommissionRate: cfg.CommissionRate,
		MinCommission:  cfg.MinCommission,
		StampTaxRate:   cfg.StampTaxRate,
		MaxHoldings:    cfg.MaxHoldings,
		SlippagePct:    cfg.SlippagePct,
	}

	result := execEng.Execute(
		sig.ActionType, sig.StockCode, sig.StockName,
		buyDate, sig.ExecDate,
		sig.PlannedQty, sig.PlannedAmount,
		openPrice, dailyChg,
		cash, existingQty, existingAvgCost,
		len(positions), 0, execCfg, // T+1 not applicable in backtest
	)

	if !result.Executed {
		sig.Status = "skipped"
		sig.SkipReason = result.SkipReason
		return nil
	}

	sig.Status = "executed"
	sig.ExecPrice = result.ExecPrice
	sig.ExecQty = result.ExecQty
	sig.ExecAmount = result.ExecAmount
	sig.Pnl = result.Pnl
	sig.PnlPct = result.PnlPct

	// Update positions map
	switch result.ActionType {
	case "buy":
		positions[sig.StockCode] = &Position{
			Code: sig.StockCode, Name: sig.StockName,
			Quantity: result.NewQuantity, BuyPrice: result.NewAvgCost,
			BuyDate: sig.ExecDate,
		}
	case "add":
		if pos, ok := positions[sig.StockCode]; ok {
			pos.Quantity = result.NewQuantity
			pos.BuyPrice = result.NewAvgCost
		}
	case "sell", "stop":
		delete(positions, sig.StockCode)
	case "reduce":
		if pos, ok := positions[sig.StockCode]; ok {
			pos.Quantity = result.NewQuantity
			if pos.Quantity <= 0 {
				delete(positions, sig.StockCode)
			}
		}
	}

	return &BacktestTrade{
		Date: sig.ExecDate, SignalDate: sig.SignalDate,
		Action: result.ActionType, Code: result.StockCode, Name: result.StockName,
		Price: result.ExecPrice, Quantity: result.ExecQty,
		Reason: sig.Reason, Pnl: result.Pnl, PnlPct: result.PnlPct,
	}
}

// ── Helpers ──


func (e *BacktestEngine) forceLiquidate(
	date string,
	positions map[string]*Position,
	data BacktestDataProvider,
	cash *float64,
	cfg BacktestConfig,
	trades *[]BacktestTrade,
) {
	for code, pos := range positions {
		closePrice := data.GetClose(code, date)
		if closePrice <= 0 {
			closePrice = pos.BuyPrice
		}
		if data.GetDailyChange(code, date) <= -9.8 {
			continue // limit-down, skip
		}
		pnl, pnlPct := e.positionSvc.Sell(cash, pos, closePrice,
			cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)
		delete(positions, code)
		*trades = append(*trades, BacktestTrade{
			Date: date, SignalDate: date,
			Action: "sell", Code: code, Name: pos.Name,
			Price: closePrice, Quantity: pos.Quantity,
			Reason: "强制平仓（最后交易日）",
			Pnl: pnl, PnlPct: pnlPct,
		})
	}
}

func (e *BacktestEngine) cleanPending(signals []BacktestSignalRecord) []BacktestSignalRecord {
	active := make([]BacktestSignalRecord, 0, len(signals))
	for _, s := range signals {
		if s.Status == "pending" {
			active = append(active, s)
		}
	}
	return active
}

func (e *BacktestEngine) buildPositionSnapshots(positions map[string]*Position, data BacktestDataProvider, date string) []PositionSnapshot {
	snaps := make([]PositionSnapshot, 0, len(positions))
	for _, pos := range positions {
		price := data.GetClose(pos.Code, date)
		snaps = append(snaps, PositionSnapshot{
			Code: pos.Code, Name: pos.Name,
			Quantity: pos.Quantity, BuyPrice: pos.BuyPrice,
			MarketPrice: price,
			MarketValue: price * float64(pos.Quantity),
			ProfitPct: pos.UnrealizedPnlPct(price),
		})
	}
	return snaps
}

func (e *BacktestEngine) calculateResult(
	initialCapital float64,
	trades []BacktestTrade,
	dailyReturns []float64,
	equityCurve []EquityPoint,
) *BacktestResult {
	svc := NewBacktestService()
	if len(equityCurve) == 0 {
		return &BacktestResult{}
	}

	finalEquity := equityCurve[len(equityCurve)-1].Equity
	perf := svc.CalculatePerformance(initialCapital, finalEquity, trades, dailyReturns, equityCurve, len(equityCurve))

	return &BacktestResult{
		FinalEquity:    perf.FinalEquity,
		TotalReturn:    perf.TotalReturn,
		TotalReturnPct: perf.TotalReturnPct,
		SharpeRatio:    perf.SharpeRatio,
		MaxDrawdown:    perf.MaxDrawdown,
		MaxDrawdownPct: perf.MaxDrawdownPct,
		WinRate:        perf.WinRatePct,
		TradeCount:     perf.TotalTrades,
		Trades:         trades,
		EquityCurve:    equityCurve,
		DailyReturns:   dailyReturns,
	}
}
