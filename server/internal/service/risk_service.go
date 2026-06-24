package service

import (
	"fmt"
	"strings"
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type RiskRule struct {
	Type        string
	Level       string
	Description string
}

// ScanUserHoldings scans all user holdings and generates risk alerts

func codesToInClause(codes []string) string {
	if len(codes) == 0 { return "''" }
	q := make([]string, len(codes))
	for i, c := range codes { q[i] = "'" + c + "'" }
	return strings.Join(q, ",")
}

func ScanUserHoldings() (int, error) {
	var holdings []model.Holding
	db.MySQL.Find(&holdings)
	if len(holdings) == 0 {
		return 0, nil
	}

	// Collect unique stock codes from all holdings
	codeSet := make(map[string]bool)
	for _, h := range holdings {
		codeSet[h.StockCode] = true
	}
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}
	inClause := codesToInClause(codes)

	// Clear old non-ignored alerts for fresh scan
	db.MySQL.Where("ignored = false").Delete(&model.RiskAlert{})

	now := time.Now()
	var alerts []model.RiskAlert
	added := make(map[string]bool) // "code:type" dedup

	addAlert := func(code, level, typ, desc string) {
		key := code + ":" + typ + ":" + desc
		if added[key] {
			return
		}
		// Check DB for existing identical alert today
		var existing int64
		db.MySQL.Model(&model.RiskAlert{}).
			Where("stock_code = ? AND type = ? AND description = ? AND DATE(hit_date) = CURRENT_DATE", code, typ, desc).
			Count(&existing)
		if existing > 0 {
			added[key] = true
			return
		}
		added[key] = true
		alerts = append(alerts, model.RiskAlert{
			StockCode:   code,
			Level:       level,
			Type:        typ,
			Description: desc,
			HitDate:     now,
		})
	}

	// ── Rule 1: 近期大跌 (last 5 trading days cumulative drop > 8%) ──
	type KlineChg struct {
		Code   string
		ChgPct float64
	}
	var drops []KlineChg
	db.PG.Raw(fmt.Sprintf(`
		WITH recent AS (
			SELECT code, close,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k
			WHERE code IN (%s)
		)
		SELECT r5.code, ((r5.close - r1.close) / NULLIF(r1.close, 0) * 100) as chg_pct
		FROM (SELECT code, close FROM recent WHERE rn = 1) r1
		JOIN (SELECT code, close FROM recent WHERE rn = 5) r5 ON r5.code = r1.code
		WHERE r1.close > 0
	`, inClause)).Scan(&drops)

	for _, d := range drops {
		if d.ChgPct < -8 {
			addAlert(d.Code, "high", "近期大跌", "近5个交易日累计跌幅 "+(fmtPct(d.ChgPct)))
		} else if d.ChgPct < -5 {
			addAlert(d.Code, "medium", "短期走弱", "近5个交易日累计跌幅 "+(fmtPct(d.ChgPct)))
		}
	}

	// ── Rule 2: 跌破均线 (price crossed below MA20 today) ──
	type MACross struct {
		Code  string
		Close float64
		MA20  float64
		PrevC float64
		PrevM float64
	}
	var crosses []MACross
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, trade_date, close,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k
			WHERE code IN (%s)
		)
		SELECT t1.code, t1.close, t1.ma20, t2.close as prev_c, t2.ma20 as prev_m
		FROM ranked t1
		JOIN ranked t2 ON t2.code = t1.code AND t2.rn = 2
		WHERE t1.rn = 1 AND t1.close < t1.ma20 AND t2.close >= t2.ma20
	`, inClause)).Scan(&crosses)

	for _, c := range crosses {
		addAlert(c.Code, "medium", "跌破均线", "收盘价 "+fmtPrice(c.Close)+" 跌破20日均线 "+fmtPrice(c.MA20))
	}

	// ── Rule 3: 估值偏高 (PE > 100 or PE > industry average * 3) ──
	type PEInfo struct {
		Code   string
		PE     float64
		IndAvg float64
	}
	var peInfos []PEInfo
	db.PG.Raw(fmt.Sprintf(`
		WITH latest_pe AS (
			SELECT DISTINCT ON (code) code, pe as pe
			FROM stocks_daily_indicator
			WHERE code IN (%s) AND pe > 0
			ORDER BY code, trade_date DESC
		),
		ind_avg AS (
			SELECT sb.industry, AVG(i.pe) as avg_pe
			FROM stocks_daily_indicator i
			JOIN stocks_basic sb ON sb.code = i.code
			WHERE i.code = %s AND i.pe > 0 AND i.pe < 500
				AND i.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_indicator WHERE code = i.code)
			GROUP BY sb.industry
		)
		SELECT lp.code, lp.pe, COALESCE(ia.avg_pe, 0) as ind_avg
		FROM latest_pe lp
		JOIN stocks_basic sb ON sb.code = lp.code
		LEFT JOIN ind_avg ia ON ia.industry = sb.industry
	`, inClause)).Scan(&peInfos)

	for _, p := range peInfos {
		if p.PE > 200 {
			addAlert(p.Code, "high", "估值极高", "市盈率 "+fmt.Sprintf("%.2f", p.PE)+"，远超合理范围")
		} else if p.PE > 100 {
			addAlert(p.Code, "medium", "估值偏高", "市盈率 "+fmt.Sprintf("%.2f", p.PE)+"（行业均值 "+fmt.Sprintf("%.2f", p.IndAvg)+"）")
		}
	}

	// ── Rule 4: 连续上榜 (stock on board for 5+ recent picks) ──
	type BoardCount struct {
		Code string
		Cnt  int
	}
	var boardCounts []BoardCount
	db.PG.Raw(fmt.Sprintf(`
		SELECT stock_code as code, COUNT(*) as cnt
		FROM algorithm_pick_details
		WHERE stock_code IN (%s)
			AND pick_date >= (SELECT MAX(pick_date) FROM algorithm_picks) - INTERVAL '20 days'
		GROUP BY stock_code
		HAVING COUNT(*) >= 5
	`, inClause)).Scan(&boardCounts)

	for _, b := range boardCounts {
		addAlert(b.Code, "medium", "连续上榜", "近20天上榜 "+fmt.Sprintf("%d", b.Cnt)+" 次，短线情绪过热")
	}

	// ── Rule 5: 业绩下滑 (latest financial shows profit decline > 30%) ──
	type FinDecline struct {
		Code    string
		Chg     float64
		RevChg  float64
	}
	var declines []FinDecline
	db.PG.Raw(fmt.Sprintf(`
		WITH latest AS (
			SELECT DISTINCT ON (code) code, profit_growth, revenue_growth
			FROM stock_financials
			WHERE code IN (%s)
			ORDER BY code, report_date DESC
		)
		SELECT code, profit_growth as chg, revenue_growth as rev_chg
		FROM latest
		WHERE profit_growth < -30
	`, inClause)).Scan(&declines)

	for _, d := range declines {
		level := "medium"
		if d.Chg < -50 {
			level = "high"
		}
		addAlert(d.Code, level, "业绩下滑", "最新财报净利润同比 "+(fmtPct(d.Chg))+"，营收同比 "+(fmtPct(d.RevChg)))
	}

	// ── Batch insert ──
	if len(alerts) > 0 {
		if err := db.MySQL.Create(&alerts).Error; err != nil {
			log.Printf("[RiskService] batch insert error: %v", err)
			return 0, err
		}
	}

	log.Printf("[RiskService] scan complete: %d alerts for %d stocks from %d holdings",
		len(alerts), len(codes), len(holdings))
	return len(alerts), nil
}

// GetUserRiskAlerts returns risk alerts for stocks in user's holdings
func GetUserRiskAlerts(userID uint) ([]model.RiskAlert, error) {
	var codes []string
	db.MySQL.Model(&model.Holding{}).Where("user_id = ?", userID).Pluck("stock_code", &codes)

	if len(codes) == 0 {
		return []model.RiskAlert{}, nil
	}

	var alerts []model.RiskAlert
	db.MySQL.Where("stock_code IN ? AND ignored = false", codes).
		Order("FIELD(level, 'high','medium','low'), hit_date DESC").
		Find(&alerts)

	// Enrich with stock names from PostgreSQL
	if len(alerts) > 0 {
		type NameRow struct {
			Code string
			Name string
		}
		var names []NameRow
		alertCodes := make([]string, len(alerts))
		for i, a := range alerts {
			alertCodes[i] = a.StockCode
		}
		db.PG.Raw(fmt.Sprintf("SELECT code, name FROM stocks_basic WHERE code IN (%s)", codesToInClause(alertCodes))).Scan(&names)
		nameMap := make(map[string]string, len(names))
		for _, n := range names {
			nameMap[n.Code] = n.Name
		}
		for i := range alerts {
			alerts[i].StockName = nameMap[alerts[i].StockCode]
		}
	}

	return alerts, nil
}

// IgnoreRiskAlert marks an alert as ignored
func IgnoreRiskAlert(userID, alertID uint) error {
	// Verify the alert belongs to user's holdings
	var codes []string
	db.MySQL.Model(&model.Holding{}).Where("user_id = ?", userID).Pluck("stock_code", &codes)

	return db.MySQL.Model(&model.RiskAlert{}).
		Where("id = ? AND stock_code IN ?", alertID, codes).
		Update("ignored", true).Error
}

// Helper formatters
func fmtPct(v float64) string {
	return fmt.Sprintf("%.0f%%", v)
}

func fmtPrice(v float64) string {
	return fmt.Sprintf("%.2f", v)
}
