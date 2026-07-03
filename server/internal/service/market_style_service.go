package service

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
)

// ── 七风格体系 ──
// 每个分类对应可执行的交易动作：做多 / 观望 / 避险

type MarketStyle string

const (
	StyleBroadRally MarketStyle = "broad_rally" // 普涨行情：顺势+广度极大 → 重仓做多
	StyleTrendUp    MarketStyle = "trend_up"    // 趋势上行：顺势+广度宽 → 正常做多
	StyleStructural MarketStyle = "structural"  // 结构行情：顺势+广度窄 → 精选个股
	StyleChoppy     MarketStyle = "choppy"      // 震荡博弈：方向不明 → 轻仓波段
	StyleWeakRange  MarketStyle = "weak_range"  // 弱势震荡：偏弱未崩 → 观望为主
	StyleDecline    MarketStyle = "decline"     // 持续下跌：趋势向下 → 减仓/空仓
	StyleCrash      MarketStyle = "crash"       // 恐慌崩盘：极端恐慌 → 清仓避险
)

var StyleNames = map[MarketStyle]string{
	StyleBroadRally: "🔥 普涨行情",
	StyleTrendUp:    "🟢 趋势上行",
	StyleStructural: "🟡 结构行情",
	StyleChoppy:     "🟡 震荡博弈",
	StyleWeakRange:  "🟠 弱势震荡",
	StyleDecline:    "🔴 持续下跌",
	StyleCrash:      "⚫ 恐慌崩盘",
}

// formatStyleDisplay returns display name for any style code (incl. legacy)
func formatStyleDisplay(code string) string {
	legacy := map[string]string{
		"bull_rally":    "🟢 趋势上行",
		"mild_bull":     "🟢 趋势上行",
		"recovery":      "🟡 震荡博弈",
		"bear":          "🔴 持续下跌",
		"divergence":    "🔴 持续下跌",
		"transitional":  "🟡 震荡博弈",
		"rotation":      "🟡 震荡博弈",
		"risk_off":      "🔴 持续下跌",
		"bottoming":     "🟠 弱势震荡",
		"trend_up_old":  "🟢 趋势上行",
	}
	if name, ok := StyleNames[MarketStyle(code)]; ok {
		return name
	}
	if name, ok := legacy[code]; ok {
		return name
	}
	return code
}


// ── Layer 1: Market Environment (大盘格局) ──
// Determines overall risk posture from index trend and volume
type MarketRegime string

const (
	RegimeExpansion   MarketRegime = "expansion"   // 上涨格局
	RegimeNeutral     MarketRegime = "neutral"     // 震荡格局
	RegimeContraction MarketRegime = "contraction" // 下跌格局
)

var RegimeNames = map[MarketRegime]string{
	RegimeExpansion:   "📈 上涨格局",
	RegimeNeutral:     "↔️ 震荡格局",
	RegimeContraction: "📉 下跌格局",
}

func (s *MarketStyleService) detectRegime(date string) MarketRegime {
	var idxRet float64
	db.PG.Raw(`
		WITH idx_data AS (
			SELECT k.trade_date, k.close,
				AVG(k.close) OVER (ORDER BY k.trade_date ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) as ma60,
				AVG(k.close) OVER (ORDER BY k.trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20
			FROM stocks_daily_k k
			WHERE k.code = 'IDX000001'
				AND k.trade_date <= ? AND k.trade_date >= (?::date - INTERVAL '90 days')
		)
		SELECT COALESCE((ma20 - ma60) / NULLIF(ma60, 0), 0)
		FROM idx_data WHERE trade_date = ?
	`, date, date, date).Scan(&idxRet)

	var volTrend float64
	db.PG.Raw(`
		SELECT COALESCE(AVG(amount) FILTER (WHERE rn <= 20), 0) /
			   NULLIF(COALESCE(AVG(amount) FILTER (WHERE rn > 20 AND rn <= 60), 0), 0) - 1
		FROM (
			SELECT total_amount as amount, ROW_NUMBER() OVER (ORDER BY trade_date DESC) as rn
			FROM market_daily_agg
			WHERE trade_date <= ?
			ORDER BY trade_date DESC LIMIT 60
		) sub
	`, date).Scan(&volTrend)

	if idxRet > 0.02 && volTrend > -0.1 {
		return RegimeExpansion
	} else if idxRet < -0.03 || (idxRet < -0.01 && volTrend < -0.2) {
		return RegimeContraction
	}
	return RegimeNeutral
}

// ── Layer 2: Leading Theme (领涨方向) ──
type ThematicLeadership string

const (
	LeadershipClear ThematicLeadership = "clear"
	LeadershipFuzzy ThematicLeadership = "fuzzy"
	LeadershipNone  ThematicLeadership = "none"
)

var LeadershipNames = map[ThematicLeadership]string{
	LeadershipClear: "🎯 主线明确",
	LeadershipFuzzy: "🔄 方向模糊",
	LeadershipNone:  "⚡ 暂无主线",
}

func (s *MarketStyleService) detectLeadership(date string) (ThematicLeadership, string, string) {
	var leadConcept, leadIndustry string
	var conceptChg, industryChg float64

	// 1. Find top concept (from concept_type='concept')
	row := db.PG.Raw(`
		SELECT cb.concept_name, AVG((k.close - prev.close) / NULLIF(prev.close, 0) * 100)
		FROM stocks_daily_k k
		JOIN stock_concepts sc ON sc.code = k.code AND sc.concept_type = 'concept'
		JOIN concept_boards cb ON cb.concept_code = sc.concept_code AND cb.concept_type = 'concept'
		JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2 WHERE k2.code = k.code AND k2.trade_date < k.trade_date ORDER BY k2.trade_date DESC LIMIT 1) prev ON true
		WHERE k.trade_date = ?
		GROUP BY cb.concept_name
		HAVING COUNT(DISTINCT k.code) >= 5
		ORDER BY 2 DESC LIMIT 1
	`, date).Row()
	if row != nil {
		row.Scan(&leadConcept, &conceptChg)
	}

	// 2. Find top industry (from concept_type='industry_l2' or 'industry')
	row2 := db.PG.Raw(`
		SELECT cb.concept_name, AVG((k.close - prev.close) / NULLIF(prev.close, 0) * 100)
		FROM stocks_daily_k k
		JOIN stock_concepts sc ON sc.code = k.code AND (sc.concept_type = 'industry_l2' OR sc.concept_type = 'industry')
		JOIN concept_boards cb ON cb.concept_code = sc.concept_code AND (cb.concept_type = 'industry_l2' OR cb.concept_type = 'industry')
		JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2 WHERE k2.code = k.code AND k2.trade_date < k.trade_date ORDER BY k2.trade_date DESC LIMIT 1) prev ON true
		WHERE k.trade_date = ?
		GROUP BY cb.concept_name
		HAVING COUNT(DISTINCT k.code) >= 5
		ORDER BY 2 DESC LIMIT 1
	`, date).Row()
	if row2 != nil {
		row2.Scan(&leadIndustry, &industryChg)
	}

	log.Printf("[market_style] %s top concept: %s (%+.2f%%), top industry: %s (%+.2f%%)", date, leadConcept, conceptChg, leadIndustry, industryChg)

	// Leadership based on concept strength
	if conceptChg > 3.0 {
		return LeadershipClear, leadConcept, leadIndustry
	} else if conceptChg > 1.5 {
		return LeadershipFuzzy, leadConcept, leadIndustry
	}
	return LeadershipNone, leadConcept, leadIndustry
}



// ── Leading Indicator: Growth/Defense Flow ──
// Positive = capital flowing to growth, Negative = rotating to defensives
// Positive → 偏好进攻(科技/新能源), Negative → 偏好防御(消费/公用)
func (s *MarketStyleService) computeGrowthDefenseFlow(date string) float64 {
	// Simple approach: count concepts with positive returns in growth vs defense categories
	growthKW := []string{"科技","AI","人工智能","半导体","芯片","软件","计算机","通信","电子","新能源","光伏","锂电","汽车","军工","机器人"}
	defenseKW := []string{"银行","保险","地产","建筑","建材","钢铁","煤炭","有色","石油","电力","公用","交通","食品","饮料","白酒","医药","医疗","农业","黄金"}

	// Build OR conditions
	growthCond := ""
	for i, kw := range growthKW {
		if i > 0 { growthCond += " OR " }
		growthCond += "concept_name ILIKE '%" + kw + "%'"
	}
	defenseCond := ""
	for i, kw := range defenseKW {
		if i > 0 { defenseCond += " OR " }
		defenseCond += "concept_name ILIKE '%" + kw + "%'"
	}

	var growthRet, defenseRet float64
	query := `
		SELECT
			COALESCE(AVG(chg) FILTER (WHERE ` + growthCond + `), 0),
			COALESCE(AVG(chg) FILTER (WHERE ` + defenseCond + `), 0)
		FROM (
			SELECT cb.concept_name,
				AVG((k.close - prev.close) / NULLIF(prev.close, 0) * 100) as chg
			FROM stocks_daily_k k
			JOIN stock_concepts sc ON sc.code = k.code
			JOIN concept_boards cb ON cb.concept_name = sc.concept_name
			JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2 WHERE k2.code = k.code AND k2.trade_date < k.trade_date ORDER BY k2.trade_date DESC LIMIT 1) prev ON true
			WHERE k.trade_date = ?
			GROUP BY cb.concept_name
		) sub
	`
	if err := db.PG.Raw(query, date).Row().Scan(&growthRet, &defenseRet); err != nil {
		log.Printf("[market_style] gdFlow error: %v", err)
		return 0
	}
	spread := growthRet - defenseRet
	log.Printf("[market_style] %s gdFlow: growth=%.2f defense=%.2f spread=%.2f", date, growthRet, defenseRet, spread)
	return spread
}



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

func DefaultStyleParams(style MarketStyle) StyleParams {
	switch style {
	case StyleBroadRally:
		return StyleParams{BuyPct: 25, AddPct: 15, BuyLogic: "or", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.50, PositionBias: 1.5, StopProfitAdj: 8, StopLossAdj: -2, TrailingStopDrawdown: 12}
	case StyleTrendUp:
		return StyleParams{BuyPct: 18, AddPct: 10, BuyLogic: "or", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.35, PositionBias: 1.1, StopProfitAdj: 5, StopLossAdj: -3, TrailingStopDrawdown: 8}
	case StyleStructural:
		return StyleParams{BuyPct: 10, AddPct: 5, BuyLogic: "and", AllowBuy: true, AllowAdd: true,
			ConceptTopPct: 0.12, PositionBias: 0.7, StopProfitAdj: 0, StopLossAdj: 3, TrailingStopDrawdown: 5}
	case StyleChoppy:
		return StyleParams{BuyPct: 5, AddPct: 0, BuyLogic: "and", AllowBuy: true, AllowAdd: false,
			ConceptTopPct: 0.20, PositionBias: 0.4, StopProfitAdj: -3, StopLossAdj: 2, TrailingStopDrawdown: 3}
	case StyleWeakRange:
		return StyleParams{BuyPct: 2, AddPct: 0, BuyLogic: "and", AllowBuy: false, AllowAdd: false,
			ConceptTopPct: 0.08, PositionBias: 0.15, StopProfitAdj: -5, StopLossAdj: 3, TrailingStopDrawdown: 2}
	case StyleDecline:
		return StyleParams{BuyPct: 0, AddPct: 0, BuyLogic: "and", AllowBuy: false, AllowAdd: false,
			SellPctMult: 1.5, PositionBias: 0.05}
	case StyleCrash:
		return StyleParams{BuyPct: 0, AddPct: 0, BuyLogic: "and", AllowBuy: false, AllowAdd: false,
			SellPctMult: 2.5, PositionBias: 0.0}
	default:
		return StyleParams{BuyPct: 6, AddPct: 0, BuyLogic: "and", AllowBuy: true, AllowAdd: false,
			ConceptTopPct: 0.15, PositionBias: 0.4}
	}
}

type StyleRow struct {
	TradeDate        string          `json:"tradeDate"`
	Style            string          `json:"style"`
	StyleName        string          `json:"styleName"`
	StyleConfidence  float64         `json:"styleConfidence"`
	CompositeScore   float64         `json:"compositeScore"`
	UpRatio          float64         `json:"upRatio"`
	SectorDiffusion  float64         `json:"sectorDiffusion"`
	Volatility       float64         `json:"volatility"`
	SectorDispersion float64         `json:"sectorDispersion"`
	ScoreChange      float64         `json:"scoreChange"`
	BreakRate        float64         `json:"breakRate"`
	Concentration    float64         `json:"concentration"`
	RotationSpeed    float64         `json:"rotationSpeed"`
	ScoreTrend       float64         `json:"scoreTrend"`
	NorthboundNet    float64         `json:"northboundNet"`
	TotalAmount      float64         `json:"totalAmount"`
	LimitUpCount     int             `json:"limitUpCount"`
	LimitDownCount   int             `json:"limitDownCount"`
	MA20Above        int             `json:"ma20Above"`
	N52High          int             `json:"n52High"`
	N60Low           int             `json:"n60Low"`
	StyleDuration    int             `json:"styleDuration"`
	TransitionSignal string          `json:"transitionSignal"`
	TopSectors       json.RawMessage `json:"topSectors"`
	TopConcepts      json.RawMessage `json:"topConcepts"`
	AnalysisSummary  string          `gorm:"column:analysis_summary" json:"analysisSummary"`
	MarketRegime     string          `json:"marketRegime"`
	LeadConcept      string          `json:"leadConcept"`
	LeadIndustry     string          `json:"leadIndustry"`
	GDFlow           float64         `gorm:"column:growth_defense_flow" json:"growthDefenseFlow"`
}

type DailyReview struct {
	StyleRow
	UpCount         int      `json:"upCount"`
	DownCount       int      `json:"downCount"`
	TotalStocks     int      `json:"totalStocks"`
	PrevAmount      float64  `json:"prevAmount"`
	OperationAdvice []string `json:"operationAdvice"`
}

type MarketStyleService struct {
	cache       map[string]MarketStyle
	styleLog    map[string]string // date → style for persistence tracking
}

func NewMarketStyleService() *MarketStyleService {
	return &MarketStyleService{
		cache:    make(map[string]MarketStyle),
		styleLog: make(map[string]string),
	}
}

// ── 分类核心：双窗口 + 阈值矩阵 ──

// shortTerm holds 5-day window metrics
type shortTerm struct {
	u5        float64 // 5-day mean up_ratio
	s5        float64 // 5-day mean composite_score
	u5Vol     float64 // 5-day up_ratio volatility
	drawdown5 float64 // 5-day score drawdown
}

func (s *MarketStyleService) computeShortTerm(date string) shortTerm {
	var st shortTerm
	type row struct{ U5, S5, U5Vol, DD5 float64 }
	var r row
	db.PG.Raw(`
		SELECT AVG(up_count::float/NULLIF(total_stocks,0)) as u5,
			AVG(composite_score) as s5,
			COALESCE(STDDEV(up_count::float/NULLIF(total_stocks,0)),0) as u5_vol,
			COALESCE(
				(MAX(composite_score) - (SELECT composite_score FROM market_sentiment WHERE trade_date = ?))
				/ NULLIF(MAX(composite_score), 0), 0
			) as dd5
		FROM market_sentiment
		WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '5 days')
	`, date, date, date).Scan(&r)
	st.u5 = r.U5; st.s5 = r.S5; st.u5Vol = r.U5Vol; st.drawdown5 = r.DD5
	return st
}

func (s *MarketStyleService) computeMedianUp20(date string) float64 {
	var med float64
	db.PG.Raw(`
		SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY up_count::float/NULLIF(total_stocks,0))
		FROM market_sentiment
		WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '20 days')
	`, date, date).Scan(&med)
	return med
}

func (s *MarketStyleService) computeRollingStats(date string) (StyleStats, error) {
	var row StyleStats
	err := db.PG.Raw(`
		WITH rolling AS (
			SELECT trade_date, composite_score,
				up_count::float/NULLIF(total_stocks,0) as up_ratio,
				sector_diffusion, volatility,
				ROW_NUMBER() OVER (ORDER BY trade_date) as rn
			FROM market_sentiment
			WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '30 days')
		),
		stats AS (
			SELECT AVG(composite_score) as avg_score,
				AVG(up_ratio) as avg_up, AVG(sector_diffusion) as avg_diff,
				AVG(volatility) as avg_vol
			FROM rolling
			WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '20 days')
		),
		trend AS (
			SELECT REGR_SLOPE(composite_score, rn::float) as slope
			FROM rolling
			WHERE trade_date <= ? AND trade_date >= (?::date - INTERVAL '10 days')
		)
		SELECT s.*, COALESCE(t.slope,0) as trend FROM stats s, trend t
	`, date, date, date, date, date, date).Scan(&row).Error
	if err != nil {
		return row, err
	}
	row.SectorDispersion = s.computeSectorDispersion(date)
	row.ScoreChange = s.computeScoreChange(date)
	return row, nil
}

// classifyStyle: 7-category system with actionable trading implications
func classifyStyle(s20, u20, d20, v20, trend, disp, scoreChange, u5, s5, u5Vol, drawdown5, u20Median, todayUpRatio float64) MarketStyle {
	// ── PRIORITY 0: Momentum Break (trend cracking) ──
	// Even if 20d trend is positive, a sharp single-day score drop signals rotation/defense.
	// scoreChange = today's composite_score minus yesterday's composite_score (from computeScoreChange).
	trendUp := trend > 0.2
	trendDown := trend < -0.3
	momentumBreak := trendUp && scoreChange < -15       // sharp drop during uptrend
	momentumCrash := trendUp && todayUpRatio < 0.35 && scoreChange < -25  // severe reversal

	if momentumCrash {
		// Severe: trend was up but today's breadth collapsed → structural or worse
		if todayUpRatio < 0.25 {
			return StyleDecline
		}
		return StyleStructural
	}
	if momentumBreak {
		// Moderate: uptrend but single-day crack → downgrade from trend_up to structural
		if todayUpRatio < 0.40 {
			return StyleWeakRange
		}
		return StyleStructural
	}

	// ── PRIORITY 1: Crash (panic) ──
	if s20 < 15 || (s5 < 22 && v20 > 0.20) {
		return StyleCrash
	}

	// ── PRIORITY 2: Decline (persistent downtrend) ──
	if trend < -1.5 && todayUpRatio < 0.40 {
		return StyleDecline
	}

	// ── PRIORITY 3: Weak range (deteriorating but not crashing) ──
	if trend < -0.3 && s20 < 40 && u20Median < 0.42 {
		return StyleWeakRange
	}

	// ── Direction + Breadth matrix ──
	broadWide := u20Median > 0.43
	broadVeryWide := u20Median > 0.55

	if trendUp && s20 > 55 && broadVeryWide {
		return StyleBroadRally
	}
	if trendUp && broadWide && s20 >= 45 {
		return StyleTrendUp
	}
	if trendUp && !broadWide {
		return StyleStructural
	}
	if trendDown {
		if s20 < 35 || u20Median < 0.35 {
			return StyleDecline
		}
		return StyleWeakRange
	}
	// Neutral zone
	if trend < 0 && u20Median < 0.38 && s20 < 40 {
		return StyleWeakRange
	}
	return StyleChoppy
}




// crossValidate checks if today's data contradicts the classified style
func crossValidate(candidate MarketStyle, todayUpRatio, sectorDiffusion, s5 float64) MarketStyle {
	switch candidate {
	case StyleBroadRally, StyleTrendUp:
		if todayUpRatio < 0.35 {
			return StyleStructural
		}
	case StyleDecline:
		if todayUpRatio > 0.55 && sectorDiffusion > 0.6 {
			log.Printf("[market_style] cross-validate: overriding decline (up=%.1f%%, diff=%.3f) → choppy", todayUpRatio*100, sectorDiffusion)
			return StyleChoppy
		}
	case StyleWeakRange:
		if todayUpRatio > 0.60 && sectorDiffusion > 0.7 {
			log.Printf("[market_style] cross-validate: upgrading weak_range → choppy", todayUpRatio*100)
			return StyleChoppy
		}
	case StyleChoppy:
		if todayUpRatio > 0.75 && sectorDiffusion > 0.8 && s5 > 55 {
			return StyleTrendUp
		}
	}
	return candidate
}

// DetectStyle with persistence: requires 2 consecutive days to flip
func (s *MarketStyleService) DetectStyle(date string) MarketStyle {
	if v, ok := s.cache[date]; ok {
		return v
	}
	row, err := s.computeRollingStats(date)
	if err != nil {
		log.Printf("[market_style] rolling stats failed for %s: %v", date, err)
		s.cache[date] = StyleChoppy
		return StyleChoppy
	}

	st := s.computeShortTerm(date)
	u20Median := s.computeMedianUp20(date)

	// Get today's actual up_ratio for cross-validation
	var todayUpRatio float64
	db.PG.Raw(`SELECT up_count::float/NULLIF(total_stocks,0) FROM market_sentiment WHERE trade_date = ?`, date).Scan(&todayUpRatio)

	candidate := classifyStyle(
		row.AvgScore, row.AvgUp, row.AvgDiff, row.AvgVol, row.Trend,
		row.SectorDispersion, row.ScoreChange,
		st.u5, st.s5, st.u5Vol, st.drawdown5, u20Median, todayUpRatio,
	)

	// Cross-validate: check if today's data contradicts the classified style
	candidate = crossValidate(candidate, todayUpRatio, row.AvgDiff, st.s5)

	// ── Style persistence filter ──
	// Only applies in real-time mode (next trading day not yet computed).
	// For backfill (next day already exists), use raw classification directly
	// to avoid deadlock: persistence requires old style on adjacent days,
	// which blocks legitimate style transitions during historical recompute.
	var hasNextDay int
	db.PG.Raw(`SELECT COUNT(*) FROM market_style_daily WHERE trade_date > ?`, date).Scan(&hasNextDay)
	isBackfill := hasNextDay > 0

	if !isBackfill {
		s.styleLog[date] = string(candidate)

		var recent []string
		db.PG.Raw(`SELECT style FROM market_style_daily WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 3`, date).Scan(&recent)

		needConfirm := 3
		if candidate == StyleCrash || candidate == StyleDecline {
			needConfirm = 1
		}
		if len(recent) > 0 {
			prevStyle := normalizeLegacy(recent[0])
			if string(candidate) != prevStyle {
				consecutive := 1
				for i := 0; i < len(recent)-1; i++ {
					if normalizeLegacy(recent[i]) == normalizeLegacy(recent[i+1]) {
						consecutive++
					} else {
						break
					}
				}
				prevDates := s.getRecentDates(date, 3)
				for _, pd := range prevDates {
					if sl, ok := s.styleLog[pd]; ok && sl == string(candidate) {
						consecutive++
					}
				}
				if consecutive < needConfirm {
					log.Printf("[market_style] %s candidate=%s held (prev=%s, need %d/%d days)", date, candidate, prevStyle, consecutive, needConfirm)
					candidate = MarketStyle(prevStyle)
				} else {
					log.Printf("[market_style] %s style confirmed: %s (persisted %d days)", date, candidate, consecutive)
				}
			}
		}
	} else {
		log.Printf("[market_style] %s backfill mode: using raw classification=%s (skip persistence)", date, candidate)
	}

	s.cache[date] = candidate
	return candidate
}

// normalizeLegacy maps old style codes to new ones for comparison
func normalizeLegacy(old string) string {
	switch old {
	case "bull_rally", "mild_bull":
		return "trend_up"
	case "recovery":
		return "rotation"
	case "bear", "divergence":
		return "risk_off"
	case "transitional":
		return "rotation"
	default:
		return old
	}
}

type StyleStats struct {
	AvgScore         float64
	AvgUp            float64
	AvgDiff          float64
	AvgVol           float64
	Trend            float64
	SectorDispersion float64
	ScoreChange      float64
}

func confidenceScore(s20, u20, d20, v20, trend, disp, scoreChange, u5, drawdown5 float64, style MarketStyle) float64 {
	switch style {
	case StyleBroadRally:
		return clampConfidence(50 + (u20-0.45)*200 + trend*3)
	case StyleTrendUp:
		return clampConfidence(50 + (u20-0.35)*200 + trend*5)
	case StyleStructural:
		return clampConfidence(50 + trend*8 + (u20-0.35)*100)
	case StyleChoppy:
		return clampConfidence(50 - math.Abs(trend)*10 + (0.5-math.Abs(u20-0.45))*50)
	case StyleWeakRange:
		return clampConfidence(50 + (0.40-u20)*150 - trend*10)
	case StyleDecline:
		return clampConfidence(50 + (0.40-u20)*200 - trend*15)
	case StyleCrash:
		return clampConfidence(50 + drawdown5*100 + (0.15-v20)*200)
	default:
		return 40
	}
}

func clampConfidence(v float64) float64 { return math.Max(0, math.Min(100, v)) }

func (s *MarketStyleService) ComputeAndStore(date string) error {
	style := s.DetectStyle(date)

	var ms struct {
		CompositeScore, SectorDiffusion, Volatility, NorthboundNet float64
		UpCount, DownCount, TotalStocks, LimitUpCount, LimitDownCount int
	}
	db.PG.Raw(`SELECT composite_score, up_count, down_count, total_stocks,
		limit_up_count, limit_down_count, sector_diffusion, volatility,
		COALESCE(northbound_net,0) AS northbound_net FROM market_sentiment WHERE trade_date = ?`, date).Scan(&ms)

	var agg struct{ MA20Above, N52High, N60Low int; TotalAmt float64 }
	db.PG.Raw(`SELECT COALESCE(ma20_count,0) AS ma20_above, COALESCE(n52_high_count,0) AS n52_high,
		COALESCE(n60_low_count,0) AS n60_low, COALESCE(total_amount,0) AS total_amt
		FROM market_daily_agg WHERE trade_date = ?`, date).Scan(&agg)

	upRatio := 0.0
	if ms.TotalStocks > 0 { upRatio = float64(ms.UpCount) / float64(ms.TotalStocks) }

	roll, _ := s.computeRollingStats(date)
	st := s.computeShortTerm(date)
	_ = s.computeMedianUp20(date) // used in classification but not here
	conf := confidenceScore(roll.AvgScore, roll.AvgUp, roll.AvgDiff, roll.AvgVol, roll.Trend,
		roll.SectorDispersion, roll.ScoreChange, st.u5, st.drawdown5, style)

	duration := s.computeDuration(date, string(style))
	transSignal := s.computeTransitionSignal(date, string(style), roll.Trend, roll.AvgVol, ms.Volatility)

	breakRate := s.computeBreakRate(date, ms.LimitUpCount)
	concentration := s.computeConcentration(date, agg.TotalAmt)
	rotationSpeed := s.computeRotationSpeed(date)

	topConcepts := s.aggregateConcepts(date, date, date, 15)
	topSectors := s.aggregateSectors(date, date, date, 10)
	analysisSummary := s.generateAISummary(date, string(style), conf,
		ms.CompositeScore, ms.UpCount, ms.DownCount, ms.TotalStocks,
		ms.LimitUpCount, ms.LimitDownCount, ms.SectorDiffusion, ms.Volatility, ms.NorthboundNet,
		upRatio, roll.SectorDispersion, roll.ScoreChange, breakRate, concentration, rotationSpeed,
		topSectors, topConcepts)

	topConceptsJSON, _ := json.Marshal(topConcepts)

	// Layer 1+2: Regime, Leadership, Flow
	// Only compute for recent dates (expensive queries), backfill later
	var regime MarketRegime
	var leadership ThematicLeadership
	var leadConcept string
	var leadIndustry string
	var growthDefenseFlow float64

	// Regime: always compute (fast, just 2 simple queries)
	regime = s.detectRegime(date)

	// Leadership & Flow: only for latest 3 trading days
	var daysAhead int
	db.PG.Raw(`SELECT COUNT(*) FROM market_sentiment WHERE trade_date > ?`, date).Scan(&daysAhead)
	log.Printf("[market_style] %s daysAhead=%d → leadership/flow: %v", date, daysAhead, daysAhead <= 2)
	if daysAhead <= 2 {
		leadership, leadConcept, leadIndustry = s.detectLeadership(date)
		growthDefenseFlow = s.computeGrowthDefenseFlow(date)
		log.Printf("[market_style] %s leadership=%s leadConcept=%s leadIndustry=%s flow=%.2f", date, leadership, leadConcept, leadIndustry, growthDefenseFlow)
	}
	topSectorsJSON, _ := json.Marshal(topSectors)

	return db.PG.Exec(`
		INSERT INTO market_style_daily (trade_date, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, sector_dispersion, volatility, score_trend,
			score_change, break_rate, concentration, rotation_speed,
			northbound_net, total_amount,
			limit_up_count, limit_down_count, ma20_above, n52_high, n60_low,
			style_duration, transition_signal, analysis_summary, top_sectors, top_concepts,
			market_regime, thematic_leadership, lead_concept, lead_industry, growth_defense_flow)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::jsonb,?::jsonb,?,?,?,?,?)
		ON CONFLICT (trade_date) DO UPDATE SET
			style=EXCLUDED.style, style_confidence=EXCLUDED.style_confidence,
			composite_score=EXCLUDED.composite_score, up_ratio=EXCLUDED.up_ratio,
			sector_diffusion=EXCLUDED.sector_diffusion, sector_dispersion=EXCLUDED.sector_dispersion,
			volatility=EXCLUDED.volatility, score_trend=EXCLUDED.score_trend,
			score_change=EXCLUDED.score_change, break_rate=EXCLUDED.break_rate,
			concentration=EXCLUDED.concentration, rotation_speed=EXCLUDED.rotation_speed,
			northbound_net=EXCLUDED.northbound_net,
			total_amount=EXCLUDED.total_amount, limit_up_count=EXCLUDED.limit_up_count,
			limit_down_count=EXCLUDED.limit_down_count, ma20_above=EXCLUDED.ma20_above,
			n52_high=EXCLUDED.n52_high, n60_low=EXCLUDED.n60_low,
			style_duration=EXCLUDED.style_duration, transition_signal=EXCLUDED.transition_signal,
			analysis_summary=EXCLUDED.analysis_summary, top_sectors=EXCLUDED.top_sectors, top_concepts=EXCLUDED.top_concepts,
			market_regime=EXCLUDED.market_regime, thematic_leadership=EXCLUDED.thematic_leadership,
			lead_concept=EXCLUDED.lead_concept, lead_industry=EXCLUDED.lead_industry, growth_defense_flow=EXCLUDED.growth_defense_flow
	`, date, string(style), conf, ms.CompositeScore, upRatio, ms.SectorDiffusion,
		roll.SectorDispersion, ms.Volatility, roll.Trend, roll.ScoreChange,
		breakRate, concentration, rotationSpeed,
		ms.NorthboundNet, agg.TotalAmt,
		ms.LimitUpCount, ms.LimitDownCount, agg.MA20Above, agg.N52High, agg.N60Low,
		duration, transSignal, analysisSummary,
		string(topSectorsJSON), string(topConceptsJSON),
		string(regime), string(leadership), leadConcept, leadIndustry, growthDefenseFlow).Error
}

func (s *MarketStyleService) computeDuration(date, style string) int {
	var prev struct{ TradeDate, Style string; Duration int }
	db.PG.Raw(`SELECT trade_date::text, style, style_duration FROM market_style_daily
		WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 1`, date).Scan(&prev)
	if prev.Style == style { return prev.Duration + 1 }
	return 1
}

func (s *MarketStyleService) computeTransitionSignal(date, style string, trend, avgVol20, todayVol float64) string {
	var prevTrends []float64
	db.PG.Raw(`SELECT score_trend FROM market_style_daily
		WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 3`, date).Pluck("score_trend", &prevTrends)
	if len(prevTrends) >= 3 && trend > prevTrends[0] && prevTrends[0] > prevTrends[1] && prevTrends[1] > prevTrends[2] {
		return "warming"
	}
	if len(prevTrends) >= 3 && trend < prevTrends[0] && prevTrends[0] < prevTrends[1] && prevTrends[1] < prevTrends[2] {
		if todayVol > avgVol20*1.1 { return "cooling" }
	}
	var styleChanges int
	db.PG.Raw(`SELECT COUNT(*) FROM (
		SELECT style, LAG(style) OVER (ORDER BY trade_date) as prev_style
		FROM market_style_daily WHERE trade_date <= ?
		ORDER BY trade_date DESC LIMIT 14
	) sub WHERE style != prev_style AND prev_style IS NOT NULL`, date).Scan(&styleChanges)
	if styleChanges >= 3 { return "reversal" }
	return "none"
}

// ── Helpers: aggregation, micro indicators, AI ──

func (s *MarketStyleService) computeSectorDispersion(date string) float64 {
	var dispersion float64
	db.PG.Raw(`
		WITH ind_ret AS (
			SELECT cb.concept_name AS name,
				AVG((k.close - prev.close) / NULLIF(prev.close, 0)) as avg_ret
			FROM stocks_daily_k k
			JOIN stock_concepts sc ON sc.code = k.code AND sc.concept_type = 'industry_l2'
			JOIN concept_boards cb ON cb.concept_code = sc.concept_code AND cb.concept_type = 'industry_l2'
			JOIN LATERAL (SELECT k2.close FROM stocks_daily_k k2
				WHERE k2.code = k.code AND k2.trade_date < CAST(? AS date)
				ORDER BY k2.trade_date DESC LIMIT 1) prev ON true
			WHERE k.trade_date = CAST(? AS date)
			GROUP BY cb.concept_name
		)
		SELECT COALESCE(STDDEV(avg_ret), 0) FROM ind_ret
	`, date, date).Scan(&dispersion)
	return dispersion
}

func (s *MarketStyleService) computeScoreChange(date string) float64 {
	var scores []float64
	db.PG.Raw(`SELECT composite_score FROM market_sentiment
		WHERE trade_date <= CAST(? AS date) ORDER BY trade_date DESC LIMIT 2`, date).Pluck("composite_score", &scores)
	if len(scores) >= 2 { return scores[0] - scores[1] }
	return 0
}

func (s *MarketStyleService) computeBreakRate(date string, limitUpCount int) float64 {
	if limitUpCount <= 0 { return 0 }
	var boardBreak int
	db.PG.Raw(`SELECT COALESCE(board_break,0) FROM limit_stats_daily WHERE trade_date = ?`, date).Scan(&boardBreak)
	total := limitUpCount + boardBreak
	if total <= 0 { return 0 }
	return math.Round(float64(boardBreak)/float64(total)*10000) / 10000
}

func (s *MarketStyleService) computeConcentration(date string, totalAmt float64) float64 {
	if totalAmt <= 0 { return 0 }
	var top100Amt float64
	db.PG.Raw(`SELECT COALESCE(SUM(amount),0) FROM (
		SELECT amount FROM stocks_daily_k WHERE trade_date = ? AND code NOT LIKE 'IDX%' ORDER BY amount DESC LIMIT 100
	) t`, date).Scan(&top100Amt)
	if totalAmt <= 0 { return 0 }
	return math.Round(top100Amt/totalAmt*10000) / 10000
}

func (s *MarketStyleService) computeRotationSpeed(date string) float64 {
	todaySectors := s.aggregateSectors(date, date, date, 5)
	if len(todaySectors) < 3 { return 0 }
	var prevDate string
	db.PG.Raw(`SELECT trade_date::text FROM stocks_daily_k WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 1`, date).Scan(&prevDate)
	if prevDate == "" { return 0 }
	yestNames := make(map[string]bool)
	var topSecJSON string
	db.PG.Raw(`SELECT COALESCE(top_sectors::text,'[]') FROM market_style_daily WHERE trade_date = ?`, prevDate).Scan(&topSecJSON)
	if topSecJSON != "" && topSecJSON != "[]" {
		var sectors []map[string]interface{}
		if err := json.Unmarshal([]byte(topSecJSON), &sectors); err == nil {
			cnt := 0
			for _, sec := range sectors {
				if cnt >= 5 { break }
				if name, ok := sec["name"].(string); ok { yestNames[name] = true; cnt++ }
			}
		}
	}
	newCnt := 0
	for _, sec := range todaySectors {
		if name, ok := sec["name"].(string); ok { if !yestNames[name] { newCnt++ } }
	}
	total := len(todaySectors)
	if total <= 0 { return 0 }
	return math.Round(float64(newCnt)/float64(total)*10000) / 10000
}

func (s *MarketStyleService) aggregateConcepts(dateFrom, dateTo, targetDate string, topN int) []map[string]interface{} {
	type row struct{ Name, Code string; ChgPct, UpRatio, VolRatio float64 }
	var rows []row
	db.PG.Raw(`
		WITH stock_chg AS (
			SELECT code, trade_date, close, amount,
				COALESCE((close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date))
					/ NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) * 100, 0) as chg_pct,
				LAG(amount) OVER (PARTITION BY code ORDER BY trade_date) as prev_amount
			FROM stocks_daily_k
			WHERE trade_date >= (?::date - INTERVAL '5 days') AND trade_date <= ?
		),
		concept_chg AS (
			SELECT sc.concept_name, MIN(cb.concept_code) as concept_code,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY s.chg_pct) as median_chg,
				AVG(CASE WHEN s.chg_pct > 0 THEN 1 ELSE 0 END) as up_ratio,
				AVG(s.amount) as avg_amount,
				AVG(s.prev_amount) as prev_avg_amount
			FROM stock_concepts sc
			JOIN stock_chg s ON s.code = sc.code AND s.trade_date = ?
			JOIN concept_boards cb ON cb.concept_name = sc.concept_name AND cb.concept_type = 'concept'
			GROUP BY sc.concept_name HAVING COUNT(*) >= 3
		)
		SELECT concept_name as name, concept_code as code, median_chg as chg_pct, up_ratio,
			CASE WHEN prev_avg_amount > 0 THEN avg_amount / prev_avg_amount ELSE 1 END as vol_ratio
		FROM concept_chg ORDER BY median_chg DESC LIMIT ?
	`, dateFrom, dateTo, targetDate, topN).Scan(&rows)
	result := make([]map[string]interface{}, 0, len(rows))
	for i, r := range rows {
		result = append(result, map[string]interface{}{
			"rank": i+1, "name": r.Name, "code": r.Code,
			"chgPct": math.Round(r.ChgPct*100)/100,
			"upRatio": math.Round(r.UpRatio*100)/100,
			"volRatio": math.Round(r.VolRatio*100)/100,
			"consecutiveDays": s.conceptConsecutiveDays(r.Name, targetDate, topN),
		})
	}
	return result
}

func (s *MarketStyleService) aggregateSectors(dateFrom, dateTo, targetDate string, topN int) []map[string]interface{} {
	type row struct{ Name string; ChgPct, UpRatio, VolRatio float64 }
	var rows []row
	db.PG.Raw(`
		WITH stock_chg AS (
			SELECT code, trade_date, close, amount,
				COALESCE((close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date))
					/ NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) * 100, 0) as chg_pct,
				LAG(amount) OVER (PARTITION BY code ORDER BY trade_date) as prev_amount
			FROM stocks_daily_k
			WHERE trade_date >= (?::date - INTERVAL '5 days') AND trade_date <= ?
		),
		sector_chg AS (
			SELECT sc.concept_name,
				PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY s.chg_pct) as median_chg,
				AVG(CASE WHEN s.chg_pct > 0 THEN 1 ELSE 0 END) as up_ratio,
				AVG(s.amount) as avg_amount,
				AVG(s.prev_amount) as prev_avg_amount
			FROM stock_concepts sc
			JOIN stock_chg s ON s.code = sc.code AND s.trade_date = ?
			WHERE sc.concept_type = 'industry_l2'
			GROUP BY sc.concept_name HAVING COUNT(*) >= 5
		)
		SELECT concept_name as name, median_chg as chg_pct, up_ratio,
			CASE WHEN prev_avg_amount > 0 THEN avg_amount / prev_avg_amount ELSE 1 END as vol_ratio
		FROM sector_chg ORDER BY median_chg DESC LIMIT ?
	`, dateFrom, dateTo, targetDate, topN).Scan(&rows)
	result := make([]map[string]interface{}, 0, len(rows))
	for i, r := range rows {
		result = append(result, map[string]interface{}{
			"rank": i+1, "name": r.Name,
			"chgPct": math.Round(r.ChgPct*100)/100,
			"upRatio": math.Round(r.UpRatio*100)/100,
			"volRatio": math.Round(r.VolRatio*100)/100,
			"consecutiveDays": s.conceptConsecutiveDays(r.Name, targetDate, topN),
		})
	}
	return result
}

func (s *MarketStyleService) conceptConsecutiveDays(conceptName, date string, topN int) int {
	var days int
	db.PG.Raw(`WITH dates AS (
		SELECT trade_date, top_concepts
		FROM market_style_daily WHERE trade_date <= ?
		ORDER BY trade_date DESC
	), flagged AS (
		SELECT trade_date,
			EXISTS(
				SELECT 1 FROM jsonb_array_elements(top_concepts) c
				WHERE (c->>'name') = ? AND (c->>'rank')::int <= ?
			) AS present
		FROM dates
	)
	SELECT COUNT(*) FROM (
		SELECT trade_date, present,
			ROW_NUMBER() OVER (ORDER BY trade_date DESC) -
			ROW_NUMBER() OVER (PARTITION BY present ORDER BY trade_date DESC) AS grp
		FROM flagged
	) sub WHERE present = true AND grp = 0`, date, conceptName, topN).Scan(&days)
	return days
}

// ── Smart Advice: sector-aware position & allocation recommendations ──

// sectorProfile holds a sector's relative strength info
type sectorProfile struct {
	name            string
	chgPct          float64
	consecutiveDays int
	isDefensive     bool
	isGrowth        bool
}

func (s *MarketStyleService) generateAdvice(date, style string, row StyleRow) []string {
	advice := make([]string, 0)
	params := DefaultStyleParams(MarketStyle(style))

	// Get top/bottom sectors for contextual advice
	topSectors := s.aggregateSectors(date, date, date, 10)
	var allSectors []sectorProfile
	for _, sec := range topSectors {
		if name, ok := sec["name"].(string); ok {
			chg, _ := sec["chgPct"].(float64)
			days, _ := sec["consecutiveDays"].(float64)
			allSectors = append(allSectors, sectorProfile{
				name: name, chgPct: chg, consecutiveDays: int(days),
				isDefensive: isDefensiveSector(name),
				isGrowth:    isGrowthSector(name),
			})
		}
	}

	// Detect trends
	defenseLeading := false
	growthLeading := false
	defenseNames := []string{}
	growthNames := []string{}
	for _, sec := range allSectors {
		if sec.isDefensive && sec.chgPct > 0.5 && sec.consecutiveDays >= 2 {
			defenseLeading = true
			defenseNames = append(defenseNames, sec.name)
		}
		if sec.isGrowth && sec.chgPct > 0.5 && sec.consecutiveDays >= 2 {
			growthLeading = true
			growthNames = append(growthNames, sec.name)
		}
	}

	growthWeakening := false
	if growthLeading && row.RotationSpeed > 0.6 && row.SectorDispersion > 0.012 {
		growthWeakening = true
	}

	// ── Position advice ──
	switch MarketStyle(style) {
	case StyleTrendUp:
		advice = append(advice,
			fmt.Sprintf("📈 顺势做多，总仓位建议 %.0f%%-%.0f%%", params.PositionBias*60, params.PositionBias*80),
			fmt.Sprintf("📊 单票仓位 %.0f%%，可加仓 %.0f%%，条件用 %s 逻辑", params.BuyPct, params.AddPct, strings.ToUpper(params.BuyLogic)),
		)
		if len(growthNames) > 0 {
			advice = append(advice, fmt.Sprintf("🎯 主攻方向：%s（顺势最强）", strings.Join(growthNames[:min(3, len(growthNames))], "、")))
		}
		if len(defenseNames) > 0 {
			advice = append(advice, fmt.Sprintf("🛡 防御配置：%s（%.0f%%仓位做对冲）", strings.Join(defenseNames[:min(2, len(defenseNames))], "、"), params.BuyPct*0.3))
		}

	case StyleStructural:
		advice = append(advice,
			fmt.Sprintf("📈 结构行情，总仓位 %.0f%%-%.0f%%（资金抱团，非普涨）", params.PositionBias*50, params.PositionBias*65),
			fmt.Sprintf("📊 单票仓位 %.0f%%，只在主线板块选股，严格 AND 条件", params.BuyPct),
		)
		if growthWeakening {
			advice = append(advice,
				"⚠️ 成长板块出现分化信号：轮动加速+离散度上升",
				"💡 建议：持有仓位逢高减仓 30%-50%，未持有观望等待回调",
			)
		}
		if defenseLeading && !growthLeading {
			advice = append(advice,
				fmt.Sprintf("🔄 资金向防御板块迁移：%s", strings.Join(defenseNames[:min(3, len(defenseNames))], "、")),
				fmt.Sprintf("💡 增加防御配置至 %.0f%% 仓位，降低成长暴露", params.BuyPct*0.6),
			)
		}
		if growthLeading {
			advice = append(advice,
				fmt.Sprintf("🎯 主线板块：%s（连续领涨，但注意集中度风险）", strings.Join(growthNames[:min(3, len(growthNames))], "、")),
				fmt.Sprintf("💡 聚焦前 %.0f%% 概念，单票止盈回撤 %.0f%%", params.ConceptTopPct*100, params.TrailingStopDrawdown),
			)
		}

	case StyleChoppy:
		advice = append(advice,
			fmt.Sprintf("📊 震荡轮动，总仓位 %.0f%%-%.0f%%（不加仓）", params.PositionBias*40, params.PositionBias*60),
			fmt.Sprintf("📊 单票仓位 %.0f%%，快进快出", params.BuyPct),
		)
		if row.RotationSpeed > 0.6 {
			advice = append(advice,
				"⚠️ 板块轮动加速，追高风险大",
				"💡 只做低吸不追高，持仓周期压缩至 1-3 天",
			)
		}
		if defenseLeading {
			advice = append(advice,
				fmt.Sprintf("🛡 防御板块走强：%s，适度配置 %.0f%% 防御仓位", strings.Join(defenseNames[:min(2, len(defenseNames))], "、"), params.BuyPct*0.3),
			)
		}

	case StyleDecline:
		advice = append(advice,
			"🔴 风险释放中，降低总仓位至 10% 以下或空仓",
			"🛑 不买新股，不加仓，只做止损/止盈平仓",
		)
		if defenseLeading {
			advice = append(advice,
				fmt.Sprintf("🛡 防御板块逆势走强（%s），可保留 %.0f%% 防御配置对冲", strings.Join(defenseNames[:min(2, len(defenseNames))], "、"), params.BuyPct),
			)
		} else {
			advice = append(advice, "💡 关注防御板块（医药/公用事业/消费）是否有异动，作为下一步配置方向")
		}
		advice = append(advice, "📉 等待 5 日 up_ratio 回升至 35% 以上再考虑入场")

	case StyleCrash:
		advice = append(advice,
			"⚫ 恐慌模式：强制空仓，保护本金",
			"🛑 不进行任何买入操作，不抄底",
			fmt.Sprintf("⏳ 等待波动率回落到 %.0f%% 以下再考虑试探", 15.0),
		)
		if row.N60Low > row.N52High {
			advice = append(advice, fmt.Sprintf("📉 新低数量(%d)远超新高(%d)，市场信心崩溃，观望为主", row.N60Low, row.N52High))
		}

	case StyleWeakRange:
		advice = append(advice,
			fmt.Sprintf("🟤 磨底阶段，总仓位 %.0f%%-%.0f%%（试探建仓）", params.PositionBias*20, params.PositionBias*30),
			"💡 只在前 10% 概念板块中试仓，见好就收",
		)
		if defenseLeading {
			advice = append(advice,
				fmt.Sprintf("🛡 防御板块率先企稳：%s，优先配置", strings.Join(defenseNames[:min(2, len(defenseNames))], "、")),
			)
		}
		advice = append(advice,
			"📈 关注回暖信号：连续 2 日 up_ratio > 40% + trend 转正，可转结构/趋势配置",
		)
	}

	// ── Risk alerts (style-independent) ──
	if row.RotationSpeed > 0.8 {
		advice = append(advice, "⚠️ 板块轮动极快（>80%），任何追涨策略风险极高")
	}
	if row.BreakRate > 0.3 {
		advice = append(advice, fmt.Sprintf("⚠️ 炸板率 %.0f%% 偏高，市场封板意愿弱，谨慎追涨停", row.BreakRate*100))
	}
	if row.Concentration > 0.35 {
		advice = append(advice, fmt.Sprintf("📊 资金高度集中（Top100=%.0f%%），非主线板块流动性差", row.Concentration*100))
	}

	// Transition signal
	switch row.TransitionSignal {
	case "warming":
		advice = append(advice, "🔥 转暖信号：情绪连续改善，可逐步加仓")
	case "cooling":
		advice = append(advice, "❄️ 转冷信号：情绪回落+波动上升，减仓防守")
	case "reversal":
		advice = append(advice, "🔄 风格切换中：降低仓位等待新主线确认")
	}

	return advice
}

func isDefensiveSector(name string) bool {
	switch name {
	case "医药", "食品饮料", "公用事业", "银行", "保险", "农林牧渔", "煤炭", "交通运输":
		return true
	}
	return false
}

func isGrowthSector(name string) bool {
	switch name {
	case "电子", "计算机", "通信", "传媒", "电力设备", "国防军工", "半导体", "软件服务", "元器件":
		return true
	}
	return false
}

func min(a, b int) int { if a < b { return a }; return b }

// ── API Query methods ──

func (s *MarketStyleService) GetStyleCurve(from, to string) ([]StyleRow, error) {
	type rawRow struct {
		StyleRow
		TopSecStr          string
		TopConStr          string
		AnalysisSummaryStr string
	}
	var raws []rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, COALESCE(sector_dispersion,0) as sector_dispersion,
			volatility, score_trend, COALESCE(score_change,0) as score_change,
			COALESCE(break_rate,0) as break_rate, COALESCE(concentration,0) as concentration,
			COALESCE(rotation_speed,0) as rotation_speed,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'') as analysis_summary_str,
			COALESCE(market_regime,'') as market_regime,
			COALESCE(lead_concept,'') as lead_concept, COALESCE(lead_industry,'') as lead_industry,
			COALESCE(growth_defense_flow,0) as growth_defense_flow
		FROM market_style_daily
		WHERE trade_date >= ? AND trade_date <= ?
		ORDER BY trade_date
	`, from, to).Scan(&raws).Error
	if err != nil { return nil, err }
	rows := make([]StyleRow, len(raws))
	for i := range raws {
		rows[i] = raws[i].StyleRow
		rows[i].TopSectors = json.RawMessage(raws[i].TopSecStr)
		rows[i].TopConcepts = json.RawMessage(raws[i].TopConStr)
		rows[i].AnalysisSummary = raws[i].AnalysisSummaryStr
		rows[i].StyleName = formatStyleDisplay(rows[i].Style)
	}
	return rows, nil
}

func (s *MarketStyleService) GetDailyReview(date string) (*DailyReview, error) {
	type rawRow struct {
		StyleRow
		TopSecStr          string
		TopConStr          string
		AnalysisSummaryStr string
	}
	var r rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, COALESCE(sector_dispersion,0) as sector_dispersion,
			volatility, score_trend, COALESCE(score_change,0) as score_change,
			COALESCE(break_rate,0) as break_rate, COALESCE(concentration,0) as concentration,
			COALESCE(rotation_speed,0) as rotation_speed,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'') as analysis_summary_str,
			COALESCE(market_regime,'') as market_regime,
			COALESCE(lead_concept,'') as lead_concept, COALESCE(lead_industry,'') as lead_industry,
			COALESCE(growth_defense_flow,0) as growth_defense_flow
		FROM market_style_daily WHERE trade_date = ?
	`, date).Scan(&r).Error
	if err != nil { return nil, err }
	row := r.StyleRow
	row.TopSectors = json.RawMessage(r.TopSecStr)
	row.TopConcepts = json.RawMessage(r.TopConStr)
	row.AnalysisSummary = r.AnalysisSummaryStr
	row.StyleName = formatStyleDisplay(row.Style)

	var ms struct{ UpCount, DownCount, TotalStocks int }
	db.PG.Raw(`SELECT up_count, down_count, total_stocks FROM market_sentiment WHERE trade_date=?`, date).Scan(&ms)

	var prevAmt float64
	db.PG.Raw(`SELECT total_amount FROM market_daily_agg WHERE trade_date < ? ORDER BY trade_date DESC LIMIT 1`, date).Scan(&prevAmt)

	advice := s.generateAdvice(date, row.Style, row)

	return &DailyReview{
		StyleRow:        row,
		UpCount:         ms.UpCount,
		DownCount:       ms.DownCount,
		TotalStocks:     ms.TotalStocks,
		PrevAmount:      prevAmt,
		OperationAdvice: advice,
	}, nil
}

func (s *MarketStyleService) GetLatestStyle() (*StyleRow, error) {
	type rawRow struct {
		StyleRow
		TopSecStr          string
		TopConStr          string
		AnalysisSummaryStr string
	}
	var r rawRow
	err := db.PG.Raw(`
		SELECT trade_date::text, style, style_confidence, composite_score,
			up_ratio, sector_diffusion, COALESCE(sector_dispersion,0) as sector_dispersion,
			volatility, score_trend, COALESCE(score_change,0) as score_change,
			COALESCE(break_rate,0) as break_rate, COALESCE(concentration,0) as concentration,
			COALESCE(rotation_speed,0) as rotation_speed,
			northbound_net, total_amount, limit_up_count, limit_down_count,
			ma20_above, n52_high, n60_low, style_duration, transition_signal,
			COALESCE(top_sectors::text,'[]') as top_sec_str,
			COALESCE(top_concepts::text,'[]') as top_con_str,
			COALESCE(analysis_summary,'') as analysis_summary_str,
			COALESCE(market_regime,'') as market_regime,
			COALESCE(lead_concept,'') as lead_concept, COALESCE(lead_industry,'') as lead_industry,
			COALESCE(growth_defense_flow,0) as growth_defense_flow
		FROM market_style_daily ORDER BY trade_date DESC LIMIT 1
	`).Scan(&r).Error
	if err != nil { return nil, err }
	row := r.StyleRow
	row.TopSectors = json.RawMessage(r.TopSecStr)
	row.TopConcepts = json.RawMessage(r.TopConStr)
	row.AnalysisSummary = r.AnalysisSummaryStr
	row.StyleName = formatStyleDisplay(row.Style)
	return &row, nil
}


// getRecentDates returns the N most recent trading dates before the given date
func (s *MarketStyleService) getRecentDates(date string, n int) []string {
	var dates []string
	db.PG.Raw(`SELECT trade_date::text FROM market_style_daily WHERE trade_date < ? ORDER BY trade_date DESC LIMIT ?`, date, n).Scan(&dates)
	return dates
}

// ── AI Summary ──

func (s *MarketStyleService) generateAISummary(date, style string, conf float64,
	compositeScore float64, upCount, downCount, totalStocks, limitUpCount, limitDownCount int,
	sectorDiffusion, volatility, northboundNet float64,
	upRatio, sectorDispersion, scoreChange, breakRate, concentration, rotationSpeed float64,
	topSectors, topConcepts []map[string]interface{}) string {

	sectorSummary := ""
	for i, sec := range topSectors {
		if i >= 5 { break }
		name, _ := sec["name"].(string)
		chg, _ := sec["chgPct"].(float64)
		sectorSummary += fmt.Sprintf("%s%+.1f%% ", name, chg)
	}
	conceptSummary := ""
	for i, con := range topConcepts {
		if i >= 3 { break }
		name, _ := con["name"].(string)
		chg, _ := con["chgPct"].(float64)
		days, _ := con["consecutiveDays"].(float64)
		conceptSummary += fmt.Sprintf("%s%+.1f%%(%d天) ", name, chg, int(days))
	}
	systemPrompt := `你是A股市场分析师。基于数据生成80-120字市场日评。格式：一段话，包含[风格诊断] + [关键信号] + [操作方向] + [风险提示]。`
	userPrompt := fmt.Sprintf(`日期：%s | 风格：%s(%.0f) | 情绪：%.1f(Δ%.1f) | 涨跌比：%.0f:%.0f
领涨行业：%s | 热门概念：%s | 炸板率：%.0f%% | 集中度：%.0f%% | 轮动：%.0f%%
涨停%d/跌停%d | 北向：%.1f亿`,
		date, style, conf, compositeScore, scoreChange, float64(upCount), float64(downCount),
		sectorSummary, conceptSummary, breakRate*100, concentration*100, rotationSpeed*100,
		limitUpCount, limitDownCount, northboundNet/1e8)

	aiSummary, err := s.callSystemAI(userPrompt, systemPrompt)
	if err != nil {
		log.Printf("[market_style] AI failed for %s: %v, fallback", date, err)
		return s.generateFallbackSummary(date, style, compositeScore, upCount, downCount, totalStocks, limitUpCount, limitDownCount, upRatio, sectorDispersion, scoreChange, breakRate, rotationSpeed)
	}
	return aiSummary
}

func (s *MarketStyleService) callSystemAI(userPrompt, systemPrompt string) (string, error) {
	var cfg struct{ AgentModelName, AgentBaseURL, AgentAPIKey string }
	err := db.PG.Raw(`SELECT COALESCE(agent_model_name,''), COALESCE(agent_base_url,''), COALESCE(agent_api_key,'')
		FROM ai_system_configs WHERE scene = 'market_style' LIMIT 1`).Scan(&cfg).Error
	if err != nil || cfg.AgentAPIKey == "" { return "", fmt.Errorf("no system AI config") }

	type msg struct{ Role, Content string }
	type req struct{ Model string; Messages []msg; MaxTokens int; Temperature float64 `json:"max_tokens"` }
	type resp struct{ Choices []struct{ Message struct{ Content string `json:"content"` } `json:"message"` } `json:"choices"` }

	body := req{Model: cfg.AgentModelName, Messages: []msg{{"system", systemPrompt}, {"user", userPrompt}}, MaxTokens: 256, Temperature: 0.5}
	jsonBody, _ := json.Marshal(body)
	httpReq, _ := http.NewRequest("POST", cfg.AgentBaseURL+"/v1/chat/completions", strings.NewReader(string(jsonBody)))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.AgentAPIKey)
	httpResp, err := (&http.Client{Timeout: 15 * time.Second}).Do(httpReq)
	if err != nil { return "", err }
	defer httpResp.Body.Close()
	var r resp
	if err := json.NewDecoder(httpResp.Body).Decode(&r); err != nil { return "", err }
	if len(r.Choices) == 0 { return "", fmt.Errorf("empty response") }
	return r.Choices[0].Message.Content, nil
}

func (s *MarketStyleService) generateFallbackSummary(date, style string,
	compositeScore float64, upCount, downCount, totalStocks, limitUpCount, limitDownCount int,
	upRatio, sectorDispersion, scoreChange, breakRate, rotationSpeed float64) string {

	parts := []string{}
	styleName := formatStyleDisplay(style)
	parts = append(parts, fmt.Sprintf("【%s】", styleName))
	if scoreChange < -20 { parts = append(parts, fmt.Sprintf("情绪暴跌%.0f分", -scoreChange))
	} else if scoreChange < -10 { parts = append(parts, fmt.Sprintf("情绪回落%.0f分", -scoreChange)) }
	if sectorDispersion > 0.012 { parts = append(parts, fmt.Sprintf("板块分化%.1f%%", sectorDispersion*100)) }
	parts = append(parts, fmt.Sprintf("涨跌%d:%d，涨停%d跌停%d", upCount, downCount, limitUpCount, limitDownCount))
	if breakRate > 0.3 { parts = append(parts, fmt.Sprintf("炸板率%.0f%%", breakRate*100)) }
	if rotationSpeed > 0.6 { parts = append(parts, "轮动加速") }
	return strings.Join(parts, "；")
}
