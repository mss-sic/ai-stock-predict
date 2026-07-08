package model

import "time"

// RiskSnapshot stores daily risk summary for trend tracking.
type RiskSnapshot struct {
	SnapshotDate       time.Time `gorm:"primaryKey;type:date" json:"snapshotDate"`
	MarketRiskLevel    string    `gorm:"size:10" json:"marketRiskLevel"`   // low / medium / high / critical
	MarketRiskScore    int       `json:"marketRiskScore"`                  // 0-100
	TotalAlertsHigh    int       `json:"totalAlertsHigh"`
	TotalAlertsMedium  int       `json:"totalAlertsMedium"`
	TotalAlertsLow     int       `json:"totalAlertsLow"`
	DimensionBreakdown JSONMap   `gorm:"type:json" json:"dimensionBreakdown"` // {"stock":12,"market":3,...}

	CreatedAt time.Time `json:"createdAt"`
}

func (RiskSnapshot) TableName() string { return "risk_snapshots" }
