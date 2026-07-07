package model

import "time"

// MarketStyleDaily stores daily market style classification and review data.
type MarketStyleDaily struct {
	TradeDate        time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Style            string    `gorm:"size:20;not null" json:"style"`
	StyleConfidence  float64   `gorm:"type:numeric(5,2)" json:"styleConfidence"`
	CompositeScore   float64   `gorm:"type:numeric(5,2)" json:"compositeScore"`
	UpRatio          float64   `gorm:"type:numeric(5,4)" json:"upRatio"`
	SectorDiffusion  float64   `gorm:"type:numeric(5,4)" json:"sectorDiffusion"`
	Volatility       float64   `gorm:"type:numeric(5,4)" json:"volatility"`
	ScoreTrend       float64   `gorm:"type:numeric(7,4)" json:"scoreTrend"`
	NorthboundNet    float64   `gorm:"type:numeric(12,2)" json:"northboundNet"`
	TotalAmount      float64   `gorm:"type:numeric(20,2)" json:"totalAmount"`
	LimitUpCount     int       `json:"limitUpCount"`
	LimitDownCount   int       `json:"limitDownCount"`
	MA20Above        int       `json:"ma20Above"`
	N52High          int       `json:"n52High"`
	N60Low           int       `json:"n60Low"`
	StyleDuration    int       `gorm:"default:0" json:"styleDuration"`
	TransitionSignal string    `gorm:"size:20" json:"transitionSignal"`
	TopSectors       string    `gorm:"type:jsonb" json:"topSectors"`
	TopConcepts      string    `gorm:"type:jsonb" json:"topConcepts"`
	AnalysisSummary  string    `gorm:"type:text" json:"analysisSummary"`
	LeadIndustry        string    `gorm:"size:100" json:"leadIndustry"`
	SectorDispersion    float64   `gorm:"type:numeric(7,4);default:0" json:"sectorDispersion"`
	ScoreChange         float64   `gorm:"type:numeric(7,2);default:0" json:"scoreChange"`
	BreakRate           float64   `gorm:"type:numeric(5,4);default:0" json:"breakRate"`
	Concentration       float64   `gorm:"type:numeric(5,4);default:0" json:"concentration"`
	RotationSpeed       float64   `gorm:"type:numeric(5,4);default:0" json:"rotationSpeed"`
	MarketRegime        string    `gorm:"size:20" json:"marketRegime"`
	ThematicLeadership  string    `gorm:"size:20" json:"thematicLeadership"`
	LeadConcept         string    `gorm:"size:100" json:"leadConcept"`
	GrowthDefenseFlow   float64   `json:"growthDefenseFlow"`
	CreatedAt           time.Time `json:"createdAt"`
}

func (MarketStyleDaily) TableName() string { return "market_style_daily" }
