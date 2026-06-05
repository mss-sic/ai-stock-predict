package model

import "time"

type Holding struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `json:"userId"`
	StockCode  string    `gorm:"size:10" json:"stockCode"`
	CostPrice  float64   `gorm:"type:numeric(12,4)" json:"costPrice"`
	Quantity   int       `json:"quantity"`
	StrategyID uint      `json:"strategyId"`
	CreatedAt  time.Time `json:"createdAt"`
}
