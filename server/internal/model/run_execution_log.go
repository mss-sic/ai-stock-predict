package model

import "time"

// RunExecutionLog stores per-run, per-day execution logs for strategy and trade execution.
type RunExecutionLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RunID     uint      `gorm:"index:idx_run_date;index:idx_run_type" json:"runId"`
	TradeDate string    `gorm:"size:10;index:idx_run_date;index:idx_run_type" json:"tradeDate"`
	LogType   string    `gorm:"size:20;index:idx_run_type;default:strategy" json:"logType"` // strategy / trade_exec / order_sync
	Level     string    `gorm:"size:10;default:info" json:"level"`
	StockCode string    `gorm:"size:10;default:''" json:"stockCode"`
	StockName string    `gorm:"size:50;default:''" json:"stockName"`
	Message   string    `gorm:"size:2000" json:"message"`
	Detail    string    `gorm:"type:text" json:"detail"` // JSON extra data
	CreatedAt time.Time `json:"createdAt"`
}

func (RunExecutionLog) TableName() string { return "run_execution_logs" }
