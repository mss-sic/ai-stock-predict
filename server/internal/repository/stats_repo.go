package repository

import (
	"time"

	"github.com/ai-stock-predict/server/internal/db"
)

type DataStat struct {
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Count     int64      `json:"count"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

func GetDataStats() []DataStat {
	var stats []DataStat

	// stock count
	var stockCount int64
	db.PG.Model(&struct{}{}).Table("stocks_basic").Count(&stockCount)
	stats = append(stats, DataStat{Key: "stocks", Label: "股票基础数据", Count: stockCount})

	// kline count
	var klineCount int64
	var klineLast time.Time
	db.PG.Table("stocks_daily_k").Count(&klineCount)
	db.PG.Table("stocks_daily_k").Select("MAX(trade_date)").Scan(&klineLast)
	stats = append(stats, DataStat{Key: "kline", Label: "日K线数据", Count: klineCount, UpdatedAt: &klineLast})

	// indicator count
	var indCount int64
	var indLast time.Time
	db.PG.Table("stocks_daily_indicator").Count(&indCount)
	db.PG.Table("stocks_daily_indicator").Select("MAX(trade_date)").Scan(&indLast)
	stats = append(stats, DataStat{Key: "indicator", Label: "PE/PB 指标数据", Count: indCount, UpdatedAt: &indLast})

	// quote count
	var quoteCount int64
	var quoteLast time.Time
	db.PG.Table("stock_quotes").Count(&quoteCount)
	db.PG.Table("stock_quotes").Select("MAX(updated_at)").Scan(&quoteLast)
	stats = append(stats, DataStat{Key: "quote", Label: "实时行情", Count: quoteCount, UpdatedAt: &quoteLast})

	// financial count
	var finCount int64
	var finLast time.Time
	db.PG.Table("stock_financials").Count(&finCount)
	db.PG.Table("stock_financials").Select("MAX(created_at)").Scan(&finLast)
	stats = append(stats, DataStat{Key: "financial", Label: "财务数据", Count: finCount, UpdatedAt: &finLast})

	// shareholder count
	var shCount int64
	var shLast time.Time
	db.PG.Table("stock_shareholders").Count(&shCount)
	db.PG.Table("stock_shareholders").Select("MAX(created_at)").Scan(&shLast)
	stats = append(stats, DataStat{Key: "shareholder", Label: "股东数据", Count: shCount, UpdatedAt: &shLast})

	// news count
	var newsCount int64
	var newsLast time.Time
	db.PG.Table("stock_news").Count(&newsCount)
	db.PG.Table("stock_news").Select("MAX(created_at)").Scan(&newsLast)
	stats = append(stats, DataStat{Key: "news", Label: "资讯数据", Count: newsCount, UpdatedAt: &newsLast})

	// reports count
	var reportsCount int64
	var reportsLast time.Time
	db.PG.Table("stock_reports").Count(&reportsCount)
	db.PG.Table("stock_reports").Select("MAX(created_at)").Scan(&reportsLast)
	stats = append(stats, DataStat{Key: "reports", Label: "研报数据", Count: reportsCount, UpdatedAt: &reportsLast})

	// algorithm picks (Excel imported board data)
	var picksCount int64
	var picksLast time.Time
	db.PG.Table("algorithm_picks").Count(&picksCount)
	db.PG.Table("algorithm_picks").Select("MAX(pick_date)").Scan(&picksLast)
	stats = append(stats, DataStat{Key: "board_picks", Label: "上榜批次", Count: picksCount, UpdatedAt: &picksLast})

	var pickDetailsCount int64
	db.PG.Table("algorithm_pick_details").Count(&pickDetailsCount)
	stats = append(stats, DataStat{Key: "board_details", Label: "上榜明细记录", Count: pickDetailsCount})

	// signals count
	var signalsCount int64
	db.PG.Table("stock_signals").Count(&signalsCount)
	stats = append(stats, DataStat{Key: "signals", Label: "信号数据", Count: signalsCount})

	return stats
}

// StockDataCoverage represents per-stock data summary
type StockDataCoverage struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Count     int64      `json:"count"`
	FirstDate *time.Time `json:"firstDate,omitempty"`
	LastDate  *time.Time `json:"lastDate,omitempty"`
}

func GetDataDetail(typ string) []StockDataCoverage {
	var results []StockDataCoverage

	switch typ {
	case "stocks":
		db.PG.Raw(`
			SELECT sb.code, sb.name, 1 as count, sb.listed_date as first_date
			FROM stocks_basic sb
			ORDER BY sb.code
		`).Scan(&results)

	case "kline":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(k.cnt, 0) as count,
				k.first_date, k.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM stocks_daily_k GROUP BY code
			) k ON k.code = sb.code
			ORDER BY COALESCE(k.cnt, 0) DESC
		`).Scan(&results)

	case "indicator":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(i.cnt, 0) as count,
				i.first_date, i.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM stocks_daily_indicator GROUP BY code
			) i ON i.code = sb.code
			ORDER BY COALESCE(i.cnt, 0) DESC
		`).Scan(&results)

	case "quote":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				CASE WHEN q.code IS NOT NULL THEN 1 ELSE 0 END as count,
				NULL::timestamp as first_date, q.updated_at as last_date
			FROM stocks_basic sb
			LEFT JOIN stock_quotes q ON q.code = sb.code
			ORDER BY count DESC, sb.code
		`).Scan(&results)

	case "financial":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(f.cnt, 0) as count,
				f.first_date, f.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(report_date) as first_date, MAX(report_date) as last_date
				FROM stock_financials GROUP BY code
			) f ON f.code = sb.code
			ORDER BY COALESCE(f.cnt, 0) DESC
		`).Scan(&results)

	case "shareholder":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(s.cnt, 0) as count,
				s.first_date, s.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(report_date) as first_date, MAX(report_date) as last_date
				FROM stock_shareholders GROUP BY code
			) s ON s.code = sb.code
			ORDER BY COALESCE(s.cnt, 0) DESC
		`).Scan(&results)

	case "news":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(n.cnt, 0) as count,
				n.first_date, n.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(publish_date) as first_date, MAX(publish_date) as last_date
				FROM stock_news GROUP BY code
			) n ON n.code = sb.code
			ORDER BY COALESCE(n.cnt, 0) DESC
		`).Scan(&results)

	case "reports":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(r.cnt, 0) as count,
				r.first_date, r.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT stock_code as code, COUNT(*) as cnt, MIN(publish_date) as first_date, MAX(publish_date) as last_date
				FROM stock_reports GROUP BY stock_code
			) r ON r.code = sb.code
			ORDER BY COALESCE(r.cnt, 0) DESC
		`).Scan(&results)

	case "signals":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				CASE WHEN sig.code IS NOT NULL THEN 1 ELSE 0 END as count,
				NULL::timestamp as first_date, sig.updated_at as last_date
			FROM stocks_basic sb
			LEFT JOIN stock_signals sig ON sig.code = sb.code
			ORDER BY count DESC, sb.code
		`).Scan(&results)

	case "board_picks":
		db.PG.Raw(`
			SELECT '' as code, TO_CHAR(pick_date, 'YYYY-MM-DD') as name,
				total_stocks as count,
				pick_date as first_date, generated_at as last_date
			FROM algorithm_picks
			ORDER BY pick_date DESC
		`).Scan(&results)

	case "board_details":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COUNT(*) as count,
				MIN(pick_date) as first_date, MAX(pick_date) as last_date
			FROM algorithm_pick_details apd
			JOIN stocks_basic sb ON sb.code = apd.stock_code
			GROUP BY sb.code, sb.name
			ORDER BY count DESC
		`).Scan(&results)
	}

	return results
}
