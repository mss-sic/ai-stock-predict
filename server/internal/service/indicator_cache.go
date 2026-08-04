package service

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/ai-stock-predict/server/internal/db"
)

// IndicatorCacheService reads precomputed indicators from stock_daily_indicators JSONB cache.
type IndicatorCacheService struct{}

// NewIndicatorCacheService creates a new cache service.
func NewIndicatorCacheService() *IndicatorCacheService {
	return &IndicatorCacheService{}
}

// GetValue returns a single indicator value from the cache for a given stock/date.
// Falls back to on-the-fly computation if cache miss.
func (s *IndicatorCacheService) GetValue(code, date, key string) (float64, bool) {
	// Try cache first
	val, ok := s.getFromCache(code, date, key)
	if ok {
		return val, true
	}
	// Fallback: compute on-the-fly using existing infrastructure
	return GetIndicatorValue(key, code, date)
}

// GetBatch returns all indicator values for a stock/date from the cache.
func (s *IndicatorCacheService) GetBatch(code, date string) (map[string]float64, error) {
	var indicatorsJSON string
	err := db.PG.Raw(
		"SELECT indicators::text FROM stock_daily_indicators WHERE code = ? AND trade_date = ?",
		code, date,
	).Scan(&indicatorsJSON).Error
	if err != nil {
		return nil, fmt.Errorf("cache miss for %s/%s: %w", code, date, err)
	}
	if indicatorsJSON == "" || indicatorsJSON == "{}" || indicatorsJSON == "null" {
		return nil, fmt.Errorf("empty cache for %s/%s", code, date)
	}

	var result map[string]float64
	if err := json.Unmarshal([]byte(indicatorsJSON), &result); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}
	return result, nil
}

// GetQuality returns the data quality flag for a given stock/date.
func (s *IndicatorCacheService) GetQuality(code, date string) string {
	var quality string
	db.PG.Raw(
		"SELECT COALESCE(data_quality, 'ok') FROM stock_daily_indicators WHERE code = ? AND trade_date = ?",
		code, date,
	).Scan(&quality)
	return quality
}

// IsCached checks whether the cache has data for a given stock/date.
func (s *IndicatorCacheService) IsCached(code, date string) bool {
	var count int64
	db.PG.Raw(
		"SELECT COUNT(*) FROM stock_daily_indicators WHERE code = ? AND trade_date = ?",
		code, date,
	).Scan(&count)
	return count > 0
}

// LatestDate returns the most recent date with cached indicators.
func (s *IndicatorCacheService) LatestDate() string {
	var dt string
	db.PG.Raw("SELECT MAX(trade_date)::text FROM stock_daily_indicators").Scan(&dt)
	return dt
}

// getFromCache reads a single key from the JSONB cache.
func (s *IndicatorCacheService) getFromCache(code, date, key string) (float64, bool) {
	var val float64
	err := db.PG.Raw(
		"SELECT COALESCE((indicators->>?)::numeric, 0) FROM stock_daily_indicators WHERE code = ? AND trade_date = ?",
		key, code, date,
	).Row().Scan(&val)
	if err != nil {
		log.Printf("[indicator_cache] miss: %s/%s/%s: %v", code, date, key, err)
		return 0, false
	}
	return val, true
}

// LatestDateForStock returns the most recent date with cached indicators for a specific stock.
func (s *IndicatorCacheService) LatestDateForStock(code string) string {
	var dt string
	db.PG.Raw("SELECT MAX(trade_date)::text FROM stock_daily_indicators WHERE code = ?", code).Scan(&dt)
	return dt
}

// AvailableDates returns recent dates with cached indicators for a stock.
func (s *IndicatorCacheService) AvailableDates(code string, limit int) []string {
	if limit <= 0 {
		limit = 20
	}
	var dates []string
	db.PG.Raw(
		"SELECT trade_date::text FROM stock_daily_indicators WHERE code = ? ORDER BY trade_date DESC LIMIT ?",
		code, limit,
	).Scan(&dates)
	return dates
}
