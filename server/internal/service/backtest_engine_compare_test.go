package service

import (
	"math"
	"testing"
)

// ═══════════════════════════════════════════════════════════════
// 双引擎对比测试套件
// 目标: 验证 V2 BacktestEngine 与旧引擎 runBacktestAsync 核心逻辑等价
//
// 方法: 对每个场景，同时运行 V2 引擎和参考实现(reference engine)，
//       对比信号生成、执行结果、最终权益、交易列表、止盈止损等输出。
//       参考实现精确模拟旧引擎的核心交易循环逻辑。
// ═══════════════════════════════════════════════════════════════

// ── Reference Engine (模拟旧 runBacktestAsync 核心逻辑) ──

type refEngine struct {
	dates    []string
	data     BacktestDataProvider
	positions map[string]*refPosition
	cash     float64
	trades   []BacktestTrade
	signals  []BacktestSignalRecord
}

type refPosition struct {
	code     string
	name     string
	qty      int
	buyPrice float64
	buyDate  string
}

func newRefEngine(dates []string, data BacktestDataProvider, initCash float64) *refEngine {
	return &refEngine{
		dates:     dates,
		data:      data,
		positions: make(map[string]*refPosition),
		cash:      initCash,
	}
}

func (r *refEngine) execBuy(code, name, date string, price float64, amount float64, commissionRate, minCommission float64) bool {
	if amount <= 0 || price <= 0 {
		return false
	}
	qty := int(amount/price/100) * 100
	if qty <= 0 {
		return false
	}
	actualAmt := float64(qty) * price
	commission := math.Max(actualAmt*commissionRate, minCommission)

	// Reduce qty until amount + commission fits cash
	for actualAmt+commission > r.cash && qty >= 100 {
		qty -= 100
		actualAmt = float64(qty) * price
		commission = math.Max(actualAmt*commissionRate, minCommission)
	}
	if qty <= 0 || actualAmt+commission > r.cash {
		return false
	}
	r.cash -= actualAmt + commission

	r.positions[code] = &refPosition{code: code, name: name, qty: qty, buyPrice: price, buyDate: date}
	r.trades = append(r.trades, BacktestTrade{
		Date: date, SignalDate: date, Action: "buy",
		Code: code, Name: name, Price: price, Quantity: qty,
		Reason: "参考引擎-买入",
	})
	return true
}

func (r *refEngine) execSell(code, date string, price float64, commissionRate, minCommission, stampTaxRate float64) (float64, float64) {
	pos, ok := r.positions[code]
	if !ok {
		return 0, 0
	}
	revenue := float64(pos.qty) * price
	commission := math.Max(revenue*commissionRate, minCommission)
	stampTax := revenue * stampTaxRate
	r.cash += revenue - commission - stampTax

	pnl := revenue - float64(pos.qty)*pos.buyPrice - commission - stampTax
	pnlPct := 0.0
	if pos.buyPrice > 0 {
		pnlPct = pnl / (float64(pos.qty) * pos.buyPrice) * 100
	}

	r.trades = append(r.trades, BacktestTrade{
		Date: date, SignalDate: date, Action: "sell",
		Code: code, Name: pos.name, Price: price, Quantity: pos.qty,
		Pnl: pnl, PnlPct: pnlPct, Reason: "参考引擎-卖出",
	})
	delete(r.positions, code)
	return pnl, pnlPct
}

func (r *refEngine) evalRefConds(conds []ConditionDef, code, date string, data BacktestDataProvider) bool {
	if len(conds) == 0 {
		return false
	}
	groups := make(map[int][]ConditionDef)
	for _, c := range conds {
		lg := c.LogicGroup
		if lg == 0 {
			lg = 1
		}
		groups[lg] = append(groups[lg], c)
	}
	for _, group := range groups {
		allMet := true
		for _, c := range group {
			val, ok := data.GetIndicatorValue(c.Indicator, code, date)
			if !ok {
				allMet = false
				break
			}
			if !checkOperator(val, c.Operator, c.Value) {
				allMet = false
				break
			}
		}
		if allMet {
			return true
		}
	}
	return false
}

// checkOperator is shared between both engines — defined in backtest_engine.go
// (this test package has access to it)

// Run the reference engine for a full cycle and return result metrics
func (r *refEngine) run(
	universe []StockInfo,
	buyConds, addConds, sellConds, reduceConds []ConditionDef,
	cfg BacktestConfig,
) *BacktestResult {
	type pendingSignal struct {
		execDate string
		code     string
		name     string
		action   string
		amount   float64
		qty      int
	}
	var pending []pendingSignal

	equityCurve := make([]EquityPoint, 0, len(r.dates))
	dailyReturns := make([]float64, 0, len(r.dates))

	for dayIdx, date := range r.dates {
		// Execute pending signals for today
		newPending := make([]pendingSignal, 0)
		for _, ps := range pending {
			if ps.execDate != date {
				newPending = append(newPending, ps)
				continue
			}
			openPrice := r.data.GetOpen(ps.code, date)
			if openPrice <= 0 {
				openPrice = r.data.GetClose(ps.code, date)
			}
			switch ps.action {
			case "buy":
				r.execBuy(ps.code, ps.name, date, openPrice, ps.amount, cfg.CommissionRate, cfg.MinCommission)
			case "sell":
				if pos, ok := r.positions[ps.code]; ok && pos.buyDate != date {
					r.execSell(ps.code, date, openPrice, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)
				}
			case "stop":
				if pos, ok := r.positions[ps.code]; ok && pos.buyDate != date {
					r.execSell(ps.code, date, openPrice, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)
				}
			}
		}
		pending = newPending

		// Generate signals (same logic as old engine)
		for _, stock := range universe {
			code := stock.Code

			// Check sell/reduce if holding
			if pos, holding := r.positions[code]; holding {
				if len(reduceConds) > 0 && r.evalRefConds(reduceConds, code, date, r.data) {
					reduceQty := int(float64(pos.qty) * cfg.ReducePositionPct / 100 / 100) * 100
					if reduceQty > 0 && reduceQty < pos.qty && pos.buyDate != date {
						closePrice := r.data.GetClose(code, date)
						r.execSellPartial(code, date, closePrice, reduceQty, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)
					}
				} else if len(sellConds) > 0 && r.evalRefConds(sellConds, code, date, r.data) {
					nextDate := r.data.GetNextDate(date)
					if nextDate != date {
						pending = append(pending, pendingSignal{execDate: nextDate, code: code, name: pos.name, action: "sell"})
					}
				}
			}

			// Check stop profit/loss
			if pos, holding := r.positions[code]; holding {
				closePrice := r.data.GetClose(code, date)
				if closePrice > 0 && pos.buyPrice > 0 {
					changePct := (closePrice - pos.buyPrice) / pos.buyPrice * 100
					if cfg.StopProfit > 0 && changePct >= cfg.StopProfit {
						nextDate := r.data.GetNextDate(date)
						if nextDate != date {
							pending = append(pending, pendingSignal{execDate: nextDate, code: code, name: pos.name, action: "stop"})
						}
					} else if cfg.StopLoss < 0 && changePct <= cfg.StopLoss {
						nextDate := r.data.GetNextDate(date)
						if nextDate != date {
							pending = append(pending, pendingSignal{execDate: nextDate, code: code, name: pos.name, action: "stop"})
						}
					}
				}
			}

			// Check buy (not on last day)
			if len(buyConds) > 0 && len(r.positions) < cfg.MaxHoldings && dayIdx < len(r.dates)-1 {
				if _, holding := r.positions[code]; !holding && r.evalRefConds(buyConds, code, date, r.data) {
					amount := r.cash * cfg.BuyPositionPct / 100
					if amount > 0 {
						nextDate := r.data.GetNextDate(date)
						if nextDate != date {
							pending = append(pending, pendingSignal{execDate: nextDate, code: code, name: stock.Name, action: "buy", amount: amount})
						}
					}
				}
			}
		}

		// Snapshot
		totalEquity := r.cash
		for _, pos := range r.positions {
			closePrice := r.data.GetClose(pos.code, date)
			totalEquity += float64(pos.qty) * closePrice
		}
		dayReturn := 0.0
		if len(equityCurve) > 0 {
			prev := equityCurve[len(equityCurve)-1].Equity
			if prev > 0 {
				dayReturn = totalEquity/prev - 1
			}
		}
		equityCurve = append(equityCurve, EquityPoint{Date: date, Equity: totalEquity})
		dailyReturns = append(dailyReturns, dayReturn)
	}

	// Force liquidate on last day
	lastDate := r.dates[len(r.dates)-1]
	for code, pos := range r.positions {
		closePrice := r.data.GetClose(code, lastDate)
		if closePrice <= 0 {
			closePrice = pos.buyPrice
		}
		if r.data.GetDailyChange(code, lastDate) <= -9.8 {
			continue
		}
		r.execSell(code, lastDate, closePrice, cfg.CommissionRate, cfg.MinCommission, cfg.StampTaxRate)
	}

	return r.calculateResult(cfg.InitialCapital, dailyReturns, equityCurve)
}

func (r *refEngine) execSellPartial(code, date string, price float64, reduceQty int, commissionRate, minCommission, stampTaxRate float64) {
	pos := r.positions[code]
	if pos.qty <= reduceQty {
		r.execSell(code, date, price, commissionRate, minCommission, stampTaxRate)
		return
	}
	revenue := float64(reduceQty) * price
	commission := math.Max(revenue*commissionRate, minCommission)
	stampTax := revenue * stampTaxRate
	r.cash += revenue - commission - stampTax

	pnl := revenue - float64(reduceQty)*pos.buyPrice - commission - stampTax
	pnlPct := 0.0
	if pos.buyPrice > 0 {
		pnlPct = pnl / (float64(reduceQty) * pos.buyPrice) * 100
	}
	pos.qty -= reduceQty

	r.trades = append(r.trades, BacktestTrade{
		Date: date, SignalDate: date, Action: "reduce",
		Code: code, Name: pos.name, Price: price, Quantity: reduceQty,
		Pnl: pnl, PnlPct: pnlPct, Reason: "参考引擎-减仓",
	})
}

func (r *refEngine) calculateResult(initialCapital float64, dailyReturns []float64, equityCurve []EquityPoint) *BacktestResult {
	svc := NewBacktestService()
	if len(equityCurve) == 0 {
		return &BacktestResult{}
	}
	finalEquity := equityCurve[len(equityCurve)-1].Equity
	perf := svc.CalculatePerformance(initialCapital, finalEquity, r.trades, dailyReturns, equityCurve, len(equityCurve))
	return &BacktestResult{
		FinalEquity:    perf.FinalEquity,
		TotalReturn:    perf.TotalReturn,
		TotalReturnPct: perf.TotalReturnPct,
		SharpeRatio:    perf.SharpeRatio,
		MaxDrawdown:    perf.MaxDrawdown,
		MaxDrawdownPct: perf.MaxDrawdownPct,
		WinRate:        perf.WinRatePct,
		TradeCount:     perf.TotalTrades,
		Trades:         r.trades,
		EquityCurve:    equityCurve,
		DailyReturns:   dailyReturns,
	}
}

// ── 对比辅助函数 ──

// compareResults checks that two BacktestResult are logically equivalent within tolerance.
func compareResults(t *testing.T, v2, ref *BacktestResult, tolerance float64, scenario string) {
	t.Helper()

	// Final equity should be close
	if math.Abs(v2.FinalEquity-ref.FinalEquity) > tolerance {
		t.Errorf("[%s] FinalEquity mismatch: v2=%.2f ref=%.2f (diff=%.2f)",
			scenario, v2.FinalEquity, ref.FinalEquity, v2.FinalEquity-ref.FinalEquity)
	}

	// Trade count should be close
	if math.Abs(float64(v2.TradeCount-ref.TradeCount)) > 2 {
		t.Errorf("[%s] TradeCount mismatch: v2=%d ref=%d",
			scenario, v2.TradeCount, ref.TradeCount)
	}

	// Win rate should be in same ballpark
	if math.Abs(v2.WinRate-ref.WinRate) > 20 && v2.TradeCount > 0 && ref.TradeCount > 0 {
		t.Errorf("[%s] WinRate mismatch: v2=%.1f%% ref=%.1f%%",
			scenario, v2.WinRate, ref.WinRate)
	}

	t.Logf("[%s] v2(equity=%.0f trades=%d winRate=%.1f%%) vs ref(equity=%.0f trades=%d winRate=%.1f%%)",
		scenario, v2.FinalEquity, v2.TradeCount, v2.WinRate,
		ref.FinalEquity, ref.TradeCount, ref.WinRate)
}

// ═══════════════════════════════════════════════════════════════
// Test 1: 基础买入持有 (Rising Stock)
// ═══════════════════════════════════════════════════════════════

func TestCompare_BuyAndHoldRising(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.MaxHoldings = 3
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2", "D3", "D4", "D5", "D6"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates:  dates,
		close:  map[string]float64{},
		open:   map[string]float64{},
		indVal: map[string]float64{},
	}
	for i, d := range dates {
		price := 10.0 + float64(i)*0.5 // 10, 10.5, 11, 11.5, 12, 12.5
		dp.close["000001|"+d] = price
		dp.open["000001|"+d] = price
		dp.indVal["rsi|000001|"+d] = 55
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	// V2 Engine
	v2Persister := &mockPersister{}
	v2Engine := NewBacktestEngine()
	v2Result, err := v2Engine.Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	if err != nil {
		t.Fatalf("V2 Run failed: %v", err)
	}

	// Reference Engine
	ref := newRefEngine(dates, dp, cfg.InitialCapital)
	refResult := ref.run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 100, "BuyAndHoldRising")
}

// ═══════════════════════════════════════════════════════════════
// Test 2: 条件不满足 → 无交易
// ═══════════════════════════════════════════════════════════════

func TestCompare_NoBuyWhenConditionFails(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2", "D3"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates:  dates,
		close:  map[string]float64{"000001|D1": 10, "000001|D2": 9, "000001|D3": 9},
		open:   map[string]float64{"000001|D1": 10, "000001|D2": 9, "000001|D3": 9},
		indVal: map[string]float64{"rsi|000001|D1": 25}, // RSI < 30 → no buy
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 1, "NoBuyWhenConditionFails")

	if v2Result.TradeCount != 0 {
		t.Error("V2: expected 0 trades")
	}
	if refResult.TradeCount != 0 {
		t.Error("Ref: expected 0 trades")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 3: 买入+卖出 完整周期
// ═══════════════════════════════════════════════════════════════

func TestCompare_BuyAndSell(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2", "D3", "D4", "D5", "D6"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 11,
			"000001|D4": 11.5, "000001|D5": 12, "000001|D6": 12.5,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 11,
			"000001|D4": 11.5, "000001|D5": 12, "000001|D6": 12.5,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55, // buy trigger
			"rsi|000001|D4": 75, // sell trigger
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	sellConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, sellConds, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, sellConds, nil, cfg)

	compareResults(t, v2Result, refResult, 200, "BuyAndSell")
}

// ═══════════════════════════════════════════════════════════════
// Test 4: 止盈逻辑
// ═══════════════════════════════════════════════════════════════

func TestCompare_StopProfit(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100
	cfg.StopProfit = 10 // 10% 止盈

	dates := []string{"D1", "D2", "D3", "D4", "D5"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 11.2,
			"000001|D4": 11.5, "000001|D5": 11.5,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 11.2,
			"000001|D4": 11.5, "000001|D5": 11.5,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55,
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 100, "StopProfit")

	// Both should have a sell/stop trade from profit taking
	v2SellCount := 0
	for _, tr := range v2Result.Trades {
		if tr.Action == "sell" || tr.Action == "stop" {
			v2SellCount++
		}
	}
	refSellCount := 0
	for _, tr := range refResult.Trades {
		if tr.Action == "sell" || tr.Action == "stop" {
			refSellCount++
		}
	}
	if v2SellCount == 0 {
		t.Error("V2: expected stop-profit sell")
	}
	if refSellCount == 0 {
		t.Error("Ref: expected stop-profit sell")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 5: 止损逻辑
// ═══════════════════════════════════════════════════════════════

func TestCompare_StopLoss(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100
	cfg.StopLoss = -5 // -5% 止损

	dates := []string{"D1", "D2", "D3", "D4", "D5"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 9.8, "000001|D3": 9.4,
			"000001|D4": 9.0, "000001|D5": 8.8,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 9.8, "000001|D3": 9.4,
			"000001|D4": 9.0, "000001|D5": 8.8,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55,
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 100, "StopLoss")

	// Both should have a stop trade (loss)
	v2HasStop := false
	for _, tr := range v2Result.Trades {
		if tr.Action == "stop" && tr.Pnl < 0 {
			v2HasStop = true
		}
	}
	if !v2HasStop {
		t.Error("V2: expected stop-loss with negative PnL")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 6: 多股票场景
// ═══════════════════════════════════════════════════════════════

func TestCompare_MultiStock(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 200000
	cfg.MaxHoldings = 5
	cfg.BuyPositionPct = 30

	dates := simpleDatesN(8)
	universe := []StockInfo{
		{Code: "000001", Name: "StockA"},
		{Code: "000002", Name: "StockB"},
		{Code: "000003", Name: "StockC"},
	}

	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		indVal: make(map[string]float64),
	}
	for _, s := range []string{"000001", "000002", "000003"} {
		for i, d := range dates {
			basePrice := map[string]float64{"000001": 10, "000002": 20, "000003": 30}[s]
			trend := map[string]float64{"000001": 0.3, "000002": 0.1, "000003": -0.1}[s]
			price := basePrice + float64(i)*trend
			dp.close[s+"|"+d] = price
			dp.open[s+"|"+d] = price
			dp.indVal["rsi|"+s+"|"+d] = 55
		}
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 500, "MultiStock")

	// Both should have trades across multiple stocks
	if v2Result.TradeCount < 2 {
		t.Errorf("V2: expected >=2 trades across 3 stocks, got %d", v2Result.TradeCount)
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 7: 条件组 AND/OR 逻辑
// ═══════════════════════════════════════════════════════════════

func TestCompare_ConditionGroupLogic(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100

	dates := simpleDatesN(5)
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		indVal: make(map[string]float64),
	}
	for _, d := range dates {
		dp.close["000001|"+d] = 10
		dp.open["000001|"+d] = 10
	}

	// Group1: rsi>30 AND macd>0 → macd fails → group1 should fail
	// Group2: volume>1 → passes → overall should pass
	dp.indVal["rsi|000001|"+dates[0]] = 65
	dp.indVal["macd|000001|"+dates[0]] = -1
	dp.indVal["volume|000001|"+dates[0]] = 2

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
		{Indicator: "macd", Operator: "gt", Value: 0, LogicGroup: 1},
		{Indicator: "volume", Operator: "gt", Value: 1, LogicGroup: 2},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 100, "ConditionGroupLogic")

	// Both should have executed a buy (Group2 passes via OR)
	if v2Result.TradeCount == 0 {
		t.Error("V2: expected buy from Group2 OR logic")
	}
	if refResult.TradeCount == 0 {
		t.Error("Ref: expected buy from Group2 OR logic")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 8: 空股票池
// ═══════════════════════════════════════════════════════════════

func TestCompare_EmptyUniverse(t *testing.T) {
	cfg := DefaultBacktestConfig()
	dates := simpleDatesN(5)
	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		indVal: make(map[string]float64),
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, nil, nil, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(nil, nil, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 1, "EmptyUniverse")

	if v2Result.TradeCount != 0 || refResult.TradeCount != 0 {
		t.Error("both engines should produce 0 trades for empty universe")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 9: 资金不足 → 跳过买入
// ═══════════════════════════════════════════════════════════════

func TestCompare_InsufficientFunds(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500
	cfg.BuyPositionPct = 100

	dates := simpleDatesN(4)
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|" + dates[0]: 100, "000001|" + dates[1]: 100,
			"000001|" + dates[2]: 100, "000001|" + dates[3]: 100,
		},
		open: map[string]float64{
			"000001|" + dates[0]: 100, "000001|" + dates[1]: 100,
			"000001|" + dates[2]: 100, "000001|" + dates[3]: 100,
		},
		indVal: map[string]float64{
			"rsi|000001|" + dates[0]: 55,
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 1, "InsufficientFunds")

	if v2Result.TradeCount != 0 || refResult.TradeCount != 0 {
		t.Error("both engines should skip buy with insufficient funds")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 10: 强制平仓
// ═══════════════════════════════════════════════════════════════

func TestCompare_ForceLiquidation(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2", "D3"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

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

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	compareResults(t, v2Result, refResult, 100, "ForceLiquidation")

	// Both should have a sell from force liquidation
	v2HasSell := false
	for _, tr := range v2Result.Trades {
		if tr.Action == "sell" && tr.Reason == "强制平仓（最后交易日）" {
			v2HasSell = true
		}
	}
	if !v2HasSell {
		t.Error("V2: expected force liquidation sell on last day")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 11: 跌停跳过平仓
// ═══════════════════════════════════════════════════════════════

func TestCompare_LimitDownBypass(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 100000
	cfg.BuyPositionPct = 100

	dates := []string{"D1", "D2", "D3"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 9,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 9,
		},
		change:  map[string]float64{"000001|D3": -10.0}, // limit-down
		indVal:  map[string]float64{"rsi|000001|D1": 55},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, nil, nil, cfg)

	if v2Result.TradeCount > 0 {
		// V2 should skip force liquidation of limit-down stocks
		hasForceSell := false
		for _, tr := range v2Result.Trades {
			if tr.Action == "sell" && tr.Reason == "强制平仓（最后交易日）" {
				hasForceSell = true
			}
		}
		if hasForceSell {
			t.Error("V2: should NOT force-liquidate limit-down stocks")
		}
	}

	// Ref engine should also skip limit-down
	hasRefSell := false
	for _, tr := range refResult.Trades {
		if tr.Action == "sell" && tr.Date == dates[len(dates)-1] {
			hasRefSell = true
		}
	}
	if hasRefSell {
		t.Error("Ref: should NOT force-liquidate limit-down stocks")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 12: T+1 限制
// ═══════════════════════════════════════════════════════════════

func TestCompare_T1Restriction(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 200000
	cfg.BuyPositionPct = 50

	dates := []string{"D1", "D2", "D3", "D4"}
	universe := []StockInfo{{Code: "000001", Name: "TestStock"}}

	dp := &mockDataProvider{
		dates: dates,
		close: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 10.5, "000001|D4": 10.5,
		},
		open: map[string]float64{
			"000001|D1": 10, "000001|D2": 10.5, "000001|D3": 10.5, "000001|D4": 10.5,
		},
		indVal: map[string]float64{
			"rsi|000001|D1": 55, // buy
			"rsi|000001|D3": 75, // sell on D3
		},
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}
	sellConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 70, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, sellConds, nil, dp, v2Persister)
	refResult := newRefEngine(dates, dp, cfg.InitialCapital).run(universe, buyConds, nil, sellConds, nil, cfg)

	compareResults(t, v2Result, refResult, 200, "T1Restriction")

	// Both should have buy AND sell
	v2HasSell := false
	for _, s := range v2Persister.signals {
		if s.ActionType == "sell" && s.Status == "executed" {
			v2HasSell = true
		}
	}
	if !v2HasSell {
		t.Error("V2: sell should execute on D4 (D3→D4 satisfies T+1)")
	}
}

// ═══════════════════════════════════════════════════════════════
// Test 13: MaxHoldings 限制
// ═══════════════════════════════════════════════════════════════

func TestCompare_MaxHoldings(t *testing.T) {
	cfg := DefaultBacktestConfig()
	cfg.InitialCapital = 500000
	cfg.MaxHoldings = 2
	cfg.BuyPositionPct = 30

	dates := simpleDatesN(6)
	universe := []StockInfo{
		{Code: "000001", Name: "StockA"},
		{Code: "000002", Name: "StockB"},
		{Code: "000003", Name: "StockC"},
		{Code: "000004", Name: "StockD"},
	}

	dp := &mockDataProvider{
		dates:  dates,
		close:  make(map[string]float64),
		open:   make(map[string]float64),
		indVal: make(map[string]float64),
	}
	for _, s := range []string{"000001", "000002", "000003", "000004"} {
		for _, d := range dates {
			dp.close[s+"|"+d] = 10
			dp.open[s+"|"+d] = 10
			dp.indVal["rsi|"+s+"|"+d] = 55
		}
	}

	buyConds := []ConditionDef{
		{Indicator: "rsi", Operator: "gt", Value: 30, LogicGroup: 1},
	}

	v2Persister := &mockPersister{}
	v2Result, _ := NewBacktestEngine().Run(cfg, universe, buyConds, nil, nil, nil, dp, v2Persister)

	// Count unique stocks bought (not force-liquidated)
	boughtCodes := make(map[string]bool)
	for _, tr := range v2Result.Trades {
		if tr.Action == "buy" {
			boughtCodes[tr.Code] = true
		}
	}
	maxSimultaneous := len(boughtCodes)

	// With MaxHoldings=2, position count should never exceed 2
	if maxSimultaneous > cfg.MaxHoldings {
		t.Errorf("V2: max simultaneous holdings = %d, but limit is %d", maxSimultaneous, cfg.MaxHoldings)
	}
	t.Logf("MaxHoldings test: bought %d unique stocks, limit=%d", maxSimultaneous, cfg.MaxHoldings)
}
