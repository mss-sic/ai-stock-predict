package handler

import (
	"testing"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// Test: checkOp — operator comparison used throughout backtest
// ═══════════════════════════════════════════════════════════════

func TestCheckOpBacktest(t *testing.T) {
	tests := []struct {
		name      string
		val       float64
		op        string
		threshold float64
		expected  bool
	}{
		{"checkop_gte_greater", 10, "gte", 5, true},
		{"checkop_gte_equal", 5, "gte", 5, true},
		{"checkop_gte_false", 3, "gte", 5, false},
		{"checkop_gte_neg", -3, "gte", -5, true},
		{"checkop_lte_less", 3, "lte", 5, true},
		{"checkop_lte_equal", 5, "lte", 5, true},
		{"checkop_lte_false", 10, "lte", 5, false},
		{"checkop_gt_true", 10, "gt", 5, true},
		{"checkop_gt_false_eq", 5, "gt", 5, false},
		{"checkop_gt_false", 3, "gt", 5, false},
		{"checkop_lt_true", 3, "lt", 5, true},
		{"checkop_lt_false_eq", 5, "lt", 5, false},
		{"checkop_lt_false", 10, "lt", 5, false},
		{"checkop_eq_true", 5, "eq", 5, true},
		{"checkop_eq_false", 5.1, "eq", 5, false},
		{"checkop_eq_zero", 0, "eq", 0, true},
		{"checkop_cross_up_pos", 1, "cross_up", 0, true},
		{"checkop_cross_up_zero", 0, "cross_up", 0, false},
		{"checkop_cross_up_neg", -1, "cross_up", 0, false},
		{"checkop_cross_down_neg", -1, "cross_down", 0, true},
		{"checkop_cross_down_zero", 0, "cross_down", 0, false},
		{"checkop_cross_down_pos", 1, "cross_down", 0, false},
		{"checkop_unknown", 5, "unknown", 5, false},
		{"checkop_empty", 5, "", 5, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkOp(tt.val, tt.op, tt.threshold)
			if got != tt.expected {
				t.Errorf("checkOp(%v, %q, %v) = %v, want %v",
					tt.val, tt.op, tt.threshold, got, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: filterConds — filter conditions by type + enabled
// ═══════════════════════════════════════════════════════════════

func TestFilterConds(t *testing.T) {
	conds := []model.StrategyCondition{
		{ID: 1, CondType: "buy", Indicator: "rsi", Enabled: true},
		{ID: 2, CondType: "buy", Indicator: "macd", Enabled: true},
		{ID: 3, CondType: "buy", Indicator: "ma_cross", Enabled: false},
		{ID: 4, CondType: "sell", Indicator: "rsi", Enabled: true},
		{ID: 5, CondType: "add", Indicator: "momentum_5", Enabled: true},
		{ID: 6, CondType: "reduce", Indicator: "drawdown_20", Enabled: true},
		{ID: 7, CondType: "sell", Indicator: "macd", Enabled: false},
	}

	buy := filterConds(conds, "buy")
	if len(buy) != 2 {
		t.Errorf("filterConds(buy) = %d, want 2", len(buy))
	}
	for _, c := range buy {
		if !c.Enabled || c.CondType != "buy" {
			t.Errorf("filterConds(buy) includes disabled/wrong-type: id=%d", c.ID)
		}
	}

	sell := filterConds(conds, "sell")
	if len(sell) != 1 {
		t.Errorf("filterConds(sell) = %d, want 1", len(sell))
	}
	if sell[0].ID != 4 {
		t.Errorf("filterConds(sell)[0].ID = %d, want 4", sell[0].ID)
	}

	none := filterConds(conds, "nonexistent")
	if len(none) != 0 {
		t.Errorf("filterConds(nonexistent) = %d, want 0", len(none))
	}

	empty := filterConds(nil, "buy")
	if len(empty) != 0 {
		t.Errorf("filterConds(nil, buy) = %d, want 0", len(empty))
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: evaluateConditions — empty/short-circuit paths
// ═══════════════════════════════════════════════════════════════

func TestEvaluateConditions_EmptyInput(t *testing.T) {
	result := evaluateConditions(nil, "000001", "2024-01-01")
	if result {
		t.Error("evaluateConditions(nil) should return false")
	}
	result = evaluateConditions([]model.StrategyCondition{}, "000001", "2024-01-01")
	if result {
		t.Error("evaluateConditions(empty) should return false")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: getPrevClose
// ═══════════════════════════════════════════════════════════════

func TestGetPrevClose_NoCache(t *testing.T) {
	result := getPrevClose(nil, "000001", "2024-01-01")
	if result != 0 {
		t.Errorf("getPrevClose(nil) = %v, want 0", result)
	}
}

func TestGetPrevClose_WithCache(t *testing.T) {
	kc := &KlineCache{
		dates:    []string{"2024-01-02", "2024-01-03", "2024-01-04"},
		dateIdx:  map[string]int{"2024-01-02": 0, "2024-01-03": 1, "2024-01-04": 2},
		closeMap: map[string][]float64{"000001": {10.0, 10.5, 11.0}},
	}

	got := getPrevClose(kc, "000001", "2024-01-03")
	if got != 10.0 {
		t.Errorf("getPrevClose(..., 2024-01-03) = %v, want 10.0", got)
	}

	got = getPrevClose(kc, "000001", "2024-01-02")
	if got != 0 {
		t.Errorf("getPrevClose(..., 2024-01-02) = %v, want 0", got)
	}

	got = getPrevClose(kc, "999999", "2024-01-03")
	if got != 0 {
		t.Errorf("getPrevClose(unknown code) = %v, want 0", got)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: getCloseNDaysAgo
// ═══════════════════════════════════════════════════════════════

func TestGetCloseNDaysAgo_NoCache(t *testing.T) {
	result := getCloseNDaysAgo(nil, "000001", "2024-01-01", 5)
	if result != 0 {
		t.Errorf("getCloseNDaysAgo(nil) = %v, want 0", result)
	}
}

func TestGetCloseNDaysAgo_WithCache(t *testing.T) {
	kc := &KlineCache{
		dates:    []string{"2024-01-02", "2024-01-03", "2024-01-04", "2024-01-05", "2024-01-08"},
		dateIdx:  map[string]int{"2024-01-02": 0, "2024-01-03": 1, "2024-01-04": 2, "2024-01-05": 3, "2024-01-08": 4},
		closeMap: map[string][]float64{"000001": {10.0, 10.5, 11.0, 10.8, 11.2}},
	}

	got := getCloseNDaysAgo(kc, "000001", "2024-01-05", 2)
	if got != 10.5 {
		t.Errorf("getCloseNDaysAgo(2 days) = %v, want 10.5", got)
	}

	got = getCloseNDaysAgo(kc, "000001", "2024-01-08", 5)
	if got != 0 {
		t.Errorf("getCloseNDaysAgo(5 days, insufficient) = %v, want 0", got)
	}

	got = getCloseNDaysAgo(kc, "000001", "2024-01-04", 0)
	if got != 11.0 {
		t.Errorf("getCloseNDaysAgo(0 days) = %v, want 11.0", got)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: KlineCache GetClose
// ═══════════════════════════════════════════════════════════════

func TestKlineCache_GetClose(t *testing.T) {
	kc := &KlineCache{
		dateIdx:  map[string]int{"2024-01-03": 1},
		closeMap: map[string][]float64{"000001": {10.0, 10.5}},
	}

	if kc.GetClose("000001", "2024-01-03") != 10.5 {
		t.Error("GetClose should return correct value")
	}
	if kc.GetClose("000002", "2024-01-03") != 0 {
		t.Error("GetClose for unknown code should return 0")
	}
	if kc.GetClose("000001", "2024-12-31") != 0 {
		t.Error("GetClose for unknown date should return 0")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: IndicatorCache get (key = code|date)
// ═══════════════════════════════════════════════════════════════

func TestIndicatorCacheGet(t *testing.T) {
	ic := &IndicatorCache{
		data: map[string]map[string]float64{
			"rsi": {"000001|2024-01-03": 65.5},
		},
	}

	val, ok := ic.get("rsi", "000001", "2024-01-03")
	if !ok || val != 65.5 {
		t.Errorf("get(rsi) = (%v, %v), want (65.5, true)", val, ok)
	}

	_, ok = ic.get("rsi", "000001", "2024-12-31")
	if ok {
		t.Error("get for unknown date should return false")
	}

	_, ok = ic.get("rsi", "999999", "2024-01-03")
	if ok {
		t.Error("get for unknown code should return false")
	}
}

func TestIndicatorCacheHasData(t *testing.T) {
	ic := &IndicatorCache{
		data:             map[string]map[string]float64{"pe": {"000001|2024-01-03": 15.2}},
		hasIndicatorData: map[string]map[string]bool{"pe": {"000001": true}},
	}

	if !ic.HasData("pe", "000001") {
		t.Error("HasData(pe, 000001) should be true")
	}
	if ic.HasData("pe", "000002") {
		t.Error("HasData(pe, 000002) should be false")
	}
	if ic.HasData("pb", "000001") {
		t.Error("HasData(pb, 000001) should be false")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: toFloat64 type conversion
// ═══════════════════════════════════════════════════════════════

func TestToFloat64Conversion(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected float64
		ok       bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(42), 42, true},
		{"int64", int64(100), 100, true},
		{"string", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.ok || got != tt.expected {
				t.Errorf("toFloat64(%v) = (%v, %v), want (%v, %v)",
					tt.input, got, ok, tt.expected, tt.ok)
			}
		})
	}
}
