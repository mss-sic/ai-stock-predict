package model

import "time"

// DividendHistory stores dividend / bonus share history records.
type DividendHistory struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Code           string    `gorm:"size:10;index" json:"code"`
	ExDividendDate string    `gorm:"size:10;uniqueIndex:idx_dividend_code_date" json:"exDividendDate"`
	BonusRmb       float64   `json:"bonusRmb"`
	TransferRatio  float64   `json:"transferRatio"`
	BonusRatio     float64   `json:"bonusRatio"`
	Progress       string    `gorm:"size:50" json:"progress"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (DividendHistory) TableName() string { return "dividend_history" }
