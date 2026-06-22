package handler

import (
	"fmt"
	"log"
)

// ═══════════════════════════════════════════════════════════════
// RiskManager — 风险管理器（具体 RiskHook 实现）
// ═══════════════════════════════════════════════════════════════

// MarketCircuitBreaker prevents all buys when market sentiment is extremely negative.
type MarketCircuitBreaker struct {
	compositeScore float64
	thresholdMin   float64
}

// NewMarketCircuitBreaker creates a market circuit breaker.
func NewMarketCircuitBreaker(compositeScore, thresholdMin float64) *MarketCircuitBreaker {
	return &MarketCircuitBreaker{
		compositeScore: compositeScore,
		thresholdMin:   thresholdMin,
	}
}

func (m *MarketCircuitBreaker) Name() string { return "MarketCircuitBreaker" }

func (m *MarketCircuitBreaker) BeforeAction(action ActionNode, a *DayAssessment) string {
	// Only block buy/add actions
	if action.Type != ActionBuy && action.Type != ActionAdd {
		return ""
	}

	if m.compositeScore < m.thresholdMin {
		reason := fmt.Sprintf("市场情绪极差(%.2f < %.2f)，禁止开仓/加仓",
			m.compositeScore, m.thresholdMin)
		log.Printf("[risk:circuit_breaker] %s", reason)
		return reason
	}
	return ""
}

// DailyLossLimit stops all trading when daily accumulated loss exceeds threshold.
type DailyLossLimit struct {
	dailyPnl float64 // running daily PnL
	maxLoss  float64 // max allowed daily loss (negative)
}

// NewDailyLossLimit creates a daily loss limit risk hook.
func NewDailyLossLimit(maxLoss float64) *DailyLossLimit {
	return &DailyLossLimit{
		dailyPnl: 0,
		maxLoss:  maxLoss,
	}
}

func (d *DailyLossLimit) Name() string { return "DailyLossLimit" }

func (d *DailyLossLimit) BeforeAction(action ActionNode, a *DayAssessment) string {
	if d.dailyPnl <= d.maxLoss {
		reason := fmt.Sprintf("当日累计亏损¥%.0f 已达上限(¥%.0f)，停止交易",
			d.dailyPnl, d.maxLoss)
		log.Printf("[risk:daily_loss] %s", reason)
		return reason
	}
	return ""
}

// AddPnl updates the running daily PnL.
func (d *DailyLossLimit) AddPnl(pnl float64) {
	d.dailyPnl += pnl
}

// PositionConcentration prevents over-concentration in a single stock.
type PositionConcentration struct {
	positions    map[string]*dcPosition
	maxSinglePct float64 // max single stock % of total equity
	totalEquity  float64
}

// NewPositionConcentration creates a position concentration risk hook.
func NewPositionConcentration(maxSinglePct float64) *PositionConcentration {
	return &PositionConcentration{
		maxSinglePct: maxSinglePct,
	}
}

func (p *PositionConcentration) Name() string { return "PositionConcentration" }

func (p *PositionConcentration) BeforeAction(action ActionNode, a *DayAssessment) string {
	// Only block buy/add actions
	if action.Type != ActionBuy && action.Type != ActionAdd {
		return ""
	}

	for _, target := range action.Targets {
		// Check if adding to this stock would exceed concentration limit
		currentMV := 0.0
		if pos, ok := p.positions[target.Code]; ok {
			currentMV = target.Price * float64(pos.Quantity)
		}
		budget := target.Price * float64(target.CurrentQty)
		newMV := currentMV + budget

		if a.TotalEquity > 0 && newMV/a.TotalEquity > p.maxSinglePct {
			reason := fmt.Sprintf("%s 单票仓位将达%.0f%%(上限%.0f%%)",
				target.Code, newMV/a.TotalEquity*100, p.maxSinglePct*100)
			log.Printf("[risk:concentration] %s", reason)
			return reason
		}
	}
	return ""
}

// MaxDrawdownProtection prevents further buys when drawdown exceeds threshold.
type MaxDrawdownProtection struct {
	peakEquity    float64
	currentEquity float64
	maxDrawdown   float64 // max allowed drawdown (negative, e.g., -0.15 for 15%)
}

// NewMaxDrawdownProtection creates a drawdown protection hook.
func NewMaxDrawdownProtection(maxDrawdown float64) *MaxDrawdownProtection {
	return &MaxDrawdownProtection{
		maxDrawdown: maxDrawdown,
	}
}

func (m *MaxDrawdownProtection) Name() string { return "MaxDrawdownProtection" }

func (m *MaxDrawdownProtection) BeforeAction(action ActionNode, a *DayAssessment) string {
	// Update peak
	if a.TotalEquity > m.peakEquity {
		m.peakEquity = a.TotalEquity
	}
	m.currentEquity = a.TotalEquity

	if m.peakEquity > 0 {
		drawdown := (m.currentEquity - m.peakEquity) / m.peakEquity
		if drawdown <= m.maxDrawdown && (action.Type == ActionBuy || action.Type == ActionAdd) {
			reason := fmt.Sprintf("回撤%.1f%% 已超上限(%.1f%%)，停止开仓",
				drawdown*100, m.maxDrawdown*-100)
			log.Printf("[risk:drawdown] %s", reason)
			return reason
		}
	}
	return ""
}

// init registers the risk manager for import
func init() {
	log.Printf("[risk_manager] registered")
}
