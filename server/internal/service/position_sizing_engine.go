package service

import (
	"fmt"
	"log"
	"math"

	"github.com/ai-stock-predict/server/internal/model"
)

// ── MarketRegime (风控强度) ──
// Strategy-level market regime derived from composite score vs thresholds.
type StrategyRegime string

const (
	RegimeAggressive StrategyRegime = "aggressive" // 进攻模式
	RegimeDefensive  StrategyRegime = "defensive"  // 防御模式
	RegimeCash       StrategyRegime = "cash"       // 空仓模式
)

var StrategyRegimeNames = map[StrategyRegime]string{
	RegimeAggressive: "🚀 进攻模式",
	RegimeDefensive:  "🛡 防御模式",
	RegimeCash:       "💤 空仓模式",
}

// ── PositionBudget ──

// PositionBudget represents the calculated position sizing limits for a trading day.
type PositionBudget struct {
	TotalPositionPct    float64        // 总仓位上限 (占资金 %)
	DailyBuyPct         float64        // 单日买入上限 (占资金 %)
	MaxBuyCash          float64        // 单日买入金额上限
	SinglePositionLimit float64        // 单票最大仓位 (占资金 %, 0=无限制)
	DailyLossLimit      float64        // 单日最大亏损 (负数, 0=无限制)
	Reason              string         // 决策理由
	Regime              StrategyRegime `json:"regime"`           // 当前风控强度
	RegimeReason        string         `json:"regimeReason"`     // 风控强度判定理由
	CompositeScore      float64        `json:"compositeScore"`   // 市场综合评分
	PositionBias        float64        `json:"positionBias"`     // 仓位乘数
	MaxSingleIndustry   float64        `json:"maxSingleIndustry"` // 单行业最大仓位 %
	MinIndustryCount    int            `json:"minIndustryCount"`  // 最少行业数
}

// ── PositionSizingEngine ──

// PositionSizingEngine calculates dynamic position sizing based on market regime,
// risk alerts, and account maturity.
type PositionSizingEngine struct {
	marketStyleSvc *MarketStyleService
}

// NewPositionSizingEngine creates a new position sizing engine.
func NewPositionSizingEngine() *PositionSizingEngine {
	return &PositionSizingEngine{
		marketStyleSvc: NewMarketStyleService(),
	}
}

// Calculate determines the position budget for a given trading day.
func (e *PositionSizingEngine) Calculate(
	date string,
	cash float64,
	runDays int,
	cumulativePnl float64,
	riskAlertCount int,
) PositionBudget {
	budget := PositionBudget{}

	// ── Layer 1: Macro Regime → Total Position Cap ──
	style := e.marketStyleSvc.DetectStyle(date)
	regime := e.marketStyleSvc.detectRegime(date)
	fearGreed := e.computeFearGreed(date)

	totalPct := e.macroPositionCap(style, regime, fearGreed)
	budget.TotalPositionPct = totalPct
	budget.SinglePositionLimit = math.Min(25, totalPct/3) // default: max 25%, or 1/3 of total

	// ── Layer 2: Risk Adjustment ──
	riskMultiplier := e.riskAdjustment(riskAlertCount)
	totalPct = totalPct * riskMultiplier
	budget.TotalPositionPct = totalPct

	// ── Layer 3: Gradual Position Building ──
	dailyPct := e.gradualBuildLimit(runDays, cumulativePnl, totalPct)
	budget.DailyBuyPct = dailyPct
	budget.MaxBuyCash = cash * dailyPct / 100
	budget.DailyLossLimit = -5.0 // default: -5% daily loss cap

	// Build reason
	budget.Reason = e.buildReason(style, fearGreed, runDays, cumulativePnl, riskAlertCount, totalPct, dailyPct)

	log.Printf("[position-sizing] %s: style=%s regime=%s fg=%.0f risk=%d → total=%.0f%% daily=%.0f%% (¥%.0f)",
		date, style, regime, fearGreed, riskAlertCount, totalPct, dailyPct, budget.MaxBuyCash)

	return budget
}

// CalculateWithStrategy determines the position budget using strategy-specific overrides,
// including dynamic regime detection based on AggressiveThreshold / DefensiveThreshold.
func (e *PositionSizingEngine) CalculateWithStrategy(
	date string,
	cash float64,
	runDays int,
	cumulativePnl float64,
	riskAlertCount int,
	strategy *model.Strategy,
) PositionBudget {
	budget := e.Calculate(date, cash, runDays, cumulativePnl, riskAlertCount)

	// ── Layer 0: Strategy Regime Detection (风控强度) ──
	aggrThr := strategy.AggressiveThreshold
	if aggrThr == 0 {
		aggrThr = 1.5
	}
	defThr := strategy.DefensiveThreshold
	marketBias := strategy.MarketPositionBias
	if marketBias == 0 {
		marketBias = 1.0
	}

	// Get market composite score for the date
	compositeScore := e.getCompositeScore(date)
	budget.CompositeScore = compositeScore

	// Determine strategy regime
	var regime StrategyRegime
	var regimeReason string
	switch {
	case compositeScore >= aggrThr:
		regime = RegimeAggressive
		regimeReason = fmt.Sprintf("市场综合分%.1f ≥ 进攻阈值%.1f，进攻模式", compositeScore, aggrThr)
	case compositeScore >= defThr:
		regime = RegimeDefensive
		regimeReason = fmt.Sprintf("市场综合分%.1f ≥ 防御阈值%.1f，防御模式", compositeScore, defThr)
	default:
		regime = RegimeCash
		regimeReason = fmt.Sprintf("市场综合分%.1f < 防御阈值%.1f，强制空仓", compositeScore, defThr)
	}

	// Circuit breaker: MarketCompositeMin
	if strategy.MarketCompositeMin > -999 && compositeScore < strategy.MarketCompositeMin {
		regime = RegimeCash
		regimeReason = fmt.Sprintf("熔断: 综合分%.1f < 最低%.1f，禁止开仓", compositeScore, strategy.MarketCompositeMin)
	}

	// ── Apply regime-specific position bias ──
	var positionBias float64
	switch regime {
	case RegimeAggressive:
		positionBias = marketBias * 1.2
		if strategy.PolicyAggressive != nil {
			if v, ok := strategy.PolicyAggressive["buyPct"].(float64); ok && v > 0 {
				budget.DailyBuyPct = math.Min(v, budget.DailyBuyPct*1.5)
			}
		}
	case RegimeDefensive:
		positionBias = marketBias * 0.8
		if strategy.PolicyDefensive != nil {
			if v, ok := strategy.PolicyDefensive["buyPct"].(float64); ok && v > 0 {
				budget.DailyBuyPct = math.Min(v, budget.DailyBuyPct*0.7)
			}
		}
	case RegimeCash:
		positionBias = 0.0
	}

	budget.Regime = regime
	budget.RegimeReason = regimeReason
	budget.PositionBias = positionBias

	// ── Apply position bias to budget ──
	if regime != RegimeCash {
		budget.TotalPositionPct = budget.TotalPositionPct * positionBias
		if strategy.MaxTotalPosition > 0 {
			budget.TotalPositionPct = math.Min(budget.TotalPositionPct, strategy.MaxTotalPosition)
		}
		budget.DailyBuyPct = math.Min(budget.DailyBuyPct*positionBias, budget.TotalPositionPct)
		budget.MaxBuyCash = cash * budget.DailyBuyPct / 100
	} else {
		budget.TotalPositionPct = 0
		budget.DailyBuyPct = 0
		budget.MaxBuyCash = 0
		budget.SinglePositionLimit = 0
	}

	// Strategy overrides (0 = use system default)
	if strategy.PositionConcentrationLimit > 0 && budget.SinglePositionLimit > 0 {
		budget.SinglePositionLimit = strategy.PositionConcentrationLimit * 100
	}
	if strategy.MaxDailyLoss < 0 {
		// Normalize: if value is <= -1 (e.g. -5), it's already in percentage, convert to budget format
		// If value is -1 < x < 0 (e.g. -0.05), it's decimal, convert to percentage
		if strategy.MaxDailyLoss <= -1 {
			budget.DailyLossLimit = strategy.MaxDailyLoss // already in % (e.g., -5 = -5%)
		} else {
			budget.DailyLossLimit = strategy.MaxDailyLoss * 100 // convert decimal to % (e.g., -0.05 → -5%)
		}
	}
	// Industry diversification overrides
	budget.MaxSingleIndustry = strategy.MaxSingleIndustry
	if budget.MaxSingleIndustry <= 0 { budget.MaxSingleIndustry = 30 }
	budget.MinIndustryCount = strategy.MinIndustryCount
	if budget.MinIndustryCount <= 0 { budget.MinIndustryCount = 3 }

	if !strategy.EnableDynamicSizing {
		if strategy.MaxTotalPosition > 0 {
			budget.TotalPositionPct = strategy.MaxTotalPosition
		}
		if strategy.DailyBuyLimit > 0 {
			budget.DailyBuyPct = strategy.DailyBuyLimit
		} else {
			budget.DailyBuyPct = budget.TotalPositionPct
		}
		budget.MaxBuyCash = cash * budget.DailyBuyPct / 100
		budget.Reason = fmt.Sprintf("%s | 固定仓位: 总≤%.0f%% 单日≤%.0f%% 单票≤%.0f%%", regimeReason, budget.TotalPositionPct, budget.DailyBuyPct, budget.SinglePositionLimit)
	} else {
		if strategy.MaxTotalPosition > 0 && strategy.MaxTotalPosition < budget.TotalPositionPct {
			budget.TotalPositionPct = strategy.MaxTotalPosition
		}
		if strategy.DailyBuyLimit > 0 && strategy.DailyBuyLimit < budget.DailyBuyPct {
			budget.DailyBuyPct = strategy.DailyBuyLimit
			budget.MaxBuyCash = cash * budget.DailyBuyPct / 100
		}
		budget.Reason = fmt.Sprintf("%s | 动态仓位: %s | 总≤%.0f%% 单日≤%.0f%% 单票≤%.0f%%", regimeReason, budget.Reason, budget.TotalPositionPct, budget.DailyBuyPct, budget.SinglePositionLimit)
	}

	log.Printf("[position-sizing] %s: strategy=%s regime=%s composite=%.1f bias=%.2f total=%.0f%% daily=%.0f%% (¥%.0f)",
		date, strategy.Name, regime, compositeScore, positionBias, budget.TotalPositionPct, budget.DailyBuyPct, budget.MaxBuyCash)

	return budget
}

// ── Layer 1: Macro Position Cap ──

func (e *PositionSizingEngine) macroPositionCap(style MarketStyle, regime MarketRegime, fearGreed float64) float64 {
	switch regime {
	case RegimeExpansion:
		if fearGreed >= 60 {
			return 80
		}
		if fearGreed >= 30 {
			return 70
		}
		return 60

	case RegimeNeutral:
		return 50

	case RegimeContraction:
		if fearGreed < 30 {
			return 20
		}
		return 30
	}

	// Fallback: use style
	switch style {
	case StyleBroadRally, StyleTrendUp:
		return 70
	case StyleStructural:
		return 60
	case StyleChoppy:
		return 50
	case StyleWeakRange:
		return 30
	case StyleDecline, StyleCrash:
		return 20
	default:
		return 30
	}
}

// ── Layer 2: Risk Adjustment ──

func (e *PositionSizingEngine) riskAdjustment(highPriorityAlerts int) float64 {
	if highPriorityAlerts >= 3 {
		return 0.5
	}
	if highPriorityAlerts >= 1 {
		return 0.7
	}
	return 1.0
}

// ── Layer 3: Gradual Position Building ──

func (e *PositionSizingEngine) gradualBuildLimit(runDays int, cumulativePnl float64, totalPct float64) float64 {
	cashRef := 100000.0

	if runDays <= 3 {
		return math.Min(totalPct, 30)
	}

	if cumulativePnl > 0 {
		if cumulativePnl > cashRef*0.05 {
			return totalPct
		}
		return totalPct * 0.6
	}

	if cumulativePnl < -cashRef*0.03 {
		return totalPct * 0.2
	}
	return totalPct * 0.4
}

// ── Fear & Greed ──

func (e *PositionSizingEngine) computeFearGreed(date string) float64 {
	data, err := ComputeFearGreedLatest()
	if err != nil || data == nil {
		return 50
	}
	return data.Score
}

// ── Reason Builder ──

func (e *PositionSizingEngine) buildReason(
	style MarketStyle, fearGreed float64,
	runDays int, cumulativePnl float64,
	riskAlerts int, totalPct, dailyPct float64,
) string {
	fgLabel := "中性"
	if fearGreed >= 60 {
		fgLabel = "贪婪"
	} else if fearGreed < 30 {
		fgLabel = "恐惧"
	}

	reason := fmt.Sprintf("市场=%s 情绪=%s(%.0f)", StyleNames[style], fgLabel, fearGreed)
	if riskAlerts > 0 {
		reason += fmt.Sprintf(" 风控告警=%d条", riskAlerts)
	}
	reason += fmt.Sprintf(" | 总仓位≤%.0f%%", totalPct)
	if runDays <= 3 {
		reason += fmt.Sprintf(" 新账户D%d 单日≤%.0f%%", runDays, dailyPct)
	} else if cumulativePnl < 0 {
		reason += fmt.Sprintf(" 亏损%.1f 收紧至%.0f%%", cumulativePnl, dailyPct)
	} else {
		reason += fmt.Sprintf(" 盈利 单日≤%.0f%%", dailyPct)
	}
	return reason
}

// getCompositeScore retrieves the market composite score for a given date.
func (e *PositionSizingEngine) getCompositeScore(date string) float64 {
	row, err := e.marketStyleSvc.GetLatestStyle()
	if err != nil || row == nil {
		return 50
	}
	return row.CompositeScore
}
