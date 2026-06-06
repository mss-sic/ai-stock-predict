package repository

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type WatchlistRepo struct{}

func (r *WatchlistRepo) List(userID uint) ([]model.Watchlist, error) {
	var items []model.Watchlist
	err := db.MySQL.Where("user_id = ?", userID).Order("added_at DESC").Find(&items).Error
	return items, err
}

func (r *WatchlistRepo) Add(userID uint, stockCode string) error {
	// Check if already exists
	var count int64
	db.MySQL.Model(&model.Watchlist{}).Where("user_id = ? AND stock_code = ?", userID, stockCode).Count(&count)
	if count > 0 {
		return nil
	}
	return db.MySQL.Create(&model.Watchlist{
		UserID:    userID,
		StockCode: stockCode,
		AddedAt:   time.Now(),
	}).Error
}

func (r *WatchlistRepo) Remove(userID uint, stockCode string) error {
	return db.MySQL.Where("user_id = ? AND stock_code = ?", userID, stockCode).
		Delete(&model.Watchlist{}).Error
}
