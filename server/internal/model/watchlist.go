package model

import "time"

type WatchlistGroup struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index:idx_wg_user_sort" json:"userId"`
	Name      string    `gorm:"size:30" json:"name"`
	SortOrder int       `gorm:"index:idx_wg_user_sort;default:0" json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

type Watchlist struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"uniqueIndex:uk_user_stock" json:"userId"`
	GroupID    uint      `gorm:"index;default:0" json:"groupId"`
	StockCode  string    `gorm:"uniqueIndex:uk_user_stock;size:10" json:"stockCode"`
	AddedPrice float64   `gorm:"default:0" json:"addedPrice"` // price at add time, for yield calc
	AddedAt    time.Time `json:"addedAt"`
}
