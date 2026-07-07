package service

import (
	"math"
	"testing"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// SignalService tests
// ═══════════════════════════════════════════════════════════════

func TestGroupConditions(t *testing.T) {
	svc := NewSignalService()
	conds := []model.StrategyCondition{
		{ID: 1, CondType: "buy", Indicator: "rsi", LogicGroup: 1, Enabled: true},
		{ID: 2, CondType: "buy", Indicator: "macd", LogicGroup: 1, Enabled: true},
		{ID: 3, CondType: "buy", Indicator: "ma_cross", LogicGroup: 2, Enabled: true},
		{ID: 4, CondType: "buy", Indicator: "momentum_5", LogicGroup: 1, Enabled: false},
	}

	groups := svc.GroupConditions(conds)
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
	if len(groups[1]) != 2 {
		t.Errorf("group 1: expected 2 enabled, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("group 2: expected 1, got %d", len(groups[2]))
	}
}

func TestEvaluateGroup_AllPass(t *testing.T) {
	svc := NewSignalService()
	group := []model.StrategyCondition{
		{Indicator: "rsi", Operator: "gt", Value: 30},
		{Indicator: "momentum_5", Operator: "gt", Value: 2},
	}
	values := map[string]float64{"rsi": 65, "momentum_5": 5.5}

	passed, details := svc.EvaluateGroup(group, values)
	if !passed {
		t.Error("expected all conditions to pass")
	}
	if len(details) != 2 {
		t.Errorf("expected 2 details, got %d", len(details))
	}
}

func TestEvaluateGroup_OneFails(t *testing.T) {
	svc := NewSignalService()
	group := []model.StrategyCondition{
		{Indicator: "rsi", Operator: "gt", Value: 30},
		{Indicator: "momentum_5", Operator: "gt", Value: 2},
	}
	values := map[string]float64{"rsi": 65, "momentum_5": -1.0}

	passed, _ := svc.EvaluateGroup(group, values)
	if passed {
		t.Error("expected failure on momentum_5")
	}
}

func TestEvaluateGroup_MissingIndicator(t *testing.T) {
	svc := NewSignalService()
	group := []model.StrategyCondition{
		{Indicator: "pe", Operator: "lt", Value: 20},
	}
	values := map[string]float64{} // PE not provided

	passed, details := svc.EvaluateGroup(group, values)
	if passed {
		t.Error("expected failure for missing indicator")
	}
	if len(details) == 0 || details[0] != "pe=N/A" {
		t.Errorf("expected 'pe=N/A', got %v", details)
	}
}

func TestEvaluateAnyGroup_FirstGroupPasses(t *testing.T) {
	svc := NewSignalService()
	groups := map[int][]model.StrategyCondition{
		1: {{Indicator: "rsi", Operator: "gt", Value: 30}},
		2: {{Indicator: "momentum_5", Operator: "lt", Value: -10}}, // won't even be checked
	}
	values := map[string]float64{"rsi": 65, "momentum_5": 5.0}

	result := svc.EvaluateAnyGroup(groups, values)
	if !result.Passed {
		t.Error("first group should pass")
	}
}

func TestEvaluateAnyGroup_SecondGroupPasses(t *testing.T) {
	svc := NewSignalService()
	groups := map[int][]model.StrategyCondition{
		1: {{Indicator: "rsi", Operator: "gt", Value: 90}}, // fails
		2: {{Indicator: "momentum_5", Operator: "gt", Value: 2}},
	}
	values := map[string]float64{"rsi": 65, "momentum_5": 3.0}

	result := svc.EvaluateAnyGroup(groups, values)
	if !result.Passed {
		t.Error("second group should pass")
	}
}

func TestEvaluateAnyGroup_AllFail(t *testing.T) {
	svc := NewSignalService()
	groups := map[int][]model.StrategyCondition{
		1: {{Indicator: "rsi", Operator: "gt", Value: 90}},
		2: {{Indicator: "momentum_5", Operator: "gt", Value: 10}},
	}
	values := map[string]float64{"rsi": 65, "momentum_5": 3.0}

	result := svc.EvaluateAnyGroup(groups, values)
	if result.Passed {
		t.Error("all groups should fail")
	}
}

func TestFilterByType(t *testing.T) {
	svc := NewSignalService()
	conds := []model.StrategyCondition{
		{ID: 1, CondType: "buy", Enabled: true},
		{ID: 2, CondType: "buy", Enabled: false},
		{ID: 3, CondType: "sell", Enabled: true},
		{ID: 4, CondType: "add", Enabled: true},
	}

	buy := svc.FilterByType(conds, "buy")
	if len(buy) != 1 {
		t.Errorf("buy: expected 1, got %d", len(buy))
	}
	sell := svc.FilterByType(conds, "sell")
	if len(sell) != 1 {
		t.Errorf("sell: expected 1, got %d", len(sell))
	}
}

// ═══════════════════════════════════════════════════════════════
// ScoringService tests
// ═══════════════════════════════════════════════════════════════

func TestAdaptiveMinScore_Normal(t *testing.T) {
	d := ScoreDistribution{Count: 100, Top1: 0.95, Median: 0.55, Mean: 0.58}
	min := d.AdaptiveMinScore()
	expected := 0.75 // 0.55 + (0.95-0.55)*0.5
	if math.Abs(min-expected) > 0.001 {
		t.Errorf("AdaptiveMinScore = %v, want ~%v", min, expected)
	}
}

func TestAdaptiveMinScore_ZeroCount(t *testing.T) {
	d := ScoreDistribution{}
	min := d.AdaptiveMinScore()
	if min != 0.30 {
		t.Errorf("AdaptiveMinScore(empty) = %v, want 0.30", min)
	}
}

func TestAdaptiveMinScore_Floor(t *testing.T) {
	d := ScoreDistribution{Count: 50, Top1: 0.35, Median: 0.33}
	min := d.AdaptiveMinScore()
	if min < 0.30 {
		t.Errorf("should respect floor: got %v", min)
	}
}

func TestComputeDistribution(t *testing.T) {
	svc := NewScoringService()
	results := []ScoreResult{
		{Code: "A", TotalScore: 4.8},
		{Code: "B", TotalScore: 4.2},
		{Code: "C", TotalScore: 3.5},
		{Code: "D", TotalScore: 2.8},
		{Code: "E", TotalScore: 1.5},
	}

	dist := svc.ComputeDistribution(results)
	if dist.Count != 5 {
		t.Errorf("count = %d, want 5", dist.Count)
	}
	if dist.Top1 != 4.8 {
		t.Errorf("top1 = %v, want 4.8", dist.Top1)
	}
	if dist.Median != 3.5 {
		t.Errorf("median = %v, want 3.5", dist.Median)
	}
}

func TestRankResults(t *testing.T) {
	svc := NewScoringService()
	results := []ScoreResult{
		{Code: "B", TotalScore: 4.2},
		{Code: "A", TotalScore: 4.8},
		{Code: "C", TotalScore: 1.5},
	}

	svc.RankResults(results)
	if results[0].Rank != 1 {
		t.Errorf("rank[0] = %d, want 1", results[0].Rank)
	}
	if results[1].Rank != 2 {
		t.Errorf("rank[1] = %d, want 2", results[1].Rank)
	}
}

func TestFilterByMinScore(t *testing.T) {
	svc := NewScoringService()
	results := []ScoreResult{
		{Code: "A", TotalScore: 4.8},
		{Code: "B", TotalScore: 2.5},
		{Code: "C", TotalScore: 1.5},
	}
	filtered := svc.FilterByMinScore(results, 3.0)
	if len(filtered) != 1 {
		t.Errorf("filtered count = %d, want 1", len(filtered))
	}
	if filtered[0].Code != "A" {
		t.Errorf("filtered[0].Code = %s, want A", filtered[0].Code)
	}
}

func TestComputeWeightedScore(t *testing.T) {
	svc := NewScoringService()

	// Binary: RSI=65 > 30, weight=1.0 → 1.0
	score := svc.ComputeWeightedScore(65, 30, "gt", 1.0, 0)
	if score != 1.0 {
		t.Errorf("binary score = %v, want 1.0", score)
	}

	// Binary fail: RSI=20 > 30 → 0
	score = svc.ComputeWeightedScore(20, 30, "gt", 1.0, 0)
	if score != 0.0 {
		t.Errorf("binary fail score = %v, want 0.0", score)
	}

	// Fuzzy: RSI=32 > 30, sigma=5, weight=1.0 → ~0.6
	score = svc.ComputeWeightedScore(32, 30, "gt", 1.0, 5.0)
	if score < 0.55 || score > 0.65 {
		t.Errorf("fuzzy score = %v, want ~0.6", score)
	}

	// Weighted: RSI=65 > 30, weight=2.0 → 2.0
	score = svc.ComputeWeightedScore(65, 30, "gt", 2.0, 0)
	if score != 2.0 {
		t.Errorf("weighted score = %v, want 2.0", score)
	}
}

func TestComputeTotalScore(t *testing.T) {
	svc := NewScoringService()
	conds := []ConditionWeight{
		{Indicator: "rsi", Operator: "gt", Threshold: 30, Weight: 1.0},
		{Indicator: "momentum_5", Operator: "gt", Threshold: 2, Weight: 1.5},
		{Indicator: "pe", Operator: "lt", Threshold: 20, Weight: 1.0},
	}
	values := map[string]float64{"rsi": 65, "momentum_5": 5.0, "pe": 15}

	total := svc.ComputeTotalScore(conds, values)
	if total != 3.5 {
		t.Errorf("total score = %v, want 3.5", total)
	}
}

func TestBuildConditionWeights(t *testing.T) {
	svc := NewScoringService()
	conds := []model.StrategyCondition{
		{ID: 1, Indicator: "rsi", Operator: "gt", Value: 30, Weight: 1.5, Enabled: true},
		{ID: 2, Indicator: "macd", Operator: "cross_up", Value: 0, Weight: 0, Enabled: true}, // 0 weight → default to 1.0
		{ID: 3, Indicator: "pe", Operator: "lt", Value: 20, Weight: 2.0, Enabled: false},
	}

	weights := svc.BuildConditionWeights(conds)
	if len(weights) != 2 {
		t.Errorf("expected 2 enabled weights, got %d", len(weights))
	}
	if weights[0].Weight != 1.5 {
		t.Errorf("weights[0].Weight = %v, want 1.5", weights[0].Weight)
	}
	if weights[1].Weight != 1.0 {
		t.Errorf("weights[1].Weight = %v, want 1.0 (default)", weights[1].Weight)
	}
}

func TestComputeFuzzyScore(t *testing.T) {
	svc := NewSignalService()

	// Value exactly at threshold → ~0.5
	score := svc.ComputeFuzzyScore(30, 30, "gt", 5.0)
	if math.Abs(score-0.5) > 0.01 {
		t.Errorf("at threshold: score = %v, want ~0.5", score)
	}

	// Value far above threshold → ~1.0
	score = svc.ComputeFuzzyScore(50, 30, "gt", 5.0)
	if score < 0.98 {
		t.Errorf("far above: score = %v, want ~1.0", score)
	}

	// Value below threshold → ~0.0
	score = svc.ComputeFuzzyScore(10, 30, "gt", 5.0)
	if score > 0.02 {
		t.Errorf("below: score = %v, want ~0.0", score)
	}

	// Binary mode (sigma=0): above threshold
	score = svc.ComputeFuzzyScore(65, 30, "gt", 0)
	if score != 1.0 {
		t.Errorf("binary above: score = %v, want 1.0", score)
	}

	// Binary mode (sigma=0): below threshold
	score = svc.ComputeFuzzyScore(20, 30, "gt", 0)
	if score != 0.0 {
		t.Errorf("binary below: score = %v, want 0.0", score)
	}
}
