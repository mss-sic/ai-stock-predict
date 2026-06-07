package model

import "time"

type BacktestResult struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `json:"userId"`
	StrategyID   uint      `json:"strategyId"`
	StockCode    string    `gorm:"size:10" json:"stockCode"`
	StartDate    time.Time `gorm:"type:date" json:"startDate"`
	EndDate      time.Time `gorm:"type:date" json:"endDate"`
	TotalReturn  float64   `gorm:"type:numeric(10,4)" json:"totalReturn"`
	SharpeRatio  float64   `gorm:"type:numeric(8,4)" json:"sharpeRatio"`
	MaxDrawdown  float64   `gorm:"type:numeric(10,4)" json:"maxDrawdown"`
	WinRate      float64   `gorm:"type:numeric(8,4)" json:"winRate"`
	TradeCount   int       `json:"tradeCount"`
	Trades       JSONMap   `gorm:"type:json" json:"trades"`
	EquityCurve  JSONMap   `gorm:"type:json" json:"equityCurve"`
	Coverage     JSONMap   `gorm:"type:json" json:"coverage"`
	CreatedAt    time.Time `json:"createdAt"`
}
