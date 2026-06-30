package service

import (
	"math"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// GetLatestSentiment returns the most recent market sentiment record.
func GetLatestSentiment() (*model.MarketSentiment, error) {
	var s model.MarketSentiment
	err := db.PG.Order("trade_date DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSentimentHistory returns sentiment records for the last N trading days.
func GetSentimentHistory(days int) ([]model.MarketSentiment, error) {
	var list []model.MarketSentiment
	err := db.PG.Order("trade_date DESC").Limit(days).Find(&list).Error
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}

// GetSentimentByDate returns sentiment for a specific date.
func GetSentimentByDate(date time.Time) (*model.MarketSentiment, error) {
	var s model.MarketSentiment
	err := db.PG.Where("trade_date = ?", date.Format("2006-01-02")).First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetSentimentDateRange returns sentiment records between start and end dates.
func GetSentimentDateRange(start, end time.Time) ([]model.MarketSentiment, error) {
	var list []model.MarketSentiment
	err := db.PG.Where("trade_date >= ? AND trade_date <= ?",
		start.Format("2006-01-02"), end.Format("2006-01-02")).
		Order("trade_date ASC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetNorthboundMinuteHistory returns recent minute-level northbound data.
func GetNorthboundMinuteHistory(days int) ([]model.NorthboundMinute, error) {
	var list []model.NorthboundMinute
	err := db.PG.Where("trade_date >= CURRENT_DATE - ?::integer", days).
		Order("trade_date DESC, time DESC").Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// GetNorthboundHistory returns recent daily northbound flow from the view.
func GetNorthboundHistory(days int) ([]model.NorthboundDailyView, error) {
	var list []model.NorthboundDailyView
	err := db.PG.Table("northbound_daily_view").Order("trade_date DESC").Limit(days).Find(&list).Error
	if err != nil {
		return nil, err
	}
	return list, nil
}

// LimitStats represents daily limit-up/down statistics.
type LimitStats struct {
	TradeDate    string  `gorm:"column:trade_date" json:"tradeDate"`
	UpCount      int     `gorm:"column:up_count" json:"upCount"`
	DownCount    int     `gorm:"column:down_count" json:"downCount"`
	RiseCount    int     `gorm:"column:rise_count" json:"riseCount"`
	FallCount    int     `gorm:"column:fall_count" json:"fallCount"`
	BoardBreak   int     `gorm:"column:board_break" json:"boardBreak"`
	MaxStreak    int     `gorm:"column:max_streak" json:"maxStreak"`
	TotalStocks  int     `gorm:"column:total_stocks" json:"totalStocks"`
}

// GetLimitStatsHistory returns daily limit-up/down statistics for the last N days.
// Reads from pre-computed limit_stats_daily table (populated by collect_limit_stats.py).
func GetLimitStatsHistory(days int) ([]LimitStats, error) {
	var list []LimitStats
	err := db.PG.Raw(`
		SELECT trade_date::text, up_count, down_count, rise_count, fall_count,
		       board_break, 0 as max_streak, total_stocks
		FROM limit_stats_daily
		ORDER BY trade_date DESC
		LIMIT ?
	`, days).Scan(&list).Error
	if err != nil {
		return nil, err
	}
	// Reverse to ascending for chart display
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, nil
}


// ── Fear & Greed Index ──────────────────────────────────────────

// FearGreedFactor is one sub-factor of the Fear & Greed composite.
type FearGreedFactor struct {
	Name  string  `json:"name"`
	Raw   float64 `json:"raw"`
	Score float64 `json:"score"` // 0-100
	Label string  `json:"label"`
}

// FearGreedData is the daily Fear & Greed composite with sub-factors.
type FearGreedData struct {
	TradeDate string            `json:"tradeDate"`
	Score     float64           `json:"score"`
	Zone      string            `json:"zone"`
	Factors   []FearGreedFactor `json:"factors"`
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}

// linearScore maps a raw value to 0-100 via piecewise thresholds.
// thresholds = [t0, t1, t2, t3] where:
//
//	raw <= t0 → 0, raw >= t3 → 100,
//	t0..t1 → 0..40, t1..t2 → 40..60, t2..t3 → 60..100
func linearScore(raw float64, thresholds [4]float64) float64 {
	t0, t1, t2, t3 := thresholds[0], thresholds[1], thresholds[2], thresholds[3]
	if raw <= t0 {
		return 0
	}
	if raw >= t3 {
		return 100
	}
	if raw <= t1 {
		return (raw - t0) / (t1 - t0) * 40
	}
	if raw <= t2 {
		return 40 + (raw-t1)/(t2-t1)*20
	}
	return 60 + (raw-t2)/(t3-t2)*40
}

// fearGreedZone maps score to label.
func fearGreedZone(score float64) string {
	switch {
	case score <= 25:
		return "极度恐惧"
	case score <= 45:
		return "恐惧"
	case score <= 55:
		return "中性"
	case score <= 75:
		return "贪婪"
	default:
		return "极度贪婪"
	}
}

// ComputeFearGreedLatest computes the Fear & Greed index for the latest trading day.
func ComputeFearGreedLatest() (*FearGreedData, error) {
	// ── fetch one row from limit_stats_daily ──
	var ls LimitStats
	if err := db.PG.Raw(`
		SELECT trade_date::text, up_count, down_count, rise_count, fall_count, board_break, total_stocks
		FROM limit_stats_daily ORDER BY trade_date DESC LIMIT 1
	`).Scan(&ls).Error; err != nil {
		return nil, err
	}

	// ── F1: 涨跌停比 ──
	upDn := float64(ls.UpCount) / math.Max(float64(ls.DownCount), 1)
	f1 := FearGreedFactor{
		Name: "涨跌停比", Raw: math.Round(upDn*100) / 100,
		Score: linearScore(upDn, [4]float64{0.3, 0.8, 1.5, 2.5}),
		Label: fmtRatioLabel(upDn),
	}

	// ── F2: 涨跌家数比 ──
	advDec := float64(ls.RiseCount) / math.Max(float64(ls.FallCount), 1)
	f2 := FearGreedFactor{
		Name: "涨跌比", Raw: math.Round(advDec*100) / 100,
		Score: linearScore(advDec, [4]float64{0.5, 0.9, 1.5, 2.5}),
		Label: fmtRatioLabel(advDec),
	}

	// ── F3: 成交量偏离 (today vs 20-day MA) ──
	var volToday, volAvg float64
	// Use T-1 (previous full trading day) for volume to avoid intraday distortion
	db.PG.Raw(`
		WITH dv AS (
			SELECT trade_date, SUM(volume) as tv FROM stocks_daily_k
			WHERE code !~ '^IDX' AND volume > 0
			GROUP BY trade_date ORDER BY trade_date DESC LIMIT 22
		)
		SELECT (SELECT tv FROM dv ORDER BY trade_date DESC LIMIT 1 OFFSET 1),
		       (SELECT AVG(tv) FROM (SELECT tv FROM dv ORDER BY trade_date DESC OFFSET 2 LIMIT 20) s)
	`).Row().Scan(&volToday, &volAvg)

	volDev := float64(0)
	if volAvg > 0 {
		volDev = (volToday - volAvg) / volAvg * 100
	}
	f3 := FearGreedFactor{
		Name: "量能偏离", Raw: math.Round(volDev*10) / 10,
		Score: linearScore(volDev, [4]float64{-25, -10, 10, 25}),
		Label: fmtVolLabel(volDev) + " (T-1)",
	}

	// ── F4: 北向资金 (z-score vs 60-day) ──
	var nbToday, nbAvg, nbStd float64
	db.PG.Raw(`
		WITH nb AS (
			SELECT trade_date, total_net FROM northbound_daily_view
			ORDER BY trade_date DESC LIMIT 61
		)
		SELECT COALESCE((SELECT total_net FROM nb ORDER BY trade_date DESC LIMIT 1), 0),
		       COALESCE((SELECT AVG(total_net) FROM (SELECT total_net FROM nb ORDER BY trade_date DESC OFFSET 1 LIMIT 60) s), 0),
		       COALESCE((SELECT STDDEV(total_net) FROM (SELECT total_net FROM nb ORDER BY trade_date DESC OFFSET 1 LIMIT 60) s), 0)
	`).Row().Scan(&nbToday, &nbAvg, &nbStd)

	nbZ := float64(0)
	if nbStd > 1 {
		nbZ = (nbToday - nbAvg) / nbStd
	}
	nbScore := clamp(50+nbZ*12, 0, 100)
	f4 := FearGreedFactor{
		Name: "北向资金", Raw: math.Round(nbToday*100) / 100,
		Score: nbScore,
		Label: fmtNBLabel(nbToday),
	}

	// ── F5: 波动率 (20-day std of index returns) ──
	var volIdx float64
	db.PG.Raw(`
		WITH idx_ret AS (
			SELECT (close - LAG(close) OVER (ORDER BY trade_date))
				/ NULLIF(LAG(close) OVER (ORDER BY trade_date), 0) as ret
			FROM stocks_daily_k
			WHERE code = 'IDX000001'
			  AND trade_date >= (SELECT MAX(trade_date) FROM market_daily_agg) - 21
		)
		SELECT COALESCE(STDDEV(ret), 0) * 100 FROM idx_ret WHERE ret IS NOT NULL
	`).Row().Scan(&volIdx)

	// Normalise: typical daily std is 0.5%-2.5%. Map 0.5% → 90, 2.5% → 10.
	volScore := clamp(100-(volIdx-0.5)/2.0*80, 0, 100)
	f5 := FearGreedFactor{
		Name: "波动率", Raw: math.Round(volIdx*100) / 100,
		Score: volScore,
		Label: fmtVolIdxLabel(volIdx),
	}

	// ── F6: 炸板率 ──
	bbr := float64(0)
	if ls.UpCount > 0 {
		bbr = float64(ls.BoardBreak) / float64(ls.UpCount) * 100
	}
	f6 := FearGreedFactor{
		Name: "炸板率", Raw: math.Round(bbr*10) / 10,
		Score: 100 - linearScore(bbr, [4]float64{5, 15, 30, 50}),
		Label: fmtBBRLabel(bbr),
	}

	// Composite
	composite := (f1.Score + f2.Score + f3.Score + f4.Score + f5.Score + f6.Score) / 6

	return &FearGreedData{
		TradeDate: ls.TradeDate,
		Score:     math.Round(composite*10) / 10,
		Zone:      fearGreedZone(composite),
		Factors:   []FearGreedFactor{f1, f2, f3, f4, f5, f6},
	}, nil
}

// ComputeFearGreedHistory returns Fear & Greed history for the last N days.
// Uses limit_stats_daily for most factors; skips vol/northbound for speed.
func ComputeFearGreedHistory(days int) ([]FearGreedData, error) {
	var list []LimitStats
	if err := db.PG.Raw(`
		SELECT trade_date::text, up_count, down_count, rise_count, fall_count, board_break, total_stocks
		FROM limit_stats_daily ORDER BY trade_date DESC LIMIT ?
	`, days).Scan(&list).Error; err != nil {
		return nil, err
	}

	// Pre-fetch volume data for all requested days
	type volRow struct {
		TradeDate string  `gorm:"column:trade_date"`
		TotalVol  float64 `gorm:"column:total_vol"`
	}
	var vols []volRow
	db.PG.Raw(`
		SELECT trade_date::text, SUM(volume) as total_vol
		FROM stocks_daily_k WHERE code !~ '^IDX' AND volume > 0
		GROUP BY trade_date ORDER BY trade_date DESC LIMIT ?
	`, days+20).Scan(&vols)

	volMap := map[string]float64{}
	for _, v := range vols {
		volMap[v.TradeDate] = v.TotalVol
	}


	// Build 20-day rolling vol avg
	rollingVolAvg := map[string]float64{}
	volDates := make([]string, 0, len(vols))
	for _, v := range vols {
		volDates = append(volDates, v.TradeDate)
	}
	for i, v := range vols {
		end := i + 21
		if end > len(vols) {
			end = len(vols)
		}
		if end-i < 2 {
			continue
		}
		sum := float64(0)
		// avg of previous 20 days (exclude current)
		count := 0
		for j := i + 1; j < end; j++ {
			sum += vols[j].TotalVol
			count++
		}
		if count > 0 {
			rollingVolAvg[v.TradeDate] = sum / float64(count)
		}
	}

	result := make([]FearGreedData, 0, len(list))
	for _, ls := range list {
		upDn := float64(ls.UpCount) / math.Max(float64(ls.DownCount), 1)
		advDec := float64(ls.RiseCount) / math.Max(float64(ls.FallCount), 1)

		volDev := float64(0)
		if avg, ok := rollingVolAvg[ls.TradeDate]; ok && avg > 0 {
			if v, ok2 := volMap[ls.TradeDate]; ok2 {
				volDev = (v - avg) / avg * 100
			}
		}

		bbr := float64(0)
		if ls.UpCount > 0 {
			bbr = float64(ls.BoardBreak) / float64(ls.UpCount) * 100
		}

		f1s := linearScore(upDn, [4]float64{0.3, 0.8, 1.5, 2.5})
		f2s := linearScore(advDec, [4]float64{0.5, 0.9, 1.5, 2.5})
		f3s := linearScore(volDev, [4]float64{-25, -10, 10, 25})
		f6s := 100 - linearScore(bbr, [4]float64{5, 15, 30, 50})

		// For history we use avg of 4 factors (skip vol and northbound for speed)
		composite := (f1s + f2s + f3s + f6s) / 4

		fg := FearGreedData{
			TradeDate: ls.TradeDate,
			Score:     math.Round(composite*10) / 10,
			Zone:      fearGreedZone(composite),
			Factors: []FearGreedFactor{
				{Name: "涨跌停比", Raw: math.Round(upDn*100) / 100, Score: f1s, Label: fmtRatioLabel(upDn)},
				{Name: "涨跌比", Raw: math.Round(advDec*100) / 100, Score: f2s, Label: fmtRatioLabel(advDec)},
				{Name: "量能偏离", Raw: math.Round(volDev*10) / 10, Score: f3s, Label: fmtVolLabel(volDev)},
				{Name: "炸板率", Raw: math.Round(bbr*10) / 10, Score: f6s, Label: fmtBBRLabel(bbr)},
			},
		}
		result = append(result, fg)
	}

	// Reverse to ascending
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result, nil
}

// ── label helpers ──

func fmtRatioLabel(r float64) string {
	switch {
	case r >= 3:
		return "极度亢奋"
	case r >= 2:
		return "偏热"
	case r >= 1:
		return "温和"
	case r >= 0.5:
		return "偏冷"
	default:
		return "冰点"
	}
}

func fmtVolLabel(dev float64) string {
	switch {
	case dev > 25:
		return "显著放量"
	case dev > 10:
		return "放量"
	case dev > -10:
		return "正常"
	case dev > -25:
		return "缩量"
	default:
		return "显著缩量"
	}
}

func fmtNBLabel(net float64) string {
	switch {
	case net > 50:
		return "大幅流入"
	case net > 10:
		return "流入"
	case net > -10:
		return "持平"
	case net > -50:
		return "流出"
	default:
		return "大幅流出"
	}
}

func fmtBBRLabel(bbr float64) string {
	switch {
	case bbr > 40:
		return "封板困难"
	case bbr > 20:
		return "偏弱"
	case bbr > 10:
		return "正常"
	default:
		return "封板坚决"
	}
}

func fmtVolIdxLabel(v float64) string {
	switch {
	case v > 2.5:
		return "剧烈波动"
	case v > 1.5:
		return "波动较大"
	case v > 0.8:
		return "正常"
	default:
		return "低波动"
	}
}
