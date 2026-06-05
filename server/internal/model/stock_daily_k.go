package model

import "time"

type StockDailyK struct {
	Code         string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate    time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	Open         float64   `gorm:"type:numeric(12,4)" json:"open"`
	High         float64   `gorm:"type:numeric(12,4)" json:"high"`
	Low          float64   `gorm:"type:numeric(12,4)" json:"low"`
	Close        float64   `gorm:"type:numeric(12,4)" json:"close"`
	Volume       int64     `json:"volume"`
	Amount       float64   `gorm:"type:numeric(20,2)" json:"amount"`
	TurnoverRate float64   `gorm:"type:numeric(10,4)" json:"turnoverRate"`
}

func (StockDailyK) TableName() string { return "stocks_daily_k" }
