package repository

import (
	"github.com/ai-stock-predict/server/internal/db"
)

// PriceRepo provides batched stock price lookups, replacing scattered LATERAL subqueries.
type PriceRepo struct{}

// NewPriceRepo creates a new PriceRepo.
func NewPriceRepo() *PriceRepo { return &PriceRepo{} }

// LatestPriceInfo holds the latest close, previous close, and trade date for a stock.
type LatestPriceInfo struct {
	Code      string
	Name      string
	Close     float64
	PrevClose float64
	PriceDate string
}

// GetLatestPrices returns latest + previous close prices for the given codes.
// Uses a single CTE with window functions instead of per-row LATERAL subqueries.
func (r *PriceRepo) GetLatestPrices(codes []string) (map[string]LatestPriceInfo, error) {
	if len(codes) == 0 {
		return map[string]LatestPriceInfo{}, nil
	}

	var rows []LatestPriceInfo
	err := db.PG.Raw(`
		WITH ranked AS (
			SELECT code, close, trade_date,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) AS rn
			FROM stocks_daily_k
			WHERE code IN ?
		)
		SELECT s.code,
			COALESCE(s.name, s.code) AS name,
			COALESCE(r1.close, 0) AS close,
			COALESCE(r2.close, 0) AS prev_close,
			TO_CHAR(r1.trade_date, 'YYYY-MM-DD') AS price_date
		FROM stocks_basic s
		LEFT JOIN ranked r1 ON r1.code = s.code AND r1.rn = 1
		LEFT JOIN ranked r2 ON r2.code = s.code AND r2.rn = 2
		WHERE s.code IN ?
	`, codes, codes).Scan(&rows).Error

	result := make(map[string]LatestPriceInfo, len(rows))
	for _, row := range rows {
		result[row.Code] = row
	}
	return result, err
}

// GetLatestClose returns the most recent closing price for a single stock.
func (r *PriceRepo) GetLatestClose(code string) (float64, error) {
	var close float64
	err := db.PG.Raw(`
		SELECT COALESCE(close, 0) FROM stocks_daily_k
		WHERE code = ? ORDER BY trade_date DESC LIMIT 1
	`, code).Scan(&close).Error
	return close, err
}
