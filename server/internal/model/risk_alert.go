package model

import "time"

type RiskAlert struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	StockCode   string    `gorm:"size:10" json:"stockCode"`
	StockName   string    `gorm:"-" json:"stockName"`
	Level       string    `gorm:"size:10;default:low" json:"level"`
	Type        string    `gorm:"size:50" json:"type"`
	Description string    `gorm:"type:text" json:"description"`
	HitDate     time.Time `json:"hitDate"`
	Ignored     bool      `json:"ignored"`
}
