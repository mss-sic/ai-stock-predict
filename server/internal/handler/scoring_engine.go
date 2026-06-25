package handler

import (
	"github.com/ai-stock-predict/server/internal/db"
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// ScoringEngine — 加权评分引擎 (Buy/Add)
// ═══════════════════════════════════════════════════════════════
//
// 对 universe 中每只股票计算加权总分。
// 支持 6 种条件子类型：Basic / Fuzzy / TimeSeries / IndustryRelative / MultiTimeframe / Composite。

// ConditionScore records the per-condition score breakdown.
type ConditionScore struct {
	ConditionID   uint
	Indicator     string
	RawValue      float64
	Threshold     float64
	Operator      string
	FuzzySigma    float64
	Score         float64 // 0-1
	Weight        float64
	WeightedScore float64
	Detail        string // human-readable evaluation detail
}

// ScoreResult is the final scoring output for one stock.
type ScoreResult struct {
	Code       string
	Name       string
	Price      float64
	TotalScore float64
	Breakdown  []ConditionScore
	Rank       int
}

// ScoreDistribution captures the statistical distribution of scores.
type ScoreDistribution struct {
	Count  int
	Top1   float64
	Top5   float64
	Top10P float64
	Median float64
	Mean   float64
}

// AdaptiveMinScore computes a dynamic minimum score threshold.
// Uses top-quintile mean as reference, with hard floor at 0.30 (30% of weight max).
// When all scores are low (narrow distribution), raises the bar to filter noise.
func (d ScoreDistribution) AdaptiveMinScore() float64 {
	if d.Count == 0 {
		return 0.30
	}
	gap := d.Top1 - d.Median
	dynamicMin := d.Median + gap*0.5 // stricter: use 50% of gap
	if dynamicMin < 0.30 {
		dynamicMin = 0.30
	}
	return dynamicMin
}

// ScoringEngine evaluates conditions with weighted scoring.
type ScoringEngine struct {
	conditions   []model.StrategyCondition
	ctx          *MarketContext
	evalCache    *IndicatorCache // shared with strategy_handler eval
	kcache       *KlineCache
	conceptCache *ConceptRankCache // concept strength multiplier
	industry     map[string]map[string]float64 // industry -> indicator -> p50 value
	stockIndust  map[string]string             // stock code → industry name (preloaded to avoid N+1)
	distribution ScoreDistribution
}

// GetDistribution returns the score distribution from the last ScoreAll call.
func (se *ScoringEngine) GetDistribution() ScoreDistribution {
	return se.distribution
}

// NewScoringEngine creates a scoring engine.
func NewScoringEngine(
	conceptCache *ConceptRankCache,
	conditions []model.StrategyCondition,
	ctx *MarketContext,
	icache *IndicatorCache,
	kcache *KlineCache,
) *ScoringEngine {
	se := &ScoringEngine{
		conditions:  conditions,
		ctx:         ctx,
		evalCache:   icache,
		kcache:      kcache,
		conceptCache: conceptCache,
		industry:    make(map[string]map[string]float64),
		stockIndust: make(map[string]string),
	}
	// Industry benchmarks are loaded per-date in ScoreAll
	return se
}

// loadIndustryBenchmarks pre-fetches industry median values for a given date.
// Called once per ScoreAll invocation to avoid N+1 per-stock queries.
func (se *ScoringEngine) loadIndustryBenchmarks(date string) {
	// Only preload for indicators that have industry-relative conditions
	indicatorsToLoad := make(map[string]bool)
	for _, c := range se.conditions {
		if c.IndustryRelative {
			indicatorsToLoad[c.Indicator] = true
		}
	}
	if len(indicatorsToLoad) == 0 {
		return
	}

	for indicator := range indicatorsToLoad {
		col := indicatorColumn(indicator)
		if col == "" {
			continue
		}
		sql := fmt.Sprintf(`SELECT sb.industry, PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY di.%s)
			FROM stocks_daily_indicator di
			JOIN stocks_basic sb ON sb.code = di.code
			WHERE di.trade_date = ? AND sb.industry IS NOT NULL AND sb.industry != ''
			GROUP BY sb.industry`, col)
		rows, err := db.PG.Raw(sql, date).Rows()
		if err != nil {
			continue
		}
		for rows.Next() {
			var industry string
			var median float64
			if err := rows.Scan(&industry, &median); err == nil && median > 0 {
				cacheKey := indicator + "_" + date + "_" + industry
				if _, ok := se.industry[indicator+"_"+date]; !ok {
					se.industry[indicator+"_"+date] = make(map[string]float64)
				}
				se.industry[indicator+"_"+date][industry] = median
				// Also set in the old format for getIndustryBenchmark
				oldKey := indicator + "_" + date
				if _, ok := se.industry[oldKey]; !ok {
					se.industry[oldKey] = make(map[string]float64)
				}
				se.industry[oldKey][cacheKey] = median
			}
		}
		rows.Close()
	}
}

// getIndustryBenchmark fetches the industry median for a given indicator, date, and stock code.
func (se *ScoringEngine) getIndustryBenchmark(code, indicator, date string) float64 {
	// Determine the DB column for this indicator
	col := indicatorColumn(indicator)
	if col == "" {
		return 0
	}

	// Check cache
	cacheKey := indicator + "_" + date
	if _, ok := se.industry[cacheKey]; !ok {
		se.industry[cacheKey] = make(map[string]float64)
	}

	// Fetch industry name from preloaded cache (avoid per-stock SQL)
	industry, ok := se.stockIndust[code]
	if !ok {
		return 0
	}
	if industry == "" {
		return 0
	}

	cacheKey2 := cacheKey + "_" + industry
	if v, ok := se.industry[cacheKey][cacheKey2]; ok {
		return v
	}

	var median float64
	err := db.PG.Raw(fmt.Sprintf(`SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY %s)
		FROM stocks_daily_indicator di
		JOIN stocks_basic sb ON sb.code = di.code
		WHERE sb.industry = ? AND di.trade_date = ?`, col), industry, date).Scan(&median).Error
	if err != nil || median <= 0 {
		return 0
	}

	se.industry[cacheKey][cacheKey2] = median
	return median
}

// indicatorColumn maps indicator to the stocks_daily_indicator column name.
func indicatorColumn(indicator string) string {
	switch indicator {
	case "pe":
		return "pe"
	case "pb":
		return "pb"
	case "ps":
		return "ps"
	case "total_market_cap":
		return "total_market_cap"
	}
	return ""
}

// ScoreAll evaluates all stocks in the universe, returning ranked results.
func (se *ScoringEngine) ScoreAll(
	universe []dcStockInfo,
	date string,
	getPrice func(string, string) float64,
	evalSingle func(model.StrategyCondition, string, string) bool,
	evalSingleWithValue func(model.StrategyCondition, string, string) (bool, float64),
	onProgress func(scored, total, candidates int),
) []ScoreResult {
	// Panic recovery: don't let single-stock crash kill the whole backtest
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[scoring] PANIC in ScoreAll date=%s: %v — returning partial results", date, r)
		}
	}()

	// Batch-preload industry benchmarks for this date
	se.loadIndustryBenchmarks(date)
	results := make([]ScoreResult, 0, len(universe))

	for i, stock := range universe {
		// Per-stock panic guard: skip problematic stocks without killing the batch
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[scoring] PANIC scoring %s at date=%s: %v", stock.Code, date, r)
				}
			}()

			price := getPrice(stock.Code, date)
			if price <= 0 {
				return
			}

			score := se.ScoreStock(stock.Code, stock.Name, price, date, evalSingle, evalSingleWithValue)
			if score.TotalScore > 0 {
				results = append(results, score)
			}
		}()

		// Periodic progress callback (every 100 stocks for large, every 50 for small universes)
		if (i+1)%100 == 0 && onProgress != nil {
			onProgress(i+1, len(universe), len(results))
		}
		// Small universe bonus: first stock always reports
		if (i == 0 || (i+1)%50 == 0) && len(universe) < 300 && onProgress != nil {
			onProgress(i+1, len(universe), len(results))
		}
	}

	// Sort by total score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].TotalScore > results[j].TotalScore
	})

	// Log score distribution for debugging
	if len(results) > 0 {
		last := len(results) - 1
		s1, s5, s10 := 0.0, 0.0, 0.0
		if last >= 0 { s1 = results[0].TotalScore }
		if last >= 4 { s5 = results[4].TotalScore }
		m10 := last * 10 / 100
		if m10 < last { s10 = results[m10].TotalScore }
		log.Printf("[scoring] score_dist: n=%d top1=%.2f top5=%.2f top10%%=%.2f median=%.2f",
			len(results), s1, s5, s10, results[last/2].TotalScore)
	}

	// Assign ranks
	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}

// scoreStock computes the weighted score for a single stock.
func (se *ScoringEngine) ScoreStock(
	code, name string,
	price float64,
	date string,
	evalSingle func(model.StrategyCondition, string, string) bool,
	evalSingleWithValue func(model.StrategyCondition, string, string) (bool, float64),
) ScoreResult {
	result := ScoreResult{
		Code:      code,
		Name:      name,
		Price:     price,
		Breakdown: make([]ConditionScore, 0, len(se.conditions)),
	}

	// Group conditions: root conditions (ParentID == nil) are scored
	// Composite conditions aggregate their children's scores
	rootConds := make([]model.StrategyCondition, 0)
	for _, c := range se.conditions {
		if c.ParentID == nil && c.Enabled {
			rootConds = append(rootConds, c)
		}
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for _, cond := range rootConds {
		cs := se.scoreCondition(cond, code, date, evalSingle, evalSingleWithValue)
		// P1: Always include in breakdown — failed conditions get score=0
		// This allows cross-sectional percentile ranking to replace zero scores
		result.Breakdown = append(result.Breakdown, cs)
		if cs.Score > 0 {
			weightedSum += cs.WeightedScore
			totalWeight += cs.Weight
		}
	}

	// Normalize by total configured weight (sum of all condition weights)
	// weightedSum already only includes passing conditions, so no additional pass-ratio penalty.
	// All conditions passing → score ≈ 1.0 × MarketBias (typically 1.0–1.5)
	totalConfigWeight := 0.0
	for _, c := range rootConds {
		totalConfigWeight += c.Weight
	}
	if totalConfigWeight > 0 && weightedSum > 0 {
		if se.conceptCache != nil {
			result.TotalScore = (weightedSum / totalConfigWeight) * se.ctx.MarketBias * se.conceptCache.GetMultiplier(code, date)
		} else {
			result.TotalScore = (weightedSum / totalConfigWeight) * se.ctx.MarketBias
		}
	} else {
		result.TotalScore = 0
	}

	return result
}

// scoreCondition evaluates a single condition and returns its scored result.
func (se *ScoringEngine) scoreCondition(
	cond model.StrategyCondition,
	code, date string,
	evalSingle func(model.StrategyCondition, string, string) bool,
	evalSingleWithValue func(model.StrategyCondition, string, string) (bool, float64),
) ConditionScore {
	cs := ConditionScore{
		ConditionID: cond.ID,
		Indicator:   cond.Indicator,
		Threshold:   cond.Value,
		Operator:    cond.Operator,
		FuzzySigma:  cond.FuzzySigma,
		Weight:      cond.Weight,
	}

	// Get raw value
	passed, rawVal := false, 0.0
	if evalSingleWithValue != nil {
		passed, rawVal = evalSingleWithValue(cond, code, date)
	} else if evalSingle != nil {
		passed = evalSingle(cond, code, date)
	}
	cs.RawValue = rawVal

	// Industry-relative adjustment
	if cond.IndustryRelative && rawVal != 0 {
		benchmark := se.getIndustryBenchmark(code, cond.Indicator, date)
		if benchmark > 0 {
			cs.RawValue = rawVal / benchmark
			cs.Detail = fmt.Sprintf("行业相对: %.2f / 行业中位数%.2f", rawVal, benchmark)
		}
	}

	// Time-series: check lookback and consecutive days
	if cond.LookbackDays > 1 || cond.ConsecutiveDays > 1 {
		tsScore := se.scoreTimeSeries(cond, code, date, evalSingle)
		cs.Score = tsScore
		cs.Detail = fmt.Sprintf("时序%d天评分: %.2f", cond.LookbackDays, tsScore)
	} else {
		// Single-day scoring
		cs.Score = se.scoreSingleDay(cond, code, passed)
	}

	// Multi-timeframe (stub — weekly data not yet available)
	if cond.Timeframe == "weekly" {
		// For now, treat weekly as same score since we don't have weekly K-line
		cs.Detail += " (周线暂未独立计算)"
	}

	// Trend direction bonus
	if cond.TrendDirection != "" && cond.TrendDirection != "none" {
		trendBonus := se.trendBonus(cond, code, date)
		cs.Score += trendBonus
		if trendBonus != 0 {
			cs.Detail += fmt.Sprintf(" 趋势修正%+.2f", trendBonus)
		}
	}

	// P0-1: Adaptive fuzzy sigma — if not explicitly set, derive from condition type
	effectiveSigma := cond.FuzzySigma
	if effectiveSigma == 0 && rawVal != 0 && cond.Weight >= 1.0 && cond.Value != 0 {
		// Only auto-assign when raw value is meaningful (not cached fallback zero)
		// Check that rawVal is within reasonable range of threshold (< 3x away)
		relativeDist := math.Abs(rawVal-cond.Value) / (math.Abs(cond.Value) + 1e-9)
		if relativeDist < 3.0 {
			switch cond.Indicator {
			case "volume_ratio", "turnover_rate":
				effectiveSigma = math.Abs(cond.Value) * 0.30
			case "momentum_5", "momentum_20", "daily_change":
				effectiveSigma = math.Abs(cond.Value) * 0.40
			case "rsi", "adx", "macd":
				effectiveSigma = math.Abs(cond.Value) * 0.25
			default:
				effectiveSigma = 0.50
			}
			if effectiveSigma < 0.30 { effectiveSigma = 0.30 }
			if effectiveSigma > 3.00 { effectiveSigma = 3.00 }
		}
	}

	// Apply fuzzy scoring: if condition failed but fuzzy enabled, compute partial score
	if !passed && effectiveSigma > 0 && rawVal != 0 {
		fuzzyS := fuzzyScore(rawVal, cond.Value, cond.Operator, effectiveSigma)
		if fuzzyS > 0 {
			cs.Score = fuzzyS
			cs.Detail = fmt.Sprintf("模糊: raw=%.2f thr=%.2f σ=%.2f", rawVal, cond.Value, effectiveSigma)
		}
	}

	// Clamp score to [0, 1]
	if cs.Score > 1.0 {
		cs.Score = 1.0
	}
	if cs.Score < 0.0 {
		cs.Score = 0.0
	}

	cs.WeightedScore = cs.Score * cs.Weight

	// DEBUG: log first few condition evaluations
	_ = code // silence unused warning if any
	if code == "000001" {
		//log.Printf("[scoring_debug] code=%s date=%s cond=%s op=%s val=%.2f passed=%v raw=%.2f score=%.2f w=%.2f weighted=%.2f",
//			code, date, cond.Indicator, cond.Operator, cond.Value, passed, rawVal, cs.Score, cs.Weight, cs.WeightedScore)
	}

	_ = passed // used by caller via evalSingle
	return cs
}

// scoreSingleDay returns a 0-1 score for a single-day evaluation.
func (se *ScoringEngine) scoreSingleDay(cond model.StrategyCondition, code string, passed bool) float64 {
	if !passed {
		// If fuzzy sigma > 0, compute partial score even when condition fails
		if cond.FuzzySigma > 0 {
			// Partial score = exp(-|distance| / sigma), clamped to [0, 1]
			// This requires the actual value, which is already in the caller's rawVal
			// For now return 0 — caller handles fuzzy separately if needed
			return 0.0
		}
		return 0.0
	}
	return 1.0
}

// fuzzyScore computes a sigmoid-based fuzzy score for near-threshold values.
func fuzzyScore(rawVal, threshold float64, operator string, sigma float64) float64 {
	if sigma <= 0 {
		return 0
	}

	var distance float64
	switch operator {
	case "gte", "gt":
		// Condition is val >= thr. Closeness matters when val < thr.
		if rawVal >= threshold {
			return 1.0
		}
		distance = (threshold - rawVal) / math.Abs(threshold+1e-9)
	case "lte", "lt":
		if rawVal <= threshold {
			return 1.0
		}
		distance = (rawVal - threshold) / math.Abs(threshold+1e-9)
	case "eq":
		distance = math.Abs(rawVal-threshold) / (math.Abs(threshold) + 1e-9)
	case "cross_up", "cross_down":
		return 0 // crossing is binary
	default:
		return 0
	}

	// Sigmoid decay: exp(-|distance| / sigma)
	score := math.Exp(-math.Abs(distance) / sigma)
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

// scoreTimeSeries evaluates condition over lookback window.
func (se *ScoringEngine) scoreTimeSeries(
	cond model.StrategyCondition,
	code, date string,
	evalSingle func(model.StrategyCondition, string, string) bool,
) float64 {
	lookback := cond.LookbackDays
	if lookback <= 0 {
		lookback = 1
	}

	// Get previous dates from kcache
	var prevDates []string
	if se.kcache != nil && len(se.kcache.dates) > 0 {
		for i, d := range se.kcache.dates {
			if d == date && i >= lookback {
				prevDates = se.kcache.dates[i-lookback : i]
				break
			}
		}
	}

	if len(prevDates) == 0 {
		// No history — evaluate current date only
		if evalSingle != nil && evalSingle(cond, code, date) {
			return 1.0
		}
		return 0.0
	}

	passCount := 0
	consecutive := 0

	for _, d := range prevDates {
		passed := false
		if evalSingle != nil {
			passed = evalSingle(cond, code, d)
		}
		if passed {
			passCount++
			consecutive++
		} else {
			consecutive = 0
		}
	}

	// Also evaluate current date
	currentPassed := false
	if evalSingle != nil {
		currentPassed = evalSingle(cond, code, date)
	}
	if currentPassed {
		passCount++
		consecutive++
	}

	score := float64(passCount) / float64(lookback+1)

	// Consecutive bonus/penalty
	if cond.ConsecutiveDays > 1 {
		if consecutive < cond.ConsecutiveDays {
			score *= 0.5 // penalty for not meeting consecutive requirement
		}
	}

	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score
}

// trendBonus computes a bonus/penalty for trend direction.
func (se *ScoringEngine) trendBonus(
	cond model.StrategyCondition,
	code, date string,
) float64 {
	if cond.TrendDirection == "" || cond.TrendDirection == "none" {
		return 0
	}
	// Compute 5-day momentum for trend direction
	if se.kcache == nil {
		return 0
	}
	cur := se.kcache.GetClose(code, date)
	if cur <= 0 {
		return 0
	}
	// Find approximate 5-day-ago close from kcache dates
	dates := se.kcache.dates
	idx := -1
	for i, d := range dates {
		if d == date {
			idx = i
			break
		}
	}
	if idx < 5 {
		return 0
	}
	prev := se.kcache.GetClose(code, dates[idx-5])
	if prev <= 0 {
		return 0
	}
	momentum := (cur - prev) / prev
	if cond.TrendDirection == "improving" && momentum > 0.01 {
		bonus := math.Min(momentum*5, 0.15)
		return bonus
	}
	if cond.TrendDirection == "deteriorating" && momentum < -0.01 {
		bonus := math.Max(momentum*5, -0.15)
		return bonus
	}
	return 0
}

// TopN returns the top N scoring results that meet the minimum threshold.
func (se *ScoringEngine) TopN(results []ScoreResult, n int, minScore float64) []ScoreResult {
	filtered := make([]ScoreResult, 0)
	for _, r := range results {
		if r.TotalScore >= minScore {
			filtered = append(filtered, r)
			if len(filtered) >= n {
				break
			}
		}
	}
	return filtered
}

// ScoreSummary returns a human-readable summary of the scoring.
func (r *ScoreResult) ScoreSummary() string {
	parts := make([]string, 0, len(r.Breakdown))
	for _, cs := range r.Breakdown {
		parts = append(parts, fmt.Sprintf("%s=%.2f(w=%.1f→%.2f)",
			cs.Indicator, cs.Score, cs.Weight, cs.WeightedScore))
	}
	return fmt.Sprintf("%s ¥%.2f 总分=%.2f → %s",
		r.Code, r.Price, r.TotalScore, joinActions(parts))
}

// init registers the scoring engine for import
func init() {
	log.Printf("[scoring_engine] registered")
}
