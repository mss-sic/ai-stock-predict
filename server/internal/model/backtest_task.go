package model

import "time"

// BacktestTask tracks async backtest execution state
type BacktestTask struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"userId"`
	StrategyID  uint      `gorm:"index" json:"strategyId"`
	Status      string    `gorm:"size:20;default:pending" json:"status"` // pending / running / completed / failed / cancelled
	Phase       string    `gorm:"size:100" json:"phase"`                 // human-readable current phase
	CurrentDay  int       `gorm:"default:0" json:"currentDay"`
	TotalDays   int       `gorm:"default:0" json:"totalDays"`
	ErrorMsg    string    `gorm:"size:500" json:"errorMsg"`
	ProgressPct float64   `gorm:"default:0" json:"progressPct"`          // 0-100

	// Snapshot of latest position data (JSON)
	CurrentPositions string `gorm:"type:text" json:"currentPositions"`

	// Parameters stored as JSON for replay
	Params     string    `gorm:"type:text" json:"params"` // {startDate, endDate, stockCodes}
	ResultID   *uint     `json:"resultId"`                // FK to backtest_results.id when completed

	StartedAt   *time.Time `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

func (BacktestTask) TableName() string { return "backtest_tasks" }
