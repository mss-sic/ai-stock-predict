package repository

import (
	"log"
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
	if err := db.PG.Table("stocks_daily_k").Select("MAX(trade_date)").Scan(&klineLast).Error; err != nil {
		log.Printf("[stats] kline last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "kline", Label: "日K线数据", Count: klineCount, UpdatedAt: &klineLast})

	// indicator count
	var indCount int64
	var indLast time.Time
	db.PG.Table("stocks_daily_indicator").Count(&indCount)
	if err := db.PG.Table("stocks_daily_indicator").Select("MAX(trade_date)").Scan(&indLast).Error; err != nil {
		log.Printf("[stats] indicator last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "indicator", Label: "PE/PB 指标数据", Count: indCount, UpdatedAt: &indLast})

	// financial count
	var finCount int64
	var finLast time.Time
	db.PG.Table("stock_financials").Count(&finCount)
	if err := db.PG.Table("stock_financials").Select("MAX(created_at)").Scan(&finLast).Error; err != nil {
		log.Printf("[stats] financials last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "financial", Label: "财务数据", Count: finCount, UpdatedAt: &finLast})

	// shareholder count
	var shCount int64
	var shLast time.Time
	db.PG.Table("stock_shareholders").Count(&shCount)
	if err := db.PG.Table("stock_shareholders").Select("MAX(created_at)").Scan(&shLast).Error; err != nil {
		log.Printf("[stats] shareholders last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "shareholder", Label: "股东数据", Count: shCount, UpdatedAt: &shLast})

	// news count
	var newsCount int64
	var newsLast time.Time
	db.PG.Table("stock_news").Count(&newsCount)
	if err := db.PG.Table("stock_news").Select("MAX(created_at)").Scan(&newsLast).Error; err != nil {
		log.Printf("[stats] news last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "news", Label: "资讯数据", Count: newsCount, UpdatedAt: &newsLast})

	// reports count
	var reportsCount int64
	var reportsLast time.Time
	db.PG.Table("stock_reports").Count(&reportsCount)
	if err := db.PG.Table("stock_reports").Select("MAX(created_at)").Scan(&reportsLast).Error; err != nil {
		log.Printf("[stats] reports last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "reports", Label: "研报数据", Count: reportsCount, UpdatedAt: &reportsLast})

	// algorithm picks (Excel imported board data)
	var picksCount int64
	var picksLast time.Time
	db.PG.Table("algorithm_picks").Count(&picksCount)
	if err := db.PG.Table("algorithm_picks").Select("MAX(pick_date)").Scan(&picksLast).Error; err != nil {
		log.Printf("[stats] picks last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "board_picks", Label: "上榜批次", Count: picksCount, UpdatedAt: &picksLast})

	var pickDetailsCount int64
	db.PG.Table("algorithm_pick_details").Count(&pickDetailsCount)
	stats = append(stats, DataStat{Key: "board_details", Label: "上榜明细记录", Count: pickDetailsCount})

	// signals count
	var signalsCount int64
	db.PG.Table("stock_signals").Count(&signalsCount)
	stats = append(stats, DataStat{Key: "signals", Label: "信号数据", Count: signalsCount})

	// ── 新增数据源统计 (v041+) ──

	// concept
	var conceptCount int64
	var conceptLast time.Time
	db.PG.Table("stock_concepts").Count(&conceptCount)
	if err := db.PG.Table("stock_concepts").Select("MAX(updated_at)").Scan(&conceptLast).Error; err != nil {
		log.Printf("[stats] concept last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "concept", Label: "概念板块关联", Count: conceptCount, UpdatedAt: &conceptLast})

	// dragon_tiger
	var dtCount int64
	var dtLast time.Time
	db.PG.Table("dragon_tiger_list").Count(&dtCount)
	if err := db.PG.Table("dragon_tiger_list").Select("MAX(trade_date)").Scan(&dtLast).Error; err != nil {
		log.Printf("[stats] dragon_tiger last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "dragon_tiger", Label: "龙虎榜上榜", Count: dtCount, UpdatedAt: &dtLast})

	// margin
	var marginCount int64
	var marginLast time.Time
	db.PG.Table("margin_trading").Count(&marginCount)
	if err := db.PG.Table("margin_trading").Select("MAX(trade_date)").Scan(&marginLast).Error; err != nil {
		log.Printf("[stats] margin last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "margin", Label: "融资融券", Count: marginCount, UpdatedAt: &marginLast})

	// block_trade
	var btCount int64
	var btLast time.Time
	db.PG.Table("block_trade").Count(&btCount)
	if err := db.PG.Table("block_trade").Select("MAX(trade_date)").Scan(&btLast).Error; err != nil {
		log.Printf("[stats] block_trade last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "block_trade", Label: "大宗交易", Count: btCount, UpdatedAt: &btLast})

	// unlock
	var unlockCount int64
	var unlockLast time.Time
	db.PG.Table("restricted_share_unlock").Count(&unlockCount)
	if err := db.PG.Table("restricted_share_unlock").Select("MAX(unlock_date)").Scan(&unlockLast).Error; err != nil {
		log.Printf("[stats] unlock last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "unlock", Label: "限售解禁", Count: unlockCount, UpdatedAt: &unlockLast})

	// ths_hot
	var thsHotCount int64
	var thsHotLast time.Time
	db.PG.Table("ths_hot_stocks").Count(&thsHotCount)
	if err := db.PG.Table("ths_hot_stocks").Select("MAX(trade_date)").Scan(&thsHotLast).Error; err != nil {
		log.Printf("[stats] ths_hot last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "ths_hot", Label: "同花顺热点", Count: thsHotCount, UpdatedAt: &thsHotLast})

	// dividend
	var divCount int64
	var divLast time.Time
	db.PG.Table("dividend_history").Count(&divCount)
	if err := db.PG.Table("dividend_history").Select("MAX(created_at)").Scan(&divLast).Error; err != nil {
		log.Printf("[stats] dividend last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "dividend", Label: "分红送转", Count: divCount, UpdatedAt: &divLast})

	// ths_eps
	var epsCount int64
	var epsLast time.Time
	db.PG.Table("ths_eps_forecast").Count(&epsCount)
	if err := db.PG.Table("ths_eps_forecast").Select("MAX(created_at)").Scan(&epsLast).Error; err != nil {
		log.Printf("[stats] ths_eps last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "ths_eps", Label: "一致预期EPS", Count: epsCount, UpdatedAt: &epsLast})

	// cninfo
	var cninfoCount int64
	var cninfoLast time.Time
	db.PG.Table("cninfo_announcements").Count(&cninfoCount)
	if err := db.PG.Table("cninfo_announcements").Select("MAX(ann_date)").Scan(&cninfoLast).Error; err != nil {
		log.Printf("[stats] cninfo last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "cninfo", Label: "巨潮公告", Count: cninfoCount, UpdatedAt: &cninfoLast})

	// macro_news
	var macroCount int64
	var macroLast time.Time
	db.PG.Table("macro_news").Count(&macroCount)
	if err := db.PG.Table("macro_news").Select("MAX(created_at)").Scan(&macroLast).Error; err != nil {
		log.Printf("[stats] macro_news last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "macro_news", Label: "宏观资讯", Count: macroCount, UpdatedAt: &macroLast})

	// market_daily_agg
	var mdaCount int64
	var mdaLast time.Time
	db.PG.Table("market_daily_agg").Count(&mdaCount)
	if err := db.PG.Table("market_daily_agg").Select("MAX(trade_date)").Scan(&mdaLast).Error; err != nil {
		log.Printf("[stats] market_daily_agg last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "market_daily_agg", Label: "市场日聚合", Count: mdaCount, UpdatedAt: &mdaLast})

	// market_sentiment
	var msCount int64
	var msLast time.Time
	db.PG.Table("market_sentiment").Count(&msCount)
	if err := db.PG.Table("market_sentiment").Select("MAX(trade_date)").Scan(&msLast).Error; err != nil {
		log.Printf("[stats] market_sentiment last date query failed: %v", err)
	}
	stats = append(stats, DataStat{Key: "market_sentiment", Label: "市场情绪", Count: msCount, UpdatedAt: &msLast})

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

	// ── 新增数据源明细 (v041+) ──

	case "concept":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(c.cnt, 0) as count,
				NULL::timestamp as first_date, c.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MAX(updated_at) as last_date
				FROM stock_concepts GROUP BY code
			) c ON c.code = sb.code
			ORDER BY COALESCE(c.cnt, 0) DESC
		`).Scan(&results)

	case "dragon_tiger":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(dt.cnt, 0) as count,
				dt.first_date, dt.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM dragon_tiger_list GROUP BY code
			) dt ON dt.code = sb.code
			ORDER BY COALESCE(dt.cnt, 0) DESC
		`).Scan(&results)

	case "margin":
		db.PG.Raw(`
			SELECT m.code, sb.name,
				COALESCE(m.cnt, 0) as count,
				m.first_date, m.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM margin_trading GROUP BY code
			) m
			LEFT JOIN stocks_basic sb ON sb.code = m.code
			ORDER BY COALESCE(m.cnt, 0) DESC
		`).Scan(&results)

	case "block_trade":
		db.PG.Raw(`
			SELECT bt.code, sb.name,
				COALESCE(bt.cnt, 0) as count,
				bt.first_date, bt.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM block_trade GROUP BY code
			) bt
			LEFT JOIN stocks_basic sb ON sb.code = bt.code
			ORDER BY COALESCE(bt.cnt, 0) DESC
		`).Scan(&results)

	case "unlock":
		db.PG.Raw(`
			SELECT ru.code, sb.name,
				COALESCE(ru.cnt, 0) as count,
				ru.first_date, ru.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(unlock_date) as first_date, MAX(unlock_date) as last_date
				FROM restricted_share_unlock GROUP BY code
			) ru
			LEFT JOIN stocks_basic sb ON sb.code = ru.code
			ORDER BY COALESCE(ru.cnt, 0) DESC
		`).Scan(&results)

	case "ths_hot":
		db.PG.Raw(`
			SELECT th.code, sb.name,
				COALESCE(th.cnt, 0) as count,
				th.first_date, th.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM ths_hot_stocks GROUP BY code
			) th
			LEFT JOIN stocks_basic sb ON sb.code = th.code
			ORDER BY COALESCE(th.cnt, 0) DESC
		`).Scan(&results)

	case "dividend":
		db.PG.Raw(`
			SELECT dh.code, sb.name,
				COALESCE(dh.cnt, 0) as count,
				dh.first_date, dh.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(ex_dividend_date) as first_date, MAX(ex_dividend_date) as last_date
				FROM dividend_history GROUP BY code
			) dh
			LEFT JOIN stocks_basic sb ON sb.code = dh.code
			ORDER BY COALESCE(dh.cnt, 0) DESC
		`).Scan(&results)

	case "ths_eps":
		db.PG.Raw(`
			SELECT te.code, sb.name,
				COALESCE(te.cnt, 0) as count,
				NULL::timestamp as first_date, te.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MAX(created_at) as last_date
				FROM ths_eps_forecast GROUP BY code
			) te
			LEFT JOIN stocks_basic sb ON sb.code = te.code
			ORDER BY COALESCE(te.cnt, 0) DESC
		`).Scan(&results)

	case "cninfo":
		db.PG.Raw(`
			SELECT ca.code, sb.name,
				COALESCE(ca.cnt, 0) as count,
				ca.first_date, ca.last_date
			FROM (
				SELECT code, COUNT(*) as cnt, MIN(ann_date) as first_date, MAX(ann_date) as last_date
				FROM cninfo_announcements GROUP BY code
			) ca
			LEFT JOIN stocks_basic sb ON sb.code = ca.code
			ORDER BY COALESCE(ca.cnt, 0) DESC
		`).Scan(&results)

	case "macro_news":
		db.PG.Raw(`
			SELECT '' as code, COALESCE(category, 'general') as name,
				COUNT(*) as count,
				MIN(news_time::timestamp) as first_date, MAX(news_time::timestamp) as last_date
			FROM macro_news
			GROUP BY category
			ORDER BY count DESC
		`).Scan(&results)

	case "market_daily_agg":
		db.PG.Raw(`
			SELECT TO_CHAR(trade_date, 'YYYY-MM-DD') as code, TO_CHAR(trade_date, 'YYYY-MM-DD') as name,
				1 as count,
				trade_date as first_date, trade_date as last_date
			FROM market_daily_agg
			ORDER BY trade_date DESC
		`).Scan(&results)

	case "market_sentiment":
		db.PG.Raw(`
			SELECT TO_CHAR(trade_date, 'YYYY-MM-DD') as code, TO_CHAR(trade_date, 'YYYY-MM-DD') as name,
				1 as count,
				trade_date as first_date, trade_date as last_date
			FROM market_sentiment
			ORDER BY trade_date DESC
		`).Scan(&results)
	}

	return results
}
