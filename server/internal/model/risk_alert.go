package model

import "time"

// RiskAlert stores individual risk warning records.
// Dedup: UNIQUE(user_id, stock_code, rule_key, hit_date).
// Market-level alerts use user_id=0, stock_code='__MARKET__'.
type RiskAlert struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"default:0" json:"userId"`
	StrategyID    uint      `gorm:"default:0" json:"strategyId"`
	StockCode     string    `gorm:"size:10" json:"stockCode"`
	StockName     string    `gorm:"-" json:"stockName"` // filled at query time from PG

	Level         string    `gorm:"size:10;default:low" json:"level"`
	Type          string    `gorm:"size:50" json:"type"`
	Description   string    `gorm:"type:text" json:"description"`

	RuleKey       string    `gorm:"size:50;default:''" json:"ruleKey"`
	Dimension     string    `gorm:"size:20;default:'stock'" json:"dimension"`
	SeverityScore int       `gorm:"default:0" json:"severityScore"`
	Evidence      JSONMap   `gorm:"type:json" json:"evidence"`

	HitDate       time.Time `json:"hitDate"`

	// Deprecated: use Status instead. Kept for backward compat.
	Ignored       bool      `json:"ignored"`

	Status        string     `gorm:"size:15;default:'active'" json:"status"` // active / acknowledged / resolved
	AcknowledgedAt *time.Time `json:"acknowledgedAt"`
	ResolvedAt    *time.Time `json:"resolvedAt"`
	ResolutionNote string    `gorm:"size:200" json:"resolutionNote"`

	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (RiskAlert) TableName() string { return "risk_alerts" }
