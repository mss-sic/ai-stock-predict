package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTARolesAllDefined(t *testing.T) {
	roles := []TradingAgentRole{
		TARoleTechnicalAnalyst,
		TARoleFundamentalAnalyst,
		TARoleMarketAnalyst,
		TARolePortfolioManager,
	}
	for i, role := range roles {
		if role == "" {
			t.Errorf("role[%d] is empty", i)
		}
	}
}

func TestTAPromptsNotEmpty(t *testing.T) {
	prompts := map[string]string{
		"technical":   TATechnicalPrompt,
		"fundamental": TAFundamentalPrompt,
		"market":      TAMarketContextPrompt,
		"portfolio":   TAPMStrategyBridgePrompt,
	}
	for name, prompt := range prompts {
		if len(prompt) < 100 {
			t.Errorf("%s prompt too short (%d chars)", name, len(prompt))
		}
	}
}

func TestTABuildPrompt(t *testing.T) {
	ctx := TradingAgentContext{StockCode: "000001", StockName: "平安银行", TradeDate: "2024-01-15"}
	result := taBuildPrompt("Analyze {stock_name} ({stock_code}) on {trade_date}", ctx)
	if !strings.Contains(result, "Analyze 平安银行 (000001) on 2024-01-15") {
		t.Errorf("got %q", result)
	}
}

func TestTABuildAnalystData(t *testing.T) {
	ctx := TradingAgentContext{
		StockCode: "000001", StockName: "平安银行", CurrentPrice: 10.5,
		RecentPrices:    []PricePoint{{Date: "2024-01-15", Open: 10.2, Close: 10.5, Volume: 1e6}},
		Indicators:      map[string]float64{"rsi": 55.2},
		PE:              8.5,
		PB:              0.9,
		NewsHeadlines:   []string{"业绩预增公告"},
		NewsSentiment:   0.3,
		SocialSentiment: 0.65,
	}

	technical := taBuildAnalystData(TARoleTechnicalAnalyst, ctx)
	if !strings.Contains(technical, "rsi") || !strings.Contains(technical, "10.5") {
		t.Error("technical data incomplete")
	}
	fundamental := taBuildAnalystData(TARoleFundamentalAnalyst, ctx)
	if !strings.Contains(fundamental, "8.5") {
		t.Error("fundamental data incomplete")
	}
	market := taBuildAnalystData(TARoleMarketAnalyst, ctx)
	if !strings.Contains(market, "业绩预增公告") {
		t.Error("market data incomplete")
	}
}

func TestTAMergeReports(t *testing.T) {
	reports := []TradingAgentMessage{
		{Role: TARoleTechnicalAnalyst, Content: "Technical: bullish"},
		{Role: TARoleMarketAnalyst, Content: "Market: positive"},
	}
	result := taMergeReports(reports)
	if !strings.Contains(result, "Technical: bullish") || !strings.Contains(result, "Market: positive") {
		t.Error("merge incomplete")
	}
}

func TestTAParseDecision(t *testing.T) {
	text := "```json\n{\"action\":\"buy\",\"confidence\":85,\"risk_level\":\"medium\"}\n```"
	decision := taParseDecision(text)
	if decision.Action != "buy" || decision.Confidence != 85 || decision.RiskLevel != "medium" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestTADecisionJSONRoundTrip(t *testing.T) {
	original := TradingAgentDecision{Action: "buy", Confidence: 75, Reasoning: "test"}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal decision: %v", err)
	}
	var decoded TradingAgentDecision
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal decision: %v", err)
	}
	if decoded != original {
		t.Fatalf("round trip mismatch: got %+v want %+v", decoded, original)
	}
}
