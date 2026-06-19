package model

import "time"

// SentimentWeights stores weight configurations for market sentiment scoring.
type SentimentWeights struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"size:50" json:"name"`
	BreadthW        float64   `gorm:"type:numeric(4,3)" json:"breadthW"`
	StyleRiskW      float64   `gorm:"type:numeric(4,3)" json:"styleRiskW"`
	ActivityW       float64   `gorm:"type:numeric(4,3)" json:"activityW"`
	ProfitW         float64   `gorm:"type:numeric(4,3)" json:"profitW"`
	VolatilityW     float64   `gorm:"type:numeric(4,3)" json:"volatilityW"`
	StrengthW       float64   `gorm:"type:numeric(4,3)" json:"strengthW"`
	RiskAppetiteW   float64   `gorm:"type:numeric(4,3)" json:"riskAppetiteW"`
	LimitW          float64   `gorm:"type:numeric(4,3)" json:"limitW"`
	SectorW         float64   `gorm:"type:numeric(4,3)" json:"sectorW"`
	NorthboundW     float64   `gorm:"type:numeric(4,3)" json:"northboundW"`
	CapitalFlowW    float64   `gorm:"type:numeric(4,3)" json:"capitalFlowW"`
	IsActive        bool      `gorm:"default:false" json:"isActive"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (SentimentWeights) TableName() string { return "sentiment_weights" }
