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

// statsCache holds cached data stats with a short TTL
var statsCache = struct {
	data      []DataStat
	expiresAt time.Time
}{}

func GetDataStats() []DataStat {
	// Return cached result if still fresh (30s TTL)
	if time.Now().Before(statsCache.expiresAt) && len(statsCache.data) > 0 {
		return statsCache.data
	}

	type statJob struct {
		key   string
		label string
		table string
		dateColumn string // trade_date, created_at, or empty for count-only
	}
	jobs := []statJob{
		{"stocks", "股票基础数据", "stocks_basic", ""},
		{"kline", "日K线数据", "stocks_daily_k", "trade_date"},
		{"indicator", "PE/PB 指标数据", "stocks_daily_indicator", "trade_date"},
		{"financial", "财务数据", "stock_financials", "created_at"},
		{"shareholder", "股东数据", "stock_shareholders", "created_at"},
		{"news", "资讯数据", "stock_news", "created_at"},
		{"reports", "研报数据", "stock_reports", "created_at"},
		{"board_picks", "上榜批次", "algorithm_picks", "pick_date"},
		{"board_details", "上榜明细记录", "algorithm_pick_details", ""},
		{"signals", "信号数据", "stock_signals", ""},
		{"concept", "概念板块关联", "stock_concepts", "updated_at"},
		{"dragon_tiger", "龙虎榜上榜", "dragon_tiger_list", "trade_date"},
		{"margin", "融资融券", "margin_trading", "trade_date"},
		{"block_trade", "大宗交易", "block_trade", "trade_date"},
		{"unlock", "限售解禁", "restricted_share_unlock", "unlock_date"},
		{"ths_hot", "同花顺热点", "ths_hot_stocks", "trade_date"},
		{"dividend", "分红送转", "dividend_history", "ex_dividend_date"},
		{"ths_eps", "一致预期EPS", "ths_eps_forecast", "created_at"},
		{"cninfo", "巨潮公告", "cninfo_announcements", "ann_date"},
		{"macro_news", "宏观资讯", "macro_news", "created_at"},
		{"market_daily_agg", "市场日聚合", "market_daily_agg", "trade_date"},
		{"market_sentiment", "市场情绪", "market_sentiment", "trade_date"},
		{"fund_flow", "资金流向", "stock_fund_flow", "trade_date"},
	}

	type result struct {
		idx   int
		key   string
		label string
		count int64
		last  *time.Time
	}

	ch := make(chan result, len(jobs))
	for i, j := range jobs {
		go func(idx int, job statJob) {
			var count int64
			var last time.Time
			var lastPtr *time.Time

			if err := db.PG.Table(job.table).Count(&count).Error; err != nil {
				log.Printf("[stats] %s count failed: %v", job.key, err)
			}

			if job.dateColumn != "" {
				if err := db.PG.Table(job.table).Select("MAX(" + job.dateColumn + ")").Scan(&last).Error; err != nil {
					log.Printf("[stats] %s last date failed: %v", job.key, err)
				}
				if !last.IsZero() {
					lastPtr = &last
				}
			}

			ch <- result{idx, job.key, job.label, count, lastPtr}
		}(i, j)
	}

	// Collect results in order
	results := make([]result, len(jobs))
	for range jobs {
		r := <-ch
		results[r.idx] = r
	}

	stats := make([]DataStat, 0, len(jobs))
	for _, r := range results {
		stats = append(stats, DataStat{Key: r.key, Label: r.label, Count: r.count, UpdatedAt: r.last})
	}

	// Cache for 30 seconds
	statsCache.data = stats
	statsCache.expiresAt = time.Now().Add(30 * time.Second)

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

	case "fund_flow":
		db.PG.Raw(`
			SELECT sb.code, sb.name,
				COALESCE(ff.cnt, 0) as count,
				ff.first_date, ff.last_date
			FROM stocks_basic sb
			LEFT JOIN (
				SELECT code, COUNT(*) as cnt, MIN(trade_date) as first_date, MAX(trade_date) as last_date
				FROM stock_fund_flow GROUP BY code
			) ff ON ff.code = sb.code
			ORDER BY COALESCE(ff.cnt, 0) DESC
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
