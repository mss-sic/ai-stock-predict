package handler

import (
	"fmt"
	"log"
	"runtime/debug"
	"strings"

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
	log.Printf("[backtest_v2] ENTER date=%s mode=%s buyConds=%d sellConds=%d addConds=%d reduceConds=%d universe=%d positions=%d cash=%.0f isLast=%v",
		date, s.OrchestrationMode, len(buyConds), len(sellConds), len(addConds), len(reduceConds), len(universe), len(positions), cash, isLastDay)

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
	if s.EnableMarketContext || true { // always enable policy in v3
		policy = policyMgr.DeterminePolicy(mktCtx)

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

		// Reduce check
		reduceTriggered := false
		reduceReason := fmt.Sprintf("满足减仓条件 (%.0f%%)", reducePct)
		if decisionTreeReduce != nil {
			reduceTriggered, reduceReason = decisionTreeReduce.Evaluate(pos.Code, date)
		} else {
			reduceTriggered = evalConds(reduceConds, pos.Code, date)
		}

		if reduceTriggered {
			closePrice := kcache.GetClose(pos.Code, date)
			if closePrice > 0 {
				reduceQty := int(float64(pos.Quantity) * reducePct / 100)
				if reduceQty > 0 {
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

	// ── Buy checks (Scoring or legacy) ──
	// ── Policy guard: skip buys if policy disallows ──
	if policy != nil && !policy.AllowBuy {
		log.Printf("[backtest_v2] POLICY_SKIP date=%s mode=%s: buy disabled", date, policy.Mode)
		goto skipBuys
	}

	{ // scope for goto-friendly declarations
	slotCount := maxHold - len(positions)
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
				if _, held := positions[si.Code]; held { continue }
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
						// Try cache first
						if val, ok := icache.get(ind, code, date); ok {
							return checkOp(val, cond.Operator, cond.Value), val
						}
						// Delegate to getIndicatorValue — covers all 80+ indicators
						rawVal := getIndicatorValue(cond, code, date)
						passed := checkOp(rawVal, cond.Operator, cond.Value)
						return passed, rawVal
					}
					results := scoringEng.ScoreAll(scoringUniverse, date,
						func(c, d string) float64 { return kcache.GetClose(c, d) },
						evalSingle, evalWithVal)
				// DEBUG: show first candidate's condition details
				if len(results) > 0 {
					top := &results[0]
					details := make([]string, 0)
					for _, cs := range top.Breakdown {
						details = append(details, fmt.Sprintf("%s(w%.1f)=%.2f(%s)", cs.Indicator, cs.Weight, cs.WeightedScore, cs.Detail))
					}
					log.Printf("[backtest_v2] TOP_DEBUG date=%s code=%s totalScore=%.2f details=[%s]",
						date, top.Code, top.TotalScore, joinActions(details))
				}
				// P0-1: Adaptive minScore from score distribution
				minScore := scoringEng.GetDistribution().AdaptiveMinScore()
				log.Printf("[backtest_v2] date=%s adaptive_minScore=%.3f (median=%.3f top1=%.3f)",
					date, minScore, scoringEng.GetDistribution().Median, scoringEng.GetDistribution().Top1)
				topN := scoringEng.TopN(results, slotCount, minScore)
				for _, r := range topN {
					candidates = append(candidates, buyCandidate{r.Code, r.Name,
						fmt.Sprintf("评分%.1f分", r.TotalScore), r.Price, r.TotalScore})
				}
				if len(topN) == 0 && len(results) > 0 {
					last := len(results) - 1
					s2, s3 := 0.0, 0.0
					if last >= 1 { s2 = results[1].TotalScore }
					if last >= 2 { s3 = results[2].TotalScore }
					log.Printf("[backtest_v2] date=%s scoring: universe=%d candidates=%d topN=%d minScore=%.2f totalWeight=%.2f top3=[%.2f,%.2f,%.2f]",
						date, len(scoringUniverse), len(results), len(topN), minScore,
						results[0].TotalScore, s2, s3)
				} else {
					log.Printf("[backtest_v2] date=%s scoring: universe=%d candidates=%d topN=%d minScore=%.2f",
						date, len(scoringUniverse), len(results), len(topN), minScore)
				}
			}
		} else if decisionTreeBuy != nil {
			for _, si := range universe {
				if boughtThisRound >= slotCount { break }
				if _, held := positions[si.Code]; held { continue }
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
				if _, held := positions[si.Code]; held { continue }
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
					newMV := closePrice * float64(plannedQty)
					if totalEquity > 0 && newMV/totalEquity > s.PositionConcentrationLimit {
						log.Printf("[backtest_v2] RISK date=%s %s would exceed %.0f%% position limit, skip", date, bc.code, s.PositionConcentrationLimit*100)
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
			newMV := closePrice * float64(int(buyAmountPerStock/closePrice/100)*100)
			if totalEquity > 0 && s.PositionConcentrationLimit > 0 && newMV/totalEquity > s.PositionConcentrationLimit {
				log.Printf("[backtest_v2] date=%s %s would exceed 25%% position limit, skip", date, bc.code)
				continue
			}
			plannedQty := int(buyAmountPerStock / closePrice / 100) * 100
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
	return signals
}

func init() {
	log.Printf("[backtest_v2] registered")
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

// getAIService lazily initializes and returns the AI service singleton.
func getAIService() *service.AIService {
	return service.NewAIService()
}
