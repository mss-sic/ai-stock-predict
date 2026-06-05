package model

import "time"

type Watchlist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"uniqueIndex:uk_user_stock" json:"userId"`
	StockCode string    `gorm:"uniqueIndex:uk_user_stock;size:10" json:"stockCode"`
	AddedAt   time.Time `json:"addedAt"`
}
