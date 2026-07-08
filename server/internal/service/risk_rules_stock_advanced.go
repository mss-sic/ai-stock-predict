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
	RegisterRule(&ShrinkingReboundRule{})
	RegisterRule(&RSIOverboughtRule{})
	RegisterRule(&MACDDivergenceRule{})
	RegisterRule(&BollingerSqueezeRule{})
}

// ── S2: Shrinking Rebound ──

type ShrinkingReboundRule struct{}

func (r *ShrinkingReboundRule) Key() string { return "shrinking_rebound" }
func (r *ShrinkingReboundRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	reboundDays := 3
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["rebound_days"].(float64); ok {
			reboundDays = int(v)
		}
	}
	inClause := codesToInClause(codes)

	type KRow struct {
		Code     string  `gorm:"column:code"`
		Close    float64 `gorm:"column:close"`
		Volume   float64 `gorm:"column:volume"`
		TradeDate string `gorm:"column:trade_date"`
	}
	var allRows []KRow
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, close, volume, trade_date::text
		FROM stocks_daily_k WHERE code IN (%s)
		ORDER BY code, trade_date DESC
	`, inClause)).Scan(&allRows)

	// Group by code
	byCode := make(map[string][]KRow)
	for _, r := range allRows {
		byCode[r.Code] = append(byCode[r.Code], r)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for code, rows := range byCode {
		if len(rows) < reboundDays+1 {
			continue
		}
		// Check last reboundDays+1: prices should be rising but volume declining
		priceRising := true
		volDeclining := true
		for i := 0; i < reboundDays; i++ {
			if rows[i].Close <= rows[i+1].Close {
				priceRising = false
				break
			}
			if rows[i].Volume >= rows[i+1].Volume {
				volDeclining = false
				break
			}
		}
		if priceRising && volDeclining {
			alerts = append(alerts, model.RiskAlert{
				StockCode:   code,
				Level:       "medium",
				Type:        "连续缩量反弹",
				Description: fmt.Sprintf("连涨%d日但量能递减，假反弹风险", reboundDays),
				RuleKey:     "shrinking_rebound",
				Dimension:   "stock",
				SeverityScore: 50,
				Evidence: model.JSONMap{
					"rebound_days": reboundDays,
					"latest_close": rows[0].Close,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}

// ── S4: RSI Overbought ──

type RSIOverboughtRule struct{}

func (r *RSIOverboughtRule) Key() string { return "rsi_overbought" }
func (r *RSIOverboughtRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	threshold := 80.0
	period := 14
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["threshold"].(float64); ok {
			threshold = v
		}
		if v, ok := def.Thresholds["period"].(float64); ok {
			period = int(v)
		}
	}
	inClause := codesToInClause(codes)

	type KRow struct {
		Code   string  `gorm:"column:code"`
		Close  float64 `gorm:"column:close"`
		PrevClose float64 `gorm:"column:pre_close"`
	}
	var allRows []KRow
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, close, LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as pre_close
		FROM stocks_daily_k WHERE code IN (%s)
		ORDER BY code, trade_date DESC
	`, inClause)).Scan(&allRows)

	byCode := make(map[string][]float64) // code → price changes (oldest first)
	for _, r := range allRows {
		byCode[r.Code] = append([]float64{r.Close - r.PrevClose}, byCode[r.Code]...)
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for code, changes := range byCode {
		if len(changes) < period+1 {
			continue
		}
		rsi := computeRSIVerified(code, period)
		if rsi > threshold {
			alerts = append(alerts, model.RiskAlert{
				StockCode:   code,
				Level:       "medium",
				Type:        "RSI超买",
				Description: fmt.Sprintf("RSI(14)=%.1f>%.0f，短期回调风险", rsi, threshold),
				RuleKey:     "rsi_overbought",
				Dimension:   "stock",
				SeverityScore: int(math.Min((rsi-threshold)*2, 100)),
				Evidence: model.JSONMap{"rsi": rsi, "threshold": threshold},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}


// computeRSIVerified computes RSI from close prices fetched from DB.
func computeRSIVerified(code string, period int) float64 {
	var closes []float64
	db.PG.Raw("SELECT close FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT ?",
		code, period*3).Pluck("close", &closes)
	if len(closes) < period+1 {
		return 50
	}
	// reverse to chronological
	for i, j := 0, len(closes)-1; i < j; i, j = i+1, j-1 {
		closes[i], closes[j] = closes[j], closes[i]
	}
	changes := make([]float64, len(closes)-1)
	for i := 0; i < len(closes)-1; i++ {
		changes[i] = closes[i+1] - closes[i]
	}
	recent := changes[len(changes)-period:]
	var avgGain, avgLoss float64
	for _, c := range recent {
		if c > 0 {
			avgGain += c
		} else {
			avgLoss += math.Abs(c)
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	if avgLoss == 0 {
		return 100
	}
	return 100 - (100 / (1 + avgGain/avgLoss))
}

// ── S5: MACD Divergence ──

type MACDDivergenceRule struct{}

func (r *MACDDivergenceRule) Key() string { return "macd_divergence" }
func (r *MACDDivergenceRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	lookback := 60
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["lookback"].(float64); ok {
			lookback = int(v)
		}
	}
	inClause := codesToInClause(codes)

	type KRow struct {
		Code   string  `gorm:"column:code"`
		Close  float64 `gorm:"column:close"`
	}
	var allRows []KRow
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, close FROM stocks_daily_k WHERE code IN (%s)
		ORDER BY code, trade_date DESC LIMIT ?
	`, inClause), lookback*len(codes)+100).Scan(&allRows) // overshoot to be safe

	byCode := make(map[string][]float64)
	for _, r := range allRows {
		byCode[r.Code] = append(byCode[r.Code], r.Close)
	}
	// Reverse each to chronological order
	for k := range byCode {
		reverse(byCode[k])
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for code, prices := range byCode {
		if len(prices) < 40 {
			continue
		}
		dif, _, macd := myComputeMACD(prices, 12, 26, 9)
		if len(dif) < 30 {
			continue
		}
		// Find peaks in price and MACD in the last 30 bars
		recentPrices := prices[len(prices)-30:]
		recentMACD := macd[len(macd)-30:]

		pricePeaks := findPeaks(recentPrices)
		macdPeaks := findPeaks(recentMACD)

		if len(pricePeaks) >= 2 && len(macdPeaks) >= 2 {
			pLatest := pricePeaks[len(pricePeaks)-1]
			pPrev := pricePeaks[len(pricePeaks)-2]
			mLatest := macdPeaks[len(macdPeaks)-1]
			mPrev := macdPeaks[len(macdPeaks)-2]

			// Price higher high, MACD lower high → divergence
			if recentPrices[pLatest] > recentPrices[pPrev] && recentMACD[mLatest] < recentMACD[mPrev] {
				alerts = append(alerts, model.RiskAlert{
					StockCode:   code,
					Level:       "high",
					Type:        "MACD顶背离",
					Description: "价格创新高但MACD未创新高，动能衰竭",
					RuleKey:     "macd_divergence",
					Dimension:   "stock",
					SeverityScore: 80,
					Evidence: model.JSONMap{
						"price_peak":   recentPrices[pLatest],
						"price_prev":   recentPrices[pPrev],
						"macd_peak":    recentMACD[mLatest],
						"macd_prev":    recentMACD[mPrev],
					},
					HitDate: now,
				})
			}
		}
	}
	return alerts, nil, nil
}


// myComputeMACD wraps the existing computeMACD for []float64
func myComputeMACD(prices []float64, fast, slow, signal int) (dif, deaVal, macdVal []float64) {
	n := len(prices)
	emaFast := make([]float64, n)
	emaSlow := make([]float64, n)
	dif = make([]float64, n)
	deaVal = make([]float64, n)
	macdVal = make([]float64, n)

	alphaFast := 2.0 / float64(fast+1)
	alphaSlow := 2.0 / float64(slow+1)
	alphaSignal := 2.0 / float64(signal+1)

	for i := 0; i < n; i++ {
		if i == 0 {
			emaFast[i] = prices[i]
			emaSlow[i] = prices[i]
		} else {
			emaFast[i] = prices[i]*alphaFast + emaFast[i-1]*(1-alphaFast)
			emaSlow[i] = prices[i]*alphaSlow + emaSlow[i-1]*(1-alphaSlow)
		}
		dif[i] = emaFast[i] - emaSlow[i]
		if i == 0 {
			deaVal[i] = dif[i]
		} else {
			deaVal[i] = dif[i]*alphaSignal + deaVal[i-1]*(1-alphaSignal)
		}
		macdVal[i] = (dif[i] - deaVal[i]) * 2
	}
	return
}

// ── S6: Bollinger Squeeze ──

type BollingerSqueezeRule struct{}

func (r *BollingerSqueezeRule) Key() string { return "bollinger_squeeze" }
func (r *BollingerSqueezeRule) Evaluate(ctx context.Context, codes []string, _ []model.Holding) ([]model.RiskAlert, []model.RiskAlert, error) {
	if len(codes) == 0 {
		return nil, nil, nil
	}
	period := 20
	percentile := 0.20
	if def, err := loadRuleDef(r.Key()); err == nil && def.Thresholds != nil {
		if v, ok := def.Thresholds["period"].(float64); ok {
			period = int(v)
		}
		if v, ok := def.Thresholds["percentile"].(float64); ok {
			percentile = v
		}
	}
	inClause := codesToInClause(codes)

	type KRow struct {
		Code  string  `gorm:"column:code"`
		Close float64 `gorm:"column:close"`
	}
	var allRows []KRow
	db.PG.Raw(fmt.Sprintf(`
		SELECT code, close FROM stocks_daily_k WHERE code IN (%s)
		ORDER BY code, trade_date DESC
	`, inClause)).Scan(&allRows)

	byCode := make(map[string][]float64)
	for _, r := range allRows {
		byCode[r.Code] = append(byCode[r.Code], r.Close)
	}
	for k := range byCode {
		reverse(byCode[k])
	}

	now := time.Now()
	var alerts []model.RiskAlert
	for code, prices := range byCode {
		if len(prices) < period+30 {
			continue
		}
		// Compute Bollinger Bands
		bandwidths := make([]float64, len(prices)-period+1)
		for i := period - 1; i < len(prices); i++ {
			sum, sumSq := 0.0, 0.0
			for j := i - period + 1; j <= i; j++ {
				sum += prices[j]
				sumSq += prices[j] * prices[j]
			}
			mean := sum / float64(period)
			std := math.Sqrt(sumSq/float64(period) - mean*mean)
			upper := mean + 2*std
			lower := mean - 2*std
			if mean > 0 {
				bandwidths[i-period+1] = (upper - lower) / mean
			}
		}
		if len(bandwidths) < 30 {
			continue
		}
		// Sort historical bandwidths to find percentile threshold
		hist := make([]float64, len(bandwidths)-1)
		copy(hist, bandwidths[:len(bandwidths)-1])
		sort.Float64s(hist)
		idx := int(float64(len(hist)) * percentile)
		if idx >= len(hist) {
			idx = len(hist) - 1
		}
		pctThreshold := hist[idx]

		currentBW := bandwidths[len(bandwidths)-1]
		if currentBW < pctThreshold {
			alerts = append(alerts, model.RiskAlert{
				StockCode:   code,
				Level:       "low",
				Type:        "布林带收窄",
				Description: fmt.Sprintf("布林带宽%.4f低于历史%.0f分位（%.4f），变盘预警",
					currentBW, percentile*100, pctThreshold),
				RuleKey:     "bollinger_squeeze",
				Dimension:   "stock",
				SeverityScore: int((1 - currentBW/pctThreshold) * 50),
				Evidence: model.JSONMap{
					"bandwidth":  currentBW,
					"percentile": percentile,
					"pct_threshold": pctThreshold,
				},
				HitDate: now,
			})
		}
	}
	return alerts, nil, nil
}

func findPeaks(data []float64) []int {
	if len(data) < 3 {
		return nil
	}
	var peaks []int
	for i := 1; i < len(data)-1; i++ {
		if data[i] > data[i-1] && data[i] > data[i+1] {
			peaks = append(peaks, i)
		}
	}
	return peaks
}

func reverse(s []float64) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
