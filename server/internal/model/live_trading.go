package model

import "time"

// ═══════════════════════════════════════════════════════════════
// 实盘交易层数据模型
// 在现有 TradingAccount + Strategy + StrategyRun + BacktestSignal 基础上
// 补齐：策略资金分配 / 实盘持仓 / 每日执行日志
// ═══════════════════════════════════════════════════════════════

// StrategyFundAllocation allocates a portion of the trading account to a strategy run.
// One account can have multiple allocations (e.g., 50% to strategy A, 30% to B, 20% cash reserve).
type StrategyFundAllocation struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"index" json:"userId"`
	AccountID      uint      `gorm:"index" json:"accountId"`       // FK → trading_accounts.id
	StrategyRunID  uint      `gorm:"index" json:"strategyRunId"`   // FK → strategy_runs.id
	AllocatedCapital float64 `gorm:"type:numeric(16,2)" json:"allocatedCapital"` // 分配给此策略的初始资金
	CurrentCash    float64   `gorm:"type:numeric(16,2)" json:"currentCash"`       // 策略当前可用现金
	PctOfAccount   float64   `gorm:"type:numeric(5,2)" json:"pctOfAccount"`       // 占账户总资金比例 (0-100)
	Status         string    `gorm:"size:20;default:active" json:"status"`        // active / paused / closed
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (StrategyFundAllocation) TableName() string { return "strategy_fund_allocations" }

// LivePosition tracks current actual holdings for a strategy run.
// Differs from backtest Position — persisted to DB, tracks realized P&L.
type LivePosition struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"userId"`
	StrategyRunID uint      `gorm:"index" json:"strategyRunId"`
	AllocationID  uint      `gorm:"index" json:"allocationId"`

	StockCode  string  `gorm:"size:10;index" json:"stockCode"`
	StockName  string  `gorm:"size:50" json:"stockName"`
	Quantity   int     `json:"quantity"`            // 当前持仓股数
	AvgCost    float64 `gorm:"type:numeric(12,4)" json:"avgCost"`  // 加权平均成本
	CurrentPrice float64 `gorm:"type:numeric(12,4)" json:"currentPrice"` // 最新市价

	// P&L
	UnrealizedPnl    float64 `gorm:"type:numeric(16,2)" json:"unrealizedPnl"`
	UnrealizedPnlPct float64 `gorm:"type:numeric(10,4)" json:"unrealizedPnlPct"`
	RealizedPnl      float64 `gorm:"type:numeric(16,2)" json:"realizedPnl"`   // 累计已实现盈亏

	// Tracking
	FirstBuyDate string `gorm:"size:10" json:"firstBuyDate"`     // 首次买入日
	LastTradeDate string `gorm:"size:10" json:"lastTradeDate"`   // 最近交易日期
	HoldDays     int    `json:"holdDays"`                        // 已持天数

	// Strategy context
	StopLossPrice   float64 `gorm:"type:numeric(12,4)" json:"stopLossPrice"`   // 策略止损价
	StopProfitPrice float64 `gorm:"type:numeric(12,4)" json:"stopProfitPrice"` // 策略止盈价
	ActionPlan      string  `gorm:"size:200" json:"actionPlan"`                 // 计划操作 (hold / sell_next / add_next / reduce_next)

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (LivePosition) TableName() string { return "live_positions" }

// LiveTrade records an executed trade in live trading.
// Extends BacktestSignal — links to the signal that triggered it.
type LiveTrade struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"userId"`
	StrategyRunID uint      `gorm:"index" json:"strategyRunId"`
	AllocationID  uint      `json:"allocationId"`
	SignalID      *uint     `json:"signalId"` // FK → backtest_signals.id (the signal that triggered this trade)

	TradeDate  string  `gorm:"size:10;index" json:"tradeDate"`
	StockCode  string  `gorm:"size:10" json:"stockCode"`
	StockName  string  `gorm:"size:50" json:"stockName"`
	ActionType string  `gorm:"size:10" json:"actionType"` // buy / sell / add / reduce / stop

	Price    float64 `gorm:"type:numeric(12,4)" json:"price"`
	Quantity int     `json:"quantity"`
	Amount   float64 `gorm:"type:numeric(16,2)" json:"amount"`

	// Costs
	Commission float64 `gorm:"type:numeric(12,4)" json:"commission"`
	StampTax   float64 `gorm:"type:numeric(12,4)" json:"stampTax"`
	TotalCost  float64 `gorm:"type:numeric(12,4)" json:"totalCost"`

	// P&L (for sell/reduce)
	Pnl    float64 `gorm:"type:numeric(16,2)" json:"pnl"`
	PnlPct float64 `gorm:"type:numeric(10,4)" json:"pnlPct"`

	// Execution context
	Reason       string `gorm:"size:500" json:"reason"`
	ExecutionMode string `gorm:"size:20;default:auto" json:"executionMode"` // auto / manual / ai_override

	CreatedAt time.Time `json:"createdAt"`
}

func (LiveTrade) TableName() string { return "live_trades" }

// DailyPortfolioSnapshot captures end-of-day portfolio state for a strategy run.
type DailyPortfolioSnapshot struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"userId"`
	StrategyRunID uint      `gorm:"index" json:"strategyRunId"`
	AllocationID  uint      `json:"allocationId"`
	SnapshotDate  string    `gorm:"size:10;index" json:"snapshotDate"`

	// Account state at EOD
	Cash            float64 `gorm:"type:numeric(16,2)" json:"cash"`
	PositionValue   float64 `gorm:"type:numeric(16,2)" json:"positionValue"`   // 持仓市值
	TotalEquity     float64 `gorm:"type:numeric(16,2)" json:"totalEquity"`     // cash + positionValue
	DailyReturn     float64 `gorm:"type:numeric(10,6)" json:"dailyReturn"`     // 日收益率 (小数)
	DailyReturnPct  float64 `gorm:"type:numeric(10,4)" json:"dailyReturnPct"`  // 日收益率%
	CumulativeReturn float64 `gorm:"type:numeric(10,4)" json:"cumulativeReturn"` // 累计收益%
	MaxDrawdownPct  float64 `gorm:"type:numeric(10,4)" json:"maxDrawdownPct"`  // 当前最大回撤%

	// Trading summary
	PositionCount   int     `json:"positionCount"`
	BuyCount        int     `json:"buyCount"`
	SellCount       int     `json:"sellCount"`
	DailyPnl        float64 `gorm:"type:numeric(16,2)" json:"dailyPnl"` // 当日已实现盈亏

	// Positions snapshot (JSON)
	PositionsJSON string `gorm:"type:text" json:"positionsJson"` // JSON array of position summary

	CreatedAt time.Time `json:"createdAt"`
}

func (DailyPortfolioSnapshot) TableName() string { return "daily_portfolio_snapshots" }
