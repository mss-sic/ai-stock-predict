package service

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/model"

	"github.com/ai-stock-predict/server/internal/db"
)

// ── DataProvider Interface ──

// DataProvider abstracts K-line and indicator data access for signal generation.
type DataProvider interface {
	GetClose(code, date string) float64
	GetOpen(code, date string) float64
	GetDailyChange(code, date string) float64
	GetIndicatorValue(indicator, code, date string) (float64, bool)
	GetNextDate(date string) string
	Dates() []string
}

// ── SignalEngineConfig ──

// SignalEngineConfig holds strategy parameters for signal generation.
type SignalEngineConfig struct {
	MaxHoldings        int
	MaxTotalBuyPct     float64 // 单日最大买入仓位 (占总资金%), 默认60
	BuyPositionPct     float64 // 单票买入仓位 (占总资金%)
	AddPositionPct     float64
	ReducePositionPct  float64
	StopLoss           float64
	StopProfit         float64
	CommissionRate     float64
	MinCommission      float64
	StampTaxRate       float64
	PositionConcentrationLimit float64
	ScoringConfig          model.ScoringConfig   // 候选评分配置
	EnableTrailingStop     bool                  // 启用移动止盈
	TrailingStopActivation float64               // 激活阈值%
	TrailingStopDrawdown   float64               // 回撤比例%
}

// ── Position ──

type SignalPosition struct {
	Code     string
	Name     string
	Quantity int
	BuyPrice float64
	BuyDate  string
}

// ── SignalRecord ──

type SignalRecord struct {
	SignalDate    string
	ExecDate      string
	StockCode     string
	StockName     string
	ActionType    string
	PlannedPrice  float64
	PlannedQty    int
	PlannedAmount float64
	Status        string
	Reason        string
}

// ── SignalEngine ──

// SignalEngine is a shared signal generation engine used by both backtest and live trading.
type SignalEngine struct{}

// NewSignalEngine creates a new signal engine.
func NewSignalEngine() *SignalEngine {
	return &SignalEngine{}
}

// GenerateSignals evaluates strategy conditions against positions and universe,
// returning pending signals for the next trading day.
func (e *SignalEngine) GenerateSignals(
	date string,
	cash float64,
	positions map[string]*SignalPosition,
	universe []StockInfo,
	buyConds, addConds, sellConds, reduceConds []ConditionDef,
	data DataProvider,
	cfg SignalEngineConfig,
	budget *PositionBudget,
	logFn func(format string, args ...interface{}),
) []SignalRecord {
	var signals []SignalRecord

	if logFn == nil {
		logFn = func(format string, args ...interface{}) {}
	}

	// Entry log
	logFn("🔧 SignalEngine: 候选池%d只 | 现持仓%d | 止损%.0f%% | 止盈%.0f%% | 移动止盈:%v",
		len(universe), len(positions), cfg.StopLoss, cfg.StopProfit, cfg.EnableTrailingStop)

	// ── Stop-profit / Stop-loss / Trailing-stop checks ──
	stopSet := make(map[string]bool)
	for code, pos := range positions {
		closePrice := data.GetClose(code, date)
		if closePrice <= 0 || pos.BuyDate == date {
			continue
		}
		chgPct := (closePrice - pos.BuyPrice) / pos.BuyPrice * 100

		// Fixed stop-loss (always active)
		if cfg.StopLoss < 0 && chgPct <= cfg.StopLoss {
			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
				PlannedAmount: closePrice * float64(pos.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止损触发 %.1f%% ≤ %.1f%%", chgPct, cfg.StopLoss),
			})
			stopSet[code] = true
			logFn("🛑 STOP %s(%s) 止损 %.1f%% ≤ %.1f%%", pos.Name, code, chgPct, cfg.StopLoss)
			continue
		}

		// Trailing stop: only triggers above activation threshold
		if cfg.EnableTrailingStop && cfg.TrailingStopActivation > 0 && chgPct >= cfg.TrailingStopActivation {
			logFn("🔺 %s(%s) 触发移动止盈评估: 涨幅%.1f%%≥激活%.0f%%", pos.Name, code, chgPct, cfg.TrailingStopActivation)
			// Compute peak price since activation (approximate with close as peak)
			drawdown := cfg.TrailingStopDrawdown
			if drawdown <= 0 { drawdown = 8 }
			peakPrice := closePrice // simplified: use close as peak for now
			stopPrice := peakPrice * (1 - drawdown/100)
			if closePrice <= stopPrice {
				signals = append(signals, SignalRecord{
					SignalDate: date, ExecDate: data.GetNextDate(date),
					StockCode: code, StockName: pos.Name,
					ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: closePrice * float64(pos.Quantity),
					Status: "pending",
					Reason: fmt.Sprintf("移动止盈: 从峰值¥%.2f回撤%.0f%% → ¥%.2f", peakPrice, drawdown, stopPrice),
				})
				stopSet[code] = true
				logFn("🎯 TRAIL %s(%s) 移动止盈 峰值¥%.2f 回撤%.0f%%", pos.Name, code, peakPrice, drawdown)
				continue
			}
		}

		// Fixed stop-profit (only when trailing stop is off or hasn't activated)
		if !cfg.EnableTrailingStop && cfg.StopProfit > 0 && chgPct >= cfg.StopProfit {
			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
				PlannedAmount: closePrice * float64(pos.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止盈触发 %.1f%% ≥ %.1f%%", chgPct, cfg.StopProfit),
			})
			stopSet[code] = true
			logFn("🎯 STOP %s(%s) 止盈 %.1f%% ≥ %.1f%%", pos.Name, code, chgPct, cfg.StopProfit)
		}
	}

	// ── Sell/Reduce checks ──
	for code, pos := range positions {
		if stopSet[code] || pos.BuyDate == date {
			continue
		}

		if ok, _ := e.evalConds(sellConds, code, date, data); ok {
			closePrice := data.GetClose(code, date)
			if closePrice <= 0 {
				continue
			}
			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: "sell", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
				PlannedAmount: closePrice * float64(pos.Quantity),
				Status: "pending",
				Reason: "满足卖出条件",
			})
			logFn("📤 SELL %s(%s) %d股 @¥%.2f", pos.Name, code, pos.Quantity, closePrice)
			continue
		}

		if ok, _ := e.evalConds(reduceConds, code, date, data); ok {
			closePrice := data.GetClose(code, date)
			if closePrice <= 0 {
				continue
			}
			reduceQty := int(float64(pos.Quantity) * cfg.ReducePositionPct / 100)
			if reduceQty >= pos.Quantity {
				reduceQty = pos.Quantity
			}
			actionType := "reduce"
			if reduceQty >= pos.Quantity {
				actionType = "sell"
			}
			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: actionType, PlannedPrice: closePrice, PlannedQty: reduceQty,
				PlannedAmount: closePrice * float64(reduceQty),
				Status: "pending",
				Reason: fmt.Sprintf("满足减仓条件 (%.0f%%)", cfg.ReducePositionPct),
			})
			logFn("📉 REDUCE %s(%s) %d股 @¥%.2f", pos.Name, code, reduceQty, closePrice)
		}
	}

	// ── Buy checks (two-stage: scan → rank + industry-diversify + allocate) ──
	activeCount := len(positions)
	for _, pos := range positions {
		if pos.Quantity <= 0 {
			activeCount--
		}
	}
	slotCount := cfg.MaxHoldings - activeCount
	maxTotalBuy := cfg.MaxTotalBuyPct
	if maxTotalBuy <= 0 { maxTotalBuy = 60 }
	maxBuyCash := cash * maxTotalBuy / 100
	buyAmountPer := cash * cfg.BuyPositionPct / 100
	if buyAmountPer <= 0 { buyAmountPer = cash * 10 / 100 }

	// Industry budget from strategy config (via budget or defaults)
	maxSingleIndustryPct := 30.0
	minIndustryCount := 3
	if budget != nil {
		if budget.MaxSingleIndustry > 0 {
			maxSingleIndustryPct = budget.MaxSingleIndustry
		}
		if budget.MinIndustryCount > 0 {
			minIndustryCount = budget.MinIndustryCount
		}
	}
	// Single position limit from budget
	singlePosLimit := buyAmountPer
	if budget != nil && budget.SinglePositionLimit > 0 {
		singlePosLimit = cash * budget.SinglePositionLimit / 100
		if singlePosLimit > buyAmountPer {
			singlePosLimit = buyAmountPer
		}
	}

	if slotCount > 0 && cash > 0 && len(buyConds) > 0 {
		totalStocks := len(universe)
		progressStep := 500
		if totalStocks < 500 { progressStep = totalStocks }
		scanStart := time.Now()

		// ── Stage 1: Collect all candidates with scores ──
		type candidate struct {
			code        string
			name        string
			price       float64
			volumeRank  float64 // 1 = highest volume
			chgPct      float64 // daily change %
			score       float64 // composite score
		}
		var candidates []candidate
		scanned := 0

		for _, si := range universe {
			if _, isHeld := positions[si.Code]; isHeld {
				continue
			}
			scanned++
			if scanned%progressStep == 0 {
				logFn("🔍 扫描进度: %d/%d (候选%d, 耗时%.0fs)", scanned, totalStocks, len(candidates), time.Since(scanStart).Seconds())
			}
			if ok, _ := e.evalConds(buyConds, si.Code, date, data); !ok {
				continue
			}
			closePrice := data.GetClose(si.Code, date)
			if closePrice <= 0 {
				continue
			}
			// Multi-dimensional scoring per strategy config
			score := e.computeCandidateScore(cfg.ScoringConfig, si.Code, date, scanned, totalStocks, data)
			if score < cfg.ScoringConfig.MinScore {
				continue
			}
			chgPct := data.GetDailyChange(si.Code, date)
			candidates = append(candidates, candidate{
				code: si.Code, name: si.Name, price: closePrice,
				chgPct: chgPct, score: score,
			})
		}
		scanElapsed := time.Since(scanStart).Seconds()
		logFn("🔍 扫描完成: %d只 → 候选%d (耗时%.0fs)", scanned, len(candidates), scanElapsed)

		// ── Stage 2: Sort by score, industry-diversify, allocate ──
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].score > candidates[j].score
			})

			// Get industry for each candidate
			type indInfo struct { industry string }
			codeToIndustry := make(map[string]string)
			industryAlloc := make(map[string]float64) // total allocated per industry
			var rankedCandidates []candidate
			for _, c := range candidates {
				ind := e.getIndustry(c.code)
				codeToIndustry[c.code] = ind
				rankedCandidates = append(rankedCandidates, c)
			}

			bought := 0
			totalAllocated := 0.0
			distinctIndustries := make(map[string]bool)

			// Phase 1: ensure min industry diversity — pick top 1 from each industry first
			var phase2Candidates []candidate
			seenIndustries := make(map[string]bool)
			for _, c := range rankedCandidates {
				ind := codeToIndustry[c.code]
				if !seenIndustries[ind] && len(seenIndustries) < minIndustryCount {
					seenIndustries[ind] = true
					// Allocate this candidate
					allocAmt := singlePosLimit
					qty := int(allocAmt/c.price/100) * 100
					if qty < 100 && allocAmt >= c.price*100 { qty = 100 }
					if qty <= 0 { continue }
					amount := c.price * float64(qty)
					if totalAllocated+amount > maxBuyCash { continue }
					if industryAlloc[ind]+amount > maxBuyCash*maxSingleIndustryPct/100 { continue }

					signals = append(signals, SignalRecord{
						SignalDate: date, ExecDate: data.GetNextDate(date),
						StockCode: c.code, StockName: c.name,
						ActionType: "buy", PlannedPrice: c.price, PlannedQty: qty,
						PlannedAmount: amount, Status: "pending",
						Reason: fmt.Sprintf("满足买入条件 | 行业[%s]分散优先 | 评分%.2f", ind, c.score),
					})
					logFn("📈 BUY  %s(%s) %d股 @¥%.2f → ¥%.0f [%s 评分%.2f 行业分散]", c.name, c.code, qty, c.price, amount, ind, c.score)
					totalAllocated += amount
					industryAlloc[ind] += amount
					distinctIndustries[ind] = true
					bought++
				} else {
					phase2Candidates = append(phase2Candidates, c)
				}
			}

			// Phase 2: fill remaining slots, respecting industry cap
			for _, c := range phase2Candidates {
				if bought >= slotCount || totalAllocated >= maxBuyCash {
					break
				}
				ind := codeToIndustry[c.code]
				allocAmt := singlePosLimit
				qty := int(allocAmt/c.price/100) * 100
				if qty < 100 && allocAmt >= c.price*100 { qty = 100 }
				if qty <= 0 { continue }
				amount := c.price * float64(qty)
				if totalAllocated+amount > maxBuyCash { continue }
				if industryAlloc[ind]+amount > maxBuyCash*maxSingleIndustryPct/100 { continue }

				signals = append(signals, SignalRecord{
					SignalDate: date, ExecDate: data.GetNextDate(date),
					StockCode: c.code, StockName: c.name,
					ActionType: "buy", PlannedPrice: c.price, PlannedQty: qty,
					PlannedAmount: amount, Status: "pending",
					Reason: fmt.Sprintf("满足买入条件 | 评分%.2f | 行业[%s]仓位%.0f%%", c.score, ind, industryAlloc[ind]/maxBuyCash*100),
				})
				logFn("📈 BUY  %s(%s) %d股 @¥%.2f → ¥%.0f [%s 评分%.2f]", c.name, c.code, qty, c.price, amount, ind, c.score)
				totalAllocated += amount
				industryAlloc[ind] += amount
				distinctIndustries[ind] = true
				bought++
			}

			logFn("✅ 买入分配完成: %d笔 覆盖%d行业 ¥%.0f/¥%.0f", bought, len(distinctIndustries), totalAllocated, maxBuyCash)
			if bought == 0 {
				logFn("ℹ 无符合条件的候选（行业分散/仓位限制）")
			}
		} else {
			logFn("ℹ 无符合条件的候选")
		}
	}

	// ── Add checks ──
	if len(addConds) > 0 {
		for code, pos := range positions {
			if pos.Quantity <= 0 {
				continue
			}
			if ok, _ := e.evalConds(addConds, code, date, data); !ok {
				continue
			}
			closePrice := data.GetClose(code, date)
			if closePrice <= 0 {
				continue
			}
			addAmount := cash * cfg.AddPositionPct / 100
			addQty := int(addAmount/closePrice/100) * 100
			if addQty <= 0 {
				continue
			}

			// Avoid duplicate add signal for same stock+date
			dup := false
			for _, s := range signals {
				if s.StockCode == code && s.ActionType == "add" {
					dup = true
					break
				}
			}
			if dup {
				continue
			}

			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: "add", PlannedPrice: closePrice, PlannedQty: addQty,
				PlannedAmount: closePrice * float64(addQty),
				Status: "pending",
				Reason: "满足加仓条件",
			})
			logFn("➕ ADD   %s(%s) %d股 @¥%.2f", pos.Name, code, addQty, closePrice)
		}
	}

	return signals
}

// EvalConds evaluates a set of conditions for a stock on a given date.
// Conditions are grouped by LogicGroup — all conditions within a group must be met (AND).
// Groups are OR'd — if any group is fully met, returns true.
// computeCandidateScore calculates a composite score for a stock based on strategy scoring config.
func (e *SignalEngine) computeCandidateScore(
	cfg model.ScoringConfig, code, date string, scanned, total int, data DataProvider,
) float64 {
	dims := cfg.Dimensions
	if len(dims) == 0 {
		dims = model.DefaultScoringConfig().Dimensions
	}
	var totalScore, totalWeight float64
	for _, dim := range dims {
		if dim.Weight <= 0 { continue }
		raw := e.computeDimensionScore(dim, code, date, scanned, total, data)
		// Normalize: direction asc (lower=better) → invert
		if dim.Direction == "asc" { raw = 1.0 - raw }
		totalScore += raw * dim.Weight
		totalWeight += dim.Weight
	}
	if totalWeight > 0 {
		return totalScore / totalWeight
	}
	return 0.5
}

// computeDimensionScore computes a single scoring dimension (0-1 range).
func (e *SignalEngine) computeDimensionScore(
	dim model.ScoringDimension, code, date string, scanned, total int, data DataProvider,
) float64 {
	params := dim.Params
	switch dim.Indicator {
	case "ma_trend":
		shortN := floatParam(params, "short", 5)
		longN := floatParam(params, "long", 20)
		shortMA, _ := data.GetIndicatorValue(fmt.Sprintf("ma%.0f", shortN), code, date)
		longMA, _ := data.GetIndicatorValue(fmt.Sprintf("ma%.0f", longN), code, date)
		if shortMA > 0 && longMA > 0 {
			return math.Min(1.0, math.Max(0.0, (shortMA/longMA-0.95)*10)) // 0.95→0, 1.05→1
		}
		return 0.3
	case "momentum_N":
		n := intParam(params, "N", 5)
		chg, _ := data.GetIndicatorValue(fmt.Sprintf("chg_%dd", n), code, date)
		// Normalize: -10%→0, +10%→1
		return math.Min(1.0, math.Max(0.0, (chg+10)/20))
	case "volume_rank":
		if total > 0 {
			return 1.0 - float64(scanned)/float64(total) // higher volume rank = higher score
		}
		return 0.5
	case "atr_pct":
		n := intParam(params, "N", 14)
		atrPct, _ := data.GetIndicatorValue(fmt.Sprintf("atr_%d_pct", n), code, date)
		if atrPct > 0 {
			return 1.0 - math.Min(1.0, atrPct/10) // lower volatility = higher score
		}
		return 0.5
	case "rsi_14":
		rsi, _ := data.GetIndicatorValue("rsi", code, date)
		if rsi > 0 {
			if rsi > 70 { return 0.2 }   // overbought
			if rsi < 30 { return 0.9 }   // oversold
			return (rsi - 30) / 40 * 0.7 + 0.3 // 30→0.3, 70→0.5
		}
		return 0.5
	case "alpha_score":
		score, _ := data.GetIndicatorValue("algo_score", code, date)
		return math.Min(1.0, math.Max(0.0, score/100))
	default:
		return 0.5
	}
}

// floatParam extracts a float64 parameter from JSONMap with fallback.
func floatParam(p model.JSONMap, key string, fallback float64) float64 {
	if p == nil { return fallback }
	if v, ok := p[key]; ok {
		switch vv := v.(type) {
		case float64: return vv
		case json.Number: if f, err := vv.Float64(); err == nil { return f }
		case int: return float64(vv)
		}
	}
	return fallback
}

// intParam extracts an int parameter from JSONMap with fallback.
func intParam(p model.JSONMap, key string, fallback int) int {
	if p == nil { return fallback }
	if v, ok := p[key]; ok {
		switch vv := v.(type) {
		case float64: return int(vv)
		case json.Number: if f, err := vv.Float64(); err == nil { return int(f) }
		case int: return vv
		}
	}
	return fallback
}

func (e *SignalEngine) EvalConds(conds []ConditionDef, code, date string, data DataProvider) bool {
	ok, _ := e.evalConds(conds, code, date, data)
	return ok
}

func (e *SignalEngine) evalConds(conds []ConditionDef, code, date string, data DataProvider) (bool, string) {
	if len(conds) == 0 {
		return false, "无买入条件"
	}
	groups := make(map[int][]ConditionDef)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}
	// Try each group (OR logic between groups, AND within group)
	var failReasons []string
	for gid, group := range groups {
		allMet := true
		for _, c := range group {
			val, ok := data.GetIndicatorValue(c.Indicator, code, date)
			if !ok {
				allMet = false
				failReasons = append(failReasons, fmt.Sprintf("[组%d] %s 无数据", gid, c.Indicator))
				break
			}
			if !checkOperator(val, c.Operator, c.Value) {
				allMet = false
				failReasons = append(failReasons, fmt.Sprintf("[组%d] %s=%.2f %s %.2f", gid, c.Indicator, val, e.opLabel(c.Operator), c.Value))
				break
			}
		}
		if allMet {
			return true, fmt.Sprintf("满足组%d全部条件", gid)
		}
	}
	return false, strings.Join(failReasons, "; ")
}

func (e *SignalEngine) opLabel(op string) string {
	switch op {
	case "gte": return "<"
	case "lte": return ">"
	case "gt": return "≤"
	case "lt": return "≥"
	case "eq": return "≠"
	case "cross_up": return "未上穿"
	case "cross_down": return "未下穿"
	default: return op
	}
}

// getIndustry returns the Shenwan L1 industry for a stock code.
func (e *SignalEngine) getIndustry(code string) string {
	var ind string
	db.PG.Raw("SELECT COALESCE(sw_l1, industry, '其他') FROM stocks_basic WHERE code = ?", code).Scan(&ind)
	if ind == "" { ind = "其他" }
	return ind
}

// ── PGDataProvider ──

// PGDataProvider implements DataProvider using PostgreSQL for live trading.
type PGDataProvider struct {
	dates   []string
	dateIdx map[string]int
}

// NewPGDataProvider creates a new PG-backed data provider.
func NewPGDataProvider(dates []string, dateIdx map[string]int) *PGDataProvider {
	return &PGDataProvider{dates: dates, dateIdx: dateIdx}
}

func (p *PGDataProvider) GetClose(code, date string) float64 {
	var v float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
	return v
}

func (p *PGDataProvider) GetOpen(code, date string) float64 {
	var v float64
	db.PG.Raw("SELECT COALESCE(open, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
	return v
}

func (p *PGDataProvider) GetDailyChange(code, date string) float64 {
	var v float64
	db.PG.Raw("SELECT COALESCE(change_pct, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
	return v
}

func (p *PGDataProvider) GetIndicatorValue(indicator, code, date string) (float64, bool) {
	// Use indicator cache or compute on the fly
	return GetIndicatorValue(indicator, code, date)
}

func (p *PGDataProvider) GetNextDate(date string) string {
	idx := p.dateIdx[date]
	if idx >= 0 && idx+1 < len(p.dates) {
		return p.dates[idx+1]
	}
	// Fallback: skip weekends
	return nextTradeDate(date)
}

func (p *PGDataProvider) Dates() []string { return p.dates }

// GetIndicatorValue computes indicator value from PG data.
func GetIndicatorValue(indicator, code, date string) (float64, bool) {
	switch indicator {
	case "MA5":
		var v float64
		db.PG.Raw(`SELECT AVG(close) FROM (
			SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?
			ORDER BY trade_date DESC LIMIT 5
		) t`, code, date).Scan(&v)
		return v, v > 0
	case "MA10":
		var v float64
		db.PG.Raw(`SELECT AVG(close) FROM (
			SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?
			ORDER BY trade_date DESC LIMIT 10
		) t`, code, date).Scan(&v)
		return v, v > 0
	case "MA20":
		var v float64
		db.PG.Raw(`SELECT AVG(close) FROM (
			SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?
			ORDER BY trade_date DESC LIMIT 20
		) t`, code, date).Scan(&v)
		return v, v > 0
	case "close", "CLOSE":
		return pGetClose(code, date), true
	case "volume", "VOLUME", "vol":
		var v float64
		db.PG.Raw("SELECT COALESCE(volume, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
		return v, true
	case "amount", "AMOUNT":
		var v float64
		db.PG.Raw("SELECT COALESCE(amount, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
		return v, true
	case "pct_chg", "PCT_CHG", "change":
		var v float64
		db.PG.Raw("SELECT COALESCE(change_pct, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
		return v, true
	case "turnover_rate", "TURNOVER":
		var v float64
		db.PG.Raw("SELECT COALESCE(turnover_rate, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
		return v, true
	case "streak_count":
		// Count consecutive days where close > prev close, working backward from date
		var v float64
		db.PG.Raw(`WITH prices AS (
			SELECT close FROM stocks_daily_k
			WHERE code = ? AND trade_date <= ?
			ORDER BY trade_date DESC LIMIT 21
		)
		SELECT COUNT(*) FROM (
			SELECT close, LAG(close) OVER (ORDER BY (SELECT 1)) as prev_close FROM prices
		) t WHERE close > prev_close AND prev_close > 0`, code, date).Scan(&v)
		return v, v > 0
	default:
		// Handle maN indicator (lowercase, e.g. ma5, ma20)
		if strings.HasPrefix(indicator, "ma") {
			nStr := strings.TrimPrefix(indicator, "ma")
			n := 5
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 { n = 5 }
			var v float64
			db.PG.Raw("SELECT COALESCE(AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN ? PRECEDING AND CURRENT ROW), 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", n-1, code, date).Scan(&v)
			return v, v > 0
		}
		// Handle chg_Nd indicator (e.g. chg_5d, chg_20d)
		if strings.HasPrefix(indicator, "chg_") {
			nStr := strings.TrimSuffix(strings.TrimPrefix(indicator, "chg_"), "d")
			n := 5
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 { n = 5 }
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1,
				0
			) * 100`, code, date, code, date, n-1).Scan(&v)
			return v, true
		}
		// Handle atr_N_pct indicator (e.g. atr_14_pct)
		if strings.HasPrefix(indicator, "atr_") && strings.HasSuffix(indicator, "_pct") {
			nStr := strings.TrimPrefix(strings.TrimSuffix(indicator, "_pct"), "atr_")
			n := 14
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 { n = 14 }
			var v float64
			// ATR% = ATR(N) / close * 100
			db.PG.Raw(`WITH tr AS (
				SELECT GREATEST(high-low, ABS(high-LAG(close,1) OVER (PARTITION BY code ORDER BY trade_date)), ABS(low-LAG(close,1) OVER (PARTITION BY code ORDER BY trade_date))) as tr_val, close
				FROM stocks_daily_k
				WHERE code = ? AND trade_date <= ?
				ORDER BY trade_date DESC LIMIT ?
			)
			SELECT COALESCE(AVG(tr_val) / NULLIF((SELECT close FROM tr LIMIT 1), 0) * 100, 0) FROM tr`, code, date, n).Scan(&v)
			return v, v > 0
		}
		// Handle RSI indicator
		if indicator == "rsi" || indicator == "RSI" {
			var v float64
			db.PG.Raw(`WITH diffs AS (
				SELECT close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as diff
				FROM stocks_daily_k
				WHERE code = ? AND trade_date <= ?
				ORDER BY trade_date DESC LIMIT 15
			)
			SELECT COALESCE(100 - 100 / (1 + NULLIF(SUM(CASE WHEN diff > 0 THEN diff ELSE 0 END) / NULLIF(ABS(SUM(CASE WHEN diff < 0 THEN diff ELSE 0 END)), 0), 0)), 50)
			FROM diffs`, code, date).Scan(&v)
			return v, v > 0
		}
		// Handle algo_score indicator
		if indicator == "algo_score" {
			var v float64
			db.PG.Raw("SELECT COALESCE(composite_score, 0) FROM ai_stock_scores WHERE code = ? AND trade_date = ? ORDER BY id DESC LIMIT 1", code, date).Scan(&v)
			return v, true
		}
		// Handle momentum_N indicator (e.g. momentum_5, momentum_20)
		if strings.HasPrefix(indicator, "momentum_") {
			nStr := strings.TrimPrefix(indicator, "momentum_")
			n := 5
			if parsed, err := fmt.Sscanf(nStr, "%d", &n); err != nil || parsed == 0 {
				n = 5
			}
			if n <= 0 { n = 5 }
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1,
				0
			) * 100`, code, date, code, date, n-1).Scan(&v)
			return v, true
		}
		log.Printf("[SignalEngine] unknown indicator: %s for %s on %s", indicator, code, date)
		return 0, false
	}
}

func pGetClose(code, date string) float64 {
	var v float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v)
	return v
}
