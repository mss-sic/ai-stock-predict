package model

import "time"

// BacktestSignal represents a trading signal generated during backtest or live strategy runs.
// Signals decouple condition evaluation (T day close) from trade execution (T+1 day open).
type BacktestSignal struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	TaskID     uint   `gorm:"index:idx_sig_task" json:"taskId"`
	StrategyID uint   `json:"strategyId"`
	UserID     uint   `json:"userId"`

	SignalDate string `gorm:"size:10;index" json:"signalDate"` // T day — when signal was generated
	ExecDate   string `gorm:"size:10;index" json:"execDate"`   // T+1 day — when to execute
	StockCode  string `gorm:"size:10" json:"stockCode"`
	StockName  string `gorm:"size:50" json:"stockName"`
	ActionType string `gorm:"size:10" json:"actionType"` // buy / add / sell / reduce / stop

	// Planned values — estimated at T day close
	PlannedPrice  float64 `gorm:"type:numeric(12,4)" json:"plannedPrice"`
	PlannedQty    int     `json:"plannedQty"`
	PlannedAmount float64 `gorm:"type:numeric(16,2)" json:"plannedAmount"`

	// Actual execution values — filled at T+1 open
	ExecPrice  float64 `gorm:"type:numeric(12,4)" json:"execPrice"`
	ExecQty    int     `json:"execQty"`
	ExecAmount float64 `gorm:"type:numeric(16,2)" json:"execAmount"`

	// Result
	Status     string `gorm:"size:10;default:pending" json:"status"` // pending / executed / skipped
	SkipReason string `gorm:"size:200" json:"skipReason"`
	Pnl        float64 `gorm:"type:numeric(16,2)" json:"pnl"`
	PnlPct     float64 `gorm:"type:numeric(10,4)" json:"pnlPct"`

	// Metadata
	Reason string `gorm:"size:500" json:"reason"` // human-readable trigger description

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (BacktestSignal) TableName() string { return "backtest_signals" }
