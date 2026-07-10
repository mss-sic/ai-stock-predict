package model

import "time"

// StrategyRun tracks a live/running strategy execution (future feature).
// Unlike backtest_tasks which are one-shot, a strategy_run is persistent and runs daily.
type StrategyRun struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"index" json:"userId"`
	StrategyID     uint       `gorm:"index" json:"strategyId"`
	Name           string     `gorm:"size:100" json:"name"`               // run label, e.g. "实盘-2026Q2"
	AccountID      uint       `gorm:"index;default:0" json:"accountId"`     // linked trading account
	Status         string     `gorm:"size:20;default:active" json:"status"` // active | paused | stopped | archived
	StockPool      string     `gorm:"size:500" json:"stockPool"`           // stock pool identifier
	StartDate      string     `gorm:"type:varchar(10)" json:"startDate"`
	EndDate        string     `gorm:"type:varchar(10)" json:"endDate"`     // empty = indefinite
	InitialCapital float64    `gorm:"type:numeric(16,2)" json:"initialCapital"`
	AvailableCash  float64    `gorm:"type:numeric(16,2);default:0" json:"availableCash"`
	PositionValue  float64    `gorm:"type:numeric(16,2);default:0" json:"positionValue"`
	CurrentEquity  float64    `gorm:"type:numeric(16,2)" json:"currentEquity"`
	TotalReturn    float64    `gorm:"type:numeric(10,4)" json:"totalReturn"`
	SharpeRatio    float64    `gorm:"type:numeric(8,4)" json:"sharpeRatio"`
	MaxDrawdown    float64    `gorm:"type:numeric(10,4)" json:"maxDrawdown"`
	WinRate        float64    `gorm:"type:numeric(8,4)" json:"winRate"`
	TradeCount     int        `gorm:"default:0" json:"tradeCount"`
	LastRunDate    string     `gorm:"type:varchar(10)" json:"lastRunDate"`
	LastRunLog     string     `gorm:"type:text" json:"lastRunLog"`          // JSON array of last execution log lines
	LastError      string     `gorm:"size:500" json:"lastError"`
	AutoDailyCron      string `gorm:"size:50" json:"autoDailyCron"`          // cron for daily signal gen
	AutoTradeExecCron  string `gorm:"size:50" json:"autoTradeExecCron"`      // cron for trade execution (formerly pre-market)
	ExecutionMode     string `gorm:"size:20;default:manual" json:"executionMode"` // manual / auto / mx / lobster
	AiReviewEnabled   bool   `gorm:"default:false" json:"aiReviewEnabled"`     // enable AI review before trade execution
	NotifyEnabled      bool   `gorm:"default:false" json:"notifyEnabled"`    // enable notifications
	NotifyChannels     string `gorm:"size:500;default:[]" json:"notifyChannels"` // JSON array of channel IDs
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

func (StrategyRun) TableName() string { return "strategy_runs" }
