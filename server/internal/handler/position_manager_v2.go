package handler

import (
	"fmt"
	"log"

	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
)

// ═══════════════════════════════════════════════════════════════
// PositionManagerV2 — 集成新引擎的仓位管理器
// ═══════════════════════════════════════════════════════════════
//
// 在原有 PositionManager 基础上集成：
// - ContextManager（市场情绪感知）
// - ScoringEngine（买入/加仓加权评分）
// - DecisionTreeEngine（卖出/减仓决策树）
// - AIAgent（AI 监督审查）
// - RiskManager（硬性风控规则）
//
// 兼容模式：当 orchestrationMode = "hybrid" 时，Buy/Add 走评分，Sell/Reduce 走决策树。
// 当 orchestrationMode = "scoring" 时，全部走评分。
// 当 orchestrationMode = "decision_tree" 时，全部走决策树。
// 未设置时（空字符串），回退到原有 AND/OR 逻辑。

// PositionManagerV2 wraps the original PositionManager with new engine capabilities.
type PositionManagerV2 struct {
	*PositionManager                                   // embed original
	orchCfg          *StrategyOrchestration
	strategy         *model.Strategy
	userID           uint
	aiSvc            *service.AIService
	scoringEngine    *ScoringEngine
	decisionTreeB    *DecisionTreeEngine // buy tree (when in decision_tree mode)
	decisionTreeS    *DecisionTreeEngine // sell tree
	decisionTreeR    *DecisionTreeEngine // reduce tree
	contextMgr       *ContextManager
	aiAgent          *AIAgent
	marketCtx        *MarketContext
	riskHooksV2      []RiskHook
}

// NewPositionManagerV2 creates the v2 position manager.
func NewPositionManagerV2(
	base *PositionManager,
	strategy *model.Strategy,
	userID uint,
	aiSvc *service.AIService,
	buyConds, sellConds, addConds, reduceConds []model.StrategyCondition,
	icache *IndicatorCache,
	kcache *KlineCache,
) *PositionManagerV2 {
	orch := &StrategyOrchestration{
		Mode:                 strategy.OrchestrationMode,
		EnableMarketContext:  strategy.EnableMarketContext,
		MarketCompositeMin:   strategy.MarketCompositeMin,
		MarketPositionBias:   strategy.MarketPositionBias,
		EnableAIAgent:        strategy.EnableAIAgent,
		EnableSectorRotation: strategy.EnableSectorRotation,
		IndustryFilter:       strategy.IndustryFilter,
	}
	if orch.Mode == "" {
		orch.Mode = "hybrid"
	}

	cm := NewContextManager(orch)
	// Default neutral context (will be updated per date)
	mktCtx := &MarketContext{Date: "", MarketBias: 1.0, RiskLevel: "low", TradeAllowed: true}

	pm := &PositionManagerV2{
		PositionManager: base,
		orchCfg:         orch,
		strategy:        strategy,
		userID:          userID,
		aiSvc:           aiSvc,
		contextMgr:      cm,
		marketCtx:       mktCtx,
	}

	// Build engines based on orchestration mode
	if orch.Mode == "scoring" || orch.Mode == "hybrid" {
		// Scoring engine for buy/add
		allBuyConds := append(append([]model.StrategyCondition{}, buyConds...), addConds...)
		pm.scoringEngine = NewScoringEngine(allBuyConds, mktCtx, icache, kcache)
	}

	if orch.Mode == "decision_tree" || orch.Mode == "hybrid" {
		// Decision tree engines for sell/reduce
		if len(sellConds) > 0 {
			pm.decisionTreeS = NewDecisionTreeEngine(sellConds, nil) // evalSingle set later
		}
		if len(reduceConds) > 0 {
			pm.decisionTreeR = NewDecisionTreeEngine(reduceConds, nil)
		}
	}
	if orch.Mode == "decision_tree" && len(buyConds) > 0 {
		pm.decisionTreeB = NewDecisionTreeEngine(buyConds, nil)
	}

	// Attach risk hooks
	pm.attachRiskHooks()

	return pm
}

// attachRiskHooks creates and attaches concrete risk hooks.
func (pm *PositionManagerV2) attachRiskHooks() {
	hooks := make([]RiskHook, 0)

	// Market circuit breaker
	if pm.orchCfg.EnableMarketContext && pm.orchCfg.MarketCompositeMin > -999 {
		hooks = append(hooks, NewMarketCircuitBreaker(pm.marketCtx.CompositeScore, pm.orchCfg.MarketCompositeMin))
	}

	// Position concentration (max 30% single stock default)
	hooks = append(hooks, NewPositionConcentration(0.30))

	// Store v2 hooks
	pm.riskHooksV2 = hooks

	// Attach to base PositionManager
	allHooks := append(pm.PositionManager.riskHooks, hooks...)
	pm.PositionManager.riskHooks = allHooks
}

// UpdateContext refreshes the market context for a new date.
func (pm *PositionManagerV2) UpdateContext(date string) error {
	ctx, err := pm.contextMgr.GetContext(date)
	if err != nil {
		return err
	}
	pm.marketCtx = ctx

	// Update scoring engine context
	if pm.scoringEngine != nil {
		pm.scoringEngine.ctx = ctx
	}

	// Rebuild AI agent
	if pm.orchCfg.EnableAIAgent && pm.aiSvc != nil {
		pm.aiAgent = NewAIAgent(pm.aiSvc, pm.userID, pm.strategy, ctx)
	}

	return nil
}

// SetEvalFunc sets the evalSingle function for decision tree engines.
func (pm *PositionManagerV2) SetEvalFunc(
	evalSingle func(model.StrategyCondition, string, string) bool,
	evalSingleWithValue func(model.StrategyCondition, string, string) (bool, float64),
	getPrice func(string, string) float64,
) {
	if pm.decisionTreeS != nil {
		pm.decisionTreeS.evalSingle = evalSingle
	}
	if pm.decisionTreeR != nil {
		pm.decisionTreeR.evalSingle = evalSingle
	}
	if pm.decisionTreeB != nil {
		pm.decisionTreeB.evalSingle = evalSingle
	}
}

// AssessV2 performs assessment using the new engines.
// Returns the assessment and any AI reasoning for logging.
func (pm *PositionManagerV2) AssessV2(
	cash float64,
	positions map[string]*dcPosition,
	universe []dcStockInfo,
	date string,
	buyConds, sellConds, addConds, reduceConds []model.StrategyCondition,
	evalConds func([]model.StrategyCondition, string, string) bool,
	getPrice func(string, string) float64,
	getName func(string) string,
) (*DayAssessment, string) {
	// Start with the original assessment (which handles stop triggers and position math)
	a := pm.PositionManager.Assess(cash, positions, universe, date,
		buyConds, sellConds, addConds, reduceConds, evalConds, getPrice)

	// ── V2: Scoring Engine for Buy/Add ──
	if pm.scoringEngine != nil && (pm.orchCfg.Mode == "scoring" || pm.orchCfg.Mode == "hybrid") {
		// Build scoring universe (stocks not already held)
		scoringUniverse := make([]dcStockInfo, 0)
		for _, s := range universe {
			if _, held := positions[s.Code]; !held {
				s.Name = getName(s.Code)
				scoringUniverse = append(scoringUniverse, s)
			}
		}

		if len(scoringUniverse) > 0 && len(buyConds) > 0 {
			results := pm.scoringEngine.ScoreAll(scoringUniverse, date, getPrice,
				nil, nil) // eval funcs set via SetEvalFunc
			minScore := pm.contextMgr.BiasForBuy(6.0) // default threshold
			topN := pm.scoringEngine.TopN(results, a.AvailableSlots, minScore)

			a.BuyCandidates = make([]ActionTarget, 0, len(topN))
			for _, r := range topN {
				a.BuyCandidates = append(a.BuyCandidates, ActionTarget{
					Code:   r.Code,
					Name:   r.Name,
					Price:  r.Price,
					Reason: fmt.Sprintf("评分%.1f分", r.TotalScore),
				})
			}

			log.Printf("[pm_v2] date=%s scoring_engine: universe=%d scored=%d topN=%d minScore=%.1f",
				date, len(scoringUniverse), len(results), len(topN), minScore)
		}
	}

	// ── V2: Decision Tree for Sell/Reduce ──
	if pm.decisionTreeS != nil && (pm.orchCfg.Mode == "decision_tree" || pm.orchCfg.Mode == "hybrid") {
		sellCandidates := make([]ActionTarget, 0)
		for code, pos := range positions {
			price := getPrice(code, date)
			if price <= 0 {
				continue
			}
			triggered, reason := pm.decisionTreeS.Evaluate(code, date)
			if triggered {
				sellCandidates = append(sellCandidates, ActionTarget{
					Code: code, Name: pos.Name, Price: price,
					CurrentQty: pos.Quantity, Reason: reason,
				})
			}
		}
		if len(sellCandidates) > 0 {
			a.SellCandidates = sellCandidates
			log.Printf("[pm_v2] date=%s decision_tree_sell: %d triggered", date, len(sellCandidates))
		}
	}

	if pm.decisionTreeR != nil && (pm.orchCfg.Mode == "decision_tree" || pm.orchCfg.Mode == "hybrid") {
		reduceCandidates := make([]ActionTarget, 0)
		for code, pos := range positions {
			price := getPrice(code, date)
			if price <= 0 {
				continue
			}
			triggered, reason := pm.decisionTreeR.Evaluate(code, date)
			if triggered {
				reduceCandidates = append(reduceCandidates, ActionTarget{
					Code: code, Name: pos.Name, Price: price,
					CurrentQty: pos.Quantity, CurrentMV: price * float64(pos.Quantity),
					Reason: reason,
				})
			}
		}
		if len(reduceCandidates) > 0 {
			a.ReduceCandidates = reduceCandidates
			log.Printf("[pm_v2] date=%s decision_tree_reduce: %d triggered", date, len(reduceCandidates))
		}
	}

	// ── V2: AI Agent Review ──
	aiReasoning := ""
	if pm.aiAgent != nil && pm.strategy.EnableAIAgent {
		// Convert buy candidates to ScoreResult for AI review
		buyScoreResults := make([]ScoreResult, 0, len(a.BuyCandidates))
		for _, t := range a.BuyCandidates {
			buyScoreResults = append(buyScoreResults, ScoreResult{
				Code:       t.Code,
				Name:       t.Name,
				Price:      t.Price,
				TotalScore: 7.0, // fallback score
			})
		}

		approved, blocked, reasoning, err := pm.aiAgent.Review(
			buyScoreResults,
			a.SellCandidates,
			positions,
			date,
		)
		if err != nil {
			log.Printf("[pm_v2] date=%s ai_agent error: %v", date, err)
		} else {
			// Apply AI review results
			approvedCodes := make(map[string]bool)
			for _, c := range approved {
				approvedCodes[c.Code] = true
			}

			// Filter buy candidates to only AI-approved
			filteredBuys := make([]ActionTarget, 0)
			for _, t := range a.BuyCandidates {
				if approvedCodes[t.Code] {
					filteredBuys = append(filteredBuys, t)
				}
			}
			a.BuyCandidates = filteredBuys

			aiReasoning = reasoning
			_ = blocked // blocked candidates already excluded
		}
	}

	return a, aiReasoning
}

// GetMarketContext returns the current market context.
func (pm *PositionManagerV2) GetMarketContext() *MarketContext {
	return pm.marketCtx
}

// ShouldUseV2 returns true if the strategy has opted into v2 orchestration.
func ShouldUseV2(s *model.Strategy) bool {
	return s.OrchestrationMode != "" && s.OrchestrationMode != "legacy"
}

// init registers the position manager v2 for import
func init() {
	log.Printf("[position_manager_v2] registered")
}
