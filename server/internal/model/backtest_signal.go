package model

import "time"

// BacktestSignal represents a trading signal generated during backtest or live strategy runs.
// Signals decouple condition evaluation (T day close) from trade execution (T+1 day open).
type BacktestSignal struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	TaskID     uint   `gorm:"index:idx_sig_task" json:"taskId"`
	RunID      uint   `gorm:"index;default:0" json:"runId"`
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

	// Order price suggestions — pre-market decision engine
	SuggestedPremium float64 `gorm:"type:numeric(5,2);default:0" json:"suggestedPremium"` // suggested premium pct (2.5 = 2.5%)
	OrderPrice       float64 `gorm:"type:numeric(12,4);default:0" json:"orderPrice"`       // suggested order price
	OrderPriceLimit  float64 `gorm:"type:numeric(12,4);default:0" json:"orderPriceLimit"`  // upper limit (buy) / lower limit (sell)
	SuggestedQty     int     `gorm:"default:0" json:"suggestedQty"`                       // suggested qty after volatility adjustment
	OriginalQty      int     `gorm:"default:0" json:"originalQty"`                         // original planned qty before adjustment

	// Pre-market context
	OpenPrice     float64 `gorm:"type:numeric(12,4);default:0" json:"openPrice"`     // today's open price
	OpenDeviation float64 `gorm:"type:numeric(6,2);default:0" json:"openDeviation"` // open deviation pct
	DecisionRule  string  `gorm:"size:50" json:"decisionRule"`                      // decision matrix rule that fired

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
