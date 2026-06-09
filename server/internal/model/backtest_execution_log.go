package model

import "time"

// BacktestExecutionLog records every decision during backtest execution.
// One row per evaluation / signal / trade / system event.
// Supports real-time console streaming and post-mortem review.
type BacktestExecutionLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TaskID     uint      `gorm:"index:idx_exec_task;not null" json:"taskId"`
	StrategyID uint      `gorm:"index" json:"strategyId"`
	UserID     uint      `gorm:"index" json:"userId"`
	Date       string    `gorm:"type:varchar(10);index:idx_exec_task" json:"date"` // YYYY-MM-DD, empty for system events
	Seq        int       `gorm:"default:0" json:"seq"`                              // ordering within the same task+date

	// Classification
	LogType string `gorm:"size:20;not null" json:"logType"` // condition_eval | signal | trade | system | error
	Level   string `gorm:"size:10;default:info" json:"level"` // info | warn | error | debug

	// Context
	StockCode string `gorm:"size:20" json:"stockCode"`   // empty for system-wide logs
	StockName string `gorm:"size:50" json:"stockName"`   // human-readable name

	// Content
	Message string `gorm:"size:1000" json:"message"`      // human-readable summary
	Detail  string `gorm:"type:json" json:"detail"`       // structured data (conditions, prices, trade info)

	CreatedAt time.Time `json:"createdAt"`
}

func (BacktestExecutionLog) TableName() string { return "backtest_execution_logs" }
