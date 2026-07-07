package service

import (
	"fmt"
	"math"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// BacktestEngine 逻辑等价性验证测试套件
// 覆盖: 完整生命周期/止盈止损/T+1/交易成本/多股票/强制平仓/跌停/资金不足/条件组/适配器
// ═══════════════════════════════════════════════════════════════

func simpleDatesN(n int) []string {
	dates := make([]string, n)
	for i := 0; i < n; i++ {
		dates[i] = fmt.Sprintf("2024-01-%02d", i+2)
	}
	return dates
}

func makeSimpleDP(dates []string, prices map[string][]float64, rsi map[string]map[int]float64) *mockDataProvider {
	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		change: make(map[string]float64),
		indVal: make(map[string]float64),
	}
	for code, ps := range prices {
		for i, d := range dates {
			if i < len(ps) {
				dp.close[code+"|"+d] = ps[i]
				dp.open[code+"|"+d] = ps[i]
			}
		}
	}
	for code, dayRSI := range rsi {
		for day, val := range dayRSI {
			if day < len(dates) {
				d := dates[day]
				dp.indVal["rsi|"+code+"|"+d] = float64(val)
			}
		}
	}
	return dp
}

// ═══════════════════════════════════════════════════════════════
// Test 1: 完整生命周期
// ═══════════════════════════════════════════════════════════════

func TestVerify_FullLifecycle(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.MaxHoldings = 3
	cfg.BuyPositionPct = 30
	cfg.AddPositionPct = 15
	cfg.ReducePositionPct = 50

	dates := simpleDatesN(10)
	universe := []StockInfo{{Code: "000001", Name: "平安银行"}}

	prices := map[string][]float64{
		"000001": {10, 10.5, 11, 11.5, 12, 12.5, 12, 11.5, 11, 10.5},
	}
	rsi := map[string]map[int]float64{
		"000001": {0: 55, 3: 55, 7: 75},
	}
	dp := makeSimpleDP(dates, prices, rsi)

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	addConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	sellConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	persister := &mockPersister{}
	result, err := engine.Run(cfg, universe, buyConds, addConds, sellConds, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	executedCount := 0
	for _, s := range persister.signals {
		if s.Status == "executed" {
			executedCount++
		}
	}
	if executedCount < 3 {
		t.Errorf("expected >= 3 executed, got %d", executedCount)
	}
	t.Logf("FullLifecycle: equity=%.0f trades=%d winRate=%.1f%%",
		result.FinalEquity, result.TradeCount, result.WinRate)
}

// ═══════════════════════════════════════════════════════════════
// Test 2: 止盈逻辑
// ═══════════════════════════════════════════════════════════════

func TestVerify_StopProfit(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100
	cfg.StopProfit = 10

	dates := []string{"D1", "D2", "D3", "D4", "D5"}
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 11.2, "000001|D4": 11.5, "000001|D5": 11.5,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 11.2, "000001|D4": 11.5, "000001|D5": 11.5,
		},
		indVal: map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// D3: (11.2-10)/10 = 12% >= 10% → stop generated, D4 executed
	hasStop := false
	for _, s := range persister.signals {
		if s.ActionType == "stop" && s.Status == "executed" {
			hasStop = true
			if s.Pnl <= 0 {
				t.Errorf("止盈应盈利, PnL=%.2f", s.Pnl)
			}
			break
		}
	}
	if !hasStop {
		t.Error("止盈应触发 (12% >= 10%)")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 3: 止损逻辑
// ═══════════════════════════════════════════════════════════════

func TestVerify_StopLoss(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100
	cfg.StopLoss = -8

	dates := []string{"D1", "D2", "D3", "D4"}
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 9.0, "000001|D4": 9.0,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 9.0, "000001|D4": 9.0,
		},
		indVal: map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// D3: (9-10)/10 = -10% <= -8% → stop
	hasStop := false
	for _, s := range persister.signals {
		if s.ActionType == "stop" {
			hasStop = true
			if s.Status == "executed" && s.Pnl >= 0 {
				t.Errorf("止损应亏损, PnL=%.2f", s.Pnl)
			}
		}
	}
	if !hasStop {
		t.Error("止损应触发 (-10% <= -8%)")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 4: T+1 限制
// ═══════════════════════════════════════════════════════════════

func TestVerify_T1Restriction(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500000
	cfg.BuyPositionPct = 50

	// D1 buy signal → D2 buy execute → D3 sell signal → D4 sell execute
	dates := []string{"D1", "D2", "D3", "D4"}
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 10.5, "000001|D4": 10.5,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 10.5, "000001|D4": 10.5,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55, // buy trigger
			"rsi|000001|D3": 75, // sell trigger on D3 (T+1 gap satisfied)
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	sellConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, sellConds, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	buyExec, sellExec := false, false
	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			buyExec = true
		}
		if s.ActionType == "sell" && s.Status == "executed" {
			sellExec = true
		}
	}
	if !buyExec {
		t.Error("应执行买入")
	}
	if !sellExec {
		t.Error("应执行卖出 (不同日, 不违反T+1)")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 5: 交易成本
// ═══════════════════════════════════════════════════════════════

func TestVerify_TransactionCosts(t *testing.T) {
	// 卖出: commission + stamp
	cost := TransactionCost(10000, 0.00025, 5.0, 0.0005)
	if math.Abs(cost-10.0) > 0.001 {
		t.Errorf("卖出成本=%.2f, want 10.00", cost)
	}
	cost = TransactionCost(50000, 0.00025, 5.0, 0.0005)
	if math.Abs(cost-37.5) > 0.001 {
		t.Errorf("大额卖出成本=%.2f, want 37.50", cost)
	}
	// 买入: 只有佣金
	cost = BuyCost(10000, 0.00025, 5.0)
	if math.Abs(cost-5.0) > 0.001 {
		t.Errorf("买入成本=%.2f, want 5.00", cost)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 6: 多股票持仓上限
// ═══════════════════════════════════════════════════════════════

func TestVerify_MultiStock(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.MaxHoldings = 2
	cfg.BuyPositionPct = 40

	dates := simpleDatesN(5)
	universe := []StockInfo{
		{Code: "000001", Name: "A"},
		{Code: "000002", Name: "B"},
		{Code: "000003", Name: "C"},
	}

	prices := map[string][]float64{
		"000001": {10, 10, 10, 10, 10},
		"000002": {20, 20, 20, 20, 20},
		"000003": {30, 30, 30, 30, 30},
	}
	rsi := map[string]map[int]float64{
		"000001": {0: 55},
		"000002": {0: 55},
		"000003": {0: 55},
	}
	dp := makeSimpleDP(dates, prices, rsi)

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	buyCount := 0
	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			buyCount++
		}
	}
	if buyCount > cfg.MaxHoldings {
		t.Errorf("买入 %d 只 > MaxHoldings %d", buyCount, cfg.MaxHoldings)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 7: 强制平仓
// ═══════════════════════════════════════════════════════════════

func TestVerify_ForceLiquidation(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500000
	cfg.BuyPositionPct = 50

	// D1 signal → D2 buy execute → D3 force liquidate
	dates := []string{"D1", "D2", "D3"}
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 12,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 12,
		},
		indVal: map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	result, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Force liquidation adds trades directly (not via persister signals)
	sellCount := 0
	for _, tr := range result.Trades {
		if tr.Action == "sell" || tr.Action == "stop" {
			sellCount++
		}
	}
	for _, s := range persister.signals {
		if (s.ActionType == "sell" || s.ActionType == "stop") && s.Status == "executed" {
			sellCount++
		}
	}
	if sellCount == 0 {
		t.Error("最后交易日应强制平仓")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 8: 跌停跳过平仓
// ═══════════════════════════════════════════════════════════════

func TestVerify_LimitDownBypass(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2"}
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 9,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 9,
		},
		change: map[string]float64{
			"000001|D2": -10,
		},
		indVal: map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	forceSell := false
	for _, s := range persister.signals {
		if s.ActionType == "sell" && s.Status == "executed" {
			forceSell = true
		}
	}
	if forceSell {
		t.Error("跌停(-10%)时不应强制平仓")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 9: 资金不足
// ═══════════════════════════════════════════════════════════════

func TestVerify_InsufficientFunds(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500

	dates := simpleDatesN(3)
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 100, "000001|D2": 100, "000001|D3": 100,
		},
		open: map[string]float64{
			"000001|D1": 100, "000001|D2": 100, "000001|D3": 100,
		},
		indVal: map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			t.Error("资金不足(500元, 100元/股)应跳过买入")
		}
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 10: AND/OR 条件组
// ═══════════════════════════════════════════════════════════════

func TestVerify_ConditionGroupLogic(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500000
	cfg.BuyPositionPct = 50

	dates := simpleDatesN(3)
	universe := []StockInfo{{Code: "000001", Name: "Test"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|"+dates[0]: 10, "000001|"+dates[1]: 10, "000001|"+dates[2]: 10,
		},
		open: map[string]float64{
			"000001|"+dates[0]: 10, "000001|"+dates[1]: 10, "000001|"+dates[2]: 10,
		},
		indVal: map[string]float64{
			"rsi|000001|"+dates[0]:    65,
			"macd|000001|"+dates[0]:   -1,
			"volume|000001|"+dates[0]:  2,
		},
	}

	// Group1: rsi>30 AND macd>0 → macd fails → group1 fails
	// Group2: volume>1 → passes → overall PASS (OR logic)
	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
		{Indicator: "macd", Operator: "gt", Value: 0, LogicGroup: 1},
		{Indicator: "volume", Operator: "gt", Value: 1, LogicGroup: 2},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	hasBuy := false
	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			hasBuy = true
		}
	}
	if !hasBuy {
		t.Error("Group2 passes → 应触发买入 (OR cross-group)")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 11: 空股票池
// ═══════════════════════════════════════════════════════════════

func TestVerify_EmptyUniverse(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()

	dates := simpleDatesN(5)
	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		indVal: make(map[string]float64),
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, nil, nil, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if len(persister.signals) > 0 {
		t.Error("空universe不应产生信号")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 12: 适配器映射
// ═══════════════════════════════════════════════════════════════

type testKC struct{}
func (t *testKC) GetClose(string, string) float64     { return 10.5 }
func (t *testKC) GetOpen(string, string) float64      { return 10.0 }
func (t *testKC) GetDailyChange(string, string) float64 { return 1.5 }

type testIC struct{}
func (t *testIC) Get(ind, code, date string) (float64, bool) {
	if ind == "rsi" {
		return 65, true
	}
	return 0, false
}

func TestVerify_AdapterMapping(t *testing.T) {
	dates := []string{"2024-01-02", "2024-01-03"}
	dateIdx := map[string]int{"2024-01-02": 0, "2024-01-03": 1}
	adapter := NewKlineCacheAdapter(&testKC{}, &testIC{}, dates, dateIdx)

	if adapter.GetClose("any", "any") != 10.5 {
		t.Error("GetClose mapping failed")
	}
	val, ok := adapter.GetIndicatorValue("rsi", "any", "any")
	if !ok || val != 65 {
		t.Errorf("GetIndicatorValue = (%v, %v)", val, ok)
	}
	if adapter.GetNextDate("2024-01-02") != "2024-01-03" {
		t.Error("GetNextDate mapping failed")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 13: Config 映射
// ═══════════════════════════════════════════════════════════════

func TestVerify_ConfigMapping(t *testing.T) {
	cfg := DefaultBacktestConfig()
	if cfg.InitialCapital != 100000 {
		t.Errorf("default InitialCapital = %.0f", cfg.InitialCapital)
	}
	cfg.InitialCapital = 200000
	cfg.MaxHoldings = 10
	cfg.StopProfit = 15
	cfg.StopLoss = -10

	if cfg.InitialCapital != 200000 {
		t.Error("InitialCapital override failed")
	}
	if cfg.MaxHoldings != 10 {
		t.Error("MaxHoldings override failed")
	}
	if cfg.StopProfit != 15 {
		t.Error("StopProfit override failed")
	}
}
