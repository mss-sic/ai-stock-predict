package repository

import (
	"github.com/ai-stock-predict/server/internal/db"
)

// KlineRow is a single row of K-line data from the database.
type KlineRow struct {
	Code  string
	Date  string
	Close float64
	Open  float64
}

// KlineRepo provides database access for K-line (stock daily) data.
type KlineRepo struct{}

// NewKlineRepo creates a new KlineRepo.
func NewKlineRepo() *KlineRepo { return &KlineRepo{} }

// GetTradingDates returns distinct trading dates in range, ordered ascending.
func (r *KlineRepo) GetTradingDates(startDate, endDate string) ([]string, error) {
	var dates []string
	err := db.PG.Raw(`SELECT DISTINCT TO_CHAR(trade_date, 'YYYY-MM-DD') as d FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ? ORDER BY d`, startDate, endDate).Scan(&dates).Error
	return dates, err
}

// BulkLoadCloses loads close + open prices for multiple codes in a date range.
// Returns rows ordered by code, trade_date.
func (r *KlineRepo) BulkLoadCloses(codes []string, startDate, endDate string) ([]KlineRow, error) {
	var rows []KlineRow
	err := db.PG.Table("stocks_daily_k").
		Select("code, TO_CHAR(trade_date, 'YYYY-MM-DD') as date, close, open").
		Where("code IN ?", codes).
		Where("trade_date >= ?", startDate).
		Where("trade_date <= ?", endDate).
		Order("code, trade_date").
		Scan(&rows).Error
	return rows, err
}

// GetStockUniverse returns distinct stock codes with K-line data in range.
// Limit controls max results; filters out ST/suspended stocks.
func (r *KlineRepo) GetStockUniverse(startDate, endDate string, limit int) ([]struct {
	Code string
	Name string
}, error) {
	var universe []struct {
		Code string
		Name string
	}
	err := db.PG.Table("stocks_daily_k k").
		Select("k.code, COALESCE(s.name, k.code) as name").
		Joins("LEFT JOIN stocks_basic s ON s.code = k.code").
		Where("k.trade_date >= ?", startDate).
		Where("k.trade_date <= ?", endDate).
		Where("s.is_st IS NULL OR s.is_st = false").
		Group("k.code, s.name").
		Order("code ASC").Limit(limit).
		Scan(&universe).Error
	return universe, err
}

// GetStockUniverseFiltered returns codes from a specific list, filtering ST.
func (r *KlineRepo) GetStockUniverseFiltered(codes []string) ([]struct {
	Code string
	Name string
}, error) {
	var universe []struct {
		Code string
		Name string
	}
	err := db.PG.Table("stocks_basic").
		Select("code, COALESCE(name,'') as name").
		Where("code IN ?", codes).
		Where("is_st IS NULL OR is_st = false").
		Scan(&universe).Error
	return universe, err
}
