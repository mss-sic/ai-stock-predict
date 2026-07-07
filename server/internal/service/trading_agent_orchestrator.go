package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// TradingAgentOrchestrator coordinates the multi-agent decision pipeline v2.
// Flow: 3 Objective Analysts (parallel) → PM Strategy Bridge (single call)
//
// Key design principles:
// 1. Analysts are STRATEGY-BLIND — they report objective data only.
// 2. PM is STRATEGY-AWARE — bridges strategy mandate with objective analysis.
// 3. No adversarial debate — debate amplifies noise, PM does gap analysis instead.
// 4. Single decision point — eliminates conflicting recommendations from multiple judges.
type TradingAgentOrchestrator struct {
	aiSvc *AIService
}

// TAOrchestratorConfig controls orchestrator behavior.
type TAOrchestratorConfig struct {
	UserID           uint
	ProgressCallback func(stockCode, phase string)
}

// NewTradingAgentOrchestrator creates a new v2 orchestrator.
func NewTradingAgentOrchestrator(aiSvc *AIService) *TradingAgentOrchestrator {
	return &TradingAgentOrchestrator{aiSvc: aiSvc}
}

// Run executes the full v2 pipeline: 3 analysts → PM.
func (o *TradingAgentOrchestrator) Run(cfg TAOrchestratorConfig, ctx TradingAgentContext) (*TradingAgentResult, error) {
	result := &TradingAgentResult{}

	// ── Phase 1: Objective Analysis (3 analysts in parallel) ──
	log.Printf("[ta_agent] Phase 1: Objective analysis for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "analysts")
	}
	result.AnalystReports = o.runObjectiveAnalysts(cfg.UserID, ctx)

	// ── Phase 2: PM Strategy Bridge (single call) ──
	log.Printf("[ta_agent] Phase 2: PM strategy bridge for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "pm")
	}
	pmDecision, pmReasoning := o.runStrategyBridge(cfg.UserID, ctx, result.AnalystReports)
	result.FinalDecision = pmDecision
	result.PMReasoning = pmReasoning

	log.Printf("[ta_agent] Pipeline complete for %s: action=%s confidence=%.0f%%",
		ctx.StockCode, result.FinalDecision.Action, result.FinalDecision.Confidence)

	return result, nil
}

// ── Phase 1: Objective Analysts (strategy-blind) ──

func (o *TradingAgentOrchestrator) runObjectiveAnalysts(userID uint, ctx TradingAgentContext) []TradingAgentMessage {
	// Three analysts, each with a specific research question.
	// They do NOT receive strategy context — pure objective data interpretation.
	analysts := []struct {
		role   TradingAgentRole
		prompt string
	}{
		{TARoleTechnicalAnalyst, TATechnicalPrompt},
		{TARoleFundamentalAnalyst, TAFundamentalPrompt},
		{TARoleMarketAnalyst, TAMarketContextPrompt},
	}

	reports := make([]TradingAgentMessage, 0, len(analysts))
	for _, a := range analysts {
		sysPrompt := taBuildPrompt(a.prompt, ctx)
		dataPrompt := taBuildAnalystData(a.role, ctx)
		fullPrompt := sysPrompt + "\n\n" + dataPrompt

		resp, err := o.aiSvc.ChatCompletion(userID, fullPrompt, nil)
		if err != nil {
			log.Printf("[ta_agent] %s failed: %v", a.role, err)
			reports = append(reports, TradingAgentMessage{Role: a.role, Content: fmt.Sprintf("Error: %v", err)})
			continue
		}
		reports = append(reports, TradingAgentMessage{Role: a.role, Content: resp})
	}
	return reports
}

// ── Phase 2: PM Strategy Bridge ──

func (o *TradingAgentOrchestrator) runStrategyBridge(userID uint, ctx TradingAgentContext, reports []TradingAgentMessage) (TradingAgentDecision, string) {
	// Build the PM prompt with strategy mandate + objective analysis
	prompt := taBuildPMBridgePrompt(ctx, reports)

	resp, err := o.aiSvc.ChatCompletion(userID, prompt, nil)
	if err != nil {
		log.Printf("[ta_agent] PM strategy bridge failed: %v", err)
		return TradingAgentDecision{
			Action: "hold", Confidence: 0,
			Reasoning: fmt.Sprintf("PM error: %v", err),
		}, fmt.Sprintf("Error: %v", err)
	}

	decision := taParseDecision(resp)
	if decision.Action == "" {
		decision = TradingAgentDecision{
			Action:    "hold",
			Confidence: 0,
			Reasoning: "Failed to parse PM decision: " + taTruncate(resp, 200),
		}
	}
	return decision, resp
}

// ── Prompt Builders ──

// taBuildPMBridgePrompt constructs the full PM prompt with strategy mandate and analysis.
func taBuildPMBridgePrompt(ctx TradingAgentContext, reports []TradingAgentMessage) string {
	// Build strategy mandate section
	mandate := TAPMStrategyBridgePrompt
	sp := ctx.Strategy

	replacements := map[string]string{
		"{strategy_name}":   sp.Name,
		"{strategy_style}":  taStyleLabel(sp.Style),
		"{strategy_thesis}": sp.Thesis,
		"{hold_days}":       fmt.Sprintf("%d", sp.HoldDays),
		"{risk_profile}":    sp.RiskProfile,
		"{stop_loss}":       fmt.Sprintf("%.1f", sp.StopLoss),
		"{stop_profit}":     fmt.Sprintf("%.1f", sp.StopProfit),
		"{position_sizing}": sp.PositionSizing,
		"{buy_pct}":         fmt.Sprintf("%.0f", sp.BuyPositionPct),
		"{stock_name}":      ctx.StockName,
		"{stock_code}":      ctx.StockCode,
		"{signal_action}":   "buy", // will be overridden by caller
		"{signal_reason}":   strings.Join(ctx.BuyConditions, "; "),
		"{signal_price}":    fmt.Sprintf("%.2f", ctx.CurrentPrice),
	}

	for k, v := range replacements {
		mandate = strings.ReplaceAll(mandate, k, v)
	}

	// Build analyst reports section
	reportsText := taMergeReports(reports)

	// Build portfolio state section
	portfolioState := taBuildPortfolioContext(ctx)

	// Assemble
	mandate = strings.ReplaceAll(mandate, "{analyst_reports}", reportsText)
	mandate = strings.ReplaceAll(mandate, "{portfolio_state}", portfolioState)

	return mandate + "\n\n请使用中文回复。"
}

// taBuildPrompt handles stock name/date substitution (analyst-level, strategy-blind).
func taBuildPrompt(template string, ctx TradingAgentContext) string {
	p := template
	p = strings.ReplaceAll(p, "{stock_name}", ctx.StockName)
	p = strings.ReplaceAll(p, "{stock_code}", ctx.StockCode)
	p = strings.ReplaceAll(p, "{trade_date}", ctx.TradeDate)
	return p + "\n\n请使用中文回复。"
}

// taStyleLabel converts a style code to a human-readable Chinese label.
func taStyleLabel(style string) string {
	switch style {
	case "momentum_chaser":
		return "动量追击 — 追涨强势股，捕捉短期爆发"
	case "swing_trader":
		return "波段交易 — 高抛低吸，利用短期价格波动"
	case "trend_follower":
		return "趋势跟随 — 顺势而为，持仓直到趋势反转"
	case "value_hunter":
		return "价值挖掘 — 寻找低估标的，等待价值回归"
	case "dip_buyer":
		return "抄底反弹 — 超跌买入，搏短期修复"
	case "grid_trader":
		return "网格做T — 震荡市中反复低买高卖"
	default:
		if style == "" {
			return "通用策略"
		}
		return style
	}
}

// ── Analyst Data Builders ──

func taBuildAnalystData(role TradingAgentRole, ctx TradingAgentContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Stock: %s (%s) | Date: %s | Price: %.2f\n\n", ctx.StockName, ctx.StockCode, ctx.TradeDate, ctx.CurrentPrice))

	switch role {
	case TARoleTechnicalAnalyst:
		sb.WriteString("## Price Data (Recent 20 days):\n")
		for _, p := range ctx.RecentPrices {
			sb.WriteString(fmt.Sprintf("%s: O=%.2f H=%.2f L=%.2f C=%.2f V=%.0f\n", p.Date, p.Open, p.High, p.Low, p.Close, p.Volume))
		}
		sb.WriteString("\n## Technical Indicators:\n")
		for k, v := range ctx.Indicators {
			sb.WriteString(fmt.Sprintf("- %s: %.4f\n", k, v))
		}

	case TARoleFundamentalAnalyst:
		sb.WriteString(fmt.Sprintf("PE: %.2f | PB: %.2f | PS: %.2f\n", ctx.PE, ctx.PB, ctx.PS))
		sb.WriteString(fmt.Sprintf("Market Cap: %.2f 亿元\n", ctx.MarketCap/1e8))

	case TARoleMarketAnalyst:
		sb.WriteString(fmt.Sprintf("Social Sentiment Score: %.2f (-1 bearish ~ +1 bullish)\n", ctx.SocialSentiment))
		sb.WriteString(fmt.Sprintf("News Sentiment Score: %.2f\n", ctx.NewsSentiment))
		sb.WriteString("\n## News Headlines:\n")
		if len(ctx.NewsHeadlines) > 0 {
			for _, h := range ctx.NewsHeadlines {
				sb.WriteString(fmt.Sprintf("- %s\n", h))
			}
		} else {
			sb.WriteString("No recent news headlines available.\n")
		}
	}

	// Strategy conditions are shown to analysts as context (what triggered the signal)
	// but without strategy philosophy — keep analysts objective
	if len(ctx.BuyConditions) > 0 {
		sb.WriteString("\n## Trigger Conditions:\n")
		for _, c := range ctx.BuyConditions {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}
	if len(ctx.SellConditions) > 0 {
		sb.WriteString("\n## Sell Conditions:\n")
		for _, c := range ctx.SellConditions {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}

	return sb.String()
}

func taBuildPortfolioContext(ctx TradingAgentContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Cash: ¥%.2f | Total Equity: ¥%.2f\n", ctx.CurrentCash, ctx.TotalEquity))
	if len(ctx.CurrentPositions) > 0 {
		sb.WriteString("Current Positions:\n")
		for _, p := range ctx.CurrentPositions {
			sb.WriteString(fmt.Sprintf("- %s (%s): %d shares @ ¥%.2f, Market: ¥%.2f, P&L: %.1f%%\n",
				p.Name, p.Code, p.Quantity, p.BuyPrice, p.MarketValue, p.ProfitPct))
		}
	} else {
		sb.WriteString("No current positions.\n")
	}
	return sb.String()
}

func taMergeReports(reports []TradingAgentMessage) string {
	var sb strings.Builder
	for _, r := range reports {
		sb.WriteString(fmt.Sprintf("\n### %s Report:\n%s\n", r.Role, r.Content))
	}
	return sb.String()
}

// ── JSON Parsing ──

func taParseDecision(text string) TradingAgentDecision {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		jsonStr := text[start : end+1]
		var d TradingAgentDecision
		if err := json.Unmarshal([]byte(jsonStr), &d); err == nil {
			return d
		}
	}
	var d TradingAgentDecision
	if err := json.Unmarshal([]byte(text), &d); err == nil {
		return d
	}
	return TradingAgentDecision{}
}

func taTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
