package handler

import (
	"fmt"
	"math"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
)

// ═══════════════════════════════════════════════════════════════
// Backtest V2 — 使用 ScoringEngine + DecisionTreeEngine 的回测信号生成
// ═══════════════════════════════════════════════════════════════

// isSTStock checks both name and stocks_basic.is_st flag for ST/suspended status.
func isSTStock(name string) bool {
	// Quick name check first (most ST stocks have ST in name)
	if strings.Contains(name, "ST") {
		return true
	}
	// For edge cases where ST flag is set but name hasn't been updated yet
	// The universe already filters by stocks_basic, but double-check here
	return false
}

// generateSignalsV2 replaces the original generateSignals for V2-enabled strategies.
// It uses ScoringEngine for buy/add and DecisionTreeEngine for sell/reduce.
func generateSignalsV2(
	date string, cash float64, isLastDay bool,
	positions map[string]*dcPosition,
	universe []dcStockInfo,
	task *model.BacktestTask,
	s *model.Strategy,
	buyConds, sellConds, addConds, reduceConds []model.StrategyCondition,
	evalSingle func(model.StrategyCondition, string, string) bool,
	kcache *KlineCache,
	icache *IndicatorCache,
	getNextDate func(*KlineCache, string) string,
	evalConds func([]model.StrategyCondition, string, string) bool,
) []model.BacktestSignal {
	enterTime := time.Now()
	log.Printf("[backtest_v2] ENTER date=%s mode=%s buyConds=%d sellConds=%d addConds=%d reduceConds=%d universe=%d positions=%d cash=%.0f isLast=%v",
		date, s.OrchestrationMode, len(buyConds), len(sellConds), len(addConds), len(reduceConds), len(universe), len(positions), cash, isLastDay)
	insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 10,
		"system", "info", "", "",
		fmt.Sprintf("▸ 市场扫描 | %d只股票 %d条买入条件 %d条卖出条件",
			len(universe), len(buyConds), len(sellConds)), nil)

	// P0-4: Panic recovery for backtest goroutine
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.Printf("[backtest_v2] PANIC recovered: %v\n%s", r, string(stack))
		}
	}()

	var signals []model.BacktestSignal

	if isLastDay {
		return signals
	}

	buyPct := s.BuyPositionPct
	if buyPct <= 0 { buyPct = 15 }
	addPct := s.AddPositionPct
	if addPct <= 0 { addPct = 10 }
	reducePct := s.ReducePositionPct
	if reducePct <= 0 { reducePct = 50 }

	maxHold := s.MaxHoldings
	if maxHold <= 0 { maxHold = 20 }

	// ── Build V2 engines ──
	var scoringEng *ScoringEngine
	var decisionTreeSell *DecisionTreeEngine
	var decisionTreeReduce *DecisionTreeEngine
	var decisionTreeBuy *DecisionTreeEngine
	var contextMgr *ContextManager
	var policy *Policy

	mode := s.OrchestrationMode
	if mode == "" { mode = "hybrid" }

	// Market context
	if s.EnableMarketContext {
		orchCfg := &StrategyOrchestration{
			Mode: mode, EnableMarketContext: true,
			MarketCompositeMin: s.MarketCompositeMin, MarketPositionBias: s.MarketPositionBias,
		}
		contextMgr = NewContextManager(orchCfg)
	}

	// ── Policy Manager v3 ──
	
	policyMgr := NewPolicyManager(s)
	mktCtx := &MarketContext{Date: date, MarketBias: 1.0, RiskLevel: "low", TradeAllowed: true}
	if contextMgr != nil {
		if ctx, err := contextMgr.GetContext(date); err == nil {
			mktCtx = ctx
		}
	}
	// Define human-readable mode name for logging (available throughout function)
	var modeCN string

	if s.EnableMarketContext { // policy controlled by strategy config
		policy = policyMgr.DeterminePolicy(mktCtx)

	// Set modeCN based on policy
	modeCN = map[string]string{"aggressive":"进攻","defensive":"防御","cash":"空仓","neutral":"中性"}[string(policy.Mode)]
	if modeCN == "" { modeCN = "未知" }

	// ── Apply regime parameter overrides ──
	if policy != nil && policy.RegimeParams != nil {
		if v, ok := policy.RegimeParams["buyPct"].(float64); ok && v > 0 { buyPct = v }
		if v, ok := policy.RegimeParams["addPct"].(float64); ok && v > 0 { addPct = v }
		if v, ok := policy.RegimeParams["reducePct"].(float64); ok && v > 0 { reducePct = v }
		if v, ok := policy.RegimeParams["stopProfit"].(float64); ok { s.StopProfit = v }
		if v, ok := policy.RegimeParams["stopLoss"].(float64); ok { s.StopLoss = v }
	}
		// Log policy decision
		log.Printf("[backtest_v2] POLICY date=%s mode=%s buy=%v add=%v bias=%.2f",
			date, policy.Mode, policy.AllowBuy, policy.AllowAdd, policy.PositionBias)
		buyStr := "允许买入"
		if !policy.AllowBuy { buyStr = "暂停买入" }
		addStr := ""
		if !policy.AllowAdd { addStr = " 禁止加仓" }
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 15,
			"system", "info", "", "",
			fmt.Sprintf("▸ 风控: %s | %s%s | %s",
				modeCN, buyStr, addStr, policy.Reason), nil)
		// Apply policy sell multiplier
		reducePct = reducePct * policy.SellPctMult
		if reducePct > 100 { reducePct = 100 }
	}

	// Scoring engine for buy/add
	if mode == "scoring" || mode == "hybrid" {
		allBuyConds := append(append([]model.StrategyCondition{}, buyConds...), addConds...)
		scoringEng = NewScoringEngine(allBuyConds, mktCtx, icache, kcache)
	}

	// Decision tree for sell/reduce/buy
	if mode == "decision_tree" || mode == "hybrid" {
		if len(sellConds) > 0 { decisionTreeSell = NewDecisionTreeEngine(sellConds, evalSingle) }
		if len(reduceConds) > 0 { decisionTreeReduce = NewDecisionTreeEngine(reduceConds, evalSingle) }
	}
	if mode == "decision_tree" && len(buyConds) > 0 {
		decisionTreeBuy = NewDecisionTreeEngine(buyConds, evalSingle)
	}

	// ── Stop-profit / Stop-loss checks ──
	for _, pos := range positions {
		if pos.Quantity <= 0 { continue }
		closePrice := kcache.GetClose(pos.Code, date)
		if closePrice <= 0 { continue }
		if pos.BuyDate == date { continue }

		chgPct := (closePrice - pos.BuyPrice) / pos.BuyPrice * 100

		if s.StopLoss < 0 && chgPct <= s.StopLoss {
			signals = append(signals, model.BacktestSignal{
				TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
				SignalDate: date, ExecDate: getNextDate(kcache, date),
				StockCode: pos.Code, StockName: pos.Name,
				ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
				PlannedAmount: closePrice * float64(pos.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止损触发 %.1f%% ≤ %.1f%%", chgPct, s.StopLoss),
			})
			continue
		}
		if s.StopProfit > 0 && chgPct >= s.StopProfit {
			signals = append(signals, model.BacktestSignal{
				TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
				SignalDate: date, ExecDate: getNextDate(kcache, date),
				StockCode: pos.Code, StockName: pos.Name,
				ActionType: "stop", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
				PlannedAmount: closePrice * float64(pos.Quantity),
				Status: "pending",
				Reason: fmt.Sprintf("止盈触发 %.1f%% ≥ %.1f%%", chgPct, s.StopProfit),
			})
		}
	}

	// ── P0-3: Score-decay sell model ──
	// Instead of instant sell on first trigger, accumulate "danger score"
	// Sell condition triggers add to danger; danger decays when conditions clear
	for _, pos := range positions {
		if pos.BuyDate == date { continue }

		sellTriggered := false
		sellReason := ""
		dangerScore := 0.0
		if decisionTreeSell != nil {
			triggered, reason := decisionTreeSell.Evaluate(pos.Code, date)
			if triggered { dangerScore = 1.0; sellReason = reason }
		} else {
			if evalConds(sellConds, pos.Code, date) {
				dangerScore = 1.0
				sellReason = "满足卖出条件"
			}
		}
		// Accumulate danger: consecutive triggers increase urgency
		pos.dangerDays++
		if dangerScore > 0 {
			pos.dangerAccum += dangerScore
		} else {
			pos.dangerAccum *= 0.5 // decay when conditions clear
			if pos.dangerAccum < 0.1 { pos.dangerDays = 0; pos.dangerAccum = 0 }
		}
		sellTriggered = pos.dangerAccum >= 1.5 // need sustained danger
		if sellTriggered {
			sellReason = fmt.Sprintf("%s (危险分%.1f/%dd)", sellReason, pos.dangerAccum, pos.dangerDays)
			closePrice := kcache.GetClose(pos.Code, date)
			if closePrice > 0 {
				signals = append(signals, model.BacktestSignal{
					TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
					SignalDate: date, ExecDate: getNextDate(kcache, date),
					StockCode: pos.Code, StockName: pos.Name,
					ActionType: "sell", PlannedPrice: closePrice, PlannedQty: pos.Quantity,
					PlannedAmount: closePrice * float64(pos.Quantity),
					Status: "pending", Reason: sellReason,
				})
			}
			continue // skip reduce if sell triggered
		}

		// Reduce check (with cooldown and minimum qty guard against fragmentation)
		reduceTriggered := false
		reduceReason := fmt.Sprintf("满足减仓条件 (%.0f%%)", reducePct)

		// Cooldown guard: skip if reduced within REDUCE_COOLDOWN_DAYS trading days
		if pos.LastReduceDate != "" && kcache.tradingDaysBetween(pos.LastReduceDate, date) <= REDUCE_COOLDOWN_DAYS {
			reduceTriggered = false
		} else if decisionTreeReduce != nil {
			reduceTriggered, reduceReason = decisionTreeReduce.Evaluate(pos.Code, date)
		} else {
			reduceTriggered = evalConds(reduceConds, pos.Code, date)
		}

		if reduceTriggered {
			closePrice := kcache.GetClose(pos.Code, date)
			if closePrice > 0 {
				reduceQty := int(float64(pos.Quantity) * reducePct / 100)
				// Fragmentation guard: if reduce drops below MIN_REDUCE_QTY, convert to full sell
				if reduceQty > 0 && pos.Quantity-reduceQty < MIN_REDUCE_QTY && pos.Quantity-reduceQty > 0 {
					// Convert reduce to full sell to avoid holding fractional scraps
					reduceQty = pos.Quantity
					signals = append(signals, model.BacktestSignal{
						TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
						SignalDate: date, ExecDate: getNextDate(kcache, date),
						StockCode: pos.Code, StockName: pos.Name,
						ActionType: "sell", PlannedPrice: closePrice, PlannedQty: reduceQty,
						PlannedAmount: closePrice * float64(reduceQty),
						Status: "pending", Reason: reduceReason + " (减仓会碎片化，转为清仓)",
					})
				} else if reduceQty >= MIN_REDUCE_QTY {
					signals = append(signals, model.BacktestSignal{
						TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
						SignalDate: date, ExecDate: getNextDate(kcache, date),
						StockCode: pos.Code, StockName: pos.Name,
						ActionType: "reduce", PlannedPrice: closePrice, PlannedQty: reduceQty,
						PlannedAmount: closePrice * float64(reduceQty),
						Status: "pending", Reason: reduceReason,
					})
				}
			}
		}
	}

	// ── Sell check summary ──
	sellSignalCount := 0
	for _, sig := range signals {
		if sig.ActionType == "sell" || sig.ActionType == "reduce" || sig.ActionType == "stop" {
			sellSignalCount++
		}
	}
	if len(positions) > 0 && sellSignalCount == 0 {
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 14,
			"system", "debug", "", "",
			fmt.Sprintf("▸ 持仓检查: %d只持仓 无卖出信号 (继续持有)", len(positions)), nil)
	} else if sellSignalCount > 0 {
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 14,
			"system", "info", "", "",
			fmt.Sprintf("▸ 持仓检查: %d只持仓 %d只触发卖出", len(positions), sellSignalCount), nil)
	}

	// ── Buy checks (Scoring or legacy) ──
	// ── Policy guard: skip buys if policy disallows ──
	if policy != nil && !policy.AllowBuy {
		log.Printf("[backtest_v2] POLICY_SKIP date=%s mode=%s: buy disabled", date, policy.Mode)
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 16,
			"system", "warn", "", "",
			fmt.Sprintf("▸ 跳过买入: %s模式 市场环境不满足开仓条件", modeCN), nil)
		goto skipBuys
	}

	{ // scope for goto-friendly declarations
	activePositions := 0
	for _, p := range positions {
		if p.Quantity > 0 { activePositions++ }
	}
	slotCount := maxHold - activePositions
	if slotCount > 0 && cash > 0 {
		// Apply policy position bias
		effectiveBuyPct := buyPct
		if policy != nil { effectiveBuyPct = buyPct * policy.PositionBias }
		buyAmountPerStock := cash * effectiveBuyPct / 100
		boughtThisRound := 0

		type buyCandidate struct {
			code, name, reason string
			price             float64
			score              float64
		}
		var candidates []buyCandidate

		if scoringEng != nil && len(buyConds) > 0 {
			// V2 Scoring Engine
			scoringUniverse := make([]dcStockInfo, 0)
			for _, si := range universe {
				if pos, held := positions[si.Code]; held && pos.Quantity > 0 { continue }
				// P0-2: Skip limit-up stocks (daily_change >= 9.8% ≈涨停)
				if chg := kcache.GetDailyChange(si.Code, date); chg >= 9.8 {
					continue
				}
				// Skip ST/suspended stocks (basic name check)
				if isSTStock(si.Name) {
					continue
				}
				scoringUniverse = append(scoringUniverse, dcStockInfo{Code: si.Code, Name: si.Name})
			}
			if len(scoringUniverse) > 0 {
				// evalSingleWithValue returns (passed, rawValue) for fuzzy scoring
					evalWithVal := func(cond model.StrategyCondition, code, date string) (bool, float64) {
						ind := cond.Indicator
						// Try cache first (preloaded indicators: rsi, volume_ratio, ma, etc.)
						if val, ok := icache.get(ind, code, date); ok {
							return checkOp(val, cond.Operator, cond.Value), val
						}
						// Fast path: momentum computed from kcache (avoids per-stock SQL)
						switch ind {
						case "daily_change":
							cur := kcache.GetClose(code, date)
							prev := getPrevClose(kcache, code, date)
							if prev > 0 {
								val := (cur - prev) / prev * 100
								return checkOp(val, cond.Operator, cond.Value), val
							}
							return false, 0
						case "momentum_5":
							cur := kcache.GetClose(code, date)
							prev := getCloseNDaysAgo(kcache, code, date, 5)
							if prev > 0 {
								val := (cur - prev) / prev * 100
								return checkOp(val, cond.Operator, cond.Value), val
							}
							return false, 0
						case "momentum_20":
							cur := kcache.GetClose(code, date)
							prev := getCloseNDaysAgo(kcache, code, date, 20)
							if prev > 0 {
								val := (cur - prev) / prev * 100
								return checkOp(val, cond.Operator, cond.Value), val
							}
							return false, 0
						case "ma_cross":
							// Fast path: compute MA cross from kcache close prices in-memory
							ma1 := int(cond.Value)
							ma2 := int(math.Round((cond.Value - float64(ma1)) * 1000))
							if ma1 < 1 { ma1 = 5 }
							if ma2 < 1 { ma2 = 20 }
							val := checkMACrossFast(kcache, code, date, ma1, ma2)
							return val != 0, val
	case "macd":
							// Fast path: compute MACD from kcache close prices in-memory
							val := checkMACDFast(kcache, code, date)
							return val != 0, val
						}
						// Slow path: SQL-based indicator (PE, PB, AI scores, etc.)
						rawVal := getIndicatorValue(cond, code, date)
						passed := checkOp(rawVal, cond.Operator, cond.Value)
						return passed, rawVal
					}
				scoreStart := time.Now()
					results := scoringEng.ScoreAll(scoringUniverse, date,
						func(c, d string) float64 { return kcache.GetClose(c, d) },
						evalSingle, evalWithVal,
						func(scored, total, candidates int) {
							// Update progress so frontend sees scoring is active
							updateProgressForScoring(task, date, scored, total, candidates)
						})
				scoreElapsed := time.Since(scoreStart)
				// ── Funnel: universe → scored → topN ──
				minScore := scoringEng.GetDistribution().AdaptiveMinScore()
				topN := scoringEng.TopN(results, slotCount, minScore)
				for _, r := range topN {
					candidates = append(candidates, buyCandidate{r.Code, r.Name,
						fmt.Sprintf("评分%.1f分", r.TotalScore), r.Price, r.TotalScore})
				}
				// Funnel log: show the narrowing pipeline
				funnelMsg := fmt.Sprintf("▸ 评分漏斗: %d只→%d只评分→入围%d只 (minScore=%.1f)",
					len(scoringUniverse), len(results), len(topN), minScore)
				if len(topN) > 0 {
					topNames := make([]string, 0, 3)
					for i := 0; i < len(topN) && i < 3; i++ {
						topNames = append(topNames, fmt.Sprintf("%s(%.1f)", topN[i].Code, topN[i].TotalScore))
					}
					funnelMsg += fmt.Sprintf(" | Top3: %s | ⏱%.1fs", joinActions(topNames), scoreElapsed.Seconds())
				} else if len(results) > 0 {
					funnelMsg += fmt.Sprintf(" | 最高%.1f分未达阈值 | ⏱%.1fs", results[0].TotalScore, scoreElapsed.Seconds())
				} else {
					funnelMsg += fmt.Sprintf(" | 无股票通过评分 | ⏱%.1fs", scoreElapsed.Seconds())
				}
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 17,
					"system", "info", "", "", funnelMsg, nil)
				log.Printf("[backtest_v2] date=%s funnel: universe=%d scored=%d topN=%d minScore=%.2f elapsed=%.1fs",
					date, len(scoringUniverse), len(results), len(topN), minScore, scoreElapsed.Seconds())
			}
		} else if decisionTreeBuy != nil {
			for _, si := range universe {
				if boughtThisRound >= slotCount { break }
				if pos, held := positions[si.Code]; held && pos.Quantity > 0 { continue }
				if kcache.GetDailyChange(si.Code, date) >= 9.8 { continue }
				if isSTStock(si.Name) { continue }
				triggered, reason := decisionTreeBuy.Evaluate(si.Code, date)
				if triggered {
					candidates = append(candidates, buyCandidate{si.Code, si.Name, reason, kcache.GetClose(si.Code, date), 0})
				}
			}
		} else {
			// Legacy evalConds
			for _, si := range universe {
				if boughtThisRound >= slotCount { break }
				if pos, held := positions[si.Code]; held && pos.Quantity > 0 { continue }
				if kcache.GetDailyChange(si.Code, date) >= 9.8 { continue }
				if isSTStock(si.Name) { continue }
				// Policy-aware buy logic
				buyLogic := "and"
				if policy != nil && policy.BuyLogic == "or" {
					buyLogic = "or"
				}
				passed := false
				if buyLogic == "or" {
					// OR logic: ANY condition in ANY group passes → trigger
					for _, c := range buyConds {
						if evalSingle(c, si.Code, date) {
							passed = true
							break
						}
					}
				} else {
					passed = evalConds(buyConds, si.Code, date)
				}
				if passed {
					candidates = append(candidates, buyCandidate{si.Code, si.Name, "满足买入条件", kcache.GetClose(si.Code, date), 0})
				}
			}
		}

		// ── Risk Management ──
		if len(candidates) > 0 {
			// Compute total equity
			totalEquity := cash
			for _, p := range positions {
				cp := kcache.GetClose(p.Code, date)
				if cp > 0 {
					totalEquity += cp * float64(p.Quantity)
				}
			}

			// 1. Market Circuit Breaker — block all buys if sentiment below threshold
			if mktCtx != nil && !mktCtx.TradeAllowed {
				log.Printf("[backtest_v2] RISK date=%s circuit_breaker: trade not allowed, skip all buys", date)
				candidates = nil
			}

			// 2. Daily Loss Limit — skip buys if daily loss exceeds configured limit
			if len(candidates) > 0 {
				maxDailyLossAmt := s.InitialCapital * s.MaxDailyLoss
				if s.MaxDailyLoss >= 0 {
					maxDailyLossAmt = s.InitialCapital * -0.05
				}
				dailyPnlCheck := totalEquity - s.InitialCapital
				if dailyPnlCheck < maxDailyLossAmt {
					log.Printf("[backtest_v2] RISK date=%s daily_loss=%.0f exceeds limit=%.0f, skip all buys", date, dailyPnlCheck, maxDailyLossAmt)
					candidates = nil
				}
			}

			// 3. Max Drawdown Protection — block buys when drawdown exceeds 20%%
			if len(candidates) > 0 && s.InitialCapital > 0 {
				drawdown := (s.InitialCapital - totalEquity) / s.InitialCapital
				if drawdown > 0.20 {
					log.Printf("[backtest_v2] RISK date=%s drawdown=%.1f%% exceeds 20%%, skip all buys", date, drawdown*100)
					candidates = nil
				}
			}

			// 4. Position Concentration — skip individual stocks exceeding limit
			if len(candidates) > 0 && s.PositionConcentrationLimit > 0 {
				filtered := make([]buyCandidate, 0, len(candidates))
				for _, bc := range candidates {
					closePrice := kcache.GetClose(bc.code, date)
					if closePrice <= 0 { continue }
					plannedQty := int(buyAmountPerStock / closePrice / 100) * 100
					if plannedQty < 100 {
						plannedQty = 100
					}
					newMV := closePrice * float64(plannedQty)
					if totalEquity > 0 && newMV/totalEquity > s.PositionConcentrationLimit {
						log.Printf("[backtest_v2] RISK date=%s %s would exceed %.0f%% position limit, skip", date, bc.code, s.PositionConcentrationLimit*100)
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 18,
					"signal", "warn", bc.code, bc.name,
					fmt.Sprintf("⏭ %s 超单票%.0f%%上限 跳过", bc.code, s.PositionConcentrationLimit*100), nil)
						continue
					}
					filtered = append(filtered, bc)
				}
				candidates = filtered
			}
		}
		// P1: AI Agent review of buy candidates (if enabled)
		if s.EnableAIAgent && len(candidates) > 0 {
			aiSvc := getAIService()
			if aiSvc != nil {
				mktCtx, _ := (&MarketContext{Date: date, MarketBias: 1.0, TradeAllowed: true}), error(nil)
				if contextMgr != nil {
					mktCtx, _ = contextMgr.GetContext(date)
				}
				agent := NewAIAgent(aiSvc, task.UserID, s, mktCtx)
				// Convert buy candidates to ScoreResult for AI review
				aiCandidates := make([]ScoreResult, len(candidates))
				for i, bc := range candidates {
					aiCandidates[i] = ScoreResult{Code: bc.code, Name: bc.name, Price: bc.price, TotalScore: bc.score}
				}
				approved, blocked, reasoning, err := agent.Review(aiCandidates, nil, positions, date)
				if err == nil && len(approved) > 0 {
					// Filter candidates to only AI-approved ones
					approvedMap := make(map[string]bool)
					for _, a := range approved { approvedMap[a.Code] = true }
					filtered := make([]buyCandidate, 0)
					for _, bc := range candidates {
						if approvedMap[bc.code] { filtered = append(filtered, bc) }
					}
					candidates = filtered
					log.Printf("[backtest_v2] AI_AGENT date=%s reviewed=%d approved=%d blocked=%d reason=%s",
						date, len(aiCandidates), len(approved), len(blocked), reasoning[:min(len(reasoning), 80)])
				} else if err != nil {
					log.Printf("[backtest_v2] AI_AGENT date=%s review error: %v (skip AI, use all)", date, err)
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 23,
					"system", "warn", "", "",
					fmt.Sprintf("AI审核失败: %v, 回退到全量信号", err), nil)
				}
			}
		}

		for _, bc := range candidates {
			if boughtThisRound >= slotCount { break }
			closePrice := kcache.GetClose(bc.code, date)
			if closePrice <= 0 { continue }

		// Compute total equity for position sizing
		totalEquity := cash
		for _, p := range positions {
			cp := kcache.GetClose(p.Code, date)
			if cp > 0 {
				totalEquity += cp * float64(p.Quantity)
			}
		}
			// P0-5: Position concentration — single stock max 25%% of total equity
			qtyCheck := int(buyAmountPerStock/closePrice/100) * 100
			if qtyCheck < 100 {
				qtyCheck = 100
			}
			newMV := closePrice * float64(qtyCheck)
			if totalEquity > 0 && s.PositionConcentrationLimit > 0 && newMV/totalEquity > s.PositionConcentrationLimit {
				log.Printf("[backtest_v2] date=%s %s would exceed 25%% position limit, skip", date, bc.code)
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 18,
				"signal", "warn", bc.code, bc.name,
				fmt.Sprintf("⏭ %s 超25%%上限 跳过", bc.code), nil)
				continue
			}
			plannedQty := int(buyAmountPerStock / closePrice / 100) * 100
					if plannedQty < 100 {
						plannedQty = 100
					}
			if plannedQty <= 0 { continue }
			signals = append(signals, model.BacktestSignal{
				TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
				SignalDate: date, ExecDate: getNextDate(kcache, date),
				StockCode: bc.code, StockName: bc.name,
				ActionType: "buy", PlannedPrice: closePrice, PlannedQty: plannedQty,
				PlannedAmount: closePrice * float64(plannedQty),
				Status: "pending", Reason: bc.reason,
			})
			boughtThisRound++
		}
	}

	} // end buy scope
skipBuys:
	// ── Add checks (Scoring or legacy) ──
	if policy != nil && !policy.AllowAdd {
		log.Printf("[backtest_v2] POLICY_SKIP date=%s mode=%s: add disabled", date, policy.Mode)
		goto skipAdds
	}
	for _, pos := range positions {
		addTriggered := false
		addReason := "满足加仓条件"

		if scoringEng != nil && len(addConds) > 0 {
			// Use scoring engine: score the stock; add if any condition passes
			// Policy-aware add logic
			addLogic := "and"
			if policy != nil && policy.AddLogic == "or" {
				addLogic = "or"
			}
			if addLogic == "or" {
				// OR logic: ANY condition passes → trigger
				for _, c := range addConds {
					if evalSingle(c, pos.Code, date) {
						addTriggered = true
						addReason = fmt.Sprintf("条件OR: %s", c.Indicator)
						break
					}
				}
			} else {
				addTriggered = evalConds(addConds, pos.Code, date)
			}
		} else if decisionTreeBuy != nil {
			addTriggered, addReason = decisionTreeBuy.Evaluate(pos.Code, date)
		} else {
			addTriggered = evalConds(addConds, pos.Code, date)
		}

		if addTriggered {
			log.Printf("[backtest_v2] ADD_TRIGGERED date=%s code=%s reason=%s", date, pos.Code, addReason)
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 55,
			"signal", "info", pos.Code, pos.Name,
			fmt.Sprintf("加仓触发: %s %s", pos.Code, addReason), nil)
			closePrice := kcache.GetClose(pos.Code, date)
			if closePrice <= 0 { continue }
			addAmount := cash * addPct / 100
			addQty := int(addAmount / closePrice / 100) * 100
			if addQty <= 0 { continue }
			signals = append(signals, model.BacktestSignal{
				TaskID: task.ID, StrategyID: task.StrategyID, UserID: task.UserID,
				SignalDate: date, ExecDate: getNextDate(kcache, date),
				StockCode: pos.Code, StockName: pos.Name,
				ActionType: "add", PlannedPrice: closePrice, PlannedQty: addQty,
				PlannedAmount: closePrice * float64(addQty),
				Status: "pending", Reason: addReason,
			})
		}
	}

skipAdds:
	// ── Timing diagnostic ──
	elapsed := time.Since(enterTime)
	if elapsed > 2*time.Second || len(signals) > 0 {
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 28,
			"system", "debug", "", "",
			fmt.Sprintf("⏱ 分析耗时 %.1fs | 生成%d个信号", elapsed.Seconds(), len(signals)), nil)
	}

	// ── No signals diagnostic ──
	if len(signals) == 0 && !isLastDay {
		// Build reason why no signals
		reasons := []string{}
		if policy != nil && !policy.AllowBuy { reasons = append(reasons, "买入被风控禁止") }
		if maxHold-len(positions) <= 0 { reasons = append(reasons, "已达最大持仓") }
		if cash <= 0 { reasons = append(reasons, "现金不足") }
		diagReason := "条件未触发"
		if len(reasons) > 0 { diagReason = strings.Join(reasons, "; ") }
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 30,
			"system", "debug", "", "",
			fmt.Sprintf("▸ 今日无信号: %s | %d只股票无一满足条件",
				diagReason, len(universe)), nil)
	}

	// Funnel summary
	if len(signals) > 0 {
		buyCount, sellCount := 0, 0
		for _, sig := range signals {
			switch sig.ActionType {
			case "buy", "add": buyCount++
			case "sell", "reduce", "stop": sellCount++
			}
		}
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 29,
			"system", "info", "", "",
			fmt.Sprintf("▸ 决策完成: %d只扫描 → 买入%d 卖出%d",
				len(universe), buyCount, sellCount), nil)
	}

	return signals
}

func init() {	log.Printf("[backtest_v2] registered")
}

// updateProgressForScoring updates the task phase during scoring to show live progress.
func updateProgressForScoring(task *model.BacktestTask, date string, scored, total, candidates int) {
	pct := 0.0
	if total > 0 { pct = float64(scored) / float64(total) * 100 }
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"phase":       fmt.Sprintf("%s — 评分中: %d/%d 候选%d", date, scored, total, candidates),
		"progress_pct": pct,
		"current_day":  task.CurrentDay,
	})
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

// getAIService lazily initializes and returns the AI service singleton.


// ═══════════════════════════════════════════════════════════════
// Fast-path in-memory indicator computation (no SQL)
// ═══════════════════════════════════════════════════════════════

// checkMACrossFast computes MA cross from cached kline data in-memory.
// Returns 1 for golden cross (ma1 > ma2), -1 for death cross, 0 otherwise.
func checkMACrossFast(kc *KlineCache, code, date string, ma1, ma2 int) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx < ma2 {
		return 0
	}
	closes, ok := kc.closeMap[code]
	if !ok || len(closes) == 0 {
		return 0
	}

	// Compute SMA for today and yesterday
	computeSMA := func(endIdx, period int) float64 {
		if endIdx < period-1 {
			return 0
		}
		sum := 0.0
		count := 0
		for i := endIdx - period + 1; i <= endIdx; i++ {
			if closes[i] > 0 {
				sum += closes[i]
				count++
			}
		}
		if count > 0 {
			return sum / float64(count)
		}
		return 0
	}

	curShort := computeSMA(idx, ma1)
	curLong := computeSMA(idx, ma2)
	prevShort := computeSMA(idx-1, ma1)
	prevLong := computeSMA(idx-1, ma2)

	if curShort <= 0 || curLong <= 0 || prevShort <= 0 || prevLong <= 0 {
		return 0
	}

	// Golden cross: short MA crosses above long MA
	if curShort > curLong && prevShort <= prevLong {
		return 1
	}
	// Death cross: short MA crosses below long MA
	if curShort < curLong && prevShort >= prevLong {
		return -1
	}
	return 0
}

// checkMACDFast computes MACD golden/death cross from cached kline data in-memory.
// Uses EMA(12), EMA(26), DIF=EMA12-EMA26, DEA=EMA(DIF,9).
// Returns 1 for golden cross (DIF crosses above DEA), -1 for death cross.
func checkMACDFast(kc *KlineCache, code, date string) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx < 26 {
		return 0
	}
	closes, ok := kc.closeMap[code]
	if !ok || len(closes) == 0 {
		return 0
	}

	// Compute EMAs iteratively from the beginning up to idx
	ema12, ema26 := 0.0, 0.0
	prevDIF, prevDEA := 0.0, 0.0
	dif, dea := 0.0, 0.0

	for i := 0; i <= idx; i++ {
		if closes[i] <= 0 {
			continue
		}
		if ema12 == 0 {
			ema12 = closes[i]
			ema26 = closes[i]
		} else {
			ema12 = 0.1538*closes[i] + 0.8462*ema12
			ema26 = 0.0741*closes[i] + 0.9259*ema26
		}

		prevDIF = dif
		prevDEA = dea
		dif = ema12 - ema26
		if dea == 0 {
			dea = dif
		} else {
			dea = 0.2*dif + 0.8*dea
		}
	}

	if dif == 0 || dea == 0 {
		return 0
	}

	// Golden cross: DIF crosses above DEA
	if dif > dea && prevDIF <= prevDEA {
		return 1
	}
	// Death cross: DIF crosses below DEA
	if dif < dea && prevDIF >= prevDEA {
		return -1
	}
	return 0
}

func getAIService() *service.AIService {
	return service.NewAIService()
}
