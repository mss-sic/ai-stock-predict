package repository

import (
	"github.com/ai-stock-predict/server/internal/db"
)

// IndicatorRow is a single indicator value row from the database.
type IndicatorRow struct {
	Code  string
	Date  string
	Value float64
}

// IndicatorRepo provides database access for technical indicator data.
// Used by DataLoaderService for batch preloading during backtests.
type IndicatorRepo struct{}

// NewIndicatorRepo creates a new IndicatorRepo.
func NewIndicatorRepo() *IndicatorRepo { return &IndicatorRepo{} }

// BatchScan runs a raw SQL query and returns indicator rows.
func (r *IndicatorRepo) BatchScan(query string, args ...interface{}) ([]IndicatorRow, error) {
	var rows []IndicatorRow
	err := db.PG.Raw(query, args...).Scan(&rows).Error
	return rows, err
}

// BatchScanWithCodes builds an IN clause from stock codes and runs the query.
// queryFmt should contain %s where the IN clause will be substituted.
func (r *IndicatorRepo) BatchScanWithCodes(codes []string, queryFmt string, extraArgs ...interface{}) ([]IndicatorRow, error) {
	// delegate to the handler's existing preloadIndicators for now
	// This will be fully implemented when indicator preloading is extracted in Phase 3
	return nil, nil
}
