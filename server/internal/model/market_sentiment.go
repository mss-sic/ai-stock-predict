package model

import "time"

// MarketSentiment stores daily market sentiment indicators and composite score.
type MarketSentiment struct {
	TradeDate         time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	MarketBreadth     float64   `gorm:"type:numeric(6,4)" json:"marketBreadth"`
	BreadthScore      float64   `gorm:"type:numeric(5,2)" json:"breadthScore"`
	StyleRiskPref     float64   `gorm:"type:numeric(6,4)" json:"styleRiskPref"`
	StyleRiskScore    float64   `gorm:"type:numeric(5,2)" json:"styleRiskScore"`
	TradeActivity     float64   `gorm:"type:numeric(6,4)" json:"tradeActivity"`
	ActivityScore     float64   `gorm:"type:numeric(5,2)" json:"activityScore"`
	ProfitEffect      float64   `gorm:"type:numeric(6,4)" json:"profitEffect"`
	ProfitScore       float64   `gorm:"type:numeric(5,2)" json:"profitScore"`
	Volatility        float64   `gorm:"type:numeric(6,4)" json:"volatility"`
	VolScore          float64   `gorm:"type:numeric(5,2)" json:"volatilityScore"`
	PriceStrength     float64   `gorm:"type:numeric(6,4)" json:"priceStrength"`
	StrengthScore     float64   `gorm:"type:numeric(5,2)" json:"strengthScore"`
	RiskAppetite      float64   `gorm:"type:numeric(6,4)" json:"riskAppetite"`
	RiskAppScore      float64   `gorm:"type:numeric(5,2)" json:"riskAppetiteScore"`
	LimitSentiment    float64   `gorm:"type:numeric(6,4)" json:"limitSentiment"`
	LimitScore        float64   `gorm:"type:numeric(5,2)" json:"limitSentimentScore"`
	SectorDiffusion   float64   `gorm:"type:numeric(6,4)" json:"sectorDiffusion"`
	SectorScore       float64   `gorm:"type:numeric(5,2)" json:"sectorDiffusionScore"`
	NorthboundNet     float64   `gorm:"type:numeric(12,2)" json:"northboundNet"`
	NorthboundScore   float64   `gorm:"type:numeric(5,2)" json:"northboundScore"`
	CapitalFlowNet    float64   `gorm:"type:numeric(12,2)" json:"capitalFlowNet"`
	CapitalFlowScore  float64   `gorm:"type:numeric(5,2)" json:"capitalFlowScore"`
	CompositeScore    float64   `gorm:"type:numeric(5,2)" json:"compositeScore"`
	UpCount           int       `json:"upCount"`
	DownCount         int       `json:"downCount"`
	LimitUpCount      int       `json:"limitUpCount"`
	LimitDownCount    int       `json:"limitDownCount"`
	BoardBreakCount   int       `json:"boardBreakCount"`
	TotalStocks       int       `json:"totalStocks"`
	CreatedAt         time.Time `json:"createdAt"`
}

func (MarketSentiment) TableName() string { return "market_sentiment" }
