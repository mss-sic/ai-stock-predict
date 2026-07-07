package service

import (
	"fmt"
	"math"
)

// ── ExecutionConfig ──

type ExecutionConfig struct {
	CommissionRate  float64 // e.g. 0.00025
	MinCommission   float64 // e.g. 5.0
	StampTaxRate    float64 // e.g. 0.0005 (sell only)
	MaxHoldings     int
	StopLossPrice   float64
	StopProfitPrice float64
	SlippagePct     float64 // 滑点百分比，回测用，实盘=0
}

// ── ExecutionResult ──

type ExecutionResult struct {
	Executed    bool
	ActionType  string
	StockCode   string
	StockName   string
	ExecPrice   float64
	ExecQty     int
	ExecAmount  float64
	Pnl         float64
	PnlPct      float64
	NewAvgCost  float64  // new average cost after add
	NewQuantity int      // new total quantity after add/reduce
	Reason      string
	SkipReason  string
}

// ── ExecutionEngine ──

// ExecutionEngine provides shared signal execution logic for both backtest and live trading.
type ExecutionEngine struct {
	posSvc *PositionService
}

// NewExecutionEngine creates a new execution engine.
func NewExecutionEngine() *ExecutionEngine {
	return &ExecutionEngine{posSvc: NewPositionService()}
}

// Execute executes a single signal against the current portfolio state.
// Returns the execution result. The caller is responsible for persisting state changes.
func (e *ExecutionEngine) Execute(
	sigActionType string,
	stockCode, stockName string,
	buyDate string, // date position was bought (empty if no position)
	execDate string, // date of execution
	plannedQty int,
	plannedAmount float64,
	currentPrice float64,
	dailyChangePct float64, // for limit-down check
	cash *float64,
	existingQty int,
	existingAvgCost float64,
	existingPositionsCount int,
	cfg ExecutionConfig,
) *ExecutionResult {
	r := &ExecutionResult{
		ActionType: sigActionType,
		StockCode:  stockCode,
		StockName:  stockName,
	}

	// Apply slippage for backtest
	if cfg.SlippagePct != 0 {
		if sigActionType == "buy" || sigActionType == "add" {
			currentPrice = currentPrice * (1 + cfg.SlippagePct/100)
		} else {
			currentPrice = currentPrice * (1 - cfg.SlippagePct/100)
		}
	}
	r.ExecPrice = currentPrice

	if currentPrice <= 0 {
		r.SkipReason = "停牌或无价格数据"
		return r
	}

	switch sigActionType {
	case "buy":
		return e.executeBuy(r, stockCode, stockName, execDate, plannedAmount, currentPrice, cash, existingPositionsCount, cfg)

	case "add":
		return e.executeAdd(r, stockCode, execDate, plannedAmount, currentPrice, cash, existingQty, existingAvgCost, cfg)

	case "sell", "stop":
		return e.executeSell(r, stockCode, buyDate, execDate, currentPrice, dailyChangePct, cash, existingQty, existingAvgCost, cfg, sigActionType)

	case "reduce":
		return e.executeReduce(r, stockCode, buyDate, execDate, plannedQty, currentPrice, dailyChangePct, cash, existingQty, existingAvgCost, cfg)

	case "hold":
		r.Executed = true
		r.Reason = "确认持有，无需操作"
		return r

	default:
		r.SkipReason = fmt.Sprintf("未知操作类型: %s", sigActionType)
		return r
	}
}

func (e *ExecutionEngine) executeBuy(
	r *ExecutionResult,
	stockCode, stockName, execDate string,
	plannedAmount, price float64,
	cash *float64,
	existingPositionsCount int,
	cfg ExecutionConfig,
) *ExecutionResult {
	if existingPositionsCount >= cfg.MaxHoldings {
		r.SkipReason = "已达最大持仓数"
		return r
	}

	pos := e.posSvc.Buy(cash, stockCode, stockName, execDate, price, plannedAmount, cfg.CommissionRate, cfg.MinCommission)
	if pos == nil {
		r.SkipReason = "资金不足"
		return r
	}

	r.Executed = true
	r.ExecQty = pos.Quantity
	r.ExecAmount = price * float64(pos.Quantity)
	r.NewAvgCost = price
	r.NewQuantity = pos.Quantity
	r.Reason = "买入成交"
	return r
}

func (e *ExecutionEngine) executeAdd(
	r *ExecutionResult,
	stockCode, execDate string,
	plannedAmount, price float64,
	cash *float64,
	existingQty int, existingAvgCost float64,
	cfg ExecutionConfig,
) *ExecutionResult {
	if existingQty <= 0 {
		r.SkipReason = "无持仓，不可加仓"
		return r
	}

	// Use position service's Add method via a temp position
	pos := &Position{
		Code: stockCode, BuyPrice: existingAvgCost, Quantity: existingQty, BuyDate: execDate,
	}
	addQty := e.posSvc.Add(cash, pos, price, plannedAmount, cfg.CommissionRate, cfg.MinCommission)
	if addQty <= 0 {
		r.SkipReason = "资金不足或数量不足"
		return r
	}

	// Calculate new average cost
	totalCost := existingAvgCost*float64(existingQty) + price*float64(addQty)
	newAvgCost := totalCost / float64(existingQty+addQty)

	r.Executed = true
	r.ExecQty = addQty
	r.ExecAmount = price * float64(addQty)
	r.NewAvgCost = newAvgCost
	r.NewQuantity = existingQty + addQty
	r.Reason = "加仓成交"
	return r
}

func (e *ExecutionEngine) executeSell(
	r *ExecutionResult,
	stockCode, buyDate, execDate string,
	price, dailyChangePct float64,
	cash *float64,
	existingQty int, existingAvgCost float64,
	cfg ExecutionConfig,
	actionType string,
) *ExecutionResult {
	if existingQty <= 0 {
		r.SkipReason = "无持仓"
		return r
	}

	// T+1 check: cannot sell on same day as buy
	if buyDate == execDate {
		r.SkipReason = "T+1限制：当日买入不可卖出"
		return r
	}

	// Limit-down check: cannot sell if limit-down
	if dailyChangePct <= -9.8 {
		r.SkipReason = "跌停无法卖出"
		return r
	}

	// Execute sell
	pos := &Position{
		Code: stockCode, BuyPrice: existingAvgCost, Quantity: existingQty, BuyDate: buyDate,
	}
	pnl, pnlPct := e.posSvc.Sell(cash, pos, price, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)

	r.Executed = true
	r.ExecQty = existingQty
	r.ExecAmount = price * float64(existingQty)
	r.Pnl = math.Round(pnl*100) / 100
	r.PnlPct = math.Round(pnlPct*100) / 100
	r.NewQuantity = 0
	r.Reason = "卖出成交"
	if actionType == "stop" {
		r.Reason = "止损/止盈卖出"
	}
	return r
}

func (e *ExecutionEngine) executeReduce(
	r *ExecutionResult,
	stockCode, buyDate, execDate string,
	plannedQty int,
	price, dailyChangePct float64,
	cash *float64,
	existingQty int, existingAvgCost float64,
	cfg ExecutionConfig,
) *ExecutionResult {
	if existingQty <= 0 {
		r.SkipReason = "无持仓"
		return r
	}

	// T+1 check
	if buyDate == execDate {
		r.SkipReason = "T+1限制：当日买入不可减持"
		return r
	}

	// Limit-down check
	if dailyChangePct <= -9.8 {
		r.SkipReason = "跌停无法卖出"
		return r
	}

	reduceQty := plannedQty
	if reduceQty <= 0 || reduceQty > existingQty {
		reduceQty = existingQty
	}

	// Execute reduce
	pos := &Position{
		Code: stockCode, BuyPrice: existingAvgCost, Quantity: existingQty, BuyDate: buyDate,
	}
	pnl, pnlPct, reduced := e.posSvc.Reduce(cash, pos, price, reduceQty, execDate, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)

	newQty := existingQty - reduced
	r.Executed = true
	r.ExecQty = reduced
	r.ExecAmount = price * float64(reduced)
	r.Pnl = math.Round(pnl*100) / 100
	r.PnlPct = math.Round(pnlPct*100) / 100
	r.NewQuantity = newQty
	r.NewAvgCost = existingAvgCost
	r.Reason = "减仓成交"
	return r
}
