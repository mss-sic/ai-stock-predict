package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// TradingAgentOrchestrator coordinates the multi-agent decision pipeline.
// Flow: Analysts → Bull/Bear Debate → Research Manager → Trader → Risk Debate → Portfolio Manager
type TradingAgentOrchestrator struct {
	aiSvc           *AIService
	maxDebateRounds int
	maxRiskRounds   int
}

// TAOrchestratorConfig controls orchestrator behavior.
type TAOrchestratorConfig struct {
	UserID           uint
	MaxDebateRounds  int
	MaxRiskRounds    int
	ProgressCallback func(stockCode, phase string) // per-phase progress hook
}

// NewTradingAgentOrchestrator creates a new multi-agent orchestrator.
func NewTradingAgentOrchestrator(aiSvc *AIService) *TradingAgentOrchestrator {
	return &TradingAgentOrchestrator{
		aiSvc:           aiSvc,
		maxDebateRounds: 1,
		maxRiskRounds:   1,
	}
}

// Run executes the full multi-agent pipeline and returns the final decision.
func (o *TradingAgentOrchestrator) Run(cfg TAOrchestratorConfig, ctx TradingAgentContext) (*TradingAgentResult, error) {
	if cfg.MaxDebateRounds > 0 {
		o.maxDebateRounds = cfg.MaxDebateRounds
	}
	if cfg.MaxRiskRounds > 0 {
		o.maxRiskRounds = cfg.MaxRiskRounds
	}

	result := &TradingAgentResult{}

	log.Printf("[ta_agent] Phase 1: Analyst reports for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "analysts")
	}
	result.AnalystReports = o.runAnalysts(cfg.UserID, ctx)

	log.Printf("[ta_agent] Phase 2: Bull/Bear debate for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "debate")
	}
	debateResult := o.runDebate(cfg.UserID, ctx, result.AnalystReports)
	result.DebateHistory = debateResult.history
	researchVerdict := debateResult.verdict

	log.Printf("[ta_agent] Phase 3: Trader decision for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "trader")
	}
	traderDecision := o.runTrader(cfg.UserID, ctx, result.AnalystReports, researchVerdict)
	result.TraderDecision = traderDecision

	log.Printf("[ta_agent] Phase 4: Risk debate + PM for %s", ctx.StockCode)
	if cfg.ProgressCallback != nil {
		cfg.ProgressCallback(ctx.StockCode, "risk")
	}
	riskResult := o.runRiskAnalysis(cfg.UserID, ctx, result.AnalystReports, researchVerdict, traderDecision)
	result.RiskDebateHistory = riskResult.history
	result.PMModifications = riskResult.pmReasoning
	result.FinalDecision = riskResult.finalDecision

	log.Printf("[ta_agent] Pipeline complete for %s: action=%s confidence=%.0f%%",
		ctx.StockCode, result.FinalDecision.Action, result.FinalDecision.Confidence)

	return result, nil
}

// ── Phase 1: Analysts ──

func (o *TradingAgentOrchestrator) runAnalysts(userID uint, ctx TradingAgentContext) []TradingAgentMessage {
	analysts := []struct {
		role   TradingAgentRole
		prompt string
	}{
		{TARoleMarketAnalyst, TAMarketAnalystPrompt},
		{TARoleSentimentAnalyst, TASentimentAnalystPrompt},
		{TARoleNewsAnalyst, TANewsAnalystPrompt},
		{TARoleFundamentalsAnalyst, TAFundamentalsAnalystPrompt},
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

// ── Phase 2: Bull/Bear Debate ──

type taDebateResult struct {
	history []TradingAgentMessage
	verdict string
}

func (o *TradingAgentOrchestrator) runDebate(userID uint, ctx TradingAgentContext, reports []TradingAgentMessage) taDebateResult {
	reportsText := taMergeReports(reports)
	var history []TradingAgentMessage

	bullPrompt := taBuildPrompt(TABullResearcherPrompt, ctx) + "\n\n### Analyst Reports:\n" + reportsText
	bearPrompt := taBuildPrompt(TABearResearcherPrompt, ctx) + "\n\n### Analyst Reports:\n" + reportsText

	for round := 0; round < o.maxDebateRounds; round++ {
		if round > 0 {
			bullPrompt += fmt.Sprintf("\n\n### Bear's Previous Argument:\n%s", taLastContent(history, TARoleBearResearcher))
		}
		bullResp, err := o.aiSvc.ChatCompletion(userID, bullPrompt, nil)
		if err != nil {
			log.Printf("[ta_agent] bull round %d failed: %v", round+1, err)
			break
		}
		history = append(history, TradingAgentMessage{Role: TARoleBullResearcher, Content: bullResp})

		bearPromptR := bearPrompt
		bearPromptR += fmt.Sprintf("\n\n### Bull's Argument:\n%s", bullResp)
		bearResp, err := o.aiSvc.ChatCompletion(userID, bearPromptR, nil)
		if err != nil {
			log.Printf("[ta_agent] bear round %d failed: %v", round+1, err)
			break
		}
		history = append(history, TradingAgentMessage{Role: TARoleBearResearcher, Content: bearResp})
	}

	judgePrompt := taBuildPrompt(TAResearchManagerPrompt, ctx) + "\n\n### Analyst Reports:\n" + reportsText
	judgePrompt += "\n\n### Debate History:\n"
	for _, h := range history {
		judgePrompt += fmt.Sprintf("\n--- %s ---\n%s\n", h.Role, h.Content)
	}

	judgeResp, err := o.aiSvc.ChatCompletion(userID, judgePrompt, nil)
	if err != nil {
		log.Printf("[ta_agent] research manager failed: %v", err)
		return taDebateResult{history: history, verdict: "Unable to reach verdict"}
	}
	history = append(history, TradingAgentMessage{Role: TARoleResearchManager, Content: judgeResp})

	return taDebateResult{history: history, verdict: judgeResp}
}

// ── Phase 3: Trader ──

func (o *TradingAgentOrchestrator) runTrader(userID uint, ctx TradingAgentContext, reports []TradingAgentMessage, verdict string) TradingAgentDecision {
	reportsText := taMergeReports(reports)

	prompt := taBuildPrompt(TATraderPrompt, ctx) +
		"\n\n### Analyst Reports:\n" + reportsText +
		"\n\n### Research Manager Verdict:\n" + verdict +
		"\n\n### Current Portfolio State:\n" + taBuildPortfolioContext(ctx) +
		"\n\nOutput ONLY the JSON decision."

	resp, err := o.aiSvc.ChatCompletion(userID, prompt, nil)
	if err != nil {
		log.Printf("[ta_agent] trader failed: %v", err)
		return TradingAgentDecision{Action: "hold", Confidence: 0, Reasoning: fmt.Sprintf("Trader error: %v", err)}
	}

	decision := taParseDecision(resp)
	if decision.Action == "" {
		decision.Action = "hold"
		decision.Reasoning = "Failed to parse trader decision: " + taTruncate(resp, 200)
	}
	return decision
}

// ── Phase 4: Risk Analysis + Portfolio Manager ──

type taRiskResult struct {
	history       []TradingAgentMessage
	pmReasoning   string
	finalDecision TradingAgentDecision
}

func (o *TradingAgentOrchestrator) runRiskAnalysis(userID uint, ctx TradingAgentContext, reports []TradingAgentMessage, verdict string, traderDecision TradingAgentDecision) taRiskResult {
	traderJSON, _ := json.MarshalIndent(traderDecision, "", "  ")
	reportsText := taMergeReports(reports)

	basePrompt := fmt.Sprintf(
		"### Context\nStock: %s (%s)\nCurrent Price: %.2f\n\n### Analyst Reports:\n%s\n\n### Research Verdict:\n%s\n\n### Trader's Proposed Decision:\n%s",
		ctx.StockName, ctx.StockCode, ctx.CurrentPrice, reportsText, verdict, string(traderJSON),
	)

	roles := []struct {
		role   TradingAgentRole
		prompt string
	}{
		{TARoleAggressiveAnalyst, TAAggressiveRiskPrompt},
		{TARoleConservativeAnalyst, TAConservativeRiskPrompt},
		{TARoleNeutralAnalyst, TANeutralRiskPrompt},
	}

	responses := make(map[TradingAgentRole]string)
	var history []TradingAgentMessage

	for _, r := range roles {
		fullPrompt := taBuildPrompt(r.prompt, ctx) + "\n\n" + basePrompt
		if r.role == TARoleNeutralAnalyst {
			fullPrompt += fmt.Sprintf("\n\n### Aggressive View:\n%s\n\n### Conservative View:\n%s",
				responses[TARoleAggressiveAnalyst], responses[TARoleConservativeAnalyst])
		}

		resp, err := o.aiSvc.ChatCompletion(userID, fullPrompt, nil)
		if err != nil {
			log.Printf("[ta_agent] %s failed: %v", r.role, err)
			resp = fmt.Sprintf("Error: %v", err)
		}
		responses[r.role] = resp
		history = append(history, TradingAgentMessage{Role: r.role, Content: resp})
	}

	pmPrompt := taBuildPrompt(TAPortfolioManagerPrompt, ctx) + "\n\n" + basePrompt +
		fmt.Sprintf("\n\n### Risk Assessments:\nAggressive: %s\nConservative: %s\nNeutral: %s\n\n### Portfolio State:\n%s",
			responses[TARoleAggressiveAnalyst], responses[TARoleConservativeAnalyst], responses[TARoleNeutralAnalyst],
			taBuildPortfolioContext(ctx)) +
		"\n\nOutput ONLY the FINAL JSON decision."

	pmResp, err := o.aiSvc.ChatCompletion(userID, pmPrompt, nil)
	if err != nil {
		log.Printf("[ta_agent] portfolio manager failed: %v", err)
		return taRiskResult{history: history, pmReasoning: fmt.Sprintf("PM error: %v", err), finalDecision: traderDecision}
	}

	finalDecision := taParseDecision(pmResp)
	if finalDecision.Action == "" {
		finalDecision = traderDecision
		finalDecision.Reasoning = "PM parse failed, using trader plan: " + taTruncate(pmResp, 200)
	}

	return taRiskResult{history: history, pmReasoning: pmResp, finalDecision: finalDecision}
}

// ── Helpers ──

func taBuildPrompt(template string, ctx TradingAgentContext) string {
	p := template
	p = strings.ReplaceAll(p, "{stock_name}", ctx.StockName)
	p = strings.ReplaceAll(p, "{stock_code}", ctx.StockCode)
	p = strings.ReplaceAll(p, "{trade_date}", ctx.TradeDate)

	// For hold signals, inject position context so AI knows cost basis and P&L
	if len(ctx.CurrentPositions) > 0 {
		for _, pos := range ctx.CurrentPositions {
			if pos.Code == ctx.StockCode {
				p += fmt.Sprintf("\n\n⚠️ 持仓分析模式：当前持有 %s %d股，成本¥%.2f，现价¥%.2f，盈亏%.1f%%。请分析是否加仓(add)、减仓(reduce)或继续持有(hold)，给出具体建议。",
					pos.Name, pos.Quantity, pos.BuyPrice, pos.MarketPrice, pos.ProfitPct)
				break
			}
		}
	}

	return p + "\n\n请使用中文回复。"
}

func taBuildAnalystData(role TradingAgentRole, ctx TradingAgentContext) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Stock: %s (%s) | Date: %s | Price: %.2f\n\n", ctx.StockName, ctx.StockCode, ctx.TradeDate, ctx.CurrentPrice))

	switch role {
	case TARoleMarketAnalyst:
		sb.WriteString("## Price Data (Recent 20 days):\n")
		for _, p := range ctx.RecentPrices {
			sb.WriteString(fmt.Sprintf("%s: O=%.2f H=%.2f L=%.2f C=%.2f V=%.0f\n", p.Date, p.Open, p.High, p.Low, p.Close, p.Volume))
		}
		sb.WriteString("\n## Technical Indicators:\n")
		for k, v := range ctx.Indicators {
			sb.WriteString(fmt.Sprintf("- %s: %.4f\n", k, v))
		}

	case TARoleSentimentAnalyst:
		sb.WriteString(fmt.Sprintf("Social Sentiment Score: %.2f (-1 bearish ~ +1 bullish)\n", ctx.SocialSentiment))
		sb.WriteString(fmt.Sprintf("News Sentiment Score: %.2f\n", ctx.NewsSentiment))
		for _, h := range ctx.NewsHeadlines {
			sb.WriteString(fmt.Sprintf("- %s\n", h))
		}

	case TARoleFundamentalsAnalyst:
		sb.WriteString(fmt.Sprintf("PE: %.2f | PB: %.2f | PS: %.2f\n", ctx.PE, ctx.PB, ctx.PS))
		sb.WriteString(fmt.Sprintf("Market Cap: %.2f 亿元\n", ctx.MarketCap/1e8))

	case TARoleNewsAnalyst:
		if len(ctx.NewsHeadlines) > 0 {
			sb.WriteString("## News Headlines:\n")
			for _, h := range ctx.NewsHeadlines {
				sb.WriteString(fmt.Sprintf("- %s\n", h))
			}
		} else {
			sb.WriteString("No recent news headlines available.\n")
		}
		sb.WriteString(fmt.Sprintf("\nNews Sentiment Score: %.2f\n", ctx.NewsSentiment))
	}

	if len(ctx.BuyConditions) > 0 {
		sb.WriteString("\n## Strategy Buy Conditions:\n")
		for _, c := range ctx.BuyConditions {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}
	if len(ctx.SellConditions) > 0 {
		sb.WriteString("\n## Strategy Sell Conditions:\n")
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

func taLastContent(history []TradingAgentMessage, role TradingAgentRole) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == role {
			return history[i].Content
		}
	}
	return "No previous argument."
}

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
