package model

import "time"

// StrategyComparison groups multiple backtest runs for side-by-side PK comparison.
type StrategyComparison struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"userId"`
	Name        string    `gorm:"size:100" json:"name"`          // comparison label, e.g. "价值vs成长vs动量"
	Description string    `gorm:"size:500" json:"description"`
	RunIDs      string    `gorm:"type:json" json:"runIds"`        // JSON array of result IDs or run IDs
	StartDate   string    `gorm:"type:varchar(10)" json:"startDate"`
	EndDate     string    `gorm:"type:varchar(10)" json:"endDate"`
	Benchmark   string    `gorm:"size:20" json:"benchmark"`      // benchmark index, e.g. "000300.SH"
	Metrics     string    `gorm:"type:json" json:"metrics"`       // JSON: aggregated comparison stats
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (StrategyComparison) TableName() string { return "strategy_comparisons" }
