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
	RegisterRule(&OvertradingRule{})
	RegisterRule(&StopLossMissedRule{})
	RegisterRule(&LiveBacktestDivergenceRule{})
}

// ── B1: Overtrading ──

type OvertradingRule struct{}

func (r *OvertradingRule) Key() string { return "overtrading" }
func (r *OvertradingRule) Evaluate(ctx context.Context, codes []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	maxTrades := 5
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["max_trades_per_day"].(float64); ok {
			maxTrades = int(v)
		}
	}
	type Key struct {
		UserID     uint
		StrategyID uint
	}
	groups := make(map[Key][]model.Holding)
	for _, h := range holdings {
		k := Key{UserID: h.UserID, StrategyID: h.StrategyID}
		groups[k] = append(groups[k], h)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for key := range groups {
		var count int64
		db.MySQL.Model(&model.LiveTrade{}).
			Where("user_id = ? AND DATE(created_at) = CURRENT_DATE",
				key.UserID).
			Count(&count)
		if int(count) > maxTrades {
			alerts = append(alerts, model.RiskAlert{
				UserID:     key.UserID,
				StrategyID: key.StrategyID,
				StockCode:  fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
				Level:      "low",
				Type:       "频繁交易",
				Description: fmt.Sprintf("当日已成交%d笔，超过%d笔上限", count, maxTrades),
				RuleKey:    "overtrading",
				Dimension:  "behavior",
				SeverityScore: int(math.Min(float64(count)/float64(maxTrades)*30, 100)),
				Evidence: model.JSONMap{"trade_count": count, "max": maxTrades},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}

// ── B2: Stop Loss Missed ──

type StopLossMissedRule struct{}

func (r *StopLossMissedRule) Key() string { return "stop_loss_missed" }
func (r *StopLossMissedRule) Evaluate(ctx context.Context, _ []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	tolerance := 0.02
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["tolerance_pct"].(float64); ok {
			tolerance = v
		}
	}
	type Key struct {
		UserID     uint
		StrategyID uint
	}
	groups := make(map[Key][]model.Holding)
	for _, h := range holdings {
		k := Key{UserID: h.UserID, StrategyID: h.StrategyID}
		groups[k] = append(groups[k], h)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for key, hList := range groups {
		// Get strategy stop-loss setting
		var strategy model.Strategy
		if err := db.MySQL.Where("id = ?", key.StrategyID).First(&strategy).Error; err != nil {
			continue
		}
		if strategy.StopLoss >= 0 {
			continue // stop loss not set or positive (invalid)
		}
		for _, h := range hList {
			if h.CostPrice <= 0 || h.Quantity <= 0 {
				continue
			}
			currentPrice := h.CurrentPrice
			if currentPrice <= 0 {
				continue
			}
			lossPct := (currentPrice - h.CostPrice) / h.CostPrice
			if lossPct < strategy.StopLoss-tolerance {
				alerts = append(alerts, model.RiskAlert{
					UserID:       key.UserID,
					StrategyID:   key.StrategyID,
					StockCode:    h.StockCode,
					Level:        "high",
					Type:         "止损未执行",
					Description:  fmt.Sprintf("持仓亏损%.1f%%已超止损线%.0f%%，未触发卖出", lossPct*100, strategy.StopLoss*100),
					RuleKey:      "stop_loss_missed",
					Dimension:    "behavior",
					SeverityScore: int(math.Min(math.Abs(lossPct)*200, 100)),
					Evidence: model.JSONMap{
						"cost_price":    h.CostPrice,
						"current_price": currentPrice,
						"loss_pct":      lossPct,
						"stop_loss":     strategy.StopLoss,
					},
					HitDate: now,
				})
			}
		}
	}
	return alerts, nil, nil
}

// ── B3: Live/Backtest Divergence ──

type LiveBacktestDivergenceRule struct{}

func (r *LiveBacktestDivergenceRule) Key() string { return "live_backtest_divergence" }
func (r *LiveBacktestDivergenceRule) Evaluate(ctx context.Context, _ []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	maxDiv := 0.15
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["max_divergence_pct"].(float64); ok {
			maxDiv = v
		}
	}
	type Key struct {
		UserID     uint
		StrategyID uint
	}
	groups := make(map[Key][]model.Holding)
	for _, h := range holdings {
		k := Key{UserID: h.UserID, StrategyID: h.StrategyID}
		groups[k] = append(groups[k], h)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for key := range groups {
		// Get latest live snapshot
		var snap model.DailyPortfolioSnapshot
		if err := db.MySQL.Where("user_id = ?", key.UserID).
			Order("snapshot_date DESC").First(&snap).Error; err != nil {
			continue
		}
		// Get latest backtest result
		var bt model.BacktestResult
		if err := db.MySQL.Where("user_id = ? AND strategy_id = ? AND status = 'completed'", key.UserID, key.StrategyID).
			Order("created_at DESC").First(&bt).Error; err != nil {
			continue
		}
		if bt.TotalReturn == 0 {
			continue
		}
		liveReturn := (snap.TotalEquity - (snap.TotalEquity - snap.CumulativeReturn/100 * snap.TotalEquity)) / snap.TotalEquity / (1 + snap.CumulativeReturn/100)
		divergence := math.Abs(liveReturn - bt.TotalReturn)
		if divergence > maxDiv {
			alerts = append(alerts, model.RiskAlert{
				UserID:       key.UserID,
				StrategyID:   key.StrategyID,
				StockCode:    fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
				Level:        "medium",
				Type:         "实盘回测偏离",
				Description:  fmt.Sprintf("实盘收益%.1f%%偏离回测%.1f%%达%.1f%%，模型可能失效",
					liveReturn*100, bt.TotalReturn*100, divergence*100),
				RuleKey:      "live_backtest_divergence",
				Dimension:    "behavior",
				SeverityScore: int(math.Min(divergence/maxDiv*50, 100)),
				Evidence: model.JSONMap{
					"live_return": liveReturn,
					"bt_return":   bt.TotalReturn,
					"divergence":  divergence,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}
