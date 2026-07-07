package service

import (
	"testing"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// StockPoolService tests
// ═══════════════════════════════════════════════════════════════

func TestStockPool_IsWatchlistPool(t *testing.T) {
	svc := NewStockPoolService()
	if !svc.IsWatchlistPool("watchlist_5") {
		t.Error("watchlist_5 should be watchlist pool")
	}
	if svc.IsWatchlistPool("all") {
		t.Error("all should not be watchlist pool")
	}
	if svc.IsWatchlistPool("portfolio") {
		t.Error("portfolio should not be watchlist pool")
	}
}

func TestStockPool_ResolvePoolLabel(t *testing.T) {
	svc := NewStockPoolService()

	if svc.ResolvePoolLabel("all", 5000) != "全部股票 (5000只)" {
		t.Error("all label mismatch")
	}
	if svc.ResolvePoolLabel("watchlist_1", 20) != "自选组 (20只)" {
		t.Error("watchlist label mismatch")
	}
	if svc.ResolvePoolLabel("portfolio", 8) != "我的持仓 (8只)" {
		t.Error("portfolio label mismatch")
	}
	if svc.ResolvePoolLabel("codes", 3) != "自选代码 (3只)" {
		t.Error("codes label mismatch")
	}
	if svc.ResolvePoolLabel("all", 0) != "全部股票" {
		t.Error("zero count should omit count")
	}
}

func TestStockPool_ShouldScanAll(t *testing.T) {
	svc := NewStockPoolService()
	if !svc.ShouldScanAll("all", nil) {
		t.Error("all + nil codes should scan all")
	}
	if !svc.ShouldScanAll("all", []string{}) {
		t.Error("all + empty codes should scan all")
	}
	if svc.ShouldScanAll("watchlist_1", nil) {
		t.Error("watchlist should not scan all")
	}
	if svc.ShouldScanAll("", []string{"000001"}) {
		t.Error("specific codes should not scan all")
	}
}

func TestStockPool_ValidatePoolSize(t *testing.T) {
	svc := NewStockPoolService()
	min, ok := svc.ValidatePoolSize(10)
	if !ok {
		t.Error("10 stocks should be valid")
	}
	if min != 5 {
		t.Errorf("min = %d, want 5", min)
	}

	_, ok = svc.ValidatePoolSize(3)
	if ok {
		t.Error("3 stocks should be invalid")
	}
}

func TestStockPool_BuildPoolLabel(t *testing.T) {
	svc := NewStockPoolService()
	label := svc.BuildPoolLabel("all", "全部股票", []string{"000001", "000002"})
	if label != "全部股票 (2只)" {
		t.Errorf("BuildPoolLabel = %s", label)
	}
}

func TestStockPool_FilterByIndustry(t *testing.T) {
	svc := NewStockPoolService()
	items := []StockPoolItem{{Code: "000001", Name: "平安银行"}, {Code: "000002", Name: "万科A"}}
	result := svc.FilterByIndustry(items, "银行")
	// Currently a pass-through; future will filter
	if len(result) != 2 {
		t.Errorf("pass-through should return all items")
	}
}

// ═══════════════════════════════════════════════════════════════
// DecisionTreeService tests
// ═══════════════════════════════════════════════════════════════

func TestDecisionTree_BuildTree_Empty(t *testing.T) {
	svc := NewDecisionTreeService()
	roots := svc.BuildTree(nil)
	if len(roots) != 0 {
		t.Errorf("empty conditions should give 0 roots, got %d", len(roots))
	}
}

func TestDecisionTree_BuildTree_FlatConditions(t *testing.T) {
	svc := NewDecisionTreeService()
	// Use model.StrategyCondition directly
	id1 := uint(1)
	id2 := uint(2)
	zero := uint(0)
	conds := []model.StrategyCondition{
		{ID: id1, Indicator: "rsi", Operator: "gt", Value: 70, Enabled: true, TreeOperator: "and", ParentID: nil},
		{ID: id2, Indicator: "macd", Operator: "cross_down", Value: 0, Enabled: true, TreeOperator: "or", ParentID: &zero},
	}

	// Both should be roots since ParentID is nil or 0
	roots := svc.BuildTree(conds)
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}
	if roots[0].Condition.Indicator != "rsi" {
		t.Errorf("root[0] indicator = %s", roots[0].Condition.Indicator)
	}
}

func TestDecisionTree_Evaluate_EmptyRoots(t *testing.T) {
	svc := NewDecisionTreeService()
	evalFn := func(cond model.StrategyCondition, code, date string) bool { return true }
	triggered, reason := svc.Evaluate(nil, evalFn, "000001", "2024-01-01")
	if triggered {
		t.Error("empty roots should not trigger")
	}
	if reason != "无条件（空决策树）" {
		t.Errorf("reason = %s", reason)
	}
}

func TestDecisionTree_Evaluate_LeafPass(t *testing.T) {
	svc := NewDecisionTreeService()
	conds := []model.StrategyCondition{
		{ID: 1, Indicator: "rsi", Operator: "gt", Value: 70, Enabled: true, TreeOperator: "and", ParentID: nil},
	}
	roots := svc.BuildTree(conds)

	evalFn := func(cond model.StrategyCondition, code, date string) bool {
		return true // always pass
	}

	triggered, reason := svc.Evaluate(roots, evalFn, "000001", "2024-01-01")
	if !triggered {
		t.Error("should trigger when condition passes")
	}
	if reason != "组1触发: rsi ✓" {
		t.Errorf("reason = %s", reason)
	}
}

func TestDecisionTree_Evaluate_LeafFail(t *testing.T) {
	svc := NewDecisionTreeService()
	conds := []model.StrategyCondition{
		{ID: 1, Indicator: "rsi", Operator: "gt", Value: 70, Enabled: true, TreeOperator: "and", ParentID: nil},
	}
	roots := svc.BuildTree(conds)

	evalFn := func(cond model.StrategyCondition, code, date string) bool {
		return false
	}

	triggered, _ := svc.Evaluate(roots, evalFn, "000001", "2024-01-01")
	if triggered {
		t.Error("should not trigger when condition fails")
	}
}

func TestDecisionTree_Evaluate_AndNode(t *testing.T) {
	svc := NewDecisionTreeService()
	// Parent with AND operator, two children
	parentID := uint(1)
	conds := []model.StrategyCondition{
		{ID: parentID, Indicator: "root", Operator: "and", Enabled: true, TreeOperator: "and", ParentID: nil},
		{ID: 2, Indicator: "rsi", Operator: "gt", Value: 70, Enabled: true, TreeOperator: "and", ParentID: &parentID},
		{ID: 3, Indicator: "macd", Operator: "cross_up", Value: 0, Enabled: true, TreeOperator: "and", ParentID: &parentID},
	}
	roots := svc.BuildTree(conds)

	// Both children pass
	evalBoth := func(cond model.StrategyCondition, code, date string) bool { return true }
	triggered, _ := svc.Evaluate(roots, evalBoth, "000001", "2024-01-01")
	if !triggered {
		t.Error("AND node with both children passing should trigger")
	}

	// One child fails
	evalOne := func(cond model.StrategyCondition, code, date string) bool {
		return cond.Indicator == "rsi"
	}
	triggered, _ = svc.Evaluate(roots, evalOne, "000001", "2024-01-01")
	if triggered {
		t.Error("AND node with one child failing should not trigger")
	}
}

func TestDecisionTree_FilterByEnabled(t *testing.T) {
	svc := NewDecisionTreeService()
	conds := []model.StrategyCondition{
		{ID: 1, Enabled: true},
		{ID: 2, Enabled: false},
		{ID: 3, Enabled: true},
	}
	filtered := svc.FilterByEnabled(conds)
	if len(filtered) != 2 {
		t.Errorf("expected 2 enabled, got %d", len(filtered))
	}
}
