package service

import (
	"math"
	"strings"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// PromptBuilder tests
// ═══════════════════════════════════════════════════════════════

func TestPromptBuilder_DefaultSystemPrompt(t *testing.T) {
	svc := NewPromptBuilderService()
	prompt := svc.DefaultSystemPrompt()
	if !strings.Contains(prompt, "量化策略专家") {
		t.Error("DefaultSystemPrompt should mention strategy expert role")
	}
	if !strings.Contains(prompt, "JSON") {
		t.Error("DefaultSystemPrompt should mention JSON format")
	}
}

func TestPromptBuilder_IndicatorRules(t *testing.T) {
	svc := NewPromptBuilderService()
	rules := svc.IndicatorRules()
	if !strings.Contains(rules, "条件构建规范") {
		t.Error("IndicatorRules should contain construction rules")
	}
	if !strings.Contains(rules, "algo_score") {
		t.Error("IndicatorRules should mention algo_score")
	}
	if !strings.Contains(rules, "condType") {
		t.Error("IndicatorRules should explain condType")
	}
}

func TestPromptBuilder_BuildStrategyPrompt(t *testing.T) {
	svc := NewPromptBuilderService()
	ctx := PromptContext{
		SystemPrompt: "",
		Indicators:   "| 字段名 | 标签 | 类别 |\n| rsi | RSI | 技术面 |",
		StrategyName: "测试策略",
		Description:  "基于RSI超卖的买入策略",
		Style:        "conservative",
	}

	prompt := svc.BuildStrategyPrompt(ctx)
	if !strings.Contains(prompt, "量化策略专家") {
		t.Error("should contain default system prompt")
	}
	if !strings.Contains(prompt, "测试策略") {
		t.Error("should contain strategy name")
	}
	if !strings.Contains(prompt, "基于RSI超卖的买入策略") {
		t.Error("should contain strategy description")
	}
	if !strings.Contains(prompt, "conservative") {
		t.Error("should contain style")
	}
}

func TestPromptBuilder_BuildStrategyPrompt_CustomSystem(t *testing.T) {
	svc := NewPromptBuilderService()
	ctx := PromptContext{
		SystemPrompt: "你是一个价值投资专家",
		Indicators:   "| pe | 市盈率 | 估值 |",
		StrategyName: "价值策略",
		Description:  "低PE买入",
		Style:        "moderate",
	}

	prompt := svc.BuildStrategyPrompt(ctx)
	if !strings.Contains(prompt, "价值投资专家") {
		t.Error("should use custom system prompt")
	}
	if strings.Contains(prompt, "量化策略专家") {
		t.Error("should not contain default role when custom prompt provided")
	}
}

func TestPromptBuilder_BuildAnalysisPrompt(t *testing.T) {
	svc := NewPromptBuilderService()
	prompt := svc.BuildAnalysisPrompt("", "000001", "PE=10, ROE=15%", "这只股票值得买入吗？")
	if !strings.Contains(prompt, "000001") {
		t.Error("should contain stock code")
	}
	if !strings.Contains(prompt, "PE=10") {
		t.Error("should contain context data")
	}
	if !strings.Contains(prompt, "值得买入吗") {
		t.Error("should contain question")
	}
}

func TestPromptBuilder_BuildReviewPrompt(t *testing.T) {
	svc := NewPromptBuilderService()
	summary := "总收益: 15%\n胜率: 60%\n最大回撤: -12%\n夏普: 1.5"
	prompt := svc.BuildReviewPrompt("", "测试策略", summary)
	if !strings.Contains(prompt, "测试策略") {
		t.Error("should contain strategy name")
	}
	if !strings.Contains(prompt, "15%") {
		t.Error("should contain performance summary")
	}
	if !strings.Contains(prompt, "胜率") {
		t.Error("should ask for win rate analysis")
	}
}

// ═══════════════════════════════════════════════════════════════
// OrchestrationService tests
// ═══════════════════════════════════════════════════════════════

func TestOrchestration_DefaultConfig(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := svc.DefaultConfig()
	if cfg.OrchestrationMode != "standard" {
		t.Errorf("mode = %s, want standard", cfg.OrchestrationMode)
	}
	if cfg.MarketPositionBias != 1.0 {
		t.Errorf("bias = %v, want 1.0", cfg.MarketPositionBias)
	}
}

func TestOrchestration_Validate_Valid(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := svc.DefaultConfig()
	issues := svc.Validate(cfg)
	if len(issues) != 0 {
		t.Errorf("default config should be valid, got: %v", issues)
	}
}

func TestOrchestration_Validate_InvalidMode(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := svc.DefaultConfig()
	cfg.OrchestrationMode = "invalid_mode"
	issues := svc.Validate(cfg)
	if len(issues) == 0 {
		t.Error("should report invalid orchestration mode")
	}
}

func TestOrchestration_Validate_InvalidComposite(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := svc.DefaultConfig()
	cfg.MarketCompositeMin = 150
	issues := svc.Validate(cfg)
	if len(issues) == 0 {
		t.Error("should report invalid marketCompositeMin")
	}
}

func TestOrchestration_Validate_AgentMode(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := svc.DefaultConfig()
	cfg.EnableAIAgent = true
	cfg.AIAgentMode = "bad_mode"
	issues := svc.Validate(cfg)
	if len(issues) == 0 {
		t.Error("should report invalid AI agent mode")
	}
}

func TestOrchestration_IsDefensiveMode(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := OrchestrationConfig{
		EnableMarketContext: true,
		MarketCompositeMin:  40,
	}

	if !svc.IsDefensiveMode(30, cfg) {
		t.Error("marketComposite=30 should be defensive when threshold=40")
	}
	if svc.IsDefensiveMode(50, cfg) {
		t.Error("marketComposite=50 should NOT be defensive when threshold=40")
	}
	if svc.IsDefensiveMode(30, OrchestrationConfig{EnableMarketContext: false}) {
		t.Error("should not be defensive when market context is disabled")
	}
}

func TestOrchestration_GetPositionBias(t *testing.T) {
	svc := NewOrchestrationService()
	cfg := OrchestrationConfig{
		EnableMarketContext: true,
		MarketCompositeMin:  40,
		MarketPositionBias:  1.0,
		DefensiveThreshold:  30,
	}

	// Neutral market (composite=60, threshold=40)
	bias := svc.GetPositionBias(60, cfg, 80)
	if bias != 1.0 {
		t.Errorf("neutral bias = %v, want 1.0", bias)
	}

	// Defensive: composite=20, DefensiveThreshold=30 → ratio=0.667 → bias=0.667
	bias = svc.GetPositionBias(20, cfg, 80)
	if math.Abs(bias-0.667) > 0.01 {
		t.Errorf("defensive bias = %v, want ~0.667", bias)
	}

	// Extreme defensive: composite=0
	bias = svc.GetPositionBias(0, cfg, 80)
	if math.Abs(bias-0.0) > 0.001 {
		t.Errorf("extreme defensive bias = %v, want 0", bias)
	}

	// No market context: return default bias
	cfg.EnableMarketContext = false
	bias = svc.GetPositionBias(10, cfg, 80)
	if bias != 1.0 {
		t.Errorf("no context bias = %v, want 1.0", bias)
	}
}
