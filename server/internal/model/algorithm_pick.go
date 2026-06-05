package model

import "time"

type AlgorithmPick struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PickDate    time.Time `gorm:"uniqueIndex;type:date" json:"pickDate"`
	TotalStocks int       `gorm:"default:50" json:"totalStocks"`
	GeneratedAt time.Time `json:"generatedAt"`
}

func (AlgorithmPick) TableName() string { return "algorithm_picks" }

type AlgorithmPickDetail struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PickDate    time.Time `gorm:"index;type:date" json:"pickDate"`
	StockCode   string    `gorm:"index;size:10" json:"stockCode"`
	Rank        int       `json:"rank"`
	Score       float64   `gorm:"type:numeric(8,2)" json:"score"`
	SignalTags  JSONArray `gorm:"type:jsonb" json:"signalTags"`
	RiskLevel   string    `gorm:"size:10;default:low" json:"riskLevel"`
	Suggestion  string    `gorm:"size:10;default:hold" json:"suggestion"`
}

func (AlgorithmPickDetail) TableName() string { return "algorithm_pick_details" }
