package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

func init() {
	RegisterRule(&HeavyVolumeDropRule{})
	RegisterRule(&MABearishAlignmentRule{})
	RegisterRule(&TurnoverAbnormalRule{})
	RegisterRule(&MajorOutflowStreakRule{})
	RegisterRule(&MarginCollapseRule{})
	RegisterRule(&BlockDiscountRule{})
	RegisterRule(&STDelistRiskRule{})
	RegisterRule(&SharpDeclineRule{})
	RegisterRule(&MA20BreakdownRule{})
	RegisterRule(&PEExtremeRule{})
	RegisterRule(&ProfitDeclineRule{})
	RegisterRule(&AIScoreCrashRule{})
}

// ── S1: Heavy Volume Drop ──

type HeavyVolumeDropRule struct{}

func (r *HeavyVolumeDropRule) Key() string { return "heavy_volume_drop" }
func (r *HeavyVolumeDropRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	dropPct := -5.0
	volRatio := 2.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["drop_pct"].(float64); ok {
			dropPct = v
		}
		if v, ok := def.Thresholds["volume_ratio"].(float64); ok {
			volRatio = v
		}
	}

	inClause := codesToInClause(codes)
	type Row struct {
		Code       string  `gorm:"column:code"`
		ChgPct     float64 `gorm:"column:chg_pct"`
		VolumeRatio float64 `gorm:"column:volume_ratio"`
		ClosePrice float64 `gorm:"column:close_price"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT k.code, k.pct_chg as chg_pct, COALESCE(i.volume_ratio, 1) as volume_ratio, k.close as close_price
		FROM stocks_daily_k k
		LEFT JOIN stocks_daily_indicator i ON i.code = k.code AND i.trade_date = k.trade_date
		WHERE k.code IN (%s) AND k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k)
		  AND k.pct_chg <= ? AND COALESCE(i.volume_ratio, 1) >= ?
	`, inClause), dropPct, volRatio).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:     row.Code,
			Level:         "high",
			Type:          "放量下跌",
			Description:   fmt.Sprintf("单日跌幅%.1f%%，量比%.1f，放量出逃信号", row.ChgPct, row.VolumeRatio),
			RuleKey:       "heavy_volume_drop",
			Dimension:     "stock",
			SeverityScore: int(math.Min(math.Abs(row.ChgPct)*10, 100)),
			Evidence: model.JSONMap{
				"chg_pct":      row.ChgPct,
				"volume_ratio": row.VolumeRatio,
				"close":        row.ClosePrice,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S3: MA Bearish Alignment ──

type MABearishAlignmentRule struct{}

func (r *MABearishAlignmentRule) Key() string { return "ma_bearish_alignment" }
func (r *MABearishAlignmentRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	inClause := codesToInClause(codes)

	type Row struct {
		Code string  `gorm:"column:code"`
		MA5  float64 `gorm:"column:ma5"`
		MA10 float64 `gorm:"column:ma10"`
		MA20 float64 `gorm:"column:ma20"`
		MA60 float64 `gorm:"column:ma60"`
		ClosePrice float64 `gorm:"column:close_price"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, trade_date, close,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as ma5,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 9 PRECEDING AND CURRENT ROW) as ma10,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) as ma60,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k WHERE code IN (%s)
		)
		SELECT code, ma5, ma10, ma20, ma60, close as close_price
		FROM ranked WHERE rn = 1 AND ma5 < ma10 AND ma10 < ma20 AND ma20 < ma60
	`, inClause)).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "medium",
			Type:        "均线空头排列",
			Description: fmt.Sprintf("MA5(%.2f)<MA10(%.2f)<MA20(%.2f)<MA60(%.2f)，趋势全面走弱", r.MA5, r.MA10, r.MA20, r.MA60),
			RuleKey:     "ma_bearish_alignment",
			Dimension:   "stock",
			SeverityScore: 60,
			Evidence: model.JSONMap{
				"ma5":   r.MA5,
				"ma10":  r.MA10,
				"ma20":  r.MA20,
				"ma60":  r.MA60,
				"close": r.ClosePrice,
			},
			HitDate: now,
		})
	}
	// Auto-resolve: stocks no longer in bearish alignment
	var toResolve []model.RiskAlert
	db.MySQL.Where("rule_key = ? AND stock_code IN ? AND status = 'active'", "ma_bearish_alignment", codes).Find(&toResolve)
	var stillActive []string
	for _, a := range alerts {
		stillActive = append(stillActive, a.StockCode)
	}
	var resolvedOut []model.RiskAlert
	for _, a := range toResolve {
		found := false
		for _, c := range stillActive {
			if c == a.StockCode {
				found = true
				break
			}
		}
		if !found {
			resolvedOut = append(resolvedOut, a)
		}
	}
	return alerts, resolvedOut, nil
}

// ── S7: Turnover Abnormal ──

type TurnoverAbnormalRule struct{}

func (r *TurnoverAbnormalRule) Key() string { return "turnover_abnormal" }
func (r *TurnoverAbnormalRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	high, low := 20.0, 0.1
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["high"].(float64); ok {
			high = v
		}
		if v, ok := def.Thresholds["low"].(float64); ok {
			low = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code         string  `gorm:"column:code"`
		TurnoverRate float64 `gorm:"column:turnover_rate"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, turnover_rate
		FROM stocks_daily_indicator
		WHERE code IN (%s) AND trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k)
		  AND (turnover_rate >= ? OR (turnover_rate > 0 AND turnover_rate <= ?))
	`, inClause), high, low).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		level := "medium"
		desc := fmt.Sprintf("换手率%.2f%%异常偏高，存在炒作嫌疑", r.TurnoverRate)
		if r.TurnoverRate <= low {
			desc = fmt.Sprintf("换手率%.2f%%异常偏低，流动性枯竭", r.TurnoverRate)
		}
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       level,
			Type:        "换手率异常",
			Description: desc,
			RuleKey:     "st_delist_risk",
			Dimension:   "stock",
			SeverityScore: int(math.Min(r.TurnoverRate*2, 100)),
			Evidence: model.JSONMap{"turnover_rate": r.TurnoverRate},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S8: Major Outflow Streak ──

type MajorOutflowStreakRule struct{}

func (r *MajorOutflowStreakRule) Key() string { return "major_outflow_streak" }
func (r *MajorOutflowStreakRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	days := 5
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["days"].(float64); ok {
			days = int(v)
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code        string  `gorm:"column:code"`
		TotalOutflow float64 `gorm:"column:total_outflow"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, SUM(main_net_inflow) as total_outflow
		FROM (
			SELECT code, main_net_inflow,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stock_fund_flow WHERE code IN (%s)
		) t WHERE rn <= ? AND main_net_inflow < 0
		GROUP BY code HAVING COUNT(*) = ?
	`, inClause), days, days).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   row.Code,
			Level:       "medium",
			Type:        "主力资金连续流出",
			Description: fmt.Sprintf("主力资金连续%d日净流出，累计%.0f万", days, row.TotalOutflow/10000),
			RuleKey:     "major_outflow_streak",
			Dimension:   "stock",
			SeverityScore: int(math.Min(math.Abs(row.TotalOutflow)/1e6*5, 100)),
			Evidence: model.JSONMap{
				"days":          days,
				"total_outflow": row.TotalOutflow,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S9: Margin Collapse ──

type MarginCollapseRule struct{}

func (r *MarginCollapseRule) Key() string { return "margin_collapse" }
func (r *MarginCollapseRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	days, dropPct := 5, -10.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["days"].(float64); ok {
			days = int(v)
		}
		if v, ok := def.Thresholds["drop_pct"].(float64); ok {
			dropPct = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code       string  `gorm:"column:code"`
		ChgPct     float64 `gorm:"column:chg_pct"`
		LatestBal  float64 `gorm:"column:latest_bal"`
		PrevBal    float64 `gorm:"column:prev_bal"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, rz_balance as bal, trade_date,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM margin_trading WHERE code IN (%s)
		)
		SELECT r1.code,
			(r1.bal - r2.bal) / NULLIF(r2.bal, 0) * 100 as chg_pct,
			r1.bal as latest_bal, r2.bal as prev_bal
		FROM (SELECT * FROM ranked WHERE rn = 1) r1
		JOIN (SELECT * FROM ranked WHERE rn = ?) r2 ON r2.code = r1.code
		WHERE r1.bal > 0 AND r2.bal > 0 AND (r1.bal - r2.bal) / NULLIF(r2.bal, 0) * 100 <= ?
	`, inClause), days, dropPct).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "high",
			Type:        "融资余额骤降",
			Description: fmt.Sprintf("融资余额%d日降幅%.1f%%，杠杆盘加速离场", days, r.ChgPct),
			RuleKey:     "margin_collapse",
			Dimension:   "stock",
			SeverityScore: int(math.Min(math.Abs(r.ChgPct)*3, 100)),
			Evidence: model.JSONMap{
				"days":       days,
				"chg_pct":    r.ChgPct,
				"latest_bal": r.LatestBal,
				"prev_bal":   r.PrevBal,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S10: Block Trade Discount ──

type BlockDiscountRule struct{}

func (r *BlockDiscountRule) Key() string { return "block_discount" }
func (r *BlockDiscountRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	discountPct := -8.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["discount_pct"].(float64); ok {
			discountPct = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code       string  `gorm:"column:code"`
		TradeDate  string  `gorm:"column:trade_date"`
		PriceChg   float64 `gorm:"column:price_chg"`
		Amount     float64 `gorm:"column:amount"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, trade_date::text, price_chg, amount
		FROM block_trade
		WHERE code IN (%s) AND trade_date >= CURRENT_DATE - INTERVAL '3 days'
		  AND price_chg <= ?
		ORDER BY trade_date DESC
	`, inClause), discountPct).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	seen := make(map[string]bool)
	for _, r := range rows {
		key := r.Code + ":" + r.TradeDate + ":" + fmt.Sprintf("%.2f", r.PriceChg)
		if seen[key] {
			continue
		}
		seen[key] = true
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "medium",
			Type:        "大宗折价交易",
			Description: fmt.Sprintf("%s大宗成交折价%.1f%%，成交额%.0f万", r.TradeDate, r.PriceChg, r.Amount/10000),
			RuleKey:     "block_discount",
			Dimension:   "stock",
			SeverityScore: int(math.Min(math.Abs(r.PriceChg)*5, 100)),
			Evidence: model.JSONMap{
				"trade_date": r.TradeDate,
				"price_chg":  r.PriceChg,
				"amount":     r.Amount,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S12: ST / Delist Risk ──

type STDelistRiskRule struct{}

func (r *STDelistRiskRule) Key() string { return "st_delist_risk" }
func (r *STDelistRiskRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code       string  `gorm:"column:code"`
		Name       string  `gorm:"column:name"`
		ClosePrice float64 `gorm:"column:close_price"`
	}
	var rows []Row
	// Check both for ST in name and for face value < 1.5
	db.PG.Raw(fmt.Sprintf(`
		SELECT sb.code, sb.name, COALESCE(k.close, 0) as close_price
		FROM stocks_basic sb
		LEFT JOIN stocks_daily_k k ON k.code = sb.code
			AND k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = sb.code)
		WHERE sb.code IN (%s) AND (sb.name LIKE '%%ST%%' OR sb.name LIKE '%%*ST%%' OR COALESCE(k.close, 999) < 1.5)
	`, inClause)).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		reason := "股票为ST/*ST标识"
		if r.ClosePrice > 0 && r.ClosePrice < 1.5 {
			reason = fmt.Sprintf("面值仅%.2f元，存在退市风险", r.ClosePrice)
		}
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "high",
			Type:        "ST/退市风险",
			Description: reason,
			RuleKey:     "ai_score_crash",
			Dimension:   "stock",
			SeverityScore: 90,
			Evidence: model.JSONMap{
				"name":        r.Name,
				"close_price": r.ClosePrice,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S13: AI Score Crash ──

type AIScoreCrashRule struct{}

func (r *AIScoreCrashRule) Key() string { return "ai_score_crash" }
func (r *AIScoreCrashRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	days, drop := 3, 2.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["days"].(float64); ok {
			days = int(v)
		}
		if v, ok := def.Thresholds["drop"].(float64); ok {
			drop = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code      string  `gorm:"column:code"`
		ScoreNow  float64 `gorm:"column:score_now"`
		ScorePrev float64 `gorm:"column:score_prev"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, c_score, trade_date,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM ai_stock_scores WHERE code IN (%s)
		)
		SELECT r1.code, r1.c_score as score_now, r2.c_score as score_prev
		FROM (SELECT * FROM ranked WHERE rn = 1) r1
		JOIN (SELECT * FROM ranked WHERE rn = ?) r2 ON r2.code = r1.code
		WHERE r1.c_score > 0 AND r2.c_score > 0 AND (r1.c_score - r2.c_score) <= ?
	`, inClause), days, -drop).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		scoreDrop := r.ScorePrev - r.ScoreNow
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "medium",
			Type:        "AI评分骤降",
			Description: fmt.Sprintf("AI综合评分%d日内从%.1f降至%.1f（-%.1f），基本面可能恶化",
				days, r.ScorePrev, r.ScoreNow, scoreDrop),
			RuleKey:     "ai_score_crash",
			Dimension:   "stock",
			SeverityScore: int(math.Min(scoreDrop*15, 100)),
			Evidence: model.JSONMap{
				"days":       days,
				"score_now":  r.ScoreNow,
				"score_prev": r.ScorePrev,
				"drop":       scoreDrop,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S14: Sharp Decline (optimized) ──

type SharpDeclineRule struct{}

func (r *SharpDeclineRule) Key() string { return "sharp_decline" }
func (r *SharpDeclineRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	days, dropPct := 5, -8.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["days"].(float64); ok {
			days = int(v)
		}
		if v, ok := def.Thresholds["drop_pct"].(float64); ok {
			dropPct = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code     string  `gorm:"column:code"`
		ChgPct   float64 `gorm:"column:chg_pct"`
		CloseNow float64 `gorm:"column:close_now"`
		ClosePrev float64 `gorm:"column:close_prev"`
		AvgVol   float64 `gorm:"column:avg_vol"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, close, volume,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k WHERE code IN (%s)
		),
		vol_avg AS (
			SELECT code, AVG(volume) as avg_vol FROM ranked WHERE rn BETWEEN 2 AND ? GROUP BY code
		)
		SELECT r1.code,
			(r1.close - r2.close) / NULLIF(r2.close, 0) * 100 as chg_pct,
			r1.close as close_now, r2.close as close_prev,
			COALESCE(v.avg_vol, 0) as avg_vol
		FROM (SELECT * FROM ranked WHERE rn = 1) r1
		JOIN (SELECT * FROM ranked WHERE rn = ?) r2 ON r2.code = r1.code
		LEFT JOIN vol_avg v ON v.code = r1.code
		WHERE r1.close > 0 AND r2.close > 0
		  AND (r1.close - r2.close) / NULLIF(r2.close, 0) * 100 <= ?
	`, inClause), days, days+1, dropPct).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		level := "high"
		if r.ChgPct > -8 {
			level = "medium"
		}
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       level,
			Type:        "近期大跌",
			Description: fmt.Sprintf("近%d个交易日累计跌幅%.0f%%", days, r.ChgPct),
			RuleKey:     "sharp_decline",
			Dimension:   "stock",
			SeverityScore: int(math.Min(math.Abs(r.ChgPct)*8, 100)),
			Evidence: model.JSONMap{
				"days":       days,
				"chg_pct":    r.ChgPct,
				"close_now":  r.CloseNow,
				"close_prev": r.ClosePrev,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S15: MA20 Breakdown (optimized with buffer) ──

type MA20BreakdownRule struct{}

func (r *MA20BreakdownRule) Key() string { return "ma20_breakdown" }
func (r *MA20BreakdownRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	inClause := codesToInClause(codes)
	bufferPct := 0.02
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["buffer_pct"].(float64); ok {
			bufferPct = v
		}
	}

	type Row struct {
		Code  string  `gorm:"column:code"`
		Close float64 `gorm:"column:close"`
		MA20  float64 `gorm:"column:ma20"`
		PrevC float64 `gorm:"column:prev_c"`
		PrevM float64 `gorm:"column:prev_m"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH ranked AS (
			SELECT code, trade_date, close,
				AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k WHERE code IN (%s)
		)
		SELECT t1.code, t1.close, t1.ma20, t2.close as prev_c, t2.ma20 as prev_m
		FROM ranked t1
		JOIN ranked t2 ON t2.code = t1.code AND t2.rn = 2
		WHERE t1.rn = 1
		  AND t1.close < t1.ma20 * (1 - ?)
		  AND t2.close >= t2.ma20
	`, inClause), bufferPct).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       "medium",
			Type:        "跌破均线",
			Description: fmt.Sprintf("收盘价%.2f跌破MA20(%.2f)，昨日站上均线", r.Close, r.MA20),
			RuleKey:     "ma20_breakdown",
			Dimension:   "stock",
			SeverityScore: int((r.MA20 - r.Close) / r.MA20 * 200),
			Evidence: model.JSONMap{
				"close": r.Close,
				"ma20":  r.MA20,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S16: PE Extreme (optimized, fixed SQL bug) ──

type PEExtremeRule struct{}

func (r *PEExtremeRule) Key() string { return "pe_extreme" }
func (r *PEExtremeRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	peHigh, peWarn := 200.0, 100.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["pe_high"].(float64); ok {
			peHigh = v
		}
		if v, ok := def.Thresholds["pe_warn"].(float64); ok {
			peWarn = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code   string  `gorm:"column:code"`
		PE     float64 `gorm:"column:pe"`
		PE_ttm float64 `gorm:"column:pe_ttm"`
		PB     float64 `gorm:"column:pb"`
		IndAvg float64 `gorm:"column:ind_avg"`
	}
	var rows []Row
	// Fixed: use parameterized industry filter, not %%s
	db.PG.Raw(fmt.Sprintf(`
		WITH latest_indicator AS (
			SELECT DISTINCT ON (code) code, pe, pe_ttm, pb
			FROM stocks_daily_indicator
			WHERE code IN (%s) AND pe > 0
			ORDER BY code, trade_date DESC
		),
		ind_avg AS (
			SELECT sb.industry, AVG(i.pe) as avg_pe
			FROM stocks_daily_indicator i
			JOIN stocks_basic sb ON sb.code = i.code
			WHERE i.pe > 0 AND i.pe < 500
				AND i.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_indicator WHERE code = i.code)
			GROUP BY sb.industry
		)
		SELECT li.code, li.pe, li.pe_ttm, li.pb, COALESCE(ia.avg_pe, 0) as ind_avg
		FROM latest_indicator li
		JOIN stocks_basic sb ON sb.code = li.code
		LEFT JOIN ind_avg ia ON ia.industry = sb.industry
		WHERE li.pe >= ?
	`, inClause), peWarn).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, r := range rows {
		level := "medium"
		desc := fmt.Sprintf("市盈率%.0f（行业均值%.0f），估值偏高", r.PE, r.IndAvg)
		if r.PE > peHigh {
			level = "high"
			desc = fmt.Sprintf("市盈率%.0f远超合理范围（行业均值%.0f）", r.PE, r.IndAvg)
		}
		alerts = append(alerts, model.RiskAlert{
			StockCode:   r.Code,
			Level:       level,
			Type:        "估值偏高",
			Description: desc,
			RuleKey:     "pe_extreme",
			Dimension:   "stock",
			SeverityScore: int(math.Min(r.PE/3, 100)),
			Evidence: model.JSONMap{
				"pe":      r.PE,
				"pe_ttm":  r.PE_ttm,
				"pb":      r.PB,
				"ind_avg": r.IndAvg,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── S17: Profit Decline (optimized) ──

type ProfitDeclineRule struct{}

func (r *ProfitDeclineRule) Key() string { return "profit_decline" }
func (r *ProfitDeclineRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	declinePct, warnPct := -50.0, -30.0
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["decline_pct"].(float64); ok {
			declinePct = v
		}
		if v, ok := def.Thresholds["warn_pct"].(float64); ok {
			warnPct = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code   string  `gorm:"column:code"`
		Chg    float64 `gorm:"column:profit_growth"`
		RevChg float64 `gorm:"column:revenue_growth"`
		ROE    float64 `gorm:"column:roe"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		WITH latest AS (
			SELECT DISTINCT ON (code) code, profit_growth, revenue_growth, roe
			FROM stock_financials WHERE code IN (%s)
			ORDER BY code, report_date DESC
		)
		SELECT code, profit_growth, revenue_growth, roe
		FROM latest WHERE profit_growth <= ?
	`, inClause), warnPct).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		level := "medium"
		if row.Chg < declinePct {
			level = "high"
		}
		alerts = append(alerts, model.RiskAlert{
			StockCode:   row.Code,
			Level:       level,
			Type:        "业绩下滑",
			Description: fmt.Sprintf("最新财报净利润同比%.0f%%，营收同比%.0f%%", row.Chg, row.RevChg),
			RuleKey:     "profit_decline",
			Dimension:   "stock",
			SeverityScore: int(math.Min(math.Abs(row.Chg)*0.8, 100)),
			Evidence: model.JSONMap{
				"profit_growth":  row.Chg,
				"revenue_growth": row.RevChg,
				"roe":            row.ROE,
			},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}
