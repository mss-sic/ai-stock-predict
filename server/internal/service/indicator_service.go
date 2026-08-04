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
// maxFetch limits rows fetched from DB (default 500 = ~2 years, >250 needed for MA250).
func ComputeIndicators(code string, days int) ([]IndicatorRow, error) {
	// Fetch latest 500 rows, ordered DESC then reverse to ASC (avoids subquery temp table)
	var rawDesc []rawKLine
	if err := db.PG.Raw(`
		SELECT trade_date::text, open, high, low, close, volume, amount
		FROM stocks_daily_k WHERE code = ? AND close > 0
		ORDER BY trade_date DESC LIMIT 500
	`, code).Scan(&rawDesc).Error; err != nil {
		return nil, err
	}
	if len(rawDesc) < 5 {
		return nil, nil
	}
	// Reverse to ascending order
	raw := make([]rawKLine, len(rawDesc))
	for i, r := range rawDesc {
		raw[len(rawDesc)-1-i] = r
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
// MACD and MA5×MA20 cross detection use batch SQL queries (P5 optimization).
// RSI and KDJ signals use per-stock computation on recently-active stocks only.
func ScanGoldenCross(minScore float64) ([]SignalResult, error) {
	var results []SignalResult

	// ── MACD cross via batch SQL (uses precomputed indicators) ──
	macdResults, err := scanMACDCrossBatch()
	if err != nil {
		return nil, err
	}
	results = append(results, macdResults...)

	// ── MA5×MA20 cross via batch SQL (window functions) ──
	maResults, err := scanMACrossBatch()
	if err != nil {
		return nil, err
	}
	results = append(results, maResults...)

	// ── RSI/KDJ cross via per-stock computation ──
	// NOTE: RSI/KDJ scan is disabled by default due to performance (5000+ stocks).
	// These signals require full indicator computation per stock and are better suited
	// for scheduled batch jobs. The MACD + MA cross scans above cover the core signals.
	// To enable: pass includeSlow=true query parameter (future enhancement).

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

// scanMACrossBatch detects MA5×MA20 golden/death crosses.
// Strategy: load last 23 close prices per stock (20 for MA20 + 3 buffer),
// compute MA5/MA20 in Go, then check cross on last 2 days.
// This avoids expensive window functions on 3.2M+ rows.
func scanMACrossBatch() ([]SignalResult, error) {
	// Step 1: get last 2 trading dates from trade_calendar
	type dateRow struct {
		TradeDate string
	}
	var dates []dateRow
	if err := db.PG.Raw(`
		SELECT trade_date::text FROM trade_calendar
			WHERE trade_date <= CURRENT_DATE AND is_trading_day = true
		ORDER BY trade_date DESC LIMIT 2
	`).Scan(&dates).Error; err != nil || len(dates) < 2 {
		// Fallback: use stocks_daily_k
		db.PG.Raw(`
			SELECT DISTINCT trade_date::text FROM stocks_daily_k
			ORDER BY trade_date DESC LIMIT 2
		`).Scan(&dates)
	}
	if len(dates) < 2 {
		return nil, nil
	}
	currDate := dates[0].TradeDate
	_ = dates[1].TradeDate // prevDate — used implicitly via close array indexing

	// Step 2: load last 23 close prices per stock (enough for MA20)
	type closeRow struct {
		Code      string
		TradeDate string
		Close     float64
	}
	var closes []closeRow
	db.PG.Raw(`
		SELECT code, trade_date::text, close FROM stocks_daily_k
		WHERE close > 0 AND trade_date > ?::date - INTERVAL '90 days'
		ORDER BY code, trade_date ASC
	`, currDate).Scan(&closes)

	// Step 3: build per-stock close arrays, compute MA5/MA20 for last 2 days
	type stockMA struct {
		code      string
		name      string
		prevMA5   float64
		prevMA20  float64
		currMA5   float64
		currMA20  float64
		currClose float64
	}
	stockCloses := map[string][]float64{}
	codeOrder := []string{}
	for _, r := range closes {
		if _, ok := stockCloses[r.Code]; !ok {
			stockCloses[r.Code] = []float64{}
			codeOrder = append(codeOrder, r.Code)
		}
		stockCloses[r.Code] = append(stockCloses[r.Code], r.Close)
	}

	var maList []stockMA
	for _, code := range codeOrder {
		arr := stockCloses[code]
		if len(arr) < 20 {
			continue
		}
		n := len(arr)
		// Last 2 entries: prev (n-2) and curr (n-1)
		prevIdx := n - 2
		currIdx := n - 1
		if prevIdx < 19 {
			continue
		}

		sum5p, sum20p := 0.0, 0.0
		for i := prevIdx - 4; i <= prevIdx; i++ {
			sum5p += arr[i]
		}
		for i := prevIdx - 19; i <= prevIdx; i++ {
			sum20p += arr[i]
		}
		prevMA5 := sum5p / 5.0
		prevMA20 := sum20p / 20.0

		sum5c, sum20c := 0.0, 0.0
		for i := currIdx - 4; i <= currIdx; i++ {
			sum5c += arr[i]
		}
		for i := currIdx - 19; i <= currIdx; i++ {
			sum20c += arr[i]
		}
		currMA5 := sum5c / 5.0
		currMA20 := sum20c / 20.0

		// Check cross
		isGolden := prevMA5 <= prevMA20 && currMA5 > currMA20
		isDeath := prevMA5 >= prevMA20 && currMA5 < currMA20
		if isGolden || isDeath {
			maList = append(maList, stockMA{
				code:      code,
				prevMA5:   prevMA5,
				prevMA20:  prevMA20,
				currMA5:   currMA5,
				currMA20:  currMA20,
				currClose: arr[currIdx],
			})
		}
	}

	// Step 4: batch-resolve names
	codeSet := map[string]bool{}
	for _, m := range maList {
		codeSet[m.code] = true
	}
	codes := make([]string, 0, len(codeSet))
	for c := range codeSet {
		codes = append(codes, c)
	}

	type nameRow struct {
		Code string
		Name string
	}
	var names []nameRow
	if len(codes) > 0 {
		db.PG.Raw(`SELECT code, name FROM stocks_basic WHERE code IN ?`, codes).Scan(&names)
	}
	nameMap := map[string]string{}
	for _, n := range names {
		nameMap[n.Code] = n.Name
	}

	results := make([]SignalResult, 0, len(maList))
	for _, m := range maList {
		isGolden := m.prevMA5 <= m.prevMA20 && m.currMA5 > m.currMA20
		signal := "MA5死叉MA20"
		direction := "bearish"
		if isGolden {
			signal = "MA5金叉MA20"
			direction = "bullish"
		}
		results = append(results, SignalResult{
			Code:      m.code,
			Name:      nameMap[m.code],
			Signal:    signal,
			Direction: direction,
			Date:      currDate,
			Close:     m.currClose,
		})
	}
	return results, nil
}

// scanMACDCrossBatch detects MACD golden/death crosses using precomputed ema12/ema26/macd_dif/dea.
// Replaces per-stock ComputeIndicators MACD part with a single SQL query (P5 optimization).
func scanMACDCrossBatch() ([]SignalResult, error) {
	type row struct {
		Code      string
		Name      string
		TradeDate string
		Close     float64
		PrevDif   float64
		PrevDea   float64
		CurrDif   float64
		CurrDea   float64
	}

	// Find latest 2 trading days for each stock, check DIF/DEA cross
	var rows []row
	err := db.PG.Raw(`
		WITH ranked AS (
			SELECT code, trade_date, close, macd_dif, macd_dea,
				ROW_NUMBER() OVER (PARTITION BY code ORDER BY trade_date DESC) AS rn
			FROM stocks_daily_k
			WHERE macd_dif IS NOT NULL AND macd_dea IS NOT NULL
		),
		pivoted AS (
			SELECT code,
				MAX(CASE WHEN rn = 2 THEN trade_date END) AS prev_date,
				MAX(CASE WHEN rn = 2 THEN close END)       AS prev_close,
				MAX(CASE WHEN rn = 2 THEN macd_dif END)    AS prev_dif,
				MAX(CASE WHEN rn = 2 THEN macd_dea END)         AS prev_dea,
				MAX(CASE WHEN rn = 1 THEN trade_date END)  AS curr_date,
				MAX(CASE WHEN rn = 1 THEN close END)       AS curr_close,
				MAX(CASE WHEN rn = 1 THEN macd_dif END)    AS curr_dif,
				MAX(CASE WHEN rn = 1 THEN macd_dea END)         AS curr_dea
			FROM ranked WHERE rn <= 2
			GROUP BY code
			HAVING COUNT(*) = 2
		)
		SELECT s.code, s.name, p.curr_date, p.curr_close,
			p.prev_dif, p.prev_dea, p.curr_dif, p.curr_dea
		FROM pivoted p
		JOIN stocks_basic s ON s.code = p.code
		WHERE (p.prev_dif <= p.prev_dea AND p.curr_dif > p.curr_dea)   -- golden cross
		   OR (p.prev_dif >= p.prev_dea AND p.curr_dif < p.curr_dea)   -- death cross
	`).Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	results := make([]SignalResult, 0, len(rows))
	for _, r := range rows {
		isGolden := r.PrevDif <= r.PrevDea && r.CurrDif > r.CurrDea
		signal := "MACD死叉"
		direction := "bearish"
		if isGolden {
			signal = "MACD金叉"
			direction = "bullish"
		}
		results = append(results, SignalResult{
			Code:      r.Code,
			Name:      r.Name,
			Signal:    signal,
			Direction: direction,
			Date:      r.TradeDate,
			Close:     r.Close,
		})
	}
	return results, nil
}
