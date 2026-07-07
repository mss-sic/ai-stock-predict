package service

import (
	"math"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// Position tests
// ═══════════════════════════════════════════════════════════════

func TestPositionTotalCost(t *testing.T) {
	p := Position{BuyPrice: 10.5, Quantity: 100}
	if p.TotalCost() != 1050.0 {
		t.Errorf("TotalCost = %v, want 1050", p.TotalCost())
	}
}

func TestPositionMarketValue(t *testing.T) {
	p := Position{BuyPrice: 10.0, Quantity: 100}
	if p.MarketValue(12.0) != 1200.0 {
		t.Errorf("MarketValue = %v, want 1200", p.MarketValue(12.0))
	}
}

func TestPositionUnrealizedPnl(t *testing.T) {
	p := Position{BuyPrice: 10.0, Quantity: 100}
	pnl := p.UnrealizedPnl(12.0)
	if pnl != 200.0 {
		t.Errorf("UnrealizedPnl = %v, want 200", pnl)
	}
	pnl = p.UnrealizedPnl(8.0)
	if pnl != -200.0 {
		t.Errorf("UnrealizedPnl(down) = %v, want -200", pnl)
	}
}

func TestPositionUnrealizedPnlPct(t *testing.T) {
	p := Position{BuyPrice: 10.0, Quantity: 100}
	pct := p.UnrealizedPnlPct(12.0)
	if pct != 20.0 {
		t.Errorf("UnrealizedPnlPct = %v, want 20", pct)
	}
	// Test with zero buy price (edge case)
	p2 := Position{BuyPrice: 0, Quantity: 100}
	pct2 := p2.UnrealizedPnlPct(10.0)
	if pct2 != 0 {
		t.Errorf("UnrealizedPnlPct(zero basis) = %v, want 0", pct2)
	}
}

// ═══════════════════════════════════════════════════════════════
// PositionService: Buy
// ═══════════════════════════════════════════════════════════════

func TestPositionService_Buy(t *testing.T) {
	svc := NewPositionService()
	cash := 100000.0
	pos := svc.Buy(&cash, "000001", "平安银行", "2024-01-03", 10.0, 15000, 0.00025, 5.0)

	if pos == nil {
		t.Fatal("Buy should succeed")
	}
	if pos.Code != "000001" {
		t.Errorf("Code = %s", pos.Code)
	}
	if pos.Quantity != 1500 { // 15000/10 = 1500
		t.Errorf("Quantity = %d, want 1500", pos.Quantity)
	}
	if pos.BuyDate != "2024-01-03" {
		t.Errorf("BuyDate = %s", pos.BuyDate)
	}
	// cash: 100000 - 15000 - commission(5) = 84995
	expectedCash := 100000.0 - 15000.0 - 5.0
	if cash != expectedCash {
		t.Errorf("cash after buy = %v, want %v", cash, expectedCash)
	}
}

func TestPositionService_BuyInsufficientFunds(t *testing.T) {
	svc := NewPositionService()
	cash := 500.0
	pos := svc.Buy(&cash, "000001", "test", "2024-01-03", 100.0, 50000, 0.00025, 5.0)
	if pos != nil {
		t.Error("Buy should fail with insufficient funds")
	}
}

func TestPositionService_Add(t *testing.T) {
	svc := NewPositionService()
	cash := 50000.0
	pos := &Position{Code: "000001", BuyPrice: 10.0, Quantity: 1000, BuyDate: "2024-01-02"}

	qty := svc.Add(&cash, pos, 10.5, 10000, 0.00025, 5.0) // add 10000 worth
	if qty != 900 { // 10000/10.5 = 952 → 900 (round to 100 shares)
		t.Errorf("added qty = %d, want 900", qty)
	}
	// New weighted avg: (10*1000 + 10.5*900) / 1900 = 19450/1900 = 10.2368...
	expectedCost := (10.0*1000 + 10.5*float64(qty)) / float64(1000+qty)
	if math.Abs(pos.BuyPrice-expectedCost) > 0.01 {
		t.Errorf("new BuyPrice = %v, want ~%v", pos.BuyPrice, expectedCost)
	}
}

func TestPositionService_Sell(t *testing.T) {
	svc := NewPositionService()
	cash := 0.0
	pos := &Position{Code: "000001", BuyPrice: 10.0, Quantity: 1000}

	pnl, pnlPct := svc.Sell(&cash, pos, 12.0, 0.00025, 5.0, 0.0005)
	if math.Abs(pnl-2000.0) > 0.01 {
		t.Errorf("pnl = %v, want 2000", pnl)
	}
	if math.Abs(pnlPct-20.0) > 0.01 {
		t.Errorf("pnlPct = %v, want 20", pnlPct)
	}
	// cash: 0 + 12000 - commission(5) - stampTax(6) = 11989
	if math.Abs(cash-11989.0) > 0.01 {
		t.Errorf("cash after sell = %v, want 11989", cash)
	}
}

func TestPositionService_Reduce(t *testing.T) {
	svc := NewPositionService()
	cash := 0.0
	pos := &Position{Code: "000001", BuyPrice: 10.0, Quantity: 1000}

	pnl, _, reduced := svc.Reduce(&cash, pos, 12.0, 500, "2024-01-05", 0.00025, 5.0, 0.0005)
	if reduced != 500 {
		t.Errorf("reduced = %d, want 500", reduced)
	}
	if pnl != 1000.0 {
		t.Errorf("pnl = %v, want 1000", pnl)
	}
	if pos.Quantity != 500 {
		t.Errorf("remaining = %d, want 500", pos.Quantity)
	}
	if pos.LastReduceDate != "2024-01-05" {
		t.Errorf("LastReduceDate = %s", pos.LastReduceDate)
	}
}

// ═══════════════════════════════════════════════════════════════
// BacktestService: Performance calculation
// ═══════════════════════════════════════════════════════════════

func TestComputeSharpe(t *testing.T) {
	svc := NewBacktestService()
	// Constant 1% daily returns → very high Sharpe
	returns := make([]float64, 252)
	for i := range returns {
		returns[i] = 0.01
	}
	sharpe := svc.computeSharpe(returns)
	if sharpe == 0 {
		t.Error("Sharpe should be non-zero for constant returns")
	}
}

func TestComputeSharpe_ZeroReturns(t *testing.T) {
	svc := NewBacktestService()
	returns := make([]float64, 252)
	sharpe := svc.computeSharpe(returns)
	if sharpe != 0 {
		t.Errorf("Sharpe = %v, want 0 for zero returns", sharpe)
	}
}

func TestComputeMaxDrawdown(t *testing.T) {
	svc := NewBacktestService()
	curve := []EquityPoint{
		{Date: "D1", Equity: 100000},
		{Date: "D2", Equity: 105000}, // peak
		{Date: "D3", Equity: 95000},  // drawdown
		{Date: "D4", Equity: 98000},
		{Date: "D5", Equity: 110000}, // new peak
	}

	amount, pct := svc.computeMaxDrawdown(curve, 100000)
	// Max DD from 105000 → 95000 = 10000, 9.52%
	if math.Abs(amount-10000) > 0.5 {
		t.Errorf("maxDD amount = %v, want ~10000", amount)
	}
	if math.Abs(pct-9.523) > 0.1 {
		t.Errorf("maxDD pct = %v, want ~9.52", pct)
	}
}

func TestComputeMaxDrawdown_NoCurve(t *testing.T) {
	svc := NewBacktestService()
	amount, pct := svc.computeMaxDrawdown(nil, 100000)
	if amount != 0 || pct != 0 {
		t.Errorf("empty curve: amount=%v pct=%v, want 0/0", amount, pct)
	}
}

func TestComputeWinRate(t *testing.T) {
	svc := NewBacktestService()
	trades := []BacktestTrade{
		{Action: "buy", Pnl: 0},
		{Action: "sell", Pnl: 500},
		{Action: "sell", Pnl: -200},
		{Action: "sell", Pnl: 300},
		{Action: "stop", Pnl: -100},
		{Action: "reduce", Pnl: 50},
	}

	rate, wins := svc.computeWinRate(trades)
	// Completed: sell*3 + stop*1 + reduce*1 = 5, wins = 3
	if wins != 3 {
		t.Errorf("wins = %d, want 3", wins)
	}
	if math.Abs(rate-0.6) > 0.01 {
		t.Errorf("winRate = %v, want 0.6", rate)
	}
}

func TestComputeAvgWinLoss(t *testing.T) {
	svc := NewBacktestService()
	trades := []BacktestTrade{
		{Action: "sell", Pnl: 500},
		{Action: "sell", Pnl: 300},
		{Action: "sell", Pnl: -200},
		{Action: "sell", Pnl: -100},
	}

	avgWin, avgLoss := svc.computeAvgWinLoss(trades)
	if avgWin != 400.0 { // (500+300)/2
		t.Errorf("avgWin = %v, want 400", avgWin)
	}
	if avgLoss != 150.0 { // (200+100)/2
		t.Errorf("avgLoss = %v, want 150", avgLoss)
	}
}

func TestComputeProfitFactor(t *testing.T) {
	svc := NewBacktestService()
	trades := []BacktestTrade{
		{Action: "sell", Pnl: 1000},
		{Action: "sell", Pnl: 500},
		{Action: "sell", Pnl: -300},
		{Action: "sell", Pnl: -200},
	}

	pf := svc.computeProfitFactor(trades)
	// (1000+500) / (300+200) = 1500/500 = 3.0
	if math.Abs(pf-3.0) > 0.01 {
		t.Errorf("profitFactor = %v, want 3.0", pf)
	}
}

func TestCalculatePerformance(t *testing.T) {
	svc := NewBacktestService()
	trades := []BacktestTrade{
		{Action: "sell", Pnl: 1000},
		{Action: "sell", Pnl: -200},
	}
	dailyReturns := []float64{0.01, -0.005, 0.02}
	curve := []EquityPoint{
		{Date: "D1", Equity: 100000},
		{Date: "D2", Equity: 101000},
		{Date: "D3", Equity: 100500},
	}

	perf := svc.CalculatePerformance(100000, 102000, trades, dailyReturns, curve, 252)

	if perf.TotalTrades != 2 {
		t.Errorf("TotalTrades = %d", perf.TotalTrades)
	}
	if perf.WinningTrades != 1 {
		t.Errorf("WinningTrades = %d, want 1", perf.WinningTrades)
	}
	if perf.InitialCapital != 100000 {
		t.Errorf("InitialCapital = %v", perf.InitialCapital)
	}
	if perf.FinalEquity != 102000 {
		t.Errorf("FinalEquity = %v", perf.FinalEquity)
	}
}

// ═══════════════════════════════════════════════════════════════
// Transaction costs
// ═══════════════════════════════════════════════════════════════

func TestTransactionCost(t *testing.T) {
	cost := TransactionCost(10000, 0.00025, 5.0, 0.0005)
	// commission = max(2.5, 5) = 5, stamp = 5 → total = 10
	if cost != 10.0 {
		t.Errorf("TransactionCost = %v, want 10.0", cost)
	}
}

func TestBuyCost(t *testing.T) {
	cost := BuyCost(10000, 0.00025, 5.0)
	if cost != 5.0 {
		t.Errorf("BuyCost = %v, want 5.0", cost)
	}

	// Large trade: commission exceeds minimum
	cost = BuyCost(50000, 0.00025, 5.0)
	if cost != 12.5 {
		t.Errorf("BuyCost(large) = %v, want 12.5", cost)
	}
}

// ═══════════════════════════════════════════════════════════════
// TopN
// ═══════════════════════════════════════════════════════════════

func TestTopN(t *testing.T) {
	svc := NewBacktestService()
	results := []ScoreResult{
		{Code: "B", TotalScore: 4.2},
		{Code: "A", TotalScore: 4.8},
		{Code: "C", TotalScore: 2.5},
		{Code: "D", TotalScore: 3.8},
	}

	top := svc.TopN(results, 2, 3.0)
	if len(top) != 2 {
		t.Errorf("TopN count = %d, want 2", len(top))
	}
	if top[0].Code != "A" {
		t.Errorf("top[0] = %s, want A", top[0].Code)
	}
	if top[1].Code != "B" {
		t.Errorf("top[1] = %s, want B", top[1].Code)
	}
}
