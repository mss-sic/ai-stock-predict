package model

import "time"

// StockCapitalFlow stores daily per-stock capital flow (主力/超大单/大单/中单/小单).
type StockCapitalFlow struct {
	Code           string    `gorm:"primaryKey;size:10" json:"code"`
	TradeDate      time.Time `gorm:"primaryKey;type:date" json:"tradeDate"`
	MainNet        float64   `gorm:"type:numeric(16,2)" json:"mainNet"`
	SuperLargeNet  float64   `gorm:"type:numeric(16,2)" json:"superLargeNet"`
	LargeNet       float64   `gorm:"type:numeric(16,2)" json:"largeNet"`
	MediumNet      float64   `gorm:"type:numeric(16,2)" json:"mediumNet"`
	SmallNet       float64   `gorm:"type:numeric(16,2)" json:"smallNet"`
}

func (StockCapitalFlow) TableName() string { return "stock_capital_flow" }
