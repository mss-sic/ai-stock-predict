package model

import "time"

// RiskRule stores configurable risk detection rule definitions.
type RiskRule struct {
	RuleKey      string  `gorm:"primaryKey;size:50" json:"ruleKey"`
	Name         string  `gorm:"size:100" json:"name"`
	Dimension    string  `gorm:"size:20" json:"dimension"`    // market / stock / portfolio / liquidity / event / operational / behavior
	DefaultLevel string  `gorm:"size:10" json:"defaultLevel"` // high / medium / low
	Enabled      bool    `gorm:"default:true" json:"enabled"`
	Thresholds   JSONMap `gorm:"type:json" json:"thresholds"`
	Description  string  `gorm:"type:text" json:"description"`
	Weight       float64 `gorm:"type:decimal(4,3);default:0.1" json:"weight"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (RiskRule) TableName() string { return "risk_rules" }
