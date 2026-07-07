package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// Agent Role Tests
// ═══════════════════════════════════════════════════════════════

func TestTARoles_AllDefined(t *testing.T) {
	roles := []TradingAgentRole{
		TARoleMarketAnalyst, TARoleSentimentAnalyst, TARoleNewsAnalyst, TARoleFundamentalsAnalyst,
		TARoleBullResearcher, TARoleBearResearcher, TARoleResearchManager,
		TARoleTrader,
		TARoleAggressiveAnalyst, TARoleConservativeAnalyst, TARoleNeutralAnalyst, TARolePortfolioManager,
	}
	for i, r := range roles {
		if r == "" {
			t.Errorf("role[%d] is empty", i)
		}
	}
	if len(roles) != 12 {
		t.Errorf("expected 12 agent roles, got %d", len(roles))
	}
}

func TestTAPhaseRoles_Mapping(t *testing.T) {
	if len(TAPhaseRoles[TAPhaseAnalysis]) != 4 {
		t.Errorf("PhaseAnalysis should have 4 roles, got %d", len(TAPhaseRoles[TAPhaseAnalysis]))
	}
	if len(TAPhaseRoles[TAPhaseResearch]) != 3 {
		t.Errorf("PhaseResearch should have 3 roles, got %d", len(TAPhaseRoles[TAPhaseResearch]))
	}
	if len(TAPhaseRoles[TAPhaseRisk]) != 4 {
		t.Errorf("PhaseRisk should have 4 roles, got %d", len(TAPhaseRoles[TAPhaseRisk]))
	}
}

func TestTAPrompts_NotEmpty(t *testing.T) {
	prompts := map[string]string{
		"Market": TAMarketAnalystPrompt, "Sentiment": TASentimentAnalystPrompt,
		"News": TANewsAnalystPrompt, "Fundamentals": TAFundamentalsAnalystPrompt,
		"Bull": TABullResearcherPrompt, "Bear": TABearResearcherPrompt,
		"ResearchMgr": TAResearchManagerPrompt, "Trader": TATraderPrompt,
		"Aggressive": TAAggressiveRiskPrompt, "Conservative": TAConservativeRiskPrompt,
		"Neutral": TANeutralRiskPrompt, "PM": TAPortfolioManagerPrompt,
	}
	for name, prompt := range prompts {
		if len(prompt) < 100 {
			t.Errorf("%s prompt too short (%d chars)", name, len(prompt))
		}
	}
}

func TestTAPrompts_ContainPlaceholders(t *testing.T) {
	withPH := []string{
		TAMarketAnalystPrompt, TAFundamentalsAnalystPrompt,
		TABullResearcherPrompt, TABearResearcherPrompt, TAResearchManagerPrompt,
	}
	for i, p := range withPH {
		if !strings.Contains(p, "{stock_name}") {
			t.Errorf("prompt[%d] missing {stock_name}", i)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Helper Tests
// ═══════════════════════════════════════════════════════════════

func TestTABuildPrompt(t *testing.T) {
	ctx := TradingAgentContext{StockCode: "000001", StockName: "平安银行", TradeDate: "2024-01-15"}
	result := taBuildPrompt("Analyze {stock_name} ({stock_code}) on {trade_date}", ctx)
	if result != "Analyze 平安银行 (000001) on 2024-01-15" {
		t.Errorf("got %q", result)
	}
}

func TestTABuildAnalystData_Market(t *testing.T) {
	ctx := TradingAgentContext{
		StockCode: "000001", StockName: "平安银行", CurrentPrice: 10.5,
		RecentPrices: []PricePoint{{Date: "2024-01-15", Open: 10.2, Close: 10.5, Volume: 1e6}},
		Indicators:   map[string]float64{"rsi": 55.2},
	}
	data := taBuildAnalystData(TARoleMarketAnalyst, ctx)
	if !strings.Contains(data, "rsi") || !strings.Contains(data, "10.5") {
		t.Error("market data incomplete")
	}
}

func TestTABuildAnalystData_Sentiment(t *testing.T) {
	ctx := TradingAgentContext{
		StockCode: "000001", StockName: "平安银行",
		SocialSentiment: 0.65, NewsSentiment: 0.3,
		NewsHeadlines: []string{"业绩预增公告"},
	}
	data := taBuildAnalystData(TARoleSentimentAnalyst, ctx)
	if !strings.Contains(data, "0.65") || !strings.Contains(data, "业绩预增公告") {
		t.Error("sentiment data incomplete")
	}
}

func TestTABuildAnalystData_Fundamentals(t *testing.T) {
	ctx := TradingAgentContext{
		StockCode: "000001", StockName: "平安银行",
		PE: 8.5, PB: 0.9, PS: 2.1, MarketCap: 2.5e11,
	}
	data := taBuildAnalystData(TARoleFundamentalsAnalyst, ctx)
	if !strings.Contains(data, "8.5") || !strings.Contains(data, "2500") {
		t.Error("fundamentals data incomplete")
	}
}

func TestTABuildPortfolioContext(t *testing.T) {
	ctx := TradingAgentContext{
		CurrentCash: 50000, TotalEquity: 150000,
		CurrentPositions: []PositionSnapshot{
			{Code: "000001", Name: "平安银行", Quantity: 1000, BuyPrice: 10.0, MarketValue: 10500, ProfitPct: 5.0},
		},
	}
	result := taBuildPortfolioContext(ctx)
	if !strings.Contains(result, "50000") || !strings.Contains(result, "000001") {
		t.Error("portfolio context incomplete")
	}
}

func TestTABuildPortfolioContext_Empty(t *testing.T) {
	ctx := TradingAgentContext{CurrentCash: 100000, TotalEquity: 100000}
	if !strings.Contains(taBuildPortfolioContext(ctx), "No current positions") {
		t.Error("should show no positions")
	}
}

func TestTAMergeReports(t *testing.T) {
	reports := []TradingAgentMessage{
		{Role: TARoleMarketAnalyst, Content: "Market: bullish"},
		{Role: TARoleSentimentAnalyst, Content: "Sentiment: positive"},
	}
	result := taMergeReports(reports)
	if !strings.Contains(result, "Market: bullish") || !strings.Contains(result, "Sentiment: positive") {
		t.Error("merge incomplete")
	}
}

// ═══════════════════════════════════════════════════════════════
// Decision Parsing Tests
// ═══════════════════════════════════════════════════════════════

func TestTAParseDecision_ValidJSON(t *testing.T) {
	jsonStr := `{"action":"buy","confidence":85,"amount":50000,"price":10.5,"stop_loss":9.8,"stop_profit":12.0,"reasoning":"Bullish","risk_level":"medium","horizon_days":10}`
	d := taParseDecision(jsonStr)
	if d.Action != "buy" || d.Confidence != 85 || d.RiskLevel != "medium" {
		t.Errorf("parse: action=%s conf=%.0f risk=%s", d.Action, d.Confidence, d.RiskLevel)
	}
}

func TestTAParseDecision_EmbeddedJSON(t *testing.T) {
	text := "Here is my analysis:\n```json\n{\"action\":\"sell\",\"confidence\":70,\"amount\":0,\"price\":11.0,\"stop_loss\":0,\"stop_profit\":0,\"reasoning\":\"Bearish\",\"risk_level\":\"high\",\"horizon_days\":0}\n```\nDone."
	d := taParseDecision(text)
	if d.Action != "sell" || d.Confidence != 70 {
		t.Errorf("embedded parse: action=%s conf=%.0f", d.Action, d.Confidence)
	}
}

func TestTAParseDecision_Empty(t *testing.T) {
	d := taParseDecision("no json here")
	if d.Action != "" {
		t.Error("expected empty decision")
	}
}

// ═══════════════════════════════════════════════════════════════
// JSON Roundtrip Tests
// ═══════════════════════════════════════════════════════════════

func TestTADecision_JSONRoundtrip(t *testing.T) {
	original := TradingAgentDecision{
		Action: "buy", Confidence: 80, Amount: 30000, Price: 15.5,
		StopLoss: 14.0, StopProfit: 18.0, Reasoning: "Test",
		RiskLevel: "medium", HorizonDays: 7,
	}
	b, _ := json.Marshal(original)
	var parsed TradingAgentDecision
	json.Unmarshal(b, &parsed)
	if parsed.Action != "buy" || parsed.Confidence != 80 {
		t.Error("roundtrip failed")
	}
}

func TestTAResult_JSONRoundtrip(t *testing.T) {
	r := TradingAgentResult{
		FinalDecision: TradingAgentDecision{Action: "buy", Confidence: 75},
		AnalystReports: []TradingAgentMessage{{Role: TARoleMarketAnalyst, Content: "Analysis"}},
		TotalTokensUsed: 5000,
	}
	b, _ := json.Marshal(r)
	var parsed TradingAgentResult
	json.Unmarshal(b, &parsed)
	if parsed.FinalDecision.Action != "buy" || len(parsed.AnalystReports) != 1 {
		t.Error("result roundtrip failed")
	}
}

// ═══════════════════════════════════════════════════════════════
// Orchestrator Creation
// ═══════════════════════════════════════════════════════════════

func TestNewTAOrchestrator(t *testing.T) {
	aiSvc := NewAIService()
	orch := NewTradingAgentOrchestrator(aiSvc)
	if orch == nil {
		t.Fatal("NewTradingAgentOrchestrator returned nil")
	}
	if orch.maxDebateRounds != 1 || orch.maxRiskRounds != 1 {
		t.Error("default rounds mismatch")
	}
}

// ═══════════════════════════════════════════════════════════════
// Context Construction
// ═══════════════════════════════════════════════════════════════

func TestTAContext_Construction(t *testing.T) {
	ctx := TradingAgentContext{
		StockCode: "600519", StockName: "贵州茅台", TradeDate: "2024-06-15",
		CurrentPrice: 1800.0,
		Indicators:   map[string]float64{"rsi": 45, "macd": -2.1},
		PE: 25.0, PB: 8.5, MarketCap: 2.2e12,
		CurrentCash: 200000, TotalEquity: 500000,
		BuyConditions: []string{"rsi < 50"},
	}
	if ctx.StockCode != "600519" || len(ctx.Indicators) != 2 {
		t.Error("context construction failed")
	}
}

// ═══════════════════════════════════════════════════════════════
// Phase Coverage
// ═══════════════════════════════════════════════════════════════

func TestTAPhaseRoles_CompleteCoverage(t *testing.T) {
	allInPhases := make(map[TradingAgentRole]bool)
	for _, roles := range TAPhaseRoles {
		for _, r := range roles {
			allInPhases[r] = true
		}
	}
	allDefined := []TradingAgentRole{
		TARoleMarketAnalyst, TARoleSentimentAnalyst, TARoleNewsAnalyst, TARoleFundamentalsAnalyst,
		TARoleBullResearcher, TARoleBearResearcher, TARoleResearchManager,
		TARoleTrader,
		TARoleAggressiveAnalyst, TARoleConservativeAnalyst, TARoleNeutralAnalyst, TARolePortfolioManager,
	}
	for _, r := range allDefined {
		if !allInPhases[r] {
			t.Errorf("role %s not in any phase", r)
		}
	}
}
