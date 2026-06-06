package model

import "time"

// AIStockScore stores comprehensive 6-dimension AI scoring results (PostgreSQL)
type AIStockScore struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Code             string    `gorm:"index;size:10" json:"code"`
	CompositeScore   float64   `gorm:"type:numeric(4,2)" json:"compositeScore"`
	FundamentalScore float64   `gorm:"type:numeric(4,2)" json:"fundamentalScore"`
	GrowthScore      float64   `gorm:"type:numeric(4,2)" json:"growthScore"`
	ValuationScore   float64   `gorm:"type:numeric(4,2)" json:"valuationScore"`
	CapitalScore     float64   `gorm:"type:numeric(4,2)" json:"capitalScore"`
	TechnicalScore   float64   `gorm:"type:numeric(4,2)" json:"technicalScore"`
	IndustryScore    float64   `gorm:"type:numeric(4,2)" json:"industryScore"`
	RiskLevel        string    `gorm:"size:20" json:"riskLevel"`
	Suggestion       string    `gorm:"size:20" json:"suggestion"`
	RiskWarnings     JSONArray `gorm:"type:jsonb" json:"riskWarnings"`
	Summary          string    `gorm:"type:text" json:"summary"`
	AnalyzedAt       time.Time `json:"analyzedAt"`
	CreatedAt        time.Time `json:"createdAt"`
}

func (AIStockScore) TableName() string { return "ai_stock_scores" }
