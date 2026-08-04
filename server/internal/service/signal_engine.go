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
	MaxHoldings                int
	MaxTotalBuyPct             float64 // 单日最大买入仓位 (占总资金%), 默认60
	BuyPositionPct             float64 // 单票买入仓位 (占总资金%)
	AddPositionPct             float64
	ReducePositionPct          float64
	StopLoss                   float64
	StopProfit                 float64
	CommissionRate             float64
	MinCommission              float64
	StampTaxRate               float64
	PositionConcentrationLimit float64
	ScoringConfig              model.ScoringConfig // 候选评分配置
	EnableTrailingStop         bool                // 启用移动止盈
	TrailingStopActivation     float64             // 激活阈值%
	TrailingStopDrawdown       float64             // 回撤比例%
}

// ── Position ──

type SignalPosition struct {
	AvailSellQty int // available sell quantity (excludes T+1 locked shares)
	Code         string
	Name         string
	Quantity     int
	BuyPrice     float64
	BuyDate      string
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
				PlannedAmount: closePrice * float64(pos.AvailSellQty),
				Status:        "pending",
				Reason:        fmt.Sprintf("止损触发 %.1f%% ≤ %.1f%%", chgPct, cfg.StopLoss),
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
			if drawdown <= 0 {
				drawdown = 8
			}
			peakPrice := closePrice // simplified: use close as peak for now
			stopPrice := peakPrice * (1 - drawdown/100)
			if closePrice <= stopPrice {
				signals = append(signals, SignalRecord{
					SignalDate: date, ExecDate: data.GetNextDate(date),
					StockCode: code, StockName: pos.Name,
					ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: closePrice * float64(pos.AvailSellQty),
					Status:        "pending",
					Reason:        fmt.Sprintf("移动止盈: 从峰值¥%.2f回撤%.0f%% → ¥%.2f", peakPrice, drawdown, stopPrice),
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
				PlannedAmount: closePrice * float64(pos.AvailSellQty),
				Status:        "pending",
				Reason:        fmt.Sprintf("止盈触发 %.1f%% ≥ %.1f%%", chgPct, cfg.StopProfit),
			})
			stopSet[code] = true
			logFn("🎯 STOP %s(%s) 止盈 %.1f%% ≥ %.1f%%", pos.Name, code, chgPct, cfg.StopProfit)
		}
	}

	stopCount := len(stopSet)
	if stopCount > 0 || len(positions) > 0 {
		logFn("🛡 止损/止盈检查: %d只持仓 → %d触发", len(positions), stopCount)
	}

	// ── Sell/Reduce checks ──
	logFn("🔍 Sell检查: %d条条件, %d只持仓", len(sellConds), len(positions))
	for code, pos := range positions {
		if stopSet[code] || pos.AvailSellQty <= 0 {
			logFn("  ⏭ %s(%s) 跳过 (stop=%v availSell=%d)", pos.Name, code, stopSet[code], pos.AvailSellQty)
			continue
		}

		if ok, reason := e.evalConds(sellConds, code, date, data); ok {
			closePrice := data.GetClose(code, date)
			if closePrice <= 0 {
				logFn("  ⚠ %s(%s): 卖出条件满足但收盘价=0，跳过", pos.Name, code)
				continue
			}
			signals = append(signals, SignalRecord{
				SignalDate: date, ExecDate: data.GetNextDate(date),
				StockCode: code, StockName: pos.Name,
				ActionType: "sell", PlannedPrice: closePrice, PlannedQty: pos.AvailSellQty,
				PlannedAmount: closePrice * float64(pos.AvailSellQty),
				Status:        "pending",
				Reason:        fmt.Sprintf("满足卖出条件: %s", reason),
			})
			logFn("📤 SELL %s(%s) %d股(可卖%d) @¥%.2f — %s", pos.Name, code, pos.AvailSellQty, pos.Quantity, closePrice, reason)
			continue
		} else if reason != "" {
			logFn("  ↳ %s(%s): %s", pos.Name, code, reason)
		} else {
			logFn("  ⚠ %s(%s): sellConds空或返回空reason (len=%d)", pos.Name, code, len(sellConds))
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
				Status:        "pending",
				Reason:        fmt.Sprintf("满足减仓条件 (%.0f%%)", cfg.ReducePositionPct),
			})
			logFn("📉 REDUCE %s(%s) %d股 @¥%.2f", pos.Name, code, reduceQty, closePrice)
		}
	}

	// Count sell/reduce signals generated so far
	sellSigCount := 0
	reduceSigCount := 0
	for _, s := range signals {
		if s.ActionType == "sell" || s.ActionType == "stop" {
			sellSigCount++
		}
		if s.ActionType == "reduce" {
			reduceSigCount++
		}
	}
	if sellSigCount == 0 && reduceSigCount == 0 && len(positions) > 0 {
		logFn("📤 卖出/减仓检查: %d只持仓 → 无触发（条件未满足或止损/止盈未到）", len(positions)-stopCount)
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
	if maxTotalBuy <= 0 {
		maxTotalBuy = 60
	}
	maxBuyCash := cash * maxTotalBuy / 100
	buyAmountPer := cash * cfg.BuyPositionPct / 100
	if buyAmountPer <= 0 {
		buyAmountPer = cash * 10 / 100
	}

	// Industry budget from strategy config (via budget or defaults)
	maxSingleIndustryPct := 100.0
	minIndustryCount := 1
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
		if totalStocks < 500 {
			progressStep = totalStocks
		}
		scanStart := time.Now()

		// ── Stage 1: Collect all candidates with scores ──
		type candidate struct {
			code       string
			name       string
			price      float64
			volumeRank float64 // 1 = highest volume
			chgPct     float64 // daily change %
			score      float64 // composite score
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
		logFn("🔍 扫描完成: %d/%d只 → 满足条件%d只 (%.0fs) | 持仓%d≥上限%d → 最多可买入%d只",
			scanned, totalStocks, len(candidates), scanElapsed, activeCount, cfg.MaxHoldings, slotCount)

		// ── Stage 2: Sort by score, industry-diversify, allocate ──
		if len(candidates) > 0 {
			sort.Slice(candidates, func(i, j int) bool {
				return candidates[i].score > candidates[j].score
			})

			// Get industry for each candidate
			type indInfo struct{ industry string }
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
					lot := BoardLotSize(c.code)
					qty := int(allocAmt/c.price/float64(lot)) * lot
					if qty < lot && allocAmt >= c.price*float64(lot) {
						qty = lot
					}
					if qty <= 0 {
						continue
					}
					amount := c.price * float64(qty)
					if totalAllocated+amount > maxBuyCash {
						continue
					}
					if industryAlloc[ind]+amount > maxBuyCash*maxSingleIndustryPct/100 {
						continue
					}

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
				lot := BoardLotSize(c.code)
				qty := int(allocAmt/c.price/float64(lot)) * lot
				if qty < lot && allocAmt >= c.price*float64(lot) {
					qty = lot
				}
				if qty <= 0 {
					continue
				}
				amount := c.price * float64(qty)
				if totalAllocated+amount > maxBuyCash {
					continue
				}
				if industryAlloc[ind]+amount > maxBuyCash*maxSingleIndustryPct/100 {
					continue
				}

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
				Status:        "pending",
				Reason:        "满足加仓条件",
			})
			logFn("➕ ADD   %s(%s) %d股 @¥%.2f", pos.Name, code, addQty, closePrice)
		}
	}

	// ── Summary log ──
	countByType := make(map[string]int)
	for _, s := range signals {
		countByType[s.ActionType]++
	}
	buyCnt := countByType["buy"]
	sellCnt := countByType["sell"] + countByType["stop"]
	addCnt := countByType["add"]
	reduceCnt := countByType["reduce"]
	logFn("📊 信号汇总: 买入%d 加仓%d 卖出%d 减仓%d", buyCnt, addCnt, sellCnt, reduceCnt)
	if buyCnt+sellCnt+addCnt+reduceCnt == 0 {
		if activeCount >= cfg.MaxHoldings {
			logFn("⚠ 信号为空原因: 持仓%d≥上限%d，买/加仓被拒绝；止损/卖出条件未触发", activeCount, cfg.MaxHoldings)
		} else if cash <= 0 {
			logFn("⚠ 信号为空原因: 可用资金为0")
		} else {
			logFn("⚠ 信号为空原因: 无候选满足买入条件或止损/卖出未触发")
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
		if dim.Weight <= 0 {
			continue
		}
		raw := e.computeDimensionScore(dim, code, date, scanned, total, data)
		// Normalize: direction asc (lower=better) → invert
		if dim.Direction == "asc" {
			raw = 1.0 - raw
		}
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
			if rsi > 70 {
				return 0.2
			} // overbought
			if rsi < 30 {
				return 0.9
			} // oversold
			return (rsi-30)/40*0.7 + 0.3 // 30→0.3, 70→0.5
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
	if p == nil {
		return fallback
	}
	if v, ok := p[key]; ok {
		switch vv := v.(type) {
		case float64:
			return vv
		case json.Number:
			if f, err := vv.Float64(); err == nil {
				return f
			}
		case int:
			return float64(vv)
		}
	}
	return fallback
}

// intParam extracts an int parameter from JSONMap with fallback.
func intParam(p model.JSONMap, key string, fallback int) int {
	if p == nil {
		return fallback
	}
	if v, ok := p[key]; ok {
		switch vv := v.(type) {
		case float64:
			return int(vv)
		case json.Number:
			if f, err := vv.Float64(); err == nil {
				return int(f)
			}
		case int:
			return vv
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
		return false, ""
	}
	groups := make(map[int][]ConditionDef)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}
	// Try each group (OR logic between groups, AND within group)
	var failReasons []string
	for gid, group := range groups {
		allMet := true
		var groupDetails []string
		for _, c := range group {
			val, ok := data.GetIndicatorValue(c.Indicator, code, date)
			if !ok {
				allMet = false
				detail := fmt.Sprintf("[组%d] %s 无数据", gid, c.Indicator)
				failReasons = append(failReasons, detail)
				groupDetails = append(groupDetails, detail)
				break
			}
			opLabel := e.operatorLabel(c.Operator)
			if !checkOperator(val, c.Operator, c.Value) {
				allMet = false
				detail := fmt.Sprintf("[组%d] %s=%.2f %s %.2f", gid, c.Indicator, val, e.opLabel(c.Operator), c.Value)
				failReasons = append(failReasons, detail)
				groupDetails = append(groupDetails, detail)
				break
			}
			groupDetails = append(groupDetails, fmt.Sprintf("%s=%.2f %s %.2f", c.Indicator, val, opLabel, c.Value))
		}
		if allMet {
			// Build detailed reason: e.g. "pe=525.51 > 35.00" or with group prefix
			reason := strings.Join(groupDetails, ", ")
			return true, reason
		}
	}
	return false, strings.Join(failReasons, "; ")
}

func (e *SignalEngine) operatorLabel(op string) string {
	switch op {
	case "gte":
		return ">="
	case "lte":
		return "<="
	case "gt":
		return ">"
	case "lt":
		return "<"
	case "eq":
		return "="
	case "cross_up":
		return "上穿"
	case "cross_down":
		return "下穿"
	}
	return op
}

func (e *SignalEngine) opLabel(op string) string {
	switch op {
	case "gte":
		return "<"
	case "lte":
		return ">"
	case "gt":
		return "≤"
	case "lt":
		return "≥"
	case "eq":
		return "≠"
	case "cross_up":
		return "未上穿"
	case "cross_down":
		return "未下穿"
	default:
		return op
	}
}

// getIndustry returns the Shenwan L1 industry for a stock code.
func (e *SignalEngine) getIndustry(code string) string {
	if db.PG == nil {
		return "其他"
	}
	var ind string
	db.PG.Raw("SELECT COALESCE(sw_l1, industry, '其他') FROM stocks_basic WHERE code = ?", code).Scan(&ind)
	if ind == "" {
		ind = "其他"
	}
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
	// Fallback: if no data for the exact date, use the latest available
	var v float64
	err := db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v).Error
	if err != nil || v == 0 {
		db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&v)
	}
	return v
}

func (p *PGDataProvider) GetOpen(code, date string) float64 {
	var v float64
	err := db.PG.Raw("SELECT COALESCE(open, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v).Error
	if err != nil || v == 0 {
		db.PG.Raw("SELECT COALESCE(open, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&v)
	}
	return v
}

func (p *PGDataProvider) GetDailyChange(code, date string) float64 {
	var v float64
	err := db.PG.Raw("SELECT COALESCE(change_pct, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v).Error
	if err != nil || v == 0 {
		db.PG.Raw("SELECT COALESCE(change_pct, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&v)
	}
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

// indicatorColumnMap maps UI indicator names to database column names.
// Keys are lowercase; if a name isn't here, it's tried as a direct column name.
var indicatorColumnMap = map[string]string{
	// ── stocks_daily_k aliases ──
	"daily_change":  "change_pct",
	"pct_chg":       "change_pct",
	"change":        "change_pct",
	"vol":           "volume",
	"turnover":      "turnover_rate",
	"amplitude":     "amplitude",
	"avg_price":     "avg_price",
	"pre_close":     "pre_close",
	"change_amount": "change_amount",
	"high_limit":    "high_limit",
	"low_limit":     "low_limit",
	"buy_vol":       "buy_vol",
	"sell_vol":      "sell_vol",
	"macd_dif":      "macd_dif",
	"macd_dea":      "macd_dea",
	"ema12":         "ema12",
	"ema26":         "ema26",
	"volume_ratio":  "volume_ratio",
	"turnover_rate": "turnover_rate",
	"close":         "close",
	"open":          "open",
	"high":          "high",
	"low":           "low",
	"volume":        "volume",
	"amount":        "amount",

	// ── stock_financials aliases ──
	"roe":            "roe",
	"eps":            "eps",
	"bps":            "bps",
	"net_profit":     "net_profit",
	"total_revenue":  "total_revenue",
	"revenue_growth": "revenue_growth",
	"profit_growth":  "profit_growth",
	"gross_margin":   "gross_margin",
	"net_margin":     "net_margin",
	"debt_ratio":     "debt_ratio",
	"total_assets":   "total_assets",
	"net_assets":     "net_assets",

	// ── Computed indicators ──
	"pe":           "__computed__",
	"pb":           "__computed__",
	"rsi":          "__computed__",
	"streak_count": "__computed__",
	"algo_score":   "__computed__",
}

// dailyKColumns are the data columns in stocks_daily_k that can be queried directly.
var dailyKColumns = map[string]bool{
	"open": true, "high": true, "low": true, "close": true,
	"volume": true, "amount": true, "turnover_rate": true,
	"ema12": true, "ema26": true, "macd_dif": true, "macd_dea": true,
	"high_limit": true, "low_limit": true, "avg_price": true,
	"buy_vol": true, "sell_vol": true, "change_pct": true,
	"amplitude": true, "volume_ratio": true, "pre_close": true,
	"change_amount": true, "is_paused": true,
}

// financialColumns are the data columns in stock_financials.
var financialColumns = map[string]bool{
	"total_revenue": true, "net_profit": true, "revenue_growth": true,
	"profit_growth": true, "total_assets": true, "total_liabilities": true,
	"net_assets": true, "roe": true, "eps": true, "bps": true,
	"gross_margin": true, "net_margin": true, "debt_ratio": true,
}

// GetIndicatorValue computes indicator value from PG data.
// Resolution order: alias map → stocks_daily_k column → stock_financials column → computed.
func GetIndicatorValue(indicator, code, date string) (float64, bool) {
	// 1. Resolve alias → column name (or "__computed__")
	col, hasAlias := indicatorColumnMap[indicator]
	if hasAlias && col == "__computed__" {
		return getComputedIndicator(indicator, code, date)
	}
	if !hasAlias {
		// Case-insensitive fallback in alias map
		lower := strings.ToLower(indicator)
		if col2, ok := indicatorColumnMap[lower]; ok {
			col = col2
			hasAlias = true
			if col == "__computed__" {
				return getComputedIndicator(lower, code, date)
			}
		} else {
			col = lower
		}
	}

	// 2. Check if this is a computed pattern (avoid noisy SQL errors)
	if isComputedPattern(indicator) {
		return getComputedIndicator(indicator, code, date)
	}

	// 3. Try stocks_daily_k direct column access
	if dailyKColumns[col] || (!hasAlias && col != "") {
		v, ok := queryDailyKColumn(col, code, date)
		if ok {
			return v, true
		}
	}

	// 3. Try stock_financials direct column access
	if financialColumns[col] || (!hasAlias && col != "") {
		v, ok := queryFinancialColumn(col, code, date)
		if ok {
			return v, true
		}
	}

	// 4. Computed indicators (pattern-based)
	return getComputedIndicator(indicator, code, date)
}

// queryDailyKColumn tries to get a value from stocks_daily_k by column name.
func queryDailyKColumn(col, code, date string) (float64, bool) {
	// Safety: only allow known columns to prevent SQL injection
	if !isSafeColumn(col) {
		return 0, false
	}
	var v float64
	// Try exact date first, fall back to latest available (for intraday/no-data scenarios)
	err := db.PG.Raw("SELECT COALESCE("+col+", 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&v).Error
	if err != nil || v == 0 {
		db.PG.Raw("SELECT COALESCE("+col+", 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&v)
	}
	return v, v > 0 || err == nil
}

// queryFinancialColumn tries to get latest value from stock_financials by column name.
func queryFinancialColumn(col, code, date string) (float64, bool) {
	if !isSafeColumn(col) {
		return 0, false
	}
	var v float64
	err := db.PG.Raw("SELECT COALESCE("+col+", 0) FROM stock_financials WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&v).Error
	return v, err == nil
}

// isSafeColumn returns true if col is a safe column name (alphanumeric + underscore only).
// isComputedPattern returns true if the indicator name matches a computed pattern
// and should skip direct column access.
func isComputedPattern(indicator string) bool {
	lower := strings.ToLower(indicator)
	// Exact computed indicators
	switch lower {
	case "pe", "pb", "rsi", "streak_count", "algo_score", "macd", "ma_cross", "boll_position", "volume_ma_ratio":
		return true
	}
	// Pattern-based computed indicators
	for _, prefix := range []string{"ma", "chg_", "momentum_", "drawdown_", "atr_"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func isSafeColumn(col string) bool {
	if len(col) == 0 || len(col) > 64 {
		return false
	}
	for _, r := range col {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// getComputedIndicator handles indicators that require computation.
func getComputedIndicator(indicator, code, date string) (float64, bool) {
	switch strings.ToLower(indicator) {
	// ── PE / PB ──
	case "pe":
		closePrice, eps := 0.0, 0.0
		db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&closePrice)
		db.PG.Raw("SELECT COALESCE(eps, 0) FROM stock_financials WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&eps)
		if eps > 0 {
			return closePrice / eps, true
		}
		return 0, false
	case "pb":
		closePrice, bps := 0.0, 0.0
		db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&closePrice)
		db.PG.Raw("SELECT COALESCE(bps, 0) FROM stock_financials WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&bps)
		if bps > 0 {
			return closePrice / bps, true
		}
		return 0, false

	// ── MA (maN) ──
	// Handled below in pattern matching

	// ── RSI ──
	case "rsi":
		var v float64
		db.PG.Raw(`WITH diffs AS (
			SELECT close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as diff
			FROM stocks_daily_k WHERE code = ? AND trade_date <= ?
			ORDER BY trade_date DESC LIMIT 15
		)
		SELECT COALESCE(100 - 100 / (1 + NULLIF(SUM(CASE WHEN diff > 0 THEN diff ELSE 0 END) / NULLIF(ABS(SUM(CASE WHEN diff < 0 THEN diff ELSE 0 END)), 0), 0)), 50)
		FROM diffs`, code, date).Scan(&v)
		return v, v > 0

	// ── streak_count ──
	case "streak_count":
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

	// ── algo_score ──
	case "algo_score":
		var v float64
		db.PG.Raw("SELECT COALESCE(composite_score, 0) FROM ai_stock_scores WHERE code = ? AND analyzed_at::date = ?::date ORDER BY id DESC LIMIT 1", code, date).Scan(&v)
		return v, true

	// ── MA cross (ma_cross) — handled by cross_up / cross_down operators ──
	case "ma_cross":
		return computeMACross(code, date)

	// ── MACD cross — handled by cross_up / cross_down operators ──
	case "macd":
		return computeMACDCross(code, date)

	default:
		// ── Pattern-based computed indicators ──

		// maN (e.g. ma5, ma20)
		if strings.HasPrefix(strings.ToLower(indicator), "ma") {
			nStr := strings.TrimPrefix(strings.ToLower(indicator), "ma")
			n := 5
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 {
				n = 5
			}
			var v float64
			db.PG.Raw("SELECT COALESCE(AVG(close) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN ? PRECEDING AND CURRENT ROW), 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1", n-1, code, date).Scan(&v)
			return v, v > 0
		}

		// chg_Nd (e.g. chg_5d, chg_20d)
		if strings.HasPrefix(strings.ToLower(indicator), "chg_") {
			nStr := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(indicator), "chg_"), "d")
			n := 5
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 {
				n = 5
			}
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1,
				0
			) * 100`, code, date, code, date, n-1).Scan(&v)
			return v, true
		}

		// atr_N_pct (e.g. atr_14_pct)
		if strings.HasPrefix(strings.ToLower(indicator), "atr_") && strings.HasSuffix(strings.ToLower(indicator), "_pct") {
			nStr := strings.TrimPrefix(strings.TrimSuffix(strings.ToLower(indicator), "_pct"), "atr_")
			n := 14
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 {
				n = 14
			}
			var v float64
			db.PG.Raw(`WITH tr AS (
				SELECT GREATEST(high-low, ABS(high-LAG(close,1) OVER (PARTITION BY code ORDER BY trade_date)), ABS(low-LAG(close,1) OVER (PARTITION BY code ORDER BY trade_date))) as tr_val, close
				FROM stocks_daily_k WHERE code = ? AND trade_date <= ?
				ORDER BY trade_date DESC LIMIT ?
			)
			SELECT COALESCE(AVG(tr_val) / NULLIF((SELECT close FROM tr LIMIT 1), 0) * 100, 0) FROM tr`, code, date, n).Scan(&v)
			return v, v > 0
		}

		// momentum_N (e.g. momentum_5, momentum_20)
		if strings.HasPrefix(strings.ToLower(indicator), "momentum_") {
			nStr := strings.TrimPrefix(strings.ToLower(indicator), "momentum_")
			n := 5
			if parsed, err := fmt.Sscanf(nStr, "%d", &n); err != nil || parsed == 0 {
				n = 5
			}
			if n <= 0 {
				n = 5
			}
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1,
				0
			) * 100`, code, date, code, date, n-1).Scan(&v)
			return v, true
		}

		// drawdown_N (e.g. drawdown_20)
		if strings.HasPrefix(strings.ToLower(indicator), "drawdown_") {
			nStr := strings.TrimPrefix(strings.ToLower(indicator), "drawdown_")
			n := 20
			fmt.Sscanf(nStr, "%d", &n)
			if n <= 0 {
				n = 20
			}
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT MAX(high) FROM (SELECT high FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT ?) t), 0) - 1,
				0
			) * 100`, code, date, code, date, n).Scan(&v)
			return v, true
		}

		// volume_ma_ratio (e.g. volume_ma_ratio with value N = lookback days)
		if strings.HasPrefix(strings.ToLower(indicator), "volume_ma_ratio") {
			// Use the Value field indirectly; default to 5 days
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 1) /
				NULLIF((SELECT AVG(volume) FROM (SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 5) t), 0),
				0
			)`, code, date, code, date).Scan(&v)
			return v, true
		}

		// boll_position: (close - MA20) / (2 * stddev)
		if strings.ToLower(indicator) == "boll_position" {
			var v float64
			db.PG.Raw(`SELECT COALESCE(
				(close - AVG(close) OVER w) / NULLIF(2 * STDDEV(close) OVER w, 0),
				0
			) FROM stocks_daily_k
			WHERE code = ? AND trade_date <= ?
			WINDOW w AS (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)
			ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&v)
			return v, true
		}

		log.Printf("[SignalEngine] unknown indicator: %s for %s on %s", indicator, code, date)
		return 0, false
	}
}

// computeMACross computes MA cross signal for ma_cross indicator.
// Returns positive for golden cross (cross_up), negative for death cross (cross_down).
func computeMACross(code, date string) (float64, bool) {
	// Default: 5-day MA crossing 20-day MA (value field stores the long period or 0 for default)
	// Return: >0 if MA5 crossed above MA20 today, <0 if crossed below, 0 if no cross
	var todayMA5, todayMA20, prevMA5, prevMA20 float64
	db.PG.Raw(`SELECT AVG(close) FROM (
		SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 5
	) t`, code, date).Scan(&todayMA5)
	db.PG.Raw(`SELECT AVG(close) FROM (
		SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 20
	) t`, code, date).Scan(&todayMA20)
	prevDate := previousTradeDate(date)
	db.PG.Raw(`SELECT AVG(close) FROM (
		SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 5
	) t`, code, prevDate).Scan(&prevMA5)
	db.PG.Raw(`SELECT AVG(close) FROM (
		SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ? ORDER BY trade_date DESC LIMIT 20
	) t`, code, prevDate).Scan(&prevMA20)

	if todayMA20 > 0 && prevMA20 > 0 {
		if prevMA5 <= prevMA20 && todayMA5 > todayMA20 {
			return 1, true // Golden cross
		}
		if prevMA5 >= prevMA20 && todayMA5 < todayMA20 {
			return -1, true // Death cross
		}
	}
	return 0, true // No cross
}

// computeMACDCross computes MACD cross signal.
// Returns positive for golden cross (dif crosses above dea), negative for death cross.
func computeMACDCross(code, date string) (float64, bool) {
	var row struct{ Dif, Dea float64 }
	db.PG.Raw("SELECT COALESCE(macd_dif,0) AS dif, COALESCE(macd_dea,0) AS dea FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&row)
	todayDif, todayDea := row.Dif, row.Dea
	prevDate := previousTradeDate(date)
	db.PG.Raw("SELECT COALESCE(macd_dif,0) AS dif, COALESCE(macd_dea,0) AS dea FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, prevDate).Scan(&row)
	prevDif, prevDea := row.Dif, row.Dea
	if prevDif <= prevDea && todayDif > todayDea {
		return 1, true
	}
	if prevDif >= prevDea && todayDif < todayDea {
		return -1, true
	}
	return 0, true
}
