package model

import "time"

// PreMarketTask tracks an async pre-market finalization job.
type PreMarketTask struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UserID           uint      `gorm:"index" json:"userId"`
	RunID            uint      `gorm:"index" json:"runId"`
	TradeDate        string    `gorm:"size:10;index" json:"tradeDate"`
	Status           string    `gorm:"size:20;default:pending" json:"status"`
	TotalSignals     int       `json:"totalSignals"`
	CompletedSignals int       `json:"completedSignals"`
	CurrentStage     string    `gorm:"size:50" json:"currentStage"`
	CurrentCode      string    `gorm:"size:20" json:"currentCode"`
	Logs             string    `gorm:"type:text" json:"logs"`
	StageDetails     string    `gorm:"type:text" json:"stageDetails"`     // JSON array of per-signal stage progress
	PositionPatrol   string    `gorm:"type:text" json:"positionPatrol"` // JSON, position patrol results
	ResultJSON       string    `gorm:"type:text" json:"resultJson"`
	SkipAI           bool      `gorm:"default:false" json:"skipAi"`
	Error            string    `gorm:"size:500" json:"error"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

func (PreMarketTask) TableName() string { return "pre_market_tasks" }
