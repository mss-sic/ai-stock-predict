package model

import "time"

// BacktestResult stores the final summary of a completed backtest run.
// Linked to backtest_tasks via TaskID.
// Daily detail lives in backtest_daily_snapshots and backtest_execution_logs.
type BacktestResult struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     uint      `gorm:"index;not null" json:"taskId"`
	UserID     uint      `json:"userId"`
	StrategyID uint      `json:"strategyId"`

	// StockPool stores a short enum key, not a display label:
	//   "all" / "watchlist_3" / "portfolio" / "codes"
	// The display label is resolved by resolveStockPoolLabel() at read time.
	StockPool       string `gorm:"column:stock_pool;size:30" json:"stockPool"`
	StockPoolLabel  string `gorm:"-" json:"stockCode"`           // computed at query time, backward compat
	StockPoolParams string `gorm:"type:json" json:"stockPoolParams"` // actual stock codes

	StartDate      time.Time `gorm:"type:date" json:"startDate"`
	EndDate        time.Time `gorm:"type:date" json:"endDate"`
	InitialCapital float64   `gorm:"type:numeric(16,2)" json:"initialCapital"`
	FinalEquity    float64   `gorm:"type:numeric(16,2)" json:"finalEquity"`
	TotalReturn    float64   `gorm:"type:numeric(10,4)" json:"totalReturn"`
	SharpeRatio    float64   `gorm:"type:numeric(8,4)" json:"sharpeRatio"`
	MaxDrawdown    float64   `gorm:"type:numeric(10,4)" json:"maxDrawdown"`
	WinRate        float64   `gorm:"type:numeric(8,4)" json:"winRate"`
	TradeCount     int       `json:"tradeCount"`
	Trades         JSONMap   `gorm:"type:json" json:"trades"`
	EquityCurve    JSONMap   `gorm:"type:json" json:"equityCurve"`
	Coverage       JSONMap   `gorm:"type:json" json:"coverage"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (BacktestResult) TableName() string { return "backtest_results" }
