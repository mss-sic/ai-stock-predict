package handler

import (
	"fmt"
	"log"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// PolicyManager — 大盘→政策 决策层
// ═══════════════════════════════════════════════════════════════
//
// 在每个交易日，根据 MarketContext 选择策略模式：
//   🟢 aggressive  — 进攻：OR逻辑 + 允许加仓 + 高仓位
//   🟡 defensive   — 防御：AND逻辑 + 禁止加仓 + 低仓位
//   🔴 cash        — 空仓：仅平仓

// PolicyMode represents the tactical mode for a trading day.
type PolicyMode string

const (
	PolicyAggressive PolicyMode = "aggressive"
	PolicyDefensive  PolicyMode = "defensive"
	PolicyCash       PolicyMode = "cash"
)

// Policy holds the active policy parameters for a trading day.
type Policy struct {
	Mode               PolicyMode `json:"mode"`
	Reason             string     `json:"reason"`
	MinScoreMultiplier float64    `json:"minScoreMultiplier"` // multiply base minScore
	PositionBias       float64    `json:"positionBias"`       // multiply position sizing
	AllowAdd           bool       `json:"allowAdd"`           // allow add signals
	AllowBuy           bool       `json:"allowBuy"`           // allow buy signals
	ReducePctMult      float64    `json:"reducePctMult"`      // multiply reduce percentage
	SellPctMult        float64    `json:"sellPctMult"`        // multiply sell percentage (1.0 = normal, >1 = aggressive selling)
	BuyLogic           string     `json:"buyLogic"`           // "and" or "or" — override condition logic
	AddLogic           string     `json:"addLogic"`           // "and" or "or"
		// RegimeParams carries strategy-level overrides from JSONMap
		RegimeParams       map[string]interface{}  `json:"regimeParams,omitempty"`
}

// PolicyManager determines the trading policy based on market context.
type PolicyManager struct {
	strategy *model.Strategy
}

// NewPolicyManager creates a policy manager for a strategy.
func NewPolicyManager(s *model.Strategy) *PolicyManager {
	return &PolicyManager{strategy: s}
}

// DeterminePolicy selects the active policy for the given market context.
// Rule-based: uses composite_score thresholds.
// Can be extended with AI-driven policy selection.
func (pm *PolicyManager) DeterminePolicy(ctx *MarketContext) *Policy {
	// ── Default: neutral policy ──
	p := &Policy{
		Mode:               PolicyDefensive,
		Reason:             "默认防御模式",
		MinScoreMultiplier: 1.0,
		PositionBias:       1.0,
		AllowAdd:           false,
		AllowBuy:           true,
		ReducePctMult:      1.0,
		SellPctMult:        1.0,
		BuyLogic:           "and",
		AddLogic:           "and",
	}

	if pm.strategy == nil {
		return p
	}

	score := ctx.CompositeScore

	// ── Rule-based policy selection ──
	// Thresholds: aggressive >= 1.5, defensive >= 0, cash < 0
	aggrThr := pm.strategy.AggressiveThreshold
	if aggrThr == 0 {
		aggrThr = 1.5
	}
	defThr := pm.strategy.DefensiveThreshold
	if defThr == 0 {
		defThr = 0.0
	}

	switch {
	case score >= aggrThr:
		p.Mode = PolicyAggressive
		p.Reason = fmt.Sprintf("市场情绪积极(%.1f≥%.1f)，启用进攻模式", score, aggrThr)
		p.MinScoreMultiplier = 0.7   // lower bar to enter
		p.PositionBias = 1.2         // larger positions
		p.AllowAdd = true            // allow adding to winners
		p.BuyLogic = "or"            // any buy signal triggers
		p.AddLogic = "or"

	case score >= defThr:
		p.Mode = PolicyDefensive
		p.Reason = fmt.Sprintf("市场情绪中性(%.1f≥%.1f)，启用防御模式", score, defThr)
		p.MinScoreMultiplier = 1.0
		p.PositionBias = 0.8
		p.AllowAdd = false // no adding in defensive
		p.BuyLogic = "and" // all buy conditions must pass
		p.AddLogic = "and"

	default:
		p.Mode = PolicyCash
		p.Reason = fmt.Sprintf("市场情绪偏空(%.1f<%.1f)，启用空仓模式", score, defThr)
		p.MinScoreMultiplier = 99 // effectively block all buys
		p.PositionBias = 0.0
		p.AllowBuy = false  // no new buys
		p.AllowAdd = false
		p.SellPctMult = 1.5 // aggressive selling
	}

	// ── Circuit breaker override ──
	if !ctx.TradeAllowed {
		p.Mode = PolicyCash
		p.Reason = fmt.Sprintf("市场熔断(%.1f<%.1f)，强制空仓", score, pm.strategy.MarketCompositeMin)
		p.AllowBuy = false
		p.AllowAdd = false
	}

	// ── Load regime-specific parameters from Strategy JSONMap ──
	switch p.Mode {
	case PolicyAggressive:
		if pm.strategy.PolicyAggressive != nil {
			p.RegimeParams = pm.strategy.PolicyAggressive
		}
	case PolicyDefensive:
		if pm.strategy.PolicyDefensive != nil {
			p.RegimeParams = pm.strategy.PolicyDefensive
		}
	case PolicyCash:
		if pm.strategy.PolicyCash != nil {
			p.RegimeParams = pm.strategy.PolicyCash
		}
	}
	// Apply RegimeParams overrides to policy fields
	if p.RegimeParams != nil {
		if v, ok := p.RegimeParams["buyPct"].(float64); ok && v > 0 { p.PositionBias = v / 15.0 }
		if v, ok := p.RegimeParams["addPct"].(float64); ok { 
			if v > 0 { p.AllowAdd = true } else { p.AllowAdd = false }
		}
		if v, ok := p.RegimeParams["allowAdd"].(bool); ok { p.AllowAdd = v }
		if v, ok := p.RegimeParams["buyLogic"].(string); ok { p.BuyLogic = v }
		if v, ok := p.RegimeParams["addLogic"].(string); ok { p.AddLogic = v }
		if v, ok := p.RegimeParams["reducePct"].(float64); ok && v > 0 { p.ReducePctMult = v / 50.0 }
	}

	// ── Risk level override ──
	if ctx.RiskLevel == "extreme" && p.Mode != PolicyCash {
		p.PositionBias *= 0.7
		p.AllowAdd = false
		p.BuyLogic = "and"
	}

	log.Printf("[policy_manager] date=%s score=%.2f mode=%s buy=%v add=%v bias=%.2f reason=%s",
		ctx.Date, score, p.Mode, p.AllowBuy, p.AllowAdd, p.PositionBias, p.Reason)

	return p
}
