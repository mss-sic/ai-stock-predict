package model

import "time"

// StrategyCashFlow records every deposit/withdrawal operation for a strategy run.
// Provides audit trail for strategy capital changes and supports P&L calculation.
type StrategyCashFlow struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	StrategyRunID uint      `gorm:"index" json:"strategyRunId"`
	AccountID     uint      `gorm:"index" json:"accountId"`
	UserID        uint      `gorm:"index" json:"userId"`
	FlowType      string    `gorm:"size:20" json:"flowType"` // deposit / withdraw / manual_sell
	Amount        float64   `gorm:"type:numeric(16,2)" json:"amount"`
	BeforeCash    float64   `gorm:"type:numeric(16,2)" json:"beforeCash"` // strategy available_cash before
	AfterCash     float64   `gorm:"type:numeric(16,2)" json:"afterCash"`  // strategy available_cash after
	Reason        string    `gorm:"size:500" json:"reason"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (StrategyCashFlow) TableName() string { return "strategy_cash_flows" }
