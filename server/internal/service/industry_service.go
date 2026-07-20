package service

import (
	"fmt"
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
)

// IndustrySummary represents aggregate metrics for one industry.
type IndustrySummary struct {
	Industry       string  `json:"industry"`
	StockCount     int     `gorm:"column:stock_count" json:"stockCount"`
	PEMedian       float64 `gorm:"column:pe_median" json:"peMedian"`
	PEP25          float64 `gorm:"column:pe_p25" json:"peP25"`
	PEP75          float64 `gorm:"column:pe_p75" json:"peP75"`
	PBMedian       float64 `gorm:"column:pb_median" json:"pbMedian"`
	PSMedian       float64 `gorm:"column:ps_median" json:"psMedian"`
	AvgMarketCap   float64 `gorm:"column:avg_market_cap" json:"avgMarketCap"`
	AvgWeekReturn  float64 `gorm:"column:avg_week_return" json:"avgWeekReturn"`
	AvgMonthReturn float64 `gorm:"column:avg_month_return" json:"avgMonthReturn"`
}

// IndustryStock represents a single stock within an industry comparison.
type IndustryStock struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	PE         float64 `gorm:"column:pe" json:"pe"`
	PB         float64 `gorm:"column:pb" json:"pb"`
	PS         float64 `gorm:"column:ps" json:"ps"`
	MarketCap  float64 `gorm:"column:market_cap" json:"marketCap"`
	PERank     int     `gorm:"column:pe_rank" json:"peRank"`
	WeekReturn float64 `gorm:"column:week_return" json:"weekReturn"`
	Close      float64 `gorm:"column:close" json:"close"`
	ChangePct  float64 `gorm:"column:change_pct" json:"changePct"`
}

func getLatestDate() (string, error) {
	var latest time.Time
	// Use stocks_daily_k which always has data (vs market_daily_agg which may be empty)
	if err := db.PG.Raw(`SELECT MAX(trade_date) FROM stocks_daily_k`).Scan(&latest).Error; err != nil {
		return "", fmt.Errorf("get latest date: %w", err)
	}
	if latest.IsZero() {
		return time.Now().Format("2006-01-02"), nil
	}
	return latest.Format("2006-01-02"), nil
}

// GetIndustryList returns industry-level aggregate comparisons for a given date.
// industryType: "sw_l1" (申万一级), "sw_l2" (申万二级), "tdx" (传统, 默认)
func GetIndustryList(date string, industryType string) ([]IndustrySummary, error) {
	if date == "" {
		var err error
		date, err = getLatestDate()
		if err != nil {
			return nil, err
		}
	}
	if industryType == "" { industryType = "tdx" }
	log.Printf("[industry] GetIndustryList industryType=%s date=%s", industryType, date)

	var col string
	var filter string
	switch industryType {
	case "sw_l1":
		col = "sb.sw_l1"
		filter = "sb.sw_l1 IS NOT NULL AND sb.sw_l1 != ''"
	case "sw_l2":
		col = "sb.sw_l2"
		filter = "sb.sw_l2 IS NOT NULL AND sb.sw_l2 != ''"
	default: // tdx
		col = "sb.industry"
		filter = "sb.industry IS NOT NULL AND sb.industry != '' AND sb.industry !~ '^[0-9]' AND sb.industry !~ '^行业[0-9]'"
	}

	sql := fmt.Sprintf(`
		WITH price_snapshot AS (
			SELECT code, close,
				LAG(close, 0) OVER (PARTITION BY code ORDER BY trade_date) as prev_close,
				LAG(close, 4) OVER (PARTITION BY code ORDER BY trade_date) as week_close,
				LAG(close, 19) OVER (PARTITION BY code ORDER BY trade_date) as month_close,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k
			WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '60 days')
		),
		latest_prices AS (
			SELECT code, close, prev_close, week_close, month_close
			FROM price_snapshot WHERE rn = 1
		),
		stock_metrics AS (
			SELECT sb.code, sb.name, %s as industry,
				COALESCE(di.pe, 0) as pe,
				COALESCE(di.pb, 0) as pb,
				COALESCE(di.ps, 0) as ps,
				COALESCE(di.total_market_cap, 0) as mcap,
				COALESCE(k.close, 0) as close,
				COALESCE(k.change_pct, 0) as change_pct,
				COALESCE((k.close - lp.week_close) / NULLIF(lp.week_close, 0) * 100, 0) as week_return,
				COALESCE((k.close - lp.month_close) / NULLIF(lp.month_close, 0) * 100, 0) as month_return
			FROM stocks_basic sb
			LEFT JOIN stocks_daily_indicator di ON di.code = sb.code AND di.trade_date = ?
			LEFT JOIN stocks_daily_k k ON k.code = sb.code AND k.trade_date = ?
			LEFT JOIN latest_prices lp ON lp.code = sb.code
			WHERE %s
		)
		SELECT industry,
			COUNT(*) as stock_count,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY pe), 0) as pe_median,
			COALESCE(PERCENTILE_CONT(0.25) WITHIN GROUP (ORDER BY pe), 0) as pe_p25,
			COALESCE(PERCENTILE_CONT(0.75) WITHIN GROUP (ORDER BY pe), 0) as pe_p75,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY pb), 0) as pb_median,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY ps), 0) as ps_median,
			COALESCE(AVG(mcap), 0) as avg_market_cap,
			COALESCE(AVG(week_return), 0) as avg_week_return,
			COALESCE(AVG(month_return), 0) as avg_month_return
		FROM stock_metrics
		GROUP BY industry
		HAVING COUNT(*) >= 3
		ORDER BY avg_week_return DESC
	`, col, filter)

	var list []IndustrySummary
	if err := db.PG.Raw(sql, date, date, date, date).Scan(&list).Error; err != nil {
		log.Printf("[industry] GetIndustryList query failed: %v", err)
		return nil, fmt.Errorf("query industry list: %w", err)
	}
	log.Printf("[industry] GetIndustryList type=%s date=%s industries=%d", industryType, date, len(list))
	return list, nil
}

// GetIndustryStocks returns all stocks in a given industry, ranked by PE ascending (default).
// industryType: "sw_l1", "sw_l2", "tdx"
func GetIndustryStocks(industry, date, sortBy, industryType string) ([]IndustryStock, error) {
	if date == "" {
		var err error
		date, err = getLatestDate()
		if err != nil {
			return nil, err
		}
	}

	if sortBy == "" {
		sortBy = "pe"
	}
	if industryType == "" {
		industryType = "tdx"
	}

	var filterCol string
	switch industryType {
	case "sw_l1":
		filterCol = "sb.sw_l1"
	case "sw_l2":
		filterCol = "sb.sw_l2"
	default:
		filterCol = "sb.industry"
	}

	orderClause := "ORDER BY CASE WHEN pe > 0 THEN pe ELSE 999999 END ASC"
	switch sortBy {
	case "return":
		orderClause = "ORDER BY week_return DESC"
	case "change":
		orderClause = "ORDER BY change_pct DESC"
	}

	sql := fmt.Sprintf(`
		WITH price_snapshot AS (
			SELECT code,
				LAG(close, 4) OVER (PARTITION BY code ORDER BY trade_date) as week_close,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k WHERE trade_date <= $1 AND trade_date >= ($1::date - INTERVAL '30 days')
		),
		latest_prices AS (
			SELECT code, week_close FROM price_snapshot WHERE rn = 1
		),
		ranked AS (
			SELECT sb.code, sb.name,
				COALESCE(di.pe, 0) as pe,
				COALESCE(di.pb, 0) as pb,
				COALESCE(di.ps, 0) as ps,
				COALESCE(di.total_market_cap, 0) as market_cap,
				COALESCE(k.close, 0) as close,
				COALESCE(k.change_pct, 0) as change_pct,
				COALESCE((k.close - lp.week_close) / NULLIF(lp.week_close, 0) * 100, 0) as week_return,
				ROW_NUMBER() OVER (ORDER BY CASE WHEN di.pe > 0 THEN di.pe ELSE 999999 END ASC) as pe_rank
			FROM stocks_basic sb
			LEFT JOIN stocks_daily_indicator di ON di.code = sb.code AND di.trade_date = $2
			LEFT JOIN stocks_daily_k k ON k.code = sb.code AND k.trade_date = $3
			LEFT JOIN latest_prices lp ON lp.code = sb.code
			WHERE %s = $4
		)
		SELECT code, name, pe, pb, ps, market_cap, week_return, close, change_pct, pe_rank
		FROM ranked
		%s
	`, filterCol, orderClause)

	var list []IndustryStock
	if err := db.PG.Raw(sql, date, date, date, industry).Scan(&list).Error; err != nil {
		log.Printf("[industry] GetIndustryStocks query failed: %v", err)
		return nil, fmt.Errorf("query industry stocks: %w", err)
	}
	return list, nil
}
