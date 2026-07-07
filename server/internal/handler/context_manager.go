package handler

import (
	"fmt"
	"log"
	"sync"

	"github.com/ai-stock-predict/server/internal/db"
)

// ═══════════════════════════════════════════════════════════════
// ContextManager — 市场上下文管理器
// ═══════════════════════════════════════════════════════════════
//
// 每个交易日开始时读取 MarketSentiment，输出统一 MarketContext。
// 用于动态调整评分权重、仓位比例、风险偏好。

// MarketContext is the unified market environment snapshot for a trading day.
type MarketContext struct {
	Date            string
	CompositeScore  float64  // MarketSentiment 综合分 (-5~+5)
	MarketBias      float64  // 风险偏好乘数 (0.5~1.5)，>1 偏激进
	SectorLeaders   []string // 强势行业
	SectorLaggards  []string // 弱势行业
	NorthboundFlow  float64  // 北向资金净流向（亿元）
	RiskLevel       string   // low / medium / high / extreme
	TradeAllowed    bool     // false = 熔断，当日只平仓不开仓
}

// ContextManager builds market context from MarketSentiment data.
type ContextManager struct {
	strategy *StrategyOrchestration
	cache    map[string]*MarketContext
	cacheMu  sync.RWMutex
}

// StrategyOrchestration holds the orchestration config for a strategy.
type StrategyOrchestration struct {
	Mode                 string  // scoring / decision_tree / hybrid
	EnableMarketContext  bool
	MarketCompositeMin   float64
	MarketPositionBias   float64
	EnableAIAgent        bool
}

// NewContextManager creates a context manager.
func NewContextManager(orch *StrategyOrchestration) *ContextManager {
	return &ContextManager{strategy: orch, cache: make(map[string]*MarketContext)}
}

// GetContext fetches market sentiment for a date and computes MarketContext.
// If EnableMarketContext is false, returns a neutral context (bias=1.0, always allowed).
func (cm *ContextManager) GetContext(date string) (*MarketContext, error) {
	ctx := &MarketContext{
		Date:         date,
		MarketBias:   1.0,
		RiskLevel:    "low",
		TradeAllowed: true,
	}

	if !cm.strategy.EnableMarketContext {
		return ctx, nil
	}

	// P0-3: Check cache
	cm.cacheMu.RLock()
	if cached, ok := cm.cache[date]; ok {
		cm.cacheMu.RUnlock()
		return cached, nil
	}
	cm.cacheMu.RUnlock()

	// Fetch latest market sentiment record (same date or most recent before date)
	var row struct {
		CompositeScore    float64
		SectorDiffusion   float64
		SectorDiffScore   float64
		NorthboundNet     float64
		NorthboundScore   float64
		RiskAppetite      float64
		RiskAppScore      float64
		CapitalFlowNet    float64
		CapitalFlowScore  float64
		LimitSentiment    float64
		LimitScore        float64
	}
	err := db.PG.Raw(`
		SELECT composite_score, sector_diffusion, sector_score,
		       northbound_net, northbound_score,
		       risk_appetite, risk_app_score,
		       capital_flow_net, capital_flow_score,
		       limit_sentiment, limit_score
		FROM market_sentiment
		WHERE trade_date <= ?
		ORDER BY trade_date DESC LIMIT 1`, date).Scan(&row).Error
	if err != nil {
		log.Printf("[context_manager] market_sentiment query failed for %s: %v", date, err)
		// Fall through with neutral defaults
		return ctx, nil
	}

	// Normalize composite_score from 0-100 to -5..+5
	ctx.CompositeScore = (row.CompositeScore - 50) / 10
	ctx.NorthboundFlow = row.NorthboundNet

	// ── Market Bias computation ──
	bias := 1.0

	// Sector diffusion: high diffusion → more sectors moving → bullish bias
	if row.SectorDiffusion > 0.6 {
		bias *= 1.2
	} else if row.SectorDiffusion < 0.3 {
		bias *= 0.85
	}

	// Northbound flow: significant outflow → cautious
	if row.NorthboundNet < -50 {
		bias *= 0.8
	} else if row.NorthboundNet > 50 {
		bias *= 1.15
	}

	// Risk appetite
	if row.RiskAppScore > 47 { // normalised 0-100
		bias *= 1.1
	} else if row.RiskAppScore < 40 {
		bias *= 0.9
	}

	// Capital flow
	if row.CapitalFlowScore > 47 {
		bias *= 1.05
	}

	// Limit sentiment (涨跌停情绪)
	if row.LimitScore < 40 {
		bias *= 0.9
	}

	if bias < 0.5 {
		bias = 0.5
	}
	if bias > 1.5 {
		bias = 1.5
	}
	ctx.MarketBias = bias

	// ── Risk Level ──
	switch {
	case row.RiskAppScore < 35 || ctx.CompositeScore < -3:
		ctx.RiskLevel = "extreme"
	case ctx.CompositeScore < -1.5:
		ctx.RiskLevel = "high"
	case ctx.CompositeScore > 2:
		ctx.RiskLevel = "low"
	default:
		ctx.RiskLevel = "medium"
	}

	// ── Trade Allowed (circuit breaker) ──
	if cm.strategy.MarketCompositeMin > -999 { // -999 = disabled
		if ctx.CompositeScore < cm.strategy.MarketCompositeMin {
			ctx.TradeAllowed = false
			log.Printf("[context_manager] date=%s circuit_breaker: composite=%.2f < min=%.2f",
				date, ctx.CompositeScore, cm.strategy.MarketCompositeMin)
		}
	}

	// ── Sector leaders/laggards (stub for now — requires sector data) ──

	log.Printf("[context_manager] date=%s bias=%.2f risk=%s trade=%v composite=%.2f",
		date, ctx.MarketBias, ctx.RiskLevel, ctx.TradeAllowed, ctx.CompositeScore)

	// P0-3: Cache result
	cm.cacheMu.Lock()
	cm.cache[date] = ctx
	if len(cm.cache) > 20 {
		for k := range cm.cache {
			delete(cm.cache, k)
			break
		}
	}
	cm.cacheMu.Unlock()

	return ctx, nil
}

// getSectorRotation retrieves top/bottom sectors by daily performance.
func (cm *ContextManager) getSectorRotation(date string) ([]string, []string) {
	var sectors []struct {
		SectorName string
		AvgChg     float64
	}
	if err := db.PG.Raw(`
		SELECT sb.industry AS sector_name, AVG(k.daily_change) AS avg_chg
		FROM stocks_daily_k k
		JOIN stocks_basic sb ON sb.code = k.code
		WHERE k.trade_date = ? AND sb.industry IS NOT NULL AND sb.industry != ''
		GROUP BY sb.industry
		ORDER BY avg_chg DESC`, date).Scan(&sectors).Error; err != nil {
		log.Printf("[context_manager] sector_rotation query failed: %v", err)
		return nil, nil
	}

	if len(sectors) == 0 {
		return nil, nil
	}

	leaders := make([]string, 0, 3)
	laggards := make([]string, 0, 3)
	for i, s := range sectors {
		if i < 3 {
			leaders = append(leaders, fmt.Sprintf("%s(%.1f%%)", s.SectorName, s.AvgChg))
		}
		if i >= len(sectors)-3 {
			laggards = append(laggards, fmt.Sprintf("%s(%.1f%%)", s.SectorName, s.AvgChg))
		}
	}
	return leaders, laggards
}

// BiasForBuy returns the adjusted buy threshold based on market bias.
// When bias > 1 (bullish), lower the buy threshold; when bias < 1 (bearish), raise it.
func (cm *ContextManager) BiasForBuy(baseThreshold float64) float64 {
	if !cm.strategy.EnableMarketContext {
		return baseThreshold
	}
	// In bullish market, accept lower scores (threshold *= 1/bias)
	return baseThreshold / cm.strategy.MarketPositionBias
}

// BiasForSell returns the adjusted sell threshold based on market bias.
// When bias < 1 (bearish), lower the sell threshold → more eager to sell.
func (cm *ContextManager) BiasForSell(baseThreshold float64) float64 {
	if !cm.strategy.EnableMarketContext {
		return baseThreshold
	}
	return baseThreshold * (2.0 - cm.strategy.MarketPositionBias)
}
