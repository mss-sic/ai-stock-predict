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
	RegisterRule(&VolumeTooLowRule{})
	RegisterRule(&LimitDownLockedRule{})
	RegisterRule(&TurnoverDecayRule{})
}

// ── L1: Volume Too Low ──

type VolumeTooLowRule struct{}

func (r *VolumeTooLowRule) Key() string { return "volume_too_low" }
func (r *VolumeTooLowRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	avgDays, minAmt := 5, 20000000.0 // 5 days, 20M
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["avg_days"].(float64); ok {
			avgDays = int(v)
		}
		if v, ok := def.Thresholds["min_amount_small_cap"].(float64); ok {
			minAmt = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code   string  `gorm:"column:code"`
		AvgAmt float64 `gorm:"column:avg_amount"`
	}
	var rows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, AVG(amount) as avg_amount
		FROM (
			SELECT code, amount,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) as rn
			FROM stocks_daily_k WHERE code IN (%s)
		) t WHERE rn <= ?
		GROUP BY code HAVING AVG(amount) < ?
	`, inClause), avgDays, minAmt).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   row.Code,
			Level:       "medium",
			Type:        "日成交额过低",
			Description: fmt.Sprintf("近%d日均成交%.0f万，流动性不足", avgDays, row.AvgAmt/10000),
			RuleKey:     "volume_too_low",
			Dimension:   "liquidity",
			SeverityScore: int(math.Min((1-row.AvgAmt/minAmt)*50, 100)),
			Evidence: model.JSONMap{"avg_amount": row.AvgAmt, "min_amount": minAmt},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── L2: Limit Down Locked ──

type LimitDownLockedRule struct{}

func (r *LimitDownLockedRule) Key() string { return "limit_down_locked" }
func (r *LimitDownLockedRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code      string  `gorm:"column:code"`
		Close     float64 `gorm:"column:close"`
		PreClose  float64 `gorm:"column:pre_close"`
		Turnover  float64 `gorm:"column:turnover_rate"`
	}
	var rows []Row
	// Limit-down detection: close == preclose * 0.9 (≈10% limit) AND turnover_rate < 0.5%
	db.PG.Raw(fmt.Sprintf(`
		SELECT k.code, k.close, k.pre_close,
		       COALESCE(i.turnover_rate, 99) as turnover_rate
		FROM stocks_daily_k k
		LEFT JOIN stocks_daily_indicator i ON i.code = k.code AND i.trade_date = k.trade_date
		WHERE k.code IN (%s) AND k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k)
		  AND k.pre_close > 0
		  AND k.close <= k.pre_close * 0.905 AND k.close >= k.pre_close * 0.895
		  AND COALESCE(i.turnover_rate, 99) < 0.5
	`, inClause)).Scan(&rows)

	now := time.Now()
	var alerts []model.RiskAlert
	for _, row := range rows {
		alerts = append(alerts, model.RiskAlert{
			StockCode:   row.Code,
			Level:       "high",
			Type:        "跌停封板",
			Description: fmt.Sprintf("跌停封板无流动性，换手率仅%.2f%%", row.Turnover),
			RuleKey:     "limit_down_locked",
			Dimension:   "liquidity",
			SeverityScore: 90,
			Evidence: model.JSONMap{"close": row.Close, "pre_close": row.PreClose, "turnover": row.Turnover},
			HitDate: now,
		})
	}
	return alerts, nil, nil
}

// ── L3: Turnover Decay ──

type TurnoverDecayRule struct{}

func (r *TurnoverDecayRule) Key() string { return "turnover_decay" }
func (r *TurnoverDecayRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	days, minTurnover := 30, 0.005
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["days"].(float64); ok {
			days = int(v)
		}
		if v, ok := def.Thresholds["min_turnover"].(float64); ok {
			minTurnover = v
		}
	}
	inClause := codesToInClause(codes)
	type Row struct {
		Code         string  `gorm:"column:code"`
		TurnoverRate float64 `gorm:"column:turnover_rate"`
	}
	var allRows []Row
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, turnover_rate, trade_date
		FROM stocks_daily_indicator WHERE code IN (%s)
		ORDER BY code, trade_date DESC
	`, inClause)).Scan(&allRows)

	byCode := make(map[string][]float64)
	for _, r := range allRows {
		byCode[r.Code] = append(byCode[r.Code], r.TurnoverRate)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for code, rates := range byCode {
		if len(rates) < days {
			continue
		}
		recent := rates[:days]
		avg := 0.0
		for _, v := range recent {
			avg += v
		}
		avg /= float64(len(recent))
		if avg >= minTurnover {
			continue
		}
		// Check downward trend via simple slope
		slope := 0.0
		n := float64(len(recent))
		for i, v := range recent {
			slope += (float64(i) - (n-1)/2) * v
		}
		slope = slope / (n * (n*n - 1) / 12)
		if slope < 0 {
			alerts = append(alerts, model.RiskAlert{
				StockCode:   code,
				Level:       "low",
				Type:        "换手率持续衰减",
				Description: fmt.Sprintf("近%d日平均换手率%.3f%%且持续走低", days, avg),
				RuleKey:     "turnover_decay",
				Dimension:   "liquidity",
				SeverityScore: int(math.Min((1-avg/minTurnover)*30, 100)),
				Evidence: model.JSONMap{"avg_turnover": avg, "days": days, "slope": slope},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}
