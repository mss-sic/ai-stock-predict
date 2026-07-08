package service

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// ── Market Rules ──

func init() {
	RegisterRule(&FearGreedOverheatRule{})
	RegisterRule(&MarketBreadthDecayRule{})
	RegisterRule(&NorthboundOutflowStreakRule{})
	RegisterRule(&VolatilitySpikeRule{})
}

// ── M1: Fear & Greed Overheat ──

type FearGreedOverheatRule struct{}

func (r *FearGreedOverheatRule) Key() string { return "fear_greed_overheat" }
func (r *FearGreedOverheatRule) Evaluate(ctx context.Context, _ []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	// Load thresholds
	threshold, consDays := 80.0, 3
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["threshold"].(float64); ok {
			threshold = v
		}
		if v, ok := def.Thresholds["consecutive_days"].(float64); ok {
			consDays = int(v)
		}
	}

	var rows []struct {
		TradeDate  string  `gorm:"column:trade_date"`
		FearGreed  float64 `gorm:"column:fear_greed"`
	}
	db.PG.Raw(`
		SELECT trade_date::text, fear_greed
		FROM market_sentiment
		ORDER BY trade_date DESC LIMIT ?
	`, consDays).Scan(&rows)

	if len(rows) < consDays {
		return nil, nil, nil
	}

	allOver := true
	for _, r := range rows {
		if r.FearGreed < threshold {
			allOver = false
			break
		}
	}
	if !allOver {
		return nil, nil, nil
	}

	// Build alert
	now := time.Now()
	alert := model.RiskAlert{
		UserID:        0,
		StrategyID:    0,
		StockCode:     "__MARKET__",
		Level:         "high",
		Type:          "恐贪指数过热",
		Description:   fmt.Sprintf("恐贪指数连续%d日高于%.0f（最新%.0f），市场情绪极度亢奋", consDays, threshold, rows[0].FearGreed),
		RuleKey:       r.Key(),
		Dimension:     "market",
		SeverityScore: int(math.Min(rows[0].FearGreed, 100)),
		Evidence: model.JSONMap{
			"fear_greed":       rows[0].FearGreed,
			"consecutive_days": consDays,
			"threshold":        threshold,
		},
		HitDate: now,
	}
	return []model.RiskAlert{alert}, nil, nil
}

// ── M2: Market Breadth Decay ──

type MarketBreadthDecayRule struct{}

func (r *MarketBreadthDecayRule) Key() string { return "market_breadth_decay" }
func (r *MarketBreadthDecayRule) Evaluate(ctx context.Context, _ []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	threshold := 0.30
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["threshold"].(float64); ok {
			threshold = v
		}
	}

	var row struct {
		TradeDate     string `gorm:"column:trade_date"`
		MA20Above     int    `gorm:"column:ma20_above"`
		TotalUp       int    `gorm:"column:total_up"`
	}
	db.PG.Raw(`
		SELECT trade_date::text, ma20_above,
			(SELECT COUNT(*) FROM stocks_daily_k WHERE trade_date = msd.trade_date) as total_up
		FROM market_style_daily msd
		ORDER BY trade_date DESC LIMIT 1
	`).Scan(&row)

	if row.TotalUp == 0 {
		return nil, nil, nil
	}

	ratio := float64(row.MA20Above) / float64(row.TotalUp)
	if ratio >= threshold {
		return nil, nil, nil
	}

	now := time.Now()
	alert := model.RiskAlert{
		UserID:        0,
		StrategyID:    0,
		StockCode:     "__MARKET__",
		Level:         "medium",
		Type:          "市场宽度恶化",
		Description:   fmt.Sprintf("全市场MA20以上占比仅%.0f%%（%d/%d），低于%.0f%%警戒线",
			ratio*100, row.MA20Above, row.TotalUp, threshold*100),
		RuleKey:       r.Key(),
		Dimension:     "market",
		SeverityScore: int((1 - ratio) * 100),
		Evidence: model.JSONMap{
			"ma20_above": row.MA20Above,
			"total":      row.TotalUp,
			"ratio":      ratio,
			"threshold":  threshold,
		},
		HitDate: now,
	}
	return []model.RiskAlert{alert}, nil, nil
}

// ── M3: Northbound Outflow Streak ──

type NorthboundOutflowStreakRule struct{}

func (r *NorthboundOutflowStreakRule) Key() string { return "northbound_outflow_streak" }
func (r *NorthboundOutflowStreakRule) Evaluate(ctx context.Context, _ []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	thresholdDays := 5
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["threshold_days"].(float64); ok {
			thresholdDays = int(v)
		}
	}

	var rows []struct {
		TradeDate string  `gorm:"column:trade_date"`
		NetFlow   float64 `gorm:"column:net_flow"`
	}
	db.PG.Raw(`
		SELECT trade_date::text, net_flow
		FROM northbound_daily_view
		ORDER BY trade_date DESC LIMIT ?
	`, thresholdDays).Scan(&rows)

	if len(rows) < thresholdDays {
		return nil, nil, nil
	}

	allOutflow := true
	totalOutflow := 0.0
	for _, r := range rows {
		if r.NetFlow >= 0 {
			allOutflow = false
			break
		}
		totalOutflow += r.NetFlow
	}
	if !allOutflow {
		return nil, nil, nil
	}

	now := time.Now()
	alert := model.RiskAlert{
		UserID:        0,
		StrategyID:    0,
		StockCode:     "__MARKET__",
		Level:         "medium",
		Type:          "北向资金连续流出",
		Description:   fmt.Sprintf("北向资金连续%d日净流出，累计%.0f亿", thresholdDays, totalOutflow/1e8),
		RuleKey:       r.Key(),
		Dimension:     "market",
		SeverityScore: int(math.Min(math.Abs(totalOutflow)/1e8*5, 100)),
		Evidence: model.JSONMap{
			"consecutive_days": thresholdDays,
			"total_outflow":    totalOutflow,
		},
		HitDate: now,
	}
	return []model.RiskAlert{alert}, nil, nil
}

// ── M4: Volatility Spike ──

type VolatilitySpikeRule struct{}

func (r *VolatilitySpikeRule) Key() string { return "volatility_spike" }
func (r *VolatilitySpikeRule) Evaluate(ctx context.Context, _ []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	percentile := 0.90
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["percentile"].(float64); ok {
			percentile = v
		}
	}

	// Calculate today's median amplitude vs historical 90th percentile
	var row struct {
		TodayMedian float64 `gorm:"column:today_median"`
		P90Hist     float64 `gorm:"column:p90_hist"`
		TodayDate   string  `gorm:"column:today_date"`
	}
	db.PG.Raw(`
		WITH today_amp AS (
			SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY (high-low)/NULLIF(pre_close,0)) as median_amp,
			       MAX(trade_date) as max_date
			FROM stocks_daily_k WHERE trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k)
		),
		hist_amp AS (
			SELECT PERCENTILE_CONT(?) WITHIN GROUP (ORDER BY (high-low)/NULLIF(pre_close,0)) as p90
			FROM stocks_daily_k
			WHERE trade_date >= (SELECT MAX(trade_date) FROM stocks_daily_k) - INTERVAL '60 days'
			  AND trade_date < (SELECT MAX(trade_date) FROM stocks_daily_k)
		)
		SELECT t.median_amp as today_median, COALESCE(h.p90, 0) as p90_hist, t.max_date::text as today_date
		FROM today_amp t, hist_amp h
	`, percentile).Scan(&row)

	if row.TodayMedian <= row.P90Hist || row.P90Hist == 0 {
		return nil, nil, nil
	}

	now := time.Now()
	severity := int(math.Min((row.TodayMedian/row.P90Hist-1)*200, 100))
	alert := model.RiskAlert{
		UserID:        0,
		StrategyID:    0,
		StockCode:     "__MARKET__",
		Level:         "medium",
		Type:          "波动率飙升",
		Description:   fmt.Sprintf("全市场振幅中位数%.2f%%突破历史%.0f分位（%.2f%%），市场剧烈波动",
			row.TodayMedian*100, percentile*100, row.P90Hist*100),
		RuleKey:       r.Key(),
		Dimension:     "market",
		SeverityScore: severity,
		Evidence: model.JSONMap{
			"today_median": row.TodayMedian,
			"p90_hist":     row.P90Hist,
			"percentile":   percentile,
		},
		HitDate: now,
	}
	return []model.RiskAlert{alert}, nil, nil
}

// loadRuleDef fetches a rule definition from the DB.
func loadRuleDef(ruleKey string) (*model.RiskRule, error) {
	var def model.RiskRule
	err := db.MySQL.Where("rule_key = ?", ruleKey).First(&def).Error
	if err != nil {
		log.Printf("[RiskRule:%s] load definition error: %v", ruleKey, err)
		return nil, err
	}
	validateThresholds(ruleKey, def.Thresholds)
	return &def, nil
}

// validateThresholds logs warnings for suspicious threshold values.
func validateThresholds(ruleKey string, t model.JSONMap) {
	if t == nil {
		return
	}
	for k, v := range t {
		switch fv := v.(type) {
	case float64:
		if fv < 0 {
			log.Printf("[RiskRule:%s] WARNING: threshold '%s' is negative (%.2f), rule will never trigger", ruleKey, k, fv)
		} else if fv > 10000 {
			log.Printf("[RiskRule:%s] WARNING: threshold '%s' is extremely large (%.2f), may be misconfigured", ruleKey, k, fv)
		}
	}
	}
}
