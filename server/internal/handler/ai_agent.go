package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
)

// ═══════════════════════════════════════════════════════════════
// AIAgent — AI 监督代理
// ═══════════════════════════════════════════════════════════════
//
// 在评分引擎产出候选后介入，做最终审查。
// AI 可调整权重、确认/否决候选，但硬性风控规则不可覆盖。

// AIAgentDecision represents the AI's review output for a single candidate.
type AIAgentAction struct {
	Code       string  `json:"code"`
	Action     string  `json:"action"` // buy / skip / sell_confirm / sell_reject
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// AIAgentReviewResult is the parsed AI review response.
type AIAgentReviewResult struct {
	MarketAssessment string          `json:"marketAssessment"`
	BiasAdjustment   float64         `json:"biasAdjustment"`
	Actions          []AIAgentAction `json:"actions"`
	Summary          string          `json:"summary"`
}

// AIAgent is the AI supervision agent.
type AIAgent struct {
	svc      *service.AIService
	userID   uint
	strategy *model.Strategy
	context  *MarketContext
}

// NewAIAgent creates an AI supervision agent.
func NewAIAgent(svc *service.AIService, userID uint, strategy *model.Strategy, ctx *MarketContext) *AIAgent {
	return &AIAgent{
		svc:      svc,
		userID:   userID,
		strategy: strategy,
		context:  ctx,
	}
}

// Review sends scoring candidates and decision tree sells to AI for final review.
// Returns the AI-reviewed candidates, a list of actions to block, and the reasoning text.
func (a *AIAgent) Review(
	buyCandidates []ScoreResult,
	sellCandidates []ActionTarget,
	positions map[string]*dcPosition,
	date string,
) (approvedBuy []ScoreResult, blockedBuy []ScoreResult, aiReasoning string, err error) {
	if !a.strategy.EnableAIAgent {
		// If AI agent is disabled, approve everything
		return buyCandidates, nil, "", nil
	}

	// Check if review scope covers this action type
	approvedBuy = make([]ScoreResult, 0)
	blockedBuy = make([]ScoreResult, 0)

	hasBuyCandidates := len(buyCandidates) > 0 &&
		(a.strategy.AIAgentReviewScope == "all" || a.strategy.AIAgentReviewScope == "buy_only")

	hasSellCandidates := len(sellCandidates) > 0 &&
		(a.strategy.AIAgentReviewScope == "all" || a.strategy.AIAgentReviewScope == "sell_only")

	if !hasBuyCandidates && !hasSellCandidates {
		return buyCandidates, nil, "", nil
	}

	// Build prompt
	prompt := a.buildReviewPrompt(buyCandidates, sellCandidates, positions, date)

	// Call AI
	reply, err := a.svc.ChatCompletionWithTokensModule(a.userID, prompt, nil, 4096, "ai_agent_review")
	if err != nil {
		log.Printf("[ai_agent] AI review failed for date=%s: %v, falling back to approve all", date, err)
		// Fallback: approve all candidates
		return buyCandidates, nil, fmt.Sprintf("AI审查失败(%v), 全部放行", err), nil
	}

	// Parse JSON response
	reply = cleanJSON(reply)
	var review AIAgentReviewResult
	if err := json.Unmarshal([]byte(reply), &review); err != nil {
		log.Printf("[ai_agent] AI response parse failed for date=%s: %v, raw=%s", date, err, truncate(reply, 300))
		return buyCandidates, nil, fmt.Sprintf("AI响应解析失败(%v), 全部放行", err), nil
	}

	aiReasoning = review.Summary
	if aiReasoning == "" {
		aiReasoning = review.MarketAssessment
	}

	// Apply AI decisions for buys
	actionMap := make(map[string]AIAgentAction)
	for _, act := range review.Actions {
		actionMap[act.Code] = act
	}

	// ── Hard Rules (AI cannot override) ──

	// Rule 1: Circuit breaker — if TradeAllowed is false, block all buys
	if !a.context.TradeAllowed {
		for _, c := range buyCandidates {
			blockedBuy = append(blockedBuy, c)
		}
		log.Printf("[ai_agent] date=%s rule=circuit_breaker blocked=%d buys", date, len(buyCandidates))
		// Record decision
		a.recordDecision(date, len(buyCandidates)+len(sellCandidates), 0, review, true)
		return nil, blockedBuy, "市场熔断，全部买入被否决。" + aiReasoning, nil
	}

	// Rule 2: ST stock filter — check stocks_basic for ST prefix or special treatment
	for _, c := range buyCandidates {
		// ST/退市/停牌 check
		if a.isBlacklisted(c.Code) {
			blockedBuy = append(blockedBuy, c)
			log.Printf("[ai_agent] date=%s rule=st_filter blocked=%s", date, c.Code)
			continue
		}

		// AI review
		act, found := actionMap[c.Code]
		if !found {
			// AI didn't mention this stock — approve by default
			approvedBuy = append(approvedBuy, c)
			continue
		}

		if act.Action == "skip" || act.Action == "sell_reject" {
			blockedBuy = append(blockedBuy, c)
		} else {
			// Adjust score based on confidence
			if act.Confidence > 0 {
				c.TotalScore *= act.Confidence
			}
			approvedBuy = append(approvedBuy, c)
		}
	}

	// Rule 3: Max daily trades — truncate if exceeds limit
	maxTrades := a.strategy.AIAgentMaxDailyTrades
	if maxTrades <= 0 {
		maxTrades = 5
	}
	if len(approvedBuy) > maxTrades {
		overflow := approvedBuy[maxTrades:]
		approvedBuy = approvedBuy[:maxTrades]
		blockedBuy = append(blockedBuy, overflow...)
		log.Printf("[ai_agent] date=%s rule=max_trades truncated=%d", date, len(overflow))
	}

	// Record decision
	a.recordDecision(date,
		len(buyCandidates)+len(sellCandidates),
		len(approvedBuy),
		review,
		len(blockedBuy) > 0)

	log.Printf("[ai_agent] date=%s approved=%d blocked=%d bias=%.2f",
		date, len(approvedBuy), len(blockedBuy), review.BiasAdjustment)

	return approvedBuy, blockedBuy, aiReasoning, nil
}

// buildReviewPrompt constructs the AI review prompt with full context.
func (a *AIAgent) buildReviewPrompt(
	buyCandidates []ScoreResult,
	sellCandidates []ActionTarget,
	positions map[string]*dcPosition,
	date string,
) string {
	var sb strings.Builder

	sb.WriteString("你是A股量化交易审查AI。请审查以下候选交易决策，基于市场环境和策略规则做出最终判断。\n\n")

	// Market context
	sb.WriteString("## 市场环境\n")
	sb.WriteString(fmt.Sprintf("- 日期: %s\n", date))
	sb.WriteString(fmt.Sprintf("- 市场情绪综合分: %.2f (-5~+5)\n", a.context.CompositeScore))
	sb.WriteString(fmt.Sprintf("- 风险偏好乘数: %.2f\n", a.context.MarketBias))
	sb.WriteString(fmt.Sprintf("- 风险等级: %s\n", a.context.RiskLevel))
	sb.WriteString(fmt.Sprintf("- 允许交易: %v\n", a.context.TradeAllowed))
	sb.WriteString(fmt.Sprintf("- 北向资金净流: %.1f亿\n", a.context.NorthboundFlow))
	if len(a.context.SectorLeaders) > 0 {
		sb.WriteString(fmt.Sprintf("- 强势行业: %v\n", a.context.SectorLeaders))
	}
	if len(a.context.SectorLaggards) > 0 {
		sb.WriteString(fmt.Sprintf("- 弱势行业: %v\n", a.context.SectorLaggards))
	}
	sb.WriteString("\n")

	// Current positions
	sb.WriteString("## 当前持仓\n")
	if len(positions) == 0 {
		sb.WriteString("（空仓）\n")
	} else {
		for _, pos := range positions {
			sb.WriteString(fmt.Sprintf("- %s(%s): 成本%.2f 持仓%d股\n",
				pos.Code, pos.Name, pos.BuyPrice, pos.Quantity))
		}
	}
	sb.WriteString("\n")

	// Buy candidates
	sb.WriteString("## 候选买入（评分引擎输出）\n")
	if len(buyCandidates) == 0 {
		sb.WriteString("（无候选）\n")
	} else {
		for _, c := range buyCandidates {
			sb.WriteString(fmt.Sprintf("- %s(%s): 价格%.2f 总分%.2f (%s)\n",
				c.Code, c.Name, c.Price, c.TotalScore, c.ScoreSummary()))
		}
	}
	sb.WriteString("\n")

	// Sell candidates
	sb.WriteString("## 候选卖出（决策树触发）\n")
	if len(sellCandidates) == 0 {
		sb.WriteString("（无候选）\n")
	} else {
		for _, c := range sellCandidates {
			sb.WriteString(fmt.Sprintf("- %s(%s): 价格%.2f 原因:%s\n",
				c.Code, c.Name, c.Price, c.Reason))
		}
	}
	sb.WriteString("\n")

	// Instructions
	sb.WriteString("## 审查要求\n")
	sb.WriteString("1. 分析市场环境是否适合开仓/加仓。\n")
	sb.WriteString("2. 审查每个候选买入：哪些值得执行(buy)、哪些应否决(skip)？给出置信度(0-1)。\n")
	sb.WriteString("3. 审查候选卖出：确认(sell_confirm)或否决(sell_reject)？\n")
	sb.WriteString("4. 返回纯JSON（无markdown代码块），格式：\n")
	sb.WriteString(`{
  "marketAssessment": "市场环境简述",
  "biasAdjustment": 1.0,
  "actions": [
    {"code": "000001", "action": "buy", "confidence": 0.85, "reason": "..."},
    {"code": "000002", "action": "skip", "confidence": 0.3, "reason": "..."}
  ],
  "summary": "总结建议"
}`)

	return sb.String()
}

// isBlacklisted checks if a stock is ST, suspended, or delisted.
func (a *AIAgent) isBlacklisted(code string) bool {
	var name string
	db.PG.Raw("SELECT COALESCE(name,'') FROM stocks_basic WHERE code = ? LIMIT 1", code).Scan(&name)
	if name == "" {
		return true // not found → exclude
	}
	if strings.Contains(name, "ST") || strings.Contains(name, "*ST") {
		return true
	}
	return false
}

// recordDecision persists the AI agent decision to the database.
func (a *AIAgent) recordDecision(
	date string,
	candidatesIn, candidatesOut int,
	review AIAgentReviewResult,
	overrides bool,
) {
	actions := make(model.JSONArray, 0, len(review.Actions))
	for _, act := range review.Actions {
		b, _ := json.Marshal(map[string]interface{}{
			"code":       act.Code,
			"action":     act.Action,
			"confidence": act.Confidence,
			"reason":     act.Reason,
		})
		actions = append(actions, string(b))
	}

	decision := model.AIAgentDecision{
		StrategyID:      a.strategy.ID,
		TradeDate:       date,
		MarketScore:     a.context.CompositeScore,
		MarketBias:      a.context.MarketBias,
		CandidatesIn:    candidatesIn,
		CandidatesOut:   candidatesOut,
		Reasoning:       review.Summary,
		Actions:         actions,
		OverridesApplied: overrides,
	}
	if err := db.PG.Create(&decision).Error; err != nil {
		log.Printf("[ai_agent] failed to record decision: %v", err)
	}
}

// init registers the AI agent for import
func init() {
	log.Printf("[ai_agent] registered")
}
