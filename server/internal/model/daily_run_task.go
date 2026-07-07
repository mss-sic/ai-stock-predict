package model

import "time"

// DailyRunTask tracks an async daily-run execution job.
type DailyRunTask struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"userId"`
	TradeDate     string    `gorm:"size:10;index" json:"tradeDate"`
	RunID         uint      `gorm:"index" json:"runId"`
	Mode          string    `gorm:"size:20;default:after_close" json:"mode"`
	Status        string    `gorm:"size:20;default:pending" json:"status"` // pending, running, completed, failed
	TotalStocks   int       `json:"totalStocks"`
	ScannedStocks int       `json:"scannedStocks"`
	CandidateCount int      `json:"candidateCount"`
	SignalCount   int       `json:"signalCount"`
	Logs          string    `gorm:"type:text" json:"logs"`
	Error         string    `gorm:"size:500" json:"error"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (DailyRunTask) TableName() string { return "daily_run_tasks" }
