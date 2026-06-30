package service

import (
	"math"
	"sort"

	"github.com/ai-stock-predict/server/internal/db"
)

// IndicatorRow holds all technical indicators for one trading day.
type IndicatorRow struct {
	TradeDate string  `json:"tradeDate"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    int64   `json:"volume"`
	Amount    float64 `json:"amount"`
	MA5       float64 `json:"ma5"`
	MA10      float64 `json:"ma10"`
	MA20      float64 `json:"ma20"`
	MA60      float64 `json:"ma60"`
	MA250     float64 `json:"ma250"`
	BOLLUpper float64 `json:"bollUpper"`
	BOLLMid   float64 `json:"bollMid"`
	BOLLLower float64 `json:"bollLower"`
	RSI6      float64 `json:"rsi6"`
	RSI14     float64 `json:"rsi14"`
	RSI24     float64 `json:"rsi24"`
	KDJK      float64 `json:"kdjK"`
	KDJD      float64 `json:"kdjD"`
	KDJJ      float64 `json:"kdjJ"`
	MACDDIF   float64 `json:"macdDif"`
	MACDDEA   float64 `json:"macdDea"`
	MACDHist  float64 `json:"macdHist"`
	VolMA5    float64 `json:"volMa5"`
	VolMA20   float64 `json:"volMa20"`
}

type rawKLine struct {
	TradeDate string
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    int64
	Amount    float64
}

// ComputeIndicators returns full technical indicators for a stock.
func ComputeIndicators(code string, days int) ([]IndicatorRow, error) {
	// Fetch enough raw data for all indicators (max window = 250 for MA250 + 14 for RSI init)
	var raw []rawKLine
	if err := db.PG.Raw(`
		SELECT trade_date::text, open, high, low, close, volume, amount
		FROM stocks_daily_k WHERE code = ? AND close > 0
		ORDER BY trade_date ASC
	`, code).Scan(&raw).Error; err != nil {
		return nil, err
	}
	if len(raw) < 5 {
		return nil, nil
	}

	// ── Compute MAs ──
	mas := computeMAs(raw)
	// ── Compute BOLL (based on MA20 + 2σ) ──
	bolls := computeBOLL(raw, 20)
	// ── Compute RSI ──
	rsi6 := computeRSI(raw, 6)
	rsi14 := computeRSI(raw, 14)
	rsi24 := computeRSI(raw, 24)
	// ── Compute KDJ (9,3,3) ──
	kdjK, kdjD, kdjJ := computeKDJ(raw, 9, 3)
	// ── Compute MACD (12,26,9) ──
	macdDif, macdDea, macdHist := computeMACD(raw, 12, 26, 9)
	// ── Compute Vol MA ──
	volMA5 := computeVolMA(raw, 5)
	volMA20 := computeVolMA(raw, 20)

	// ── Assemble output (trim to requested days) ──
	start := len(raw) - days
	if start < 0 {
		start = 0
	}

	result := make([]IndicatorRow, 0, len(raw)-start)
	for i := start; i < len(raw); i++ {
		result = append(result, IndicatorRow{
			TradeDate: raw[i].TradeDate,
			Open:      raw[i].Open,
			High:      raw[i].High,
			Low:       raw[i].Low,
			Close:     raw[i].Close,
			Volume:    raw[i].Volume,
			Amount:    raw[i].Amount,
			MA5:       mas[5][i],
			MA10:      mas[10][i],
			MA20:      mas[20][i],
			MA60:      mas[60][i],
			MA250:     mas[250][i],
			BOLLUpper: bolls[i].upper,
			BOLLMid:   bolls[i].mid,
			BOLLLower: bolls[i].lower,
			RSI6:      rsi6[i],
			RSI14:     rsi14[i],
			RSI24:     rsi24[i],
			KDJK:      kdjK[i],
			KDJD:      kdjD[i],
			KDJJ:      kdjJ[i],
			MACDDIF:   macdDif[i],
			MACDDEA:   macdDea[i],
			MACDHist:  macdHist[i],
			VolMA5:    volMA5[i],
			VolMA20:   volMA20[i],
		})
	}
	return result, nil
}

// ── Indicator computation helpers ──

func computeMAs(raw []rawKLine) map[int][]float64 {
	periods := []int{5, 10, 20, 60, 250}
	result := map[int][]float64{}
	for _, p := range periods {
		result[p] = make([]float64, len(raw))
		sum := float64(0)
		for i := range raw {
			sum += raw[i].Close
			if i >= p {
				sum -= raw[i-p].Close
			}
			if i >= p-1 {
				result[p][i] = math.Round(sum/float64(p)*100) / 100
			}
		}
	}
	return result
}

func computeBOLL(raw []rawKLine, period int) []struct{ upper, mid, lower float64 } {
	result := make([]struct{ upper, mid, lower float64 }, len(raw))
	for i := period - 1; i < len(raw); i++ {
		sum := float64(0)
		for j := i - period + 1; j <= i; j++ {
			sum += raw[j].Close
		}
		mid := sum / float64(period)
		sqSum := float64(0)
		for j := i - period + 1; j <= i; j++ {
			d := raw[j].Close - mid
			sqSum += d * d
		}
		std := math.Sqrt(sqSum / float64(period))
		result[i] = struct{ upper, mid, lower float64 }{
			upper: math.Round((mid+2*std)*100) / 100,
			mid:   math.Round(mid*100) / 100,
			lower: math.Round((mid-2*std)*100) / 100,
		}
	}
	return result
}

func computeRSI(raw []rawKLine, period int) []float64 {
	result := make([]float64, len(raw))
	if len(raw) <= period {
		return result
	}
	avgGain, avgLoss := float64(0), float64(0)
	for i := 1; i <= period; i++ {
		chg := raw[i].Close - raw[i-1].Close
		if chg > 0 {
			avgGain += chg
		} else {
			avgLoss -= chg
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)
	for i := period; i < len(raw); i++ {
		if avgLoss == 0 {
			result[i] = 100
		} else {
			rs := avgGain / avgLoss
			result[i] = math.Round((100-100/(1+rs))*100) / 100
		}
		if i+1 < len(raw) {
			chg := raw[i+1].Close - raw[i].Close
			gain, loss := float64(0), float64(0)
			if chg > 0 {
				gain = chg
			} else {
				loss = -chg
			}
			avgGain = (avgGain*float64(period-1) + gain) / float64(period)
			avgLoss = (avgLoss*float64(period-1) + loss) / float64(period)
		}
	}
	return result
}

func computeKDJ(raw []rawKLine, n, m int) (k, d, j []float64) {
	k = make([]float64, len(raw))
	d = make([]float64, len(raw))
	j = make([]float64, len(raw))
	if len(raw) < n {
		return
	}
	// Initialize with 50
	for i := n - 1; i < len(raw); i++ {
		lowest := raw[i].Low
		highest := raw[i].High
		for x := i - n + 1; x <= i; x++ {
			if raw[x].Low < lowest {
				lowest = raw[x].Low
			}
			if raw[x].High > highest {
				highest = raw[x].High
			}
		}
		rsv := float64(50)
		if highest != lowest {
			rsv = (raw[i].Close - lowest) / (highest - lowest) * 100
		}
		if i == n-1 {
			k[i] = 50
			d[i] = 50
		} else {
			k[i] = (k[i-1]*float64(m-1) + rsv) / float64(m)
			d[i] = (d[i-1]*float64(m-1) + k[i]) / float64(m)
		}
		k[i] = math.Round(k[i]*100) / 100
		d[i] = math.Round(d[i]*100) / 100
		j[i] = math.Round((3*k[i]-2*d[i])*100) / 100
	}
	return
}

func computeMACD(raw []rawKLine, fast, slow, signal int) (dif, dea, hist []float64) {
	dif = make([]float64, len(raw))
	dea = make([]float64, len(raw))
	hist = make([]float64, len(raw))
	if len(raw) < slow {
		return
	}
	emaFast := raw[0].Close
	emaSlow := raw[0].Close
	af := 2.0 / float64(fast+1)
	as := 2.0 / float64(slow+1)
	ad := 2.0 / float64(signal+1)
	for i := 0; i < len(raw); i++ {
		if i == 0 {
			emaFast = raw[i].Close
			emaSlow = raw[i].Close
		} else {
			emaFast = raw[i].Close*af + emaFast*(1-af)
			emaSlow = raw[i].Close*as + emaSlow*(1-as)
		}
		d := emaFast - emaSlow
		if i == 0 {
			dea[i] = d
		} else {
			dea[i] = d*ad + dea[i-1]*(1-ad)
		}
		dif[i] = math.Round(d*10000) / 10000
		dea[i] = math.Round(dea[i]*10000) / 10000
		hist[i] = math.Round((dif[i]-dea[i])*2*10000) / 10000
	}
	return
}

func computeVolMA(raw []rawKLine, period int) []float64 {
	result := make([]float64, len(raw))
	sum := int64(0)
	for i := range raw {
		sum += raw[i].Volume
		if i >= period {
			sum -= raw[i-period].Volume
		}
		if i >= period-1 {
			result[i] = float64(sum) / float64(period)
		}
	}
	return result
}

// ── Signal scanning ──

// SignalResult is a detected technical signal.
type SignalResult struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Signal    string  `json:"signal"`
	Direction string  `json:"direction"` // bullish / bearish
	Date      string  `json:"date"`
	Close     float64 `json:"close"`
}

// ScanGoldenCross scans all stocks for golden/death cross signals (MA5×MA20, MACD).
func ScanGoldenCross(minScore float64) ([]SignalResult, error) {
	type stock struct {
		Code string
		Name string
	}
	var stocks []stock
	db.PG.Raw(`SELECT code, name FROM stocks_basic WHERE code !~ '^IDX' ORDER BY code`).Scan(&stocks)

	var results []SignalResult
	for _, s := range stocks {
		indicators, err := ComputeIndicators(s.Code, 60)
		if err != nil || len(indicators) < 3 {
			continue
		}
		// Check last 3 days for cross signals
		for i := len(indicators) - 3; i < len(indicators); i++ {
			if i <= 0 {
				continue
			}
			prev := indicators[i-1]
			curr := indicators[i]

			// MA5 × MA20 golden cross
			if prev.MA5 > 0 && prev.MA20 > 0 && curr.MA5 > 0 && curr.MA20 > 0 {
				if prev.MA5 <= prev.MA20 && curr.MA5 > curr.MA20 {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "MA5金叉MA20",
						Direction: "bullish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
				if prev.MA5 >= prev.MA20 && curr.MA5 < curr.MA20 {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "MA5死叉MA20",
						Direction: "bearish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
			}

			// MACD golden cross
			if prev.MACDDIF != 0 && curr.MACDDIF != 0 {
				if prev.MACDDIF <= prev.MACDDEA && curr.MACDDIF > curr.MACDDEA {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "MACD金叉",
						Direction: "bullish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
				if prev.MACDDIF >= prev.MACDDEA && curr.MACDDIF < curr.MACDDEA {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "MACD死叉",
						Direction: "bearish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
			}

			// KDJ golden cross
			if prev.KDJK > 0 && curr.KDJK > 0 {
				if prev.KDJK <= prev.KDJD && curr.KDJK > curr.KDJD && curr.KDJK < 80 {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "KDJ金叉",
						Direction: "bullish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
			}

			// RSI oversold/overbought
			if curr.RSI14 > 0 {
				if prev.RSI14 >= 30 && curr.RSI14 < 30 {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "RSI超卖",
						Direction: "bullish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
				if prev.RSI14 <= 70 && curr.RSI14 > 70 {
					results = append(results, SignalResult{
						Code: s.Code, Name: s.Name, Signal: "RSI超买",
						Direction: "bearish", Date: curr.TradeDate, Close: curr.Close,
					})
				}
			}
		}
	}

	// Sort by date descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Date > results[j].Date
	})

	// Deduplicate per stock per signal per date
	seen := map[string]bool{}
	deduped := make([]SignalResult, 0, len(results))
	for _, r := range results {
		key := r.Code + r.Signal + r.Date
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, r)
		}
	}
	return deduped, nil
}
