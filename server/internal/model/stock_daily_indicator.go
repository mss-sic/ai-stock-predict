package model

import "time"

type StockDailyIndicator struct {
	Code                 string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate            time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	PE                   float64   `gorm:"type:numeric(14,4)" json:"pe"`
	PB                   float64   `gorm:"type:numeric(14,4)" json:"pb"`
	PS                   float64   `gorm:"type:numeric(14,4)" json:"ps"`
	TotalMarketCap       float64   `gorm:"type:numeric(20,2)" json:"totalMarketCap"`
	CirculatingMarketCap float64   `gorm:"type:numeric(20,2)" json:"circulatingMarketCap"`
}

func (StockDailyIndicator) TableName() string { return "stocks_daily_indicator" }
