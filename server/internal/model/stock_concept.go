package model

import "time"

// ConceptBoard represents a concept/industry board metadata
type ConceptBoard struct {
	ConceptCode string    `gorm:"primaryKey;size:20" json:"conceptCode"`
	ConceptName string    `gorm:"size:100;index" json:"conceptName"`
	ConceptType string    `gorm:"size:20;default:concept" json:"conceptType"` // concept / industry
	StockCount  int       `gorm:"default:0" json:"stockCount"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (ConceptBoard) TableName() string { return "concept_boards" }

// StockConcept maps a stock to a concept board
type StockConcept struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"size:10;index" json:"code"`
	ConceptCode string    `gorm:"size:20;index" json:"conceptCode"`
	ConceptName string    `gorm:"size:100;index" json:"conceptName"`
	ConceptType string    `gorm:"size:20;default:concept" json:"conceptType"`
	StockName   string    `gorm:"size:50" json:"stockName"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (StockConcept) TableName() string { return "stock_concepts" }

// ConceptHeatmapItem for heatmap display
type ConceptHeatmapItem struct {
	ConceptCode string  `json:"conceptCode"`
	ConceptName string  `json:"conceptName"`
	ConceptType string  `json:"conceptType"`
	StockCount  int     `json:"stockCount"`
	AvgChgPct   float64 `json:"avgChgPct"`
	UpCount     int     `json:"upCount"`
	DownCount   int     `json:"downCount"`
}
