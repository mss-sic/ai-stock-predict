package model

import "time"

// StockSignal stores algorithmic signal scores imported from Excel sheet1
type StockSignal struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Code        string    `gorm:"uniqueIndex;size:10" json:"code"`
	SignalValue float64   `gorm:"type:numeric(12,6)" json:"signalValue"`
	Source      string    `gorm:"size:50;default:excel_import" json:"source"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (StockSignal) TableName() string { return "stock_signals" }
