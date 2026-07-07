package model

import "time"

type Holding struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `json:"userId"`
	AccountID  uint      `gorm:"index;default:0" json:"accountId"`
	StockCode  string    `gorm:"size:10" json:"stockCode"`
	CostPrice  float64   `gorm:"type:numeric(12,4)" json:"costPrice"`
	Quantity   int       `json:"quantity"`
	TotalCost  float64   `gorm:"type:numeric(16,2);default:0" json:"totalCost"` // costPrice * quantity
	BuyDate    string    `gorm:"size:10" json:"buyDate"`
	StrategyID uint      `json:"strategyId"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
