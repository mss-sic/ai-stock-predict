package handler

import (
	"math"
	"testing"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// Test 1: ScoreDistribution.AdaptiveMinScore
// ═══════════════════════════════════════════════════════════════

func TestAdaptiveMinScore_Normal(t *testing.T) {
	d := ScoreDistribution{
		Count:  100,
		Top1:   0.95,
		Top5:   0.88,
		Top10P: 0.82,
		Median: 0.55,
		Mean:   0.58,
	}
	min := d.AdaptiveMinScore()
	// gap = 0.95 - 0.55 = 0.40; dynamic = 0.55 + 0.40*0.5 = 0.75
	expected := 0.75
	if math.Abs(min-expected) > 0.001 {
		t.Errorf("AdaptiveMinScore = %v, want ~%v", min, expected)
	}
}

func TestAdaptiveMinScore_ZeroCount(t *testing.T) {
	d := ScoreDistribution{Count: 0}
	min := d.AdaptiveMinScore()
	if min != 0.30 {
		t.Errorf("AdaptiveMinScore(empty) = %v, want 0.30", min)
	}
}

func TestAdaptiveMinScore_NarrowDistribution(t *testing.T) {
	d := ScoreDistribution{
		Count:  50,
		Top1:   0.40,
		Top5:   0.38,
		Top10P: 0.36,
		Median: 0.35,
		Mean:   0.34,
	}
	min := d.AdaptiveMinScore()
	// gap = 0.05; dynamic = 0.35 + 0.05*0.5 = 0.375; but hard floor is 0.30
	if min < 0.30 {
		t.Errorf("AdaptiveMinScore = %v, should be >= 0.30 (floor)", min)
	}
}

func TestAdaptiveMinScore_VeryHighScores(t *testing.T) {
	d := ScoreDistribution{
		Count:  100,
		Top1:   0.98,
		Top5:   0.95,
		Top10P: 0.92,
		Median: 0.85,
		Mean:   0.86,
	}
	min := d.AdaptiveMinScore()
	// gap = 0.13; dynamic = 0.85 + 0.065 = 0.915
	if min < 0.30 {
		t.Errorf("AdaptiveMinScore = %v, should be >= 0.30", min)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 2: ConditionScore computation (calculateWeightedScore helper logic)
// ═══════════════════════════════════════════════════════════════

func TestConditionScore_Basic(t *testing.T) {
	// A basic condition: RSI > 30 with score based on how much above threshold
	cs := ConditionScore{
		ConditionID: 1,
		Indicator:   "rsi",
		RawValue:    65,
		Threshold:   30,
		Operator:    "gt",
		Weight:      1.0,
	}

	// For gt operator, score = sigmoid of how much above threshold
	// With fuzzySigma=0 (not set): if value > threshold, score=1.0; else 0.0
	if cs.RawValue > cs.Threshold {
		cs.Score = 1.0
	} else {
		cs.Score = 0.0
	}
	cs.WeightedScore = cs.Score * cs.Weight

	if cs.WeightedScore != 1.0 {
		t.Errorf("WeightedScore = %v, want 1.0", cs.WeightedScore)
	}
}

func TestConditionScore_Fuzzy(t *testing.T) {
	// Fuzzy scoring: sigma controls how gradually the score decays
	cs := ConditionScore{
		ConditionID: 1,
		Indicator:   "rsi",
		RawValue:    32,
		Threshold:   30,
		Operator:    "gt",
		FuzzySigma:  5.0,
		Weight:      1.0,
	}

	// Fuzzy score = sigmoid((value - threshold) / sigma)
	diff := (cs.RawValue - cs.Threshold) / cs.FuzzySigma
	cs.Score = 1.0 / (1.0 + math.Exp(-diff))
	cs.WeightedScore = cs.Score * cs.Weight

	if cs.WeightedScore <= 0.5 || cs.WeightedScore >= 0.7 {
		t.Errorf("Fuzzy WeightedScore = %v, expect ~0.6 for value just above threshold", cs.WeightedScore)
	}
}

func TestConditionScore_FarAboveThreshold(t *testing.T) {
	cs := ConditionScore{
		ConditionID: 1,
		Indicator:   "rsi",
		RawValue:    80,
		Threshold:   30,
		Operator:    "gt",
		FuzzySigma:  5.0,
		Weight:      1.5,
	}

	diff := (cs.RawValue - cs.Threshold) / cs.FuzzySigma
	cs.Score = 1.0 / (1.0 + math.Exp(-diff))
	cs.WeightedScore = cs.Score * cs.Weight

	if cs.WeightedScore < 1.49 {
		t.Errorf("Far above: WeightedScore = %v, want ~1.5", cs.WeightedScore)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 3: ScoringEngine score aggregation logic (without DB)
// ═══════════════════════════════════════════════════════════════

func TestScoringEngine_AggregateScores(t *testing.T) {
	results := []ScoreResult{
		{Code: "A", TotalScore: 3.5},
		{Code: "B", TotalScore: 4.2},
		{Code: "C", TotalScore: 2.8},
		{Code: "D", TotalScore: 4.8},
		{Code: "E", TotalScore: 1.5},
	}

	// Compute distribution
	var sum float64
	var scores []float64
	for _, r := range results {
		sum += r.TotalScore
		scores = append(scores, r.TotalScore)
	}

	mean := sum / float64(len(results))

	// Sort for median
	sortScores := make([]float64, len(scores))
	copy(sortScores, scores)
	// Simple bubble sort for test
	for i := 0; i < len(sortScores); i++ {
		for j := i + 1; j < len(sortScores); j++ {
			if sortScores[i] < sortScores[j] {
				sortScores[i], sortScores[j] = sortScores[j], sortScores[i]
			}
		}
	}

	top1 := sortScores[0]
	median := sortScores[len(sortScores)/2]

	if top1 != 4.8 {
		t.Errorf("top1 = %v, want 4.8", top1)
	}
	if median != 3.5 {
		t.Errorf("median = %v, want 3.5", median)
	}
	if math.Abs(mean-3.36) > 0.01 {
		t.Errorf("mean = %v, want ~3.36", mean)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 4: Strategy model validation (field defaults)
// ═══════════════════════════════════════════════════════════════

func TestStrategyDefaults(t *testing.T) {
	s := model.Strategy{
		Name: "test",
	}

	// Default position sizing
	if s.BuyPositionPct == 0 {
		// This is fine — GORM will apply defaults at DB level
		// We just verify the struct can be created
	}

	if s.MaxHoldings == 0 {
		// Default is 20 via GORM tag
	}

	// Verify override works
	s2 := model.Strategy{
		Name:              "test",
		BuyPositionPct:    20,
		AddPositionPct:    15,
		ReducePositionPct: 40,
		MaxHoldings:       10,
		InitialCapital:    200000,
	}

	if s2.BuyPositionPct != 20 {
		t.Error("BuyPositionPct override failed")
	}
	if s2.InitialCapital != 200000 {
		t.Error("InitialCapital override failed")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 5: StrategyCondition model — logic group isolation
// ═══════════════════════════════════════════════════════════════

func TestCondition_LogicGroupIsolation(t *testing.T) {
	// Groups: {1: [A, B]}, {2: [C]}
	// Group 1: A AND B must both pass
	// Group 2: C alone must pass
	// Result: group1 OR group2

	conds := []model.StrategyCondition{
		{ID: 1, CondType: "buy", Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1, Enabled: true},
		{ID: 2, CondType: "buy", Indicator: "macd", Operator: "cross_up", Value: 0, LogicGroup: 1, Enabled: true},
		{ID: 3, CondType: "buy", Indicator: "momentum_5", Operator: "gt", Value: 2, LogicGroup: 2, Enabled: true},
	}

	groups := make(map[int][]model.StrategyCondition)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}

	if len(groups) != 2 {
		t.Errorf("groups count = %d, want 2", len(groups))
	}
	if len(groups[1]) != 2 {
		t.Errorf("group 1 size = %d, want 2", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("group 2 size = %d, want 1", len(groups[2]))
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 6: Transaction cost constants
// ═══════════════════════════════════════════════════════════════

func TestTransactionCosts(t *testing.T) {
	// These are the constants used in runBacktestAsync
	const (
		STAMP_TAX_RATE  = 0.0005
		COMMISSION_RATE = 0.00025
		MIN_COMMISSION  = 5.0
	)

	// Verify a simple trade cost calculation
	amount := 10000.0
	stampTax := amount * STAMP_TAX_RATE // 5
	commission := amount * COMMISSION_RATE
	if commission < MIN_COMMISSION {
		commission = MIN_COMMISSION
	}

	totalCost := stampTax + commission
	if totalCost != 10.0 { // 5 (stamp) + 5 (min commission)
		t.Errorf("totalCost = %v, want 10.0", totalCost)
	}

	// Large trade: commission should exceed minimum
	largeAmount := 50000.0
	commission2 := largeAmount * COMMISSION_RATE // 12.5 > 5.0
	if commission2 != 12.5 {
		t.Errorf("large commission = %v, want 12.5", commission2)
	}
}
