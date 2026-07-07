package service

import (
	"math"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// Mock DataProvider
// ═══════════════════════════════════════════════════════════════

type mockDataProvider struct {
	dates   []string
	close   map[string]float64
	open    map[string]float64
	change  map[string]float64
	indVal  map[string]float64
}

func (m *mockDataProvider) GetClose(code, date string) float64 {
	key := code + "|" + date
	if v, ok := m.close[key]; ok { return v }
	return 0
}

func (m *mockDataProvider) GetOpen(code, date string) float64 {
	key := code + "|" + date
	if v, ok := m.open[key]; ok { return v }
	return 0
}

func (m *mockDataProvider) GetDailyChange(code, date string) float64 {
	key := code + "|" + date
	if v, ok := m.change[key]; ok { return v }
	return 0
}

func (m *mockDataProvider) GetIndicatorValue(ind, code, date string) (float64, bool) {
	key := ind + "|" + code + "|" + date
	v, ok := m.indVal[key]
	return v, ok
}

func (m *mockDataProvider) GetNextDate(date string) string {
	for i, d := range m.dates {
		if d == date && i+1 < len(m.dates) {
			return m.dates[i+1]
		}
	}
	return date
}

func (m *mockDataProvider) Dates() []string { return m.dates }

// ═══════════════════════════════════════════════════════════════
// Mock Persister
// ═══════════════════════════════════════════════════════════════

type mockPersister struct {
	signals   []BacktestSignalRecord
	snapshots []BacktestSnapshot
	logs      []BacktestLogEntry
	cancelled bool
}

func (m *mockPersister) SaveSignal(sig *BacktestSignalRecord) error {
	m.signals = append(m.signals, *sig)
	return nil
}
func (m *mockPersister) SaveSnapshot(snap *BacktestSnapshot) error {
	m.snapshots = append(m.snapshots, *snap)
	return nil
}
func (m *mockPersister) SaveLog(entry *BacktestLogEntry) error {
	m.logs = append(m.logs, *entry)
	return nil
}
func (m *mockPersister) UpdateProgress(day, total int, phase string) {}
func (m *mockPersister) IsCancelled() bool { return m.cancelled }

// ═══════════════════════════════════════════════════════════════
// Test: Full backtest with buy-and-hold on a rising stock
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_BuyAndHoldRising(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.MaxHoldings = 5
	cfg.BuyPositionPct = 100 // use all capital on first buy

	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	// Rising stock: 10 → 12 over 10 days
	dates := []string{"D1", "D2", "D3", "D4", "D5", "D6", "D7", "D8", "D9", "D10"}
	dp := &mockDataProvider{
		dates: dates,
		close: make(map[string]float64),
		open:  make(map[string]float64),
		indVal: make(map[string]float64),
	}
	for i, d := range dates {
		price := 10.0 + float64(i)*0.2 // 10.0, 10.2, 10.4, ..., 11.8
		dp.close["000001|"+d] = price
		dp.open["000001|"+d] = price
		dp.indVal["rsi|000001|"+d] = 55 // neutral RSI
	}

	// Buy condition: RSI > 30 (always true)
	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	result, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have executed a buy signal
	if len(persister.signals) == 0 {
		t.Fatal("expected at least 1 signal")
	}
	buySignal := persister.signals[0]
	if buySignal.ActionType != "buy" {
		t.Errorf("expected buy, got %s", buySignal.ActionType)
	}

	// Final equity should be > initial capital (stock went up)
	if result.FinalEquity <= cfg.InitialCapital {
		t.Errorf("FinalEquity = %.0f, should be > %.0f", result.FinalEquity, cfg.InitialCapital)
	}
	if result.TotalReturnPct <= 0 {
		t.Errorf("TotalReturnPct = %.1f, should be positive", result.TotalReturnPct)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: Falling stock — no buy (condition not met)
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_NoBuyWhenConditionFails(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()

	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}
	dates := []string{"D1", "D2", "D3"}
	dp := &mockDataProvider{
		dates:  dates,
		close:  map[string]float64{"000001|D1": 10, "000001|D2": 10, "000001|D3": 10},
		open:   map[string]float64{"000001|D1": 10, "000001|D2": 10, "000001|D3": 10},
		indVal: map[string]float64{"rsi|000001|D1": 20, "rsi|000001|D2": 20, "rsi|000001|D3": 20},
	}

	// Buy condition: RSI > 70 (never met because RSI=20)
	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	persister := &mockPersister{}
	result, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// No buys because RSI never > 70
	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			t.Error("unexpected buy execution")
		}
	}

	// Equity should equal initial capital (no trades)
	if math.Abs(result.FinalEquity-cfg.InitialCapital) > 0.01 {
		t.Errorf("FinalEquity = %.0f, want %.0f", result.FinalEquity, cfg.InitialCapital)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: Sell when sell condition triggers
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_SellOnCondition(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100

	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}
	dates := []string{"D1", "D2", "D3", "D4", "D5"}
	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5,
			"000001|D3": 11, "000001|D4": 11.5, "000001|D5": 12,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5,
			"000001|D3": 11, "000001|D4": 11.5, "000001|D5": 12,
		},
		indVal: map[string]float64{
			// Buy condition: RSI > 30 (always true for D1)
			"rsi|000001|D1": 55,
			// Sell condition: RSI > 70 triggers on D3
			"rsi|000001|D3": 75,
			"rsi|000001|D4": 55,
			"rsi|000001|D5": 55,
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	sellConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	persister := &mockPersister{}
	result, err := engine.Run(cfg, universe, buyConds, nil, sellConds, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have buy AND sell signals
	hasBuy := false
	hasSell := false
	for _, s := range persister.signals {
		if s.ActionType == "buy" && s.Status == "executed" {
			hasBuy = true
		}
		if (s.ActionType == "sell" || s.ActionType == "stop") && s.Status == "executed" {
			hasSell = true
		}
	}
	if !hasBuy {
		t.Error("expected a buy execution")
	}
	if !hasSell {
		t.Error("expected a sell execution when RSI > 70")
	}

	_ = result
}

// ═══════════════════════════════════════════════════════════════
// Test: Stop loss triggers
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_StopLoss(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100
	cfg.StopLoss = -5 // 5% stop loss

	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}
	// D1: buy signal, D2: buy at 10, D3: stop signal generated at -10%
	// D4: stop executes at open (T+1)
	dates := []string{"D1", "D2", "D3", "D4"}
	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 9.0, "000001|D4": 9.0,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10, "000001|D3": 9.0, "000001|D4": 9.0,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55, // buy trigger
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	persister := &mockPersister{}
	_, err := engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, persister)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Should have a stop signal (D2: 9.4 from 10 = -6% < -5%)
	hasStop := false
	for _, s := range persister.signals {
		if s.ActionType == "stop" {
			hasStop = true
			break
		}
	}
	if !hasStop {
		t.Error("expected stop loss to trigger when price drops below -5%")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: Cancellation mid-run
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_Cancellation(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()

	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}
	dates := []string{"D1", "D2", "D3", "D4", "D5"}
	dp := &mockDataProvider{
		dates:  dates,
		close:  map[string]float64{"000001|D1": 10, "000001|D2": 10, "000001|D3": 10, "000001|D4": 10, "000001|D5": 10},
		open:   map[string]float64{"000001|D1": 10, "000001|D2": 10, "000001|D3": 10, "000001|D4": 10, "000001|D5": 10},
		indVal: map[string]float64{},
	}

	persister := &mockPersister{cancelled: true} // already cancelled
	_, err := engine.Run(cfg, universe, nil, nil, nil, nil, dp, persister)
	if err == nil {
		t.Error("expected cancellation error")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: Empty dates
// ═══════════════════════════════════════════════════════════════

func TestBacktestEngine_EmptyDates(t *testing.T) {
	engine := NewBacktestEngine()
	cfg := DefaultBacktestConfig()

	dp := &mockDataProvider{dates: nil}
	persister := &mockPersister{}
	_, err := engine.Run(cfg, nil, nil, nil, nil, nil, dp, persister)
	if err == nil {
		t.Error("expected error for empty dates")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test: DefaultBacktestConfig values
// ═══════════════════════════════════════════════════════════════

func TestDefaultBacktestConfig_Values(t *testing.T) {
	cfg := DefaultBacktestConfig()
	if cfg.InitialCapital != 100000 {
		t.Errorf("InitialCapital = %v", cfg.InitialCapital)
	}
	if cfg.MaxHoldings != 20 {
		t.Errorf("MaxHoldings = %d", cfg.MaxHoldings)
	}
	if cfg.CommissionRate != 0.00025 {
		t.Errorf("CommissionRate = %v", cfg.CommissionRate)
	}
	if cfg.StampTaxRate != 0.0005 {
		t.Errorf("StampTaxRate = %v", cfg.StampTaxRate)
	}
}
