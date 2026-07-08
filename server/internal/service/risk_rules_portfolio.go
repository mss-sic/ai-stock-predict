package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

func init() {
	RegisterRule(&IndustryConcentrationRule{})
	RegisterRule(&CorrelationHighRule{})
	RegisterRule(&VaRBreachRule{})
	RegisterRule(&PositionOverlimitRule{})
}

// ── P1: Industry Concentration ──

type IndustryConcentrationRule struct{}

func (r *IndustryConcentrationRule) Key() string { return "industry_concentration" }
func (r *IndustryConcentrationRule) Evaluate(ctx context.Context, _ []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	maxPct := 0.40
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["max_pct"].(float64); ok {
			maxPct = v
		}
	}

	// Group holdings by user+strategy
	type Key struct {
		UserID     uint
		StrategyID uint
	}
	groups := make(map[Key][]model.Holding)
	for _, h := range holdings {
		k := Key{UserID: h.UserID, StrategyID: h.StrategyID}
		groups[k] = append(groups[k], h)
	}
	if len(groups) == 0 {
		return nil, nil, nil
	}

	// Get industry for each stock code
	codes := make([]string, 0)
	for _, h := range holdings {
		codes = append(codes, h.StockCode)
	}
	type IndRow struct {
		Code     string `gorm:"column:code"`
		Industry string `gorm:"column:industry"`
	}
	var indRows []IndRow
	db.PG.Raw(fmt.Sprintf("SELECT code, COALESCE(industry, '未知') as industry FROM stocks_basic WHERE code IN (%s)", codesToInClause(codes))).Scan(&indRows)
	indMap := make(map[string]string)
	for _, r := range indRows {
		indMap[r.Code] = r.Industry
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for key, hList := range groups {
		indCount := make(map[string]float64)
		totalValue := 0.0
		for _, h := range hList {
			val := float64(h.Quantity) * h.CurrentPrice
			if val <= 0 {
				val = float64(h.Quantity) * h.CostPrice
			}
			totalValue += val
			ind := indMap[h.StockCode]
			if ind == "" {
				ind = "未知"
			}
			indCount[ind] += val
		}
		if totalValue == 0 {
			continue
		}
		for ind, val := range indCount {
			pct := val / totalValue
			if pct > maxPct {
				alerts = append(alerts, model.RiskAlert{
					UserID:     key.UserID,
					StrategyID: key.StrategyID,
					StockCode:  fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
					Level:      "medium",
					Type:       "行业集中度过高",
					Description: fmt.Sprintf("%s行业占比%.0f%%超过%.0f%%上限", ind, pct*100, maxPct*100),
					RuleKey:    "industry_concentration",
					Dimension:  "portfolio",
					SeverityScore: int(math.Min(pct*200, 100)),
					Evidence: model.JSONMap{
						"industry":    ind,
						"pct":         pct,
						"max_pct":     maxPct,
						"total_value": totalValue,
					},
					HitDate: now,
				})
			}
		}
	}
	return alerts, nil, nil
}

// ── P2: Correlation High ──

type CorrelationHighRule struct{}

func (r *CorrelationHighRule) Key() string { return "correlation_high" }
func (r *CorrelationHighRule) Evaluate(ctx context.Context, codes []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	threshold := 0.70
	minDays := 60
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["threshold"].(float64); ok {
			threshold = v
		}
		if v, ok := def.Thresholds["min_history_days"].(float64); ok {
			minDays = int(v)
		}
	}

	// Group by user+strategy
	type Key struct {
		UserID     uint
		StrategyID uint
	}
	groups := make(map[Key][]model.Holding)
	for _, h := range holdings {
		k := Key{UserID: h.UserID, StrategyID: h.StrategyID}
		groups[k] = append(groups[k], h)
	}

	// Get returns for each stock
	if len(codes) == 0 {
		return nil, nil, nil
	}
	type RetRow struct {
		Code   string  `gorm:"column:code"`
		Ret    float64 `gorm:"column:ret"`
		TDate  string  `gorm:"column:trade_date"`
	}
	var retRows []RetRow
	inClause := codesToInClause(codes)
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, (close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date)) / NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) as ret,
		       trade_date::text
		FROM stocks_daily_k WHERE code IN (%s)
		ORDER BY code, trade_date
	`, inClause)).Scan(&retRows)

	// Pivot: stock → return series
	stockRets := make(map[string][]float64)
	for _, r := range retRows {
		if r.Ret == 0 {
			continue
		}
		stockRets[r.Code] = append(stockRets[r.Code], r.Ret)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for key, hList := range groups {
		if len(hList) < 2 {
			continue
		}
		// Compute pairwise correlations
		var corrs []float64
		for i := 0; i < len(hList); i++ {
			for j := i + 1; j < len(hList); j++ {
				a := stockRets[hList[i].StockCode]
				b := stockRets[hList[j].StockCode]
				// Use min length
				n := len(a)
				if len(b) < n {
					n = len(b)
				}
				if n < minDays {
					continue
				}
				corr := pearsonCorr(a[len(a)-n:], b[len(b)-n:])
				corrs = append(corrs, corr)
			}
		}
		if len(corrs) == 0 {
			continue
		}
		avgCorr := 0.0
		for _, c := range corrs {
			avgCorr += c
		}
		avgCorr /= float64(len(corrs))
		if avgCorr > threshold {
			alerts = append(alerts, model.RiskAlert{
				UserID:     key.UserID,
				StrategyID: key.StrategyID,
				StockCode:  fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
				Level:      "medium",
				Type:       "持仓相关性过高",
				Description: fmt.Sprintf("持仓股票平均相关系数%.2f大于%.2f，分散不足", avgCorr, threshold),
				RuleKey:    "correlation_high",
				Dimension:  "portfolio",
				SeverityScore: int(math.Min(avgCorr*120, 100)),
				Evidence: model.JSONMap{
					"avg_corr":  avgCorr,
					"threshold": threshold,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}

func pearsonCorr(x, y []float64) float64 {
	n := len(x)
	if n < 3 {
		return 0
	}
	sumX, sumY, sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0, 0.0, 0.0
	for i := 0; i < n; i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}
	num := float64(n)*sumXY - sumX*sumY
	den := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))
	if den == 0 {
		return 0
	}
	return num / den
}

// ── P3: VaR Breach ──

type VaRBreachRule struct{}

func (r *VaRBreachRule) Key() string { return "var_breach" }
func (r *VaRBreachRule) Evaluate(ctx context.Context, _ []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	confidence, maxVarPct := 0.95, 0.05
	minDays := 90
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["confidence"].(float64); ok {
			confidence = v
		}
		if v, ok := def.Thresholds["max_var_pct"].(float64); ok {
			maxVarPct = v
		}
		if v, ok := def.Thresholds["min_history_days"].(float64); ok {
			minDays = int(v)
		}
	}

	// Simplified: compute VaR from portfolio daily returns (approximation using weighted basket)
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
		if len(hList) == 0 {
			continue
		}
		totalVal := 0.0
		for _, h := range hList {
			val := float64(h.Quantity) * h.CurrentPrice
			if val <= 0 {
				val = float64(h.Quantity) * h.CostPrice
			}
			totalVal += val
		}
		if totalVal <= 0 {
			continue
		}

		// Get daily returns for all stocks and compute weighted portfolio returns
		codes := make([]string, len(hList))
		weights := make([]float64, len(hList))
		for i, h := range hList {
			codes[i] = h.StockCode
			val := float64(h.Quantity) * h.CurrentPrice
			if val <= 0 {
				val = float64(h.Quantity) * h.CostPrice
			}
			weights[i] = val / totalVal
		}
		inClause := codesToInClause(codes)

		type RetRow struct {
			Code   string  `gorm:"column:code"`
			Ret    float64 `gorm:"column:ret"`
		}
		var retRows []RetRow
		db.PG.Raw(fmt.Sprintf(`
			SELECT code, (close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date)) / NULLIF(LAG(close) OVER (PARTITION BY code ORDER BY trade_date), 0) as ret
			FROM stocks_daily_k WHERE code IN (%s)
			ORDER BY code, trade_date
		`, inClause)).Scan(&retRows)

		// Pivot by code
		codeRets := make(map[string][]float64)
		for _, r := range retRows {
			codeRets[r.Code] = append(codeRets[r.Code], r.Ret)
		}

		// Compute weighted portfolio returns (align by date — simplified: use min length)
		minLen := 999999
		for _, c := range codes {
			if len(codeRets[c]) < minLen {
				minLen = len(codeRets[c])
			}
		}
		if minLen < minDays {
			continue // insufficient data
		}
		portfolioRets := make([]float64, minLen)
		for i := 0; i < minLen; i++ {
			for j, c := range codes {
				rets := codeRets[c]
				portfolioRets[i] += weights[j] * rets[len(rets)-minLen+i]
			}
		}

		// Historical VaR
		sorted := make([]float64, len(portfolioRets))
		copy(sorted, portfolioRets)
		sort.Float64s(sorted)
		varIdx := int(float64(len(sorted)) * (1 - confidence))
		if varIdx >= len(sorted) {
			varIdx = len(sorted) - 1
		}
		varValue := sorted[varIdx]

		if math.Abs(varValue) > maxVarPct {
			alerts = append(alerts, model.RiskAlert{
				UserID:     key.UserID,
				StrategyID: key.StrategyID,
				StockCode:  fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
				Level:      "high",
				Type:       "VaR超限",
				Description: fmt.Sprintf("95%%VaR=%.2f%%超过%.1f%%上限", math.Abs(varValue)*100, maxVarPct*100),
				RuleKey:    "var_breach",
				Dimension:  "portfolio",
				SeverityScore: int(math.Min(math.Abs(varValue)/maxVarPct*50, 100)),
				Evidence: model.JSONMap{
					"var":        varValue,
					"confidence": confidence,
					"max_var":    maxVarPct,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}

// ── P4: Position Overlimit ──

type PositionOverlimitRule struct{}

func (r *PositionOverlimitRule) Key() string { return "position_overlimit" }
func (r *PositionOverlimitRule) Evaluate(ctx context.Context, _ []string, holdings []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	maxTotal := 0.80
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["max_total_pct"].(float64); ok {
			maxTotal = v
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
		posVal := 0.0
		for _, h := range hList {
			val := float64(h.Quantity) * h.CurrentPrice
			if val <= 0 {
				val = float64(h.Quantity) * h.CostPrice
			}
			posVal += val
		}
		if posVal <= 0 {
			continue
		}
		// Get strategy fund allocation
		var alloc model.StrategyFundAllocation
		db.MySQL.Where("user_id = ? AND strategy_id = ? AND status = 'active'", key.UserID, key.StrategyID).
			Order("created_at DESC").First(&alloc)
		totalCapital := alloc.CurrentCash + posVal
		if totalCapital <= 0 {
			continue
		}
		pct := posVal / totalCapital
		if pct > maxTotal {
			alerts = append(alerts, model.RiskAlert{
				UserID:     key.UserID,
				StrategyID: key.StrategyID,
				StockCode:  fmt.Sprintf("__PORTFOLIO_%d__", key.StrategyID),
				Level:      "high",
				Type:       "总仓位超限",
				Description: fmt.Sprintf("总仓位%.0f%%超过策略上限%.0f%%", pct*100, maxTotal*100),
				RuleKey:    "position_overlimit",
				Dimension:  "portfolio",
				SeverityScore: int(math.Min((pct-maxTotal)*500, 100)),
				Evidence: model.JSONMap{
					"position_pct": pct,
					"max_pct":      maxTotal,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}
