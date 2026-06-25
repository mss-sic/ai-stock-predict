package service

import (
	"encoding/json"
	"fmt"
	"log"
	"math"

	"github.com/ai-stock-predict/server/internal/db"
)

// MarketStyle labels
type MarketStyle string

const (
	StyleBullRally    MarketStyle = "bull_rally"
	StyleMildBull     MarketStyle = "mild_bull"
	StyleRecovery     MarketStyle = "recovery"
	StyleStructural   MarketStyle = "structural"
	StyleRotation     MarketStyle = "rotation"
	StyleBottoming    MarketStyle = "bottoming"
	StyleBear         MarketStyle = "bear"
	StyleCrash        MarketStyle = "crash"
	StyleTransitional MarketStyle = "transitional"
)

// StyleNames maps style codes to display names
var StyleNames = map[MarketStyle]string{
	StyleBullRally:    "🟢 牛市普涨",
	StyleMildBull:     "🟢 温和上涨",
	StyleRecovery:     "🟡 回暖修复",
	StyleStructural:   "🟠 结构分化",
	StyleRotation:     "🟡 震荡轮动",
	StyleBottoming:    "🟤 底部磨底",
	StyleBear:         "🔴 熊市下跌",
	StyleCrash:        "⚫ 恐慌暴跌",
	StyleTransitional: "⬜ 过渡整理",
}

// StyleParams holds strategy parameter adjustments for each style
type StyleParams struct {
	BuyPct               float64 `json:"buyPct"`
	AddPct               float64 `json:"addPct"`
	BuyLogic             string  `json:"buyLogic"`
	AllowBuy             bool    `json:"allowBuy"`
	AllowAdd             bool    `json:"allowAdd"`
	SellPctMult          float64 `json:"sellPctMult"`
	ConceptTopPct        float64 `json:"conceptTopPct"`
	PositionBias         float64 `json:"positionBias"`
	StopProfitAdj        float64 `json:"stopProfitAdj"`
	StopLossAdj          float64 `json:"stopLossAdj"`
	TrailingStopDrawdown float64 `json:"trailingStopDrawdown"`
}

// DefaultStyleParams returns optimal parameters per market style
func DefaultStyleParams(style MarketStyle) StyleParams {
	switch style {
	case StyleBullRally, StyleMildBull:
		return StyleParams{BuyPct: 20, AddPct: 15, BuyLogic: "or", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.50, PositionBias: 1.2, StopProfitAdj: 5, StopLossAdj: -2, TrailingStopDrawdown: 10}
	case StyleRecovery:
		return StyleParams{BuyPct: 15, AddPct: 10, BuyLogic: "and", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.40, PositionBias: 1.0, TrailingStopDrawdown: 8}
	case StyleStructural:
		return StyleParams{BuyPct: 12, AddPct: 10, BuyLogic: "and", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.20, PositionBias: 1.0, TrailingStopDrawdown: 8}
	case StyleRotation:
		return StyleParams{BuyPct: 6, AddPct: 0, BuyLogic: "and", AllowBuy: true, AllowAdd: false,
			ConceptTopPct: 0.30, PositionBias: 0.6, StopProfitAdj: -5, StopLossAdj: 2, TrailingStopDrawdown: 5}
	case StyleBottoming:
		return StyleParams{BuyPct: 4, AddPct: 0, BuyLogic: "and", AllowBuy: true, AllowAdd: false,
			ConceptTopPct: 0.15, PositionBias: 0.4, StopProfitAdj: -5, StopLossAdj: 3, TrailingStopDrawdown: 4}
	case StyleBear, StyleCrash:
		return StyleParams{BuyPct: 0, AddPct: 0, BuyLogic: "and", AllowBuy: false, AllowAdd: false,
			SellPctMult: 2.0}
	default:
		return StyleParams{BuyPct: 10, AddPct: 5, BuyLogic: "and", AllowBuy: true, AllowAdd: false,
			ConceptTopPct: 0.50, PositionBias: 0.8}
	}
}

// StyleRow holds one row from market_style_daily
type StyleRow struct {
	TradeDate        string  `json:"tradeDate"`
	Style            string  `json:"style"`
	StyleName        string  `json:"styleName"`
	StyleConfidence  float64 `json:"styleConfidence"`
	CompositeScore   float64 `json:"compositeScore"`
	UpRatio          float64 `json:"upRatio"`
	SectorDiffusion  float64 `json:"sectorDiffusion"`
	Volatility       float64 `json:"volatility"`
	ScoreTrend       float64 `json:"scoreTrend"`
	NorthboundNet    float64 `json:"northboundNet"`
	TotalAmount      float64 `json:"totalAmount"`
	LimitUpCount     int     `json:"limitUpCount"`
	LimitDownCount   int     `json:"limitDownCount"`
	MA20Above        int     `json:"ma20Above"`
	N52High          int     `json:"n52High"`
	N60Low           int     `json:"n60Low"`
	StyleDuration    int     `json:"styleDuration"`
	TransitionSignal string  `json:"transitionSignal"`
	TopSectors       json.RawMessage `json:"topSectors"`
	TopConcepts      json.RawMessage `json:"topConcepts"`
	AnalysisSummary  string  `json:"analysisSummary"`
}

// DailyReview holds the full daily review response
type DailyReview struct {
	StyleRow
	UpCount       int      `json:"upCount"`
	DownCount     int      `json:"downCount"`
	TotalStocks   int      `json:"totalStocks"`
	PrevAmount    float64  `json:"prevAmount"`
	OperationAdvice []string `json:"operationAdvice"`
}

// MarketStyleService provides market style detection and review
type MarketStyleService struct {
	cache map[string]MarketStyle
}

// NewMarketStyleService creates a new service
func NewMarketStyleService() *MarketStyleService {
	return &MarketStyleService{cache: make(map[string]MarketStyle)}
}

// DetectStyle classifies the market regime for a given date
func (s *MarketStyleService) DetectStyle(date string) MarketStyle {
	if v, ok := s.cache[date]; ok {
		return v
	}
	var row struct {
		AvgScore float64
		AvgUp    float64
		AvgDiff  float64
		AvgVol   float64
		Trend    float64
	}
	err := db.PG.Raw(`
		WITH rolling AS (
			SELECT trade_date, composite_score,
				up_count::float/NULLIF(total_stocks,0) as up_ratio,
				sector_diffusion, volatility,
				ROW_NUMBER() OVER (ORDER BY trade_date) as rn
			FROM market_sentiment
			WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '30 days')
		),
		stats AS (
			SELECT AVG(composite_score) as avg_score,
				AVG(up_ratio) as avg_up, AVG(sector_diffusion) as avg_diff,
				AVG(volatility) as avg_vol
			FROM rolling
			WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '20 days')
		),
		trend AS (
			SELECT REGR_SLOPE(composite_score, rn::float) as slope
			FROM rolling
			WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '10 days')
		)
		SELECT s.*, COALESCE(t.slope,0) as trend FROM stats s, trend t
	`, date, date, date, date, date, date).Scan(&row).Error

	var style MarketStyle
	if err != nil {
		log.Printf("[market_style] query failed for %s: %v", date, err)
		style = StyleTransitional
	} else {
		style = classifyStyle(row.AvgScore, row.AvgUp, row.AvgDiff, row.AvgVol, row.Trend)
	}
	s.cache[date] = style
	return style
}

func classifyStyle(s20, u20, d20, v20, trend float64) MarketStyle {
	if s20 < 18 || (s20 < 25 && v20 > 0.18) {
		return StyleCrash
	}
	if trend < 0 && u20 < 0.30 && s20 < 32 {
		return StyleBear
	}
	if s20 < 30 && u20 < 0.35 && trend >= -0.5 && trend <= 0.5 {
		return StyleBottoming
	}
	if trend > 0.5 && u20 < 0.48 {
		return StyleRecovery
	}
	if u20 > 0.48 && d20 > 0.45 && trend > 0.3 {
		return StyleBullRally
	}
	if u20 > 0.45 && trend > 0.2 {
		return StyleMildBull
	}
	if u20 < 0.35 && d20 < 0.30 {
		return StyleStructural
	}
	if u20 >= 0.30 && u20 < 0.50 && d20 >= 0.30 {
		return StyleRotation
	}
	return StyleTransitional
}

// confidenceScore computes 0-100 confidence for the style classification
func confidenceScore(s20, u20, d20, v20, trend float64, style MarketStyle) float64 {
	switch style {
	case StyleBullRally:
		return clampConfidence(50 + (u20-0.48)*200 + (d20-0.45)*200 + trend*30)
	case StyleMildBull:
		return clampConfidence(50 + (u20-0.45)*200 + trend*40)
	case StyleRecovery:
		return clampConfidence(50 + (trend-0.5)*100 + (0.48-u20)*100)
	case StyleStructural:
		return clampConfidence(50 + (0.35-u20)*200 + (0.30-d20)*200)
	case StyleRotation:
		return clampConfidence(50 + (u20-0.30)*150 + (d20-0.30)*150)
	case StyleBottoming:
		return clampConfidence(50 + (30-s20)*5 + (0.35-u20)*200)
	case StyleBear:
		return clampConfidence(50 + (32-s20)*5 + (0.30-u20)*200 - trend*50)
	case StyleCrash:
		return clampConfidence(50 + (25-s20)*5 + (v20-0.18)*200)
	default:
		return 40
	}
}

func clampConfidence(v float64) float64 {
	return math.Max(0, math.Min(100, v))
}

// ComputeAndStore runs style detection + concept aggregation for a date and persists
func (s *MarketStyleService) ComputeAndStore(date string) error {
	// Detect style
	style := s.DetectStyle(date)

	// Get raw data from market_sentiment
	var ms struct {
		CompositeScore  float64
		UpCount         int
		DownCount       int
		TotalStocks     int
		LimitUpCount    int
		LimitDownCount  int
		SectorDiffusion float64
		Volatility      float64
		NorthboundNet   float64
	}
	db.PG.Raw(`SELECT composite_score, up_count, down_count, total_stocks,
		limit_up_count, limit_down_count, sector_diffusion, volatility,
		COALESCE(northbound_net,0)
		FROM market_sentiment WHERE trade_date = ?`, date).Scan(&ms)

	// Get market_daily_agg data
	var agg struct {
		MA20Above int
		N52High   int
		N60Low    int
		TotalAmt  float64
	}
	db.PG.Raw(`SELECT COALESCE(ma20_count,0) as ma20_above, COALESCE(n52_high_count,0) as n52_high,
		COALESCE(n60_low_count,0) as n60_low, COALESCE(total_amount,0) as total_amt
		FROM market_daily_agg WHERE trade_date = ?`, date).Scan(&agg)

	upRatio := 0.0
	if ms.TotalStocks > 0 {
		upRatio = float64(ms.UpCount) / float64(ms.TotalStocks)
	}

	// Compute rolling stats for confidence
	var roll struct {
		AvgScore float64
		AvgUp    float64
		AvgDiff  float64
		AvgVol   float64
		Trend    float64
	}
	db.PG.Raw(`
		WITH rolling AS (
			SELECT trade_date, composite_score,
				up_count::float/NULLIF(total_stocks,0) as up_ratio,
				sector_diffusion, volatility,
				ROW_NUMBER() OVER (ORDER BY trade_date) as rn
			FROM market_sentiment
			WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '30 days')
		)
		SELECT COALESCE(AVG(composite_score),0), COALESCE(AVG(up_ratio),0),
			COALESCE(AVG(sector_diffusion),0), COALESCE(AVG(volatility),0), 0
		FROM rolling
		WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '20 days')
	`, date, date, date, date).Scan(&roll)

	// Also get trend
	db.PG.Raw(`
		WITH rolling AS (
			SELECT ROW_NUMBER() OVER (ORDER BY trade_date) as rn, composite_score
			FROM market_sentiment
			WHERE trade_date <= ?::date AND trade_date >= (?::date - INTERVAL '10 days')
		)
		SELECT COALESCE(REGR_SLOPE(composite_score, rn::float),0) FROM rolling
	`, date, date).Scan(&roll.Trend)

	conf := confidenceScore(roll.AvgScore, roll.AvgUp, roll.AvgDiff, roll.AvgVol, roll.Trend, style)

	// Compute style_duration and transition_signal
	duration := s.computeDuration(date, string(style))
	transSignal := s.computeTransitionSignal(date, string(style), roll.Trend, roll.AvgVol, ms.Volatility)

	// Aggregate top concepts and sectors
	topConcepts := s.aggregateConcepts(date, date, date, 15)
	topSectors := s.aggregateSectors(date, date, date, 10)

	topConceptsJSON, _ := json.Marshal(topConcepts)
	topSectorsJSON, _ := json.Marshal(topSectors)

	return db.PG.Exec(`
		INSERT INTO market_style_daily (trade_date, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, volatility, score_trend, northbound_net, total_amount,
			limit_up_count, limit_down_count, ma20_above, n52_high, n60_low,
			style_duration, transition_signal, top_sectors, top_concepts)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::jsonb,?::jsonb)
		ON CONFLICT (trade_date) DO UPDATE SET
			style=EXCLUDED.style, style_confidence=EXCLUDED.style_confidence,
			composite_score=EXCLUDED.composite_score, up_ratio=EXCLUDED.up_ratio,
			sector_diffusion=EXCLUDED.sector_diffusion, volatility=EXCLUDED.volatility,
			score_trend=EXCLUDED.score_trend, northbound_net=EXCLUDED.northbound_net,
			total_amount=EXCLUDED.total_amount, limit_up_count=EXCLUDED.limit_up_count,
			limit_down_count=EXCLUDED.limit_down_count, ma20_above=EXCLUDED.ma20_above,
			n52_high=EXCLUDED.n52_high, n60_low=EXCLUDED.n60_low,
			style_duration=EXCLUDED.style_duration, transition_signal=EXCLUDED.transition_signal,
			top_sectors=EXCLUDED.top_sectors, top_concepts=EXCLUDED.top_concepts
	`, date, string(style), conf, ms.CompositeScore, upRatio, ms.SectorDiffusion,
		ms.Volatility, roll.Trend, ms.NorthboundNet, agg.TotalAmt,
		ms.LimitUpCount, ms.LimitDownCount, agg.MA20Above, agg.N52High, agg.N60Low,
		duration, transSignal, string(topSectorsJSON), string(topConceptsJSON)).Error
}

func (s *MarketStyleService) computeDuration(date, style string) int {
	var prev struct {
		TradeDate string
		Style     string
		Duration  int
	}
	db.PG.Raw(`SELECT trade_date::text, style, style_duration FROM market_style_daily
		WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 1`, date).Scan(&prev)
	if prev.Style == style {
		return prev.Duration + 1
	}
	return 1
}

func (s *MarketStyleService) computeTransitionSignal(date, style string, trend, avgVol20, todayVol float64) string {
	// Get previous 3 days trends
	var prevTrends []float64
	db.PG.Raw(`SELECT score_trend FROM market_style_daily
		WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 3`, date).Pluck("score_trend", &prevTrends)

	// warming: 3 consecutive rising trends + up_ratio crossing threshold
	if len(prevTrends) >= 3 && trend > prevTrends[0] && prevTrends[0] > prevTrends[1] && prevTrends[1] > prevTrends[2] {
		return "warming"
	}
	// cooling: 3 consecutive falling trends + vol rising
	if len(prevTrends) >= 3 && trend < prevTrends[0] && prevTrends[0] < prevTrends[1] && prevTrends[1] < prevTrends[2] {
		if todayVol > avgVol20*1.1 {
			return "cooling"
		}
	}
	// reversal: 2+ style changes in 14 days
	var styleChanges int
	db.PG.Raw(`SELECT COUNT(*) FROM (
		SELECT style, LAG(style) OVER (ORDER BY trade_date) as prev_style
		FROM market_style_daily WHERE trade_date <= ?
		ORDER BY trade_date DESC LIMIT 14
	) sub WHERE style != prev_style AND prev_style IS NOT NULL`, date).Scan(&styleChanges)
	if styleChanges >= 2 {
		return "reversal"
	}
	return "none"
}

// aggregateConcepts computes top N concepts by daily performance using stock_concepts + stocks_daily_k
func (s *MarketStyleService) aggregateConcepts(dateFrom, dateTo, targetDate string, topN int) []map[string]interface{} {
	type row struct {
		Name     string
		Code     string
		ChgPct   float64
		UpRatio  float64
		VolRatio float64
	}
	var rows []row
	db.PG.Raw(`
		WITH stock_chg AS (
			SELECT code, trade_date, close, amount,
				COALESCE((close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date))
					/ NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) * 100, 0) as chg_pct,
				LAG(amount) OVER (PARTITION BY code ORDER BY trade_date) as prev_amount
			FROM stocks_daily_k
			WHERE trade_date >= (?::date - INTERVAL '5 days') AND trade_date <= ?::date
		),
		concept_chg AS (
			SELECT sc.concept_name, MIN(cb.concept_code) as concept_code,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY s.chg_pct) as median_chg,
				AVG(CASE WHEN s.chg_pct > 0 THEN 1 ELSE 0 END) as up_ratio,
				AVG(s.amount) as avg_amount,
				AVG(s.prev_amount) as prev_avg_amount
			FROM stock_concepts sc
			JOIN stock_chg s ON s.code = sc.code AND s.trade_date = ?::date
			JOIN concept_boards cb ON cb.concept_name = sc.concept_name AND cb.concept_type = 'concept'
			GROUP BY sc.concept_name
			HAVING COUNT(*) >= 3
		)
		SELECT concept_name as name, concept_code as code, median_chg as chg_pct, up_ratio,
			CASE WHEN prev_avg_amount > 0 THEN avg_amount / prev_avg_amount ELSE 1 END as vol_ratio
		FROM concept_chg
		ORDER BY median_chg DESC
		LIMIT ?
	`, dateFrom, dateTo, targetDate, topN).Scan(&rows)

	result := make([]map[string]interface{}, 0, len(rows))
	for i, r := range rows {
		result = append(result, map[string]interface{}{
			"rank":             i + 1,
			"name":             r.Name,
			"code":             r.Code,
			"chgPct":           math.Round(r.ChgPct*100) / 100,
			"upRatio":          math.Round(r.UpRatio*100) / 100,
			"volRatio":         math.Round(r.VolRatio*100) / 100,
			"consecutiveDays":  s.conceptConsecutiveDays(r.Name, targetDate, topN),
		})
	}
	return result
}

// conceptConsecutiveDays counts consecutive days a concept has been in top N
func (s *MarketStyleService) conceptConsecutiveDays(conceptName, date string, topN int) int {
	var days int
	db.PG.Raw(`
		WITH ranked AS (
			SELECT trade_date, top_concepts,
				ROW_NUMBER() OVER (ORDER BY trade_date DESC) as rn
			FROM market_style_daily WHERE trade_date <= ?
		)
		SELECT COUNT(*) FROM ranked r,
			jsonb_array_elements(r.top_concepts) c
		WHERE (c->>'name') = ? AND (c->>'rank')::int <= ?
	`, date, conceptName, topN).Scan(&days)
	return days
}

// aggregateSectors computes top N industry sectors
func (s *MarketStyleService) aggregateSectors(dateFrom, dateTo, targetDate string, topN int) []map[string]interface{} {
	type row struct {
		Name     string
		ChgPct   float64
		UpRatio  float64
		VolRatio float64
	}
	var rows []row
	db.PG.Raw(`
		WITH stock_chg AS (
			SELECT code, trade_date, close, amount,
				COALESCE((close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date))
					/ NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) * 100, 0) as chg_pct,
				LAG(amount) OVER (PARTITION BY code ORDER BY trade_date) as prev_amount
			FROM stocks_daily_k
			WHERE trade_date >= (?::date - INTERVAL '5 days') AND trade_date <= ?::date
		),
		sector_chg AS (
			SELECT sc.concept_name,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY s.chg_pct) as median_chg,
				AVG(CASE WHEN s.chg_pct > 0 THEN 1 ELSE 0 END) as up_ratio,
				AVG(s.amount) as avg_amount,
				AVG(s.prev_amount) as prev_avg_amount
			FROM stock_concepts sc
			JOIN stock_chg s ON s.code = sc.code AND s.trade_date = ?::date
			WHERE sc.concept_type = 'industry'
			GROUP BY sc.concept_name
			HAVING COUNT(*) >= 5
		)
		SELECT concept_name as name, median_chg as chg_pct, up_ratio,
			CASE WHEN prev_avg_amount > 0 THEN avg_amount / prev_avg_amount ELSE 1 END as vol_ratio
		FROM sector_chg
		ORDER BY median_chg DESC
		LIMIT ?
	`, dateFrom, dateTo, targetDate, topN).Scan(&rows)

	result := make([]map[string]interface{}, 0, len(rows))
	for i, r := range rows {
		result = append(result, map[string]interface{}{
			"rank":             i + 1,
			"name":             r.Name,
			"chgPct":           math.Round(r.ChgPct*100) / 100,
			"upRatio":          math.Round(r.UpRatio*100) / 100,
			"volRatio":         math.Round(r.VolRatio*100) / 100,
			"consecutiveDays":  s.conceptConsecutiveDays(r.Name, targetDate, topN),
		})
	}
	return result
}

// GetStyleCurve returns style timeline for a date range
func (s *MarketStyleService) GetStyleCurve(from, to string) ([]StyleRow, error) {
	type rawRow struct {
		StyleRow
		TopSecStr string
		TopConStr string
	}
	var raws []rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, volatility, score_trend,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'')
		FROM market_style_daily
		WHERE trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date
	`, from, to).Scan(&raws).Error
	if err != nil {
		return nil, err
	}
	rows := make([]StyleRow, len(raws))
	for i := range raws {
		rows[i] = raws[i].StyleRow
		rows[i].TopSectors = json.RawMessage(raws[i].TopSecStr)
		rows[i].TopConcepts = json.RawMessage(raws[i].TopConStr)
		if name, ok := StyleNames[MarketStyle(rows[i].Style)]; ok {
			rows[i].StyleName = name
		}
	}
	return rows, nil
}

// GetDailyReview returns full daily review data
func (s *MarketStyleService) GetDailyReview(date string) (*DailyReview, error) {
	type rawRow struct {
		StyleRow
		TopSecStr string
		TopConStr string
	}
	var r rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, volatility, score_trend,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'')
		FROM market_style_daily WHERE trade_date = ?
	`, date).Scan(&r).Error
	if err != nil {
		return nil, err
	}
	row := r.StyleRow
	row.TopSectors = json.RawMessage(r.TopSecStr)
	row.TopConcepts = json.RawMessage(r.TopConStr)
	if name, ok := StyleNames[MarketStyle(row.Style)]; ok {
		row.StyleName = name
	}

	// Get extra data from market_sentiment
	var ms struct {
		UpCount     int
		DownCount   int
		TotalStocks int
	}
	db.PG.Raw(`SELECT up_count, down_count, total_stocks FROM market_sentiment WHERE trade_date=?`, date).Scan(&ms)

	// Get prev day amount
	var prevAmt float64
	db.PG.Raw(`SELECT total_amount FROM market_daily_agg WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 1`, date).Scan(&prevAmt)

	// Generate operation advice
	advice := s.generateAdvice(row.Style, row)

	review := &DailyReview{
		StyleRow:        row,
		UpCount:         ms.UpCount,
		DownCount:       ms.DownCount,
		TotalStocks:     ms.TotalStocks,
		PrevAmount:      prevAmt,
		OperationAdvice: advice,
	}
	return review, nil
}

func (s *MarketStyleService) generateAdvice(style string, row StyleRow) []string {
	params := DefaultStyleParams(MarketStyle(style))
	advice := make([]string, 0)

	switch MarketStyle(style) {
	case StyleBullRally, StyleMildBull:
		advice = append(advice,
			fmt.Sprintf("牛市行情，单票仓位建议 %.0f%%，可加仓", params.BuyPct),
			fmt.Sprintf("聚焦前 %.0f%% 概念板块选股", params.ConceptTopPct*100),
			fmt.Sprintf("止盈上调 +%d%%，止损收紧 %d%%", int(params.StopProfitAdj), int(params.StopLossAdj)),
			fmt.Sprintf("移动止盈回撤设为 %.0f%%，让利润奔跑", params.TrailingStopDrawdown))
	case StyleRecovery:
		advice = append(advice,
			fmt.Sprintf("回暖修复，单票仓位 %.0f%%，条件用 AND 逻辑严格筛选", params.BuyPct),
			fmt.Sprintf("聚焦前 %.0f%% 概念板块", params.ConceptTopPct*100),
			"温和加仓，每次不超过 10%",
			"关注风格切换信号，若转为牛市及时调整")
	case StyleStructural:
		advice = append(advice,
			fmt.Sprintf("结构分化行情，资金聚焦少数板块，单票仓位 %.0f%%", params.BuyPct),
			fmt.Sprintf("只在前 %.0f%% 概念板块中选股", params.ConceptTopPct*100),
			"条件用 AND 逻辑，严格筛选",
			fmt.Sprintf("移动止盈回撤 %.0f%%，跟随趋势", params.TrailingStopDrawdown))
	case StyleRotation:
		advice = append(advice,
			fmt.Sprintf("轮动行情，单票仓位 %.0f%%，不加仓", params.BuyPct),
			fmt.Sprintf("聚焦前 %.0f%% 概念板块", params.ConceptTopPct*100),
			"止盈收紧 5%，快进快出",
			"止损放宽 2%，给震荡空间")
	case StyleBottoming:
		advice = append(advice,
			fmt.Sprintf("磨底行情，单票仓位 %.0f%%，不加仓", params.BuyPct),
			fmt.Sprintf("只在前 %.0f%% 概念板块中试仓", params.ConceptTopPct*100),
			"止盈收紧 5%，见好就收",
			"关注回暖信号，若 score_trend 转正可转修复策略")
	case StyleBear:
		advice = append(advice,
			"熊市行情，建议空仓观望",
			"不买新股，只平仓止损",
			"关注底部信号：连续3日 score_trend 转正 + up_ratio > 0.35")
	case StyleCrash:
		advice = append(advice,
			"恐慌暴跌，强制清仓保护本金",
			"不进行任何买入操作",
			fmt.Sprintf("等待波动率回落至 %.1f%% 以下再考虑入场", 0.15*100))
	default:
		advice = append(advice,
			fmt.Sprintf("过渡整理，单票仓位 %.0f%%，不加仓", params.BuyPct),
			"维持观望，等待明确信号",
			"关注风格切换方向")
	}

	// Add transition signal advice
	switch row.TransitionSignal {
	case "warming":
		advice = append(advice, "⚠ 风格转暖信号：市场情绪连续改善，准备加仓")
	case "cooling":
		advice = append(advice, "⚠ 风格冷却信号：情绪回落+波动上升，注意风控")
	case "reversal":
		advice = append(advice, "⚠ 风格切换信号：近期风格频繁切换，降低仓位等待确认")
	}

	return advice
}

// GetLatestStyle returns the most recent style
func (s *MarketStyleService) GetLatestStyle() (*StyleRow, error) {
	type rawRow struct {
		StyleRow
		TopSecStr string
		TopConStr string
	}
	var r rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, volatility, score_trend,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'')
		FROM market_style_daily ORDER BY trade_date DESC LIMIT 1
	`).Scan(&r).Error
	if err != nil {
		return nil, err
	}
	row := r.StyleRow
	row.TopSectors = json.RawMessage(r.TopSecStr)
	row.TopConcepts = json.RawMessage(r.TopConStr)
	if name, ok := StyleNames[MarketStyle(row.Style)]; ok {
		row.StyleName = name
	}
	return &row, nil
}
