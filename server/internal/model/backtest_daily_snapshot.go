package model

import "time"

// BacktestDailySnapshot records portfolio state at the end of each trading day.
// One row per (task_id, date).  Used to build equity curves and drill into daily positions.
type BacktestDailySnapshot struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	TaskID          uint      `gorm:"index:idx_snap_task;not null" json:"taskId"`
	StrategyID      uint      `gorm:"index" json:"strategyId"`
	UserID          uint      `gorm:"index" json:"userId"`
	Date            string    `gorm:"type:date;index:idx_snap_task;not null" json:"date"`       // YYYY-MM-DD
	DayIndex        int       `gorm:"default:0" json:"dayIndex"`                                // 1-based trading day index
	Cash            float64   `gorm:"type:numeric(16,2)" json:"cash"`                           // remaining cash
	TotalEquity     float64   `gorm:"type:numeric(16,2)" json:"totalEquity"`                    // cash + positions market value
	DailyReturn     float64   `gorm:"type:numeric(10,4)" json:"dailyReturn"`                    // daily return %
	CumulativeReturn float64  `gorm:"type:numeric(10,4)" json:"cumulativeReturn"`               // cumulative return % from start
	PositionCount   int       `gorm:"default:0" json:"positionCount"`                           // number of holdings
	Positions       string    `gorm:"type:json" json:"positions"`                               // JSON array of position objects
	MaxDrawdown     float64   `gorm:"type:numeric(10,4)" json:"maxDrawdown"`                    // peak-to-current drawdown %
	CreatedAt       time.Time `json:"createdAt"`
}

func (BacktestDailySnapshot) TableName() string { return "backtest_daily_snapshots" }
