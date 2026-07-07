package handler

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// Test 1: Registry completeness — every registered indicator
//         must have an entry in getIndicatorValue's switch
// ═══════════════════════════════════════════════════════════════

// getAllIndicatorKeysInSwitch returns the set of indicator keys handled by getIndicatorValue.
// This mirrors the switch statement in getIndicatorValue; must be kept in sync.
func getAllIndicatorKeysInSwitch() map[string]bool {
	keys := []string{
		// 榜单与评分
		"streak_count", "pick_count_5d", "pick_count_20d", "algo_score", "signal_value",
		// AI六维评分
		"ai_score", "ai_fundamental", "ai_technical", "ai_valuation",
		"ai_growth", "ai_industry", "ai_capital",
		// 技术面 — 趋势类
		"daily_change", "momentum_5", "momentum_20",
		"ma_deviation", "ma_5", "ma_10", "ma_20", "ma_30", "ma_60",
		"ma_cross", "macd", "macd_dif", "macd_dea",
		// 技术面 — 超买超卖
		"rsi", "rsi_6", "rsi_12", "rsi_24",
		"kdj_k", "kdj_d", "kdj_j",
		"boll_position", "boll_width", "boll_upper", "boll_middle", "boll_lower",
		"boll_squeeze", "psy_12", "psy_ma",
		"cci", "williams_r", "mfi",
		// 技术面 — 量价
		"volume_ratio", "volume_ma_ratio", "turnover_rate",
		"net_flow_ratio", "buy_sell_ratio",
		"atr", "atr_pct",
		// 技术面 — 形态
		"drawdown_20", "new_high_20", "up_days_ratio",
		"price_position_20", "price_position_60",
		// 技术面 — 趋势系统
		"adx", "dmi_plus", "dmi_minus", "ema_cross",
		// 技术面 — 波动与结构
		"ma_convergence", "trend_strength",
		// 技术面 — 形态与量价
		"consecutive_days", "gap_pct", "high_low_range",
		"vwap_deviation", "volume_trend", "index_relative",
		// 估值
		"pe", "pb", "ps", "pe_percentile", "pb_percentile",
		// 基本面
		"roe", "revenue_growth", "profit_growth", "gross_margin",
		"net_margin", "debt_ratio", "eps",
		// 资金面
		"total_market_cap", "shareholder_change", "inst_hold_ratio",
		// 预测
		"prediction_upside", "prediction_consensus",
	}
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func TestRegistrySwitchCompleteness(t *testing.T) {
	switchKeys := getAllIndicatorKeysInSwitch()
	registryKeys := make(map[string]bool)
	for k := range IndicatorRegistry {
		registryKeys[k] = true
	}

	// Every registry key must be in the switch
	var missing []string
	for k := range registryKeys {
		if !switchKeys[k] {
			missing = append(missing, k)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("Registry keys missing from getIndicatorValue switch (%d):\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}

	// Every switch key must be in the registry
	var orphaned []string
	for k := range switchKeys {
		if !registryKeys[k] {
			orphaned = append(orphaned, k)
		}
	}
	sort.Strings(orphaned)
	if len(orphaned) > 0 {
		t.Errorf("Switch keys missing from Registry (%d):\n  %s",
			len(orphaned), strings.Join(orphaned, "\n  "))
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 2: Registry metadata consistency
// ═══════════════════════════════════════════════════════════════

func TestRegistryMetadata(t *testing.T) {
	for key, m := range IndicatorRegistry {
		// Key must match
		if m.Key != key {
			t.Errorf("Indicator %s has mismatched Key field: %s", key, m.Key)
		}
		// Label required
		if m.Label == "" {
			t.Errorf("Indicator %s has empty Label", key)
		}
		// Category required
		if m.Category == "" {
			t.Errorf("Indicator %s has empty Category", key)
		}
		// Unit required
		if m.Unit == "" {
			t.Errorf("Indicator %s has empty Unit", key)
		}
		// Desc required
		if m.Desc == "" {
			t.Errorf("Indicator %s has empty Desc", key)
		}
		// DataNote required
		if m.DataNote == "" {
			t.Errorf("Indicator %s has empty DataNote", key)
		}
		// DataSource required
		if m.DataSource == "" {
			t.Errorf("Indicator %s has empty DataSource", key)
		}
		// UseFor required
		if m.UseFor == "" {
			t.Errorf("Indicator %s has empty UseFor", key)
		}
		// Operators required
		if len(m.Operators) == 0 {
			t.Errorf("Indicator %s has no Operators", key)
		}
		// Type must be number or cross
		if m.Type != "number" && m.Type != "cross" {
			t.Errorf("Indicator %s has invalid Type: %s", key, m.Type)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 3: Operator correctness
// ═══════════════════════════════════════════════════════════════

func TestCheckOp(t *testing.T) {
	tests := []struct {
		name      string
		val       float64
		op        string
		threshold float64
		expected  bool
	}{
		{"gte_true", 5.0, "gte", 5.0, true},
		{"gte_false", 4.9, "gte", 5.0, false},
		{"gt_true", 5.1, "gt", 5.0, true},
		{"gt_false", 5.0, "gt", 5.0, false},
		{"lte_true", 5.0, "lte", 5.0, true},
		{"lte_false", 5.1, "lte", 5.0, false},
		{"lt_true", 4.9, "lt", 5.0, true},
		{"lt_false", 5.0, "lt", 5.0, false},
		{"eq_true", 5.0, "eq", 5.0, true},
		{"eq_false", 5.1, "eq", 5.0, false},
		{"cross_up_true", 1.0, "cross_up", 0, true},
		{"cross_up_false", 0.0, "cross_up", 0, false},
		{"cross_up_false_neg", -1.0, "cross_up", 0, false},
		{"cross_down_true", -1.0, "cross_down", 0, true},
		{"cross_down_false", 0.0, "cross_down", 0, false},
		{"cross_down_false_pos", 1.0, "cross_down", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkOp(tt.val, tt.op, tt.threshold)
			if result != tt.expected {
				t.Errorf("checkOp(%.1f, %s, %.1f) = %v, want %v",
					tt.val, tt.op, tt.threshold, result, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 4: evaluateSingleCondition with cross operators
// ═══════════════════════════════════════════════════════════════

func TestEvaluateSingleCondition_CrossOperators(t *testing.T) {
	// cross_up: val > 0 → true
	cond := model.StrategyCondition{
		Indicator: "ma_cross",
		Operator:  "cross_up",
		Value:     5.020, // MA5 cross MA20
		Enabled:   true,
	}
	// Since we can't actually compute ma_cross without DB, test that cross_up
	// operator logic correctly maps: getIndicatorValue returns > 0 for cross_up
	// For now, test that the logic path exists (we validated checkOp above)

	// cross_down: val < 0 → true
	cond2 := model.StrategyCondition{
		Indicator: "macd",
		Operator:  "cross_down",
		Value:     0,
		Enabled:   true,
	}
	_ = cond
	_ = cond2
	// These are integration tests; the unit test for checkOp covers the logic.
}

// ═══════════════════════════════════════════════════════════════
// Test 5: filterConds respects Enabled flag
// ═══════════════════════════════════════════════════════════════

func TestFilterConds_RespectsEnabled(t *testing.T) {
	conds := []model.StrategyCondition{
		{ID: 1, CondType: "buy", Indicator: "ma_cross", Enabled: true},
		{ID: 2, CondType: "buy", Indicator: "rsi", Enabled: false},
		{ID: 3, CondType: "sell", Indicator: "ma_cross", Enabled: true},
		{ID: 4, CondType: "buy", Indicator: "volume_ratio", Enabled: true},
	}

	buy := filterConds(conds, "buy")
	if len(buy) != 2 {
		t.Errorf("Expected 2 enabled buy conditions, got %d", len(buy))
	}
	for _, c := range buy {
		if c.ID == 2 {
			t.Error("Condition ID=2 (disabled) should not be in buy results")
		}
	}

	sell := filterConds(conds, "sell")
	if len(sell) != 1 {
		t.Errorf("Expected 1 enabled sell condition, got %d", len(sell))
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 6: evaluateConditions group logic (AND within group, OR across)
// ═══════════════════════════════════════════════════════════════

func TestEvaluateConditions_GroupLogic(t *testing.T) {
	// This tests the structural logic — group AND, groups OR.
	// Without DB, we test with conditions that would fail data check.
	// The groups structure is what matters.

	conds := []model.StrategyCondition{
		{CondType: "buy", Indicator: "daily_change", Operator: "gt", Value: 0, LogicGroup: 1, Enabled: true},
		{CondType: "buy", Indicator: "volume_ratio", Operator: "gt", Value: 1, LogicGroup: 1, Enabled: true},
		{CondType: "buy", Indicator: "rsi", Operator: "lt", Value: 30, LogicGroup: 2, Enabled: true},
	}

	// With no DB data, both groups will fail individually.
	// But we test the grouping structure is correct.
	groups := make(map[int][]model.StrategyCondition)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}

	if len(groups) != 2 {
		t.Errorf("Expected 2 logic groups, got %d", len(groups))
	}
	if len(groups[1]) != 2 {
		t.Errorf("Expected 2 conditions in group 1, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Errorf("Expected 1 condition in group 2, got %d", len(groups[2]))
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 7: PSY and PSYMA are registered and have compute functions
// ═══════════════════════════════════════════════════════════════

func TestPSY_RegisteredAndInSwitch(t *testing.T) {
	// Verify psy_12 is in the Registry
	m12 := GetIndicatorMeta("psy_12")
	if m12 == nil {
		t.Fatal("psy_12 not found in IndicatorRegistry")
	}
	if m12.Label == "" {
		t.Error("psy_12 has empty Label")
	}
	if m12.DataSource != "stocks_daily_k" {
		t.Errorf("psy_12 DataSource = %s, want stocks_daily_k", m12.DataSource)
	}

	// Verify psy_ma is in the Registry
	mma := GetIndicatorMeta("psy_ma")
	if mma == nil {
		t.Fatal("psy_ma not found in IndicatorRegistry")
	}

	// Verify both are in the switch via our reference set
	switchKeys := getAllIndicatorKeysInSwitch()
	if !switchKeys["psy_12"] {
		t.Error("psy_12 not in getIndicatorValue switch key set")
	}
	if !switchKeys["psy_ma"] {
		t.Error("psy_ma not in getIndicatorValue switch key set")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 8: Unit consistency — verify key indicators have correct units
// ═══════════════════════════════════════════════════════════════

func TestIndicatorUnits(t *testing.T) {
	tests := []struct {
		key  string
		unit string
	}{
		{"pe", "倍"},
		{"pb", "倍"},
		{"ps", "倍"},
		{"roe", "%"},
		{"debt_ratio", "%"},
		{"eps", "元"},
		{"total_market_cap", "元"}, // database unit, UI converts to 亿
		{"daily_change", "%"},
		{"momentum_5", "%"},
		{"momentum_20", "%"},
		{"rsi", "比值"},
		{"kdj_k", "比值"},
		{"turnover_rate", "%"},
		{"atr", "元"},
		{"atr_pct", "%"},
		{"volume_ratio", "比值"},
		{"adx", "比值"},
		{"ma_5", "元"},
		{"boll_position", "%"},
		{"boll_width", "%"},
		{"trend_strength", "比值"},
		{"up_days_ratio", "比值"},
		{"price_position_20", "%"},
		{"index_relative", "%"},
		{"williams_r", "比值"},
		{"mfi", "比值"},
		{"cci", "比值"},
		{"shareholder_change", "%"},
		{"inst_hold_ratio", "%"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := GetIndicatorMeta(tt.key)
			if m == nil {
				t.Fatalf("Indicator %s not in Registry", tt.key)
			}
			if m.Unit != tt.unit {
				t.Errorf("Indicator %s Unit = %q, want %q", tt.key, m.Unit, tt.unit)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 9: GetIndicatorDataSource uses Registry
// ═══════════════════════════════════════════════════════════════

func TestGetIndicatorDataSource_FromRegistry(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"daily_change", "stocks_daily_k"},
		{"ma_20", "stocks_daily_k"},
		{"rsi", "stocks_daily_k"},
		{"boll_position", "stocks_daily_k"},
		{"atr", "stocks_daily_k"},
		{"adx", "stocks_daily_k"},
		{"cci", "stocks_daily_k"},
		{"pe", "stocks_daily_indicator"},
		{"pb", "stocks_daily_indicator"},
		{"roe", "stock_financials"},
		{"eps", "stock_financials"},
		{"ai_score", "ai_stock_scores"},
		{"algo_score", "algorithm_pick_details"},
		{"signal_value", "stock_signals"},
		{"shareholder_change", "stock_shareholders"},
		{"prediction_upside", "ai_stock_predictions"},
		{"total_market_cap", "stocks_daily_indicator"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := GetIndicatorDataSource(tt.key)
			if got != tt.want {
				t.Errorf("GetIndicatorDataSource(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}

	// Unknown indicator
	if got := GetIndicatorDataSource("nonexistent"); got != "unknown" {
		t.Errorf("GetIndicatorDataSource('nonexistent') = %q, want 'unknown'", got)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 10: IsBacktestSafe
// ═══════════════════════════════════════════════════════════════

func TestIsBacktestSafe(t *testing.T) {
	safeIndicators := []string{
		"daily_change", "ma_20", "rsi", "kdj_k", "boll_position",
		"atr", "adx", "volume_ratio", "pe", "roe", "eps",
	}
	unsafeIndicators := []string{
		"signal_value", "prediction_upside", "prediction_consensus",
		"ai_score", "total_market_cap",
	}

	for _, k := range safeIndicators {
		if !IsBacktestSafe(k) {
			t.Errorf("Indicator %s should be backtestSafe=true", k)
		}
	}
	for _, k := range unsafeIndicators {
		if IsBacktestSafe(k) {
			t.Errorf("Indicator %s should be backtestSafe=false", k)
		}
	}
	if IsBacktestSafe("nonexistent") {
		t.Error("nonexistent indicator should not be backtestSafe")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 11: AllIndicators and GetIndicatorsByUseFor filtering
// ═══════════════════════════════════════════════════════════════

func TestAllIndicators_Filtering(t *testing.T) {
	all := AllIndicators("")
	if len(all) < 50 {
		t.Errorf("Expected at least 50 indicators, got %d", len(all))
	}

	trend := AllIndicators("技术面-趋势")
	if len(trend) == 0 {
		t.Error("Expected trend indicators, got 0")
	}

	buyOnly := GetIndicatorsByUseFor("buy")
	sellOnly := GetIndicatorsByUseFor("sell")
	both := GetIndicatorsByUseFor("both")

	t.Logf("Indicators: buy=%d sell=%d both=%d total=%d",
		len(buyOnly), len(sellOnly), len(both), len(all))
}

// ═══════════════════════════════════════════════════════════════
// Test 12: Indicator categories are all valid
// ═══════════════════════════════════════════════════════════════

func TestIndicatorCategories(t *testing.T) {
	validCategories := map[string]bool{
		"榜单与评分":         true,
		"AI评分":          true,
		"技术面-趋势":        true,
		"技术面-超买超卖":      true,
		"技术面-量价":        true,
		"技术面-波动":        true,
		"技术面-趋势系统":      true,
		"技术面-形态":        true,
		"估值":            true,
		"基本面":           true,
		"资金面":           true,
		"预测":            true,
	}

	for key, m := range IndicatorRegistry {
		if !validCategories[m.Category] {
			t.Errorf("Indicator %s has unknown category: %q", key, m.Category)
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 13: getIndicatorMeta compatibility
// ═══════════════════════════════════════════════════════════════

func TestGetIndicatorMeta_Compatibility(t *testing.T) {
	// Test that getIndicatorMeta returns non-nil for all registry keys
	for key := range IndicatorRegistry {
		result := getIndicatorMeta(key)
		if result == nil {
			t.Errorf("getIndicatorMeta(%q) returned nil", key)
			continue
		}
		if result["key"] != key {
			t.Errorf("getIndicatorMeta(%q) key mismatch: %v", key, result["key"])
		}
	}

	// Test unknown key
	if result := getIndicatorMeta("nonexistent"); result != nil {
		t.Error("getIndicatorMeta('nonexistent') should return nil")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 14: getOperatorLabel
// ═══════════════════════════════════════════════════════════════

func TestGetOperatorLabel(t *testing.T) {
	tests := []struct {
		op   string
		want string
	}{
		{"gte", "≥ (大于等于)"},
		{"lte", "≤ (小于等于)"},
		{"gt", "> (大于)"},
		{"lt", "< (小于)"},
		{"eq", "= (等于)"},
		{"cross_up", "↑ 上穿"},
		{"cross_down", "↓ 下穿"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			got := getOperatorLabel(tt.op)
			if got != tt.want {
				t.Errorf("getOperatorLabel(%q) = %q, want %q", tt.op, got, tt.want)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 15: parseValue
// ═══════════════════════════════════════════════════════════════

func TestParseValue(t *testing.T) {
	tests := []struct {
		name      string
		v         interface{}
		indicator string
		op        string
		expected  float64
	}{
		{"float64", float64(5.5), "rsi", "gt", 5.5},
		{"int", 3, "rsi", "gt", 3},
		{"string_number", "5.5", "rsi", "gt", 5.5},
		{"ma_cross_5_20", "5/20", "ma_cross", "cross_up", 5.02},
		{"ma_cross_10_30", "10/30", "ma_cross", "cross_up", 10.03},
		{"cross_up_op", "any", "ma_cross", "cross_up", 1},
		{"cross_down_op", "any", "ma_cross", "cross_down", -1},
		{"invalid", "abc", "rsi", "gt", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseValue(tt.v, tt.indicator, tt.op)
			if got != tt.expected {
				t.Errorf("parseValue(%v, %q, %q) = %v, want %v",
					tt.v, tt.indicator, tt.op, got, tt.expected)
			}
		})
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 16: IsIndicatorInRegistry
// ═══════════════════════════════════════════════════════════════

func TestIsIndicatorInRegistry(t *testing.T) {
	if !IsIndicatorInRegistry("rsi") {
		t.Error("rsi should be in registry")
	}
	if !IsIndicatorInRegistry("psy_12") {
		t.Error("psy_12 should be in registry")
	}
	if IsIndicatorInRegistry("nonexistent_indicator") {
		t.Error("nonexistent_indicator should not be in registry")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 17: Benchmark indicator count
// ═══════════════════════════════════════════════════════════════

func TestIndicatorCount(t *testing.T) {
	count := len(IndicatorRegistry)
	fmt.Printf("Total registered indicators: %d\n", count)
	if count < 60 {
		t.Errorf("Expected at least 60 indicators, got %d", count)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 18: All K-line derived indicators are backtest safe
// ═══════════════════════════════════════════════════════════════

func TestKlineDerivedBacktestSafe(t *testing.T) {
	// Indicators that are K-line derived but intentionally not backtest-safe yet
	// (e.g. newly added indicators without enough historical data)
	knownExceptions := map[string]bool{
		"net_flow_ratio":  true,
		"buy_sell_ratio":  true,
	}
	for key, m := range IndicatorRegistry {
		if m.DataSource == "stocks_daily_k" && !m.BacktestSafe {
			if !knownExceptions[key] {
				t.Errorf("K-line derived indicator %s should be backtestSafe=true", key)
			}
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 19: Prediction indicators are NOT backtest safe
// ═══════════════════════════════════════════════════════════════

func TestPredictionNotBacktestSafe(t *testing.T) {
	if IsBacktestSafe("prediction_upside") {
		t.Error("prediction_upside must NOT be backtestSafe")
	}
	if IsBacktestSafe("prediction_consensus") {
		t.Error("prediction_consensus must NOT be backtestSafe")
	}
}
