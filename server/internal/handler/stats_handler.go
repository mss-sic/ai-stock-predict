package handler

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type StatsHandler struct{}

type klineSummary struct {
	TotalRows    int64  `json:"totalRows"`
	TotalStocks  int64  `json:"totalStocks"`
	MinDate      string `json:"minDate"`
	MaxDate      string `json:"maxDate"`
	QualityOk    int64  `json:"qualityOk"`
	QualitySus   int64  `json:"qualitySuspect"`
	QualityBad   int64  `json:"qualityBad"`
	StaleStocks  int64  `json:"staleStocks"`
	SparseStocks int64  `json:"sparseStocks"`
}

type financialSummary struct {
	TotalRows   int64   `json:"totalRows"`
	TotalStocks int64   `json:"totalStocks"`
	HasCashFlow int64   `json:"hasCashFlow"`
	CashFlowPct float64 `json:"cashFlowPct"`
}

type statsResult struct {
	KLine      klineSummary     `json:"kline"`
	Financials financialSummary `json:"financials"`
}

var (
	statsCache     *statsResult
	statsCacheMu   sync.RWMutex
	statsCacheTime time.Time
	statsCacheTTL  = 5 * time.Minute
)

// PrewarmStatsCache is a no-op. ANALYZE is handled by PostgreSQL autovacuum.
// Stats are computed lazily on first request and cached for 5 minutes.
func PrewarmStatsCache() {}

func computeStats() (*statsResult, error) {
	var res statsResult

	// Quality distribution + total_rows in one pass
	rows, err := db.PG.Raw(
		"SELECT data_quality, COUNT(*) FROM stocks_daily_k GROUP BY data_quality",
	).Rows()
	if err != nil {
		return nil, fmt.Errorf("query kline quality stats: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q string
		var cnt int64
		if err := rows.Scan(&q, &cnt); err != nil {
			return nil, fmt.Errorf("scan kline quality stats: %w", err)
		}
		res.KLine.TotalRows += cnt
		switch q {
		case "ok":
			res.KLine.QualityOk = cnt
		case "suspect":
			res.KLine.QualitySus = cnt
		case "bad":
			res.KLine.QualityBad = cnt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate kline quality stats: %w", err)
	}

	if err := db.PG.Raw("SELECT COUNT(DISTINCT code) FROM stocks_daily_k").
		Scan(&res.KLine.TotalStocks).Error; err != nil {
		return nil, fmt.Errorf("query stock count: %w", err)
	}

	// Min/Max date
	if err := db.PG.Raw("SELECT COALESCE(MIN(trade_date)::text, ''), COALESCE(MAX(trade_date)::text, '') FROM stocks_daily_k").
		Row().Scan(&res.KLine.MinDate, &res.KLine.MaxDate); err != nil {
		return nil, fmt.Errorf("query kline date range: %w", err)
	}

	// Stale stocks
	if err := db.PG.Raw(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM stocks_daily_k GROUP BY code
			HAVING MAX(trade_date) < (SELECT MAX(trade_date) FROM stocks_daily_k) - 3
		) t
	`).Scan(&res.KLine.StaleStocks).Error; err != nil {
		return nil, fmt.Errorf("query stale stocks: %w", err)
	}

	// Sparse stocks
	if err := db.PG.Raw(`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM stocks_daily_k GROUP BY code HAVING COUNT(*) < 100
		) t
	`).Scan(&res.KLine.SparseStocks).Error; err != nil {
		return nil, fmt.Errorf("query sparse stocks: %w", err)
	}

	// Financial stats
	if err := db.PG.Raw(`
		SELECT COUNT(*), COUNT(DISTINCT code),
			COUNT(*) FILTER (WHERE operating_cf IS NOT NULL AND operating_cf != 0)
		FROM stock_financials
	`).Row().Scan(&res.Financials.TotalRows, &res.Financials.TotalStocks, &res.Financials.HasCashFlow); err != nil {
		return nil, fmt.Errorf("query financial stats: %w", err)
	}
	if res.Financials.TotalRows > 0 {
		res.Financials.CashFlowPct = math.Round(float64(res.Financials.HasCashFlow)/float64(res.Financials.TotalRows)*1000) / 10
	}

	return &res, nil
}

func (h *StatsHandler) DataStatsSummary(c *gin.Context) {
	statsCacheMu.RLock()
	if statsCache != nil && time.Since(statsCacheTime) < statsCacheTTL {
		r := *statsCache
		statsCacheMu.RUnlock()
		response.Success(c, r)
		return
	}
	statsCacheMu.RUnlock()

	statsCacheMu.Lock()
	defer statsCacheMu.Unlock()
	if statsCache != nil && time.Since(statsCacheTime) < statsCacheTTL {
		response.Success(c, *statsCache)
		return
	}

	res, err := computeStats()
	if err != nil {
		log.Printf("[stats] compute dashboard stats failed: %v", err)
		response.InternalError(c, "数据统计查询失败")
		return
	}
	statsCache = res
	statsCacheTime = time.Now()

	response.Success(c, *res)
}
