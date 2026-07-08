package service

import (
	"math"
)

// ── SnapshotConfig ──

type SnapshotConfig struct {
	InitialCapital float64
}

// ── SnapshotResult ──

type SnapshotResult struct {
	Date             string
	Cash             float64
	PositionValue    float64
	TotalEquity      float64
	DailyReturn      float64 // daily return pct
	DailyReturnAmt   float64 // daily return amount
	CumulativeReturn float64 // cumulative return pct
	MaxDrawdownPct   float64 // running max drawdown pct
	PositionCount    int
}

// ── SnapshotEngine ──

// SnapshotEngine computes daily portfolio snapshots with performance metrics.
type SnapshotEngine struct {
	initialCapital float64
	prevDayEquity  float64
	peakEquity     float64
}

// NewSnapshotEngine creates a new snapshot engine.
func NewSnapshotEngine(initialCapital float64) *SnapshotEngine {
	return &SnapshotEngine{
		initialCapital: initialCapital,
		prevDayEquity:  initialCapital,
		peakEquity:     initialCapital,
	}
}

// TakeSnapshot computes a daily snapshot and updates running metrics.
func (e *SnapshotEngine) TakeSnapshot(
	date string,
	cash float64,
	positions map[string]SnapshotPosition,
) SnapshotResult {
	posValue := 0.0
	for _, p := range positions {
		posValue += p.MarketPrice * float64(p.Quantity)
	}

	totalEquity := cash + posValue

	// Daily return
	dailyRet := 0.0
	if e.prevDayEquity > 0 {
		dailyRet = (totalEquity - e.prevDayEquity) / e.prevDayEquity * 100
	}

	// Cumulative return
	cumRet := 0.0
	if e.initialCapital > 0 {
		cumRet = (totalEquity - e.initialCapital) / e.initialCapital * 100
	}

	// Max drawdown
	if totalEquity > e.peakEquity {
		e.peakEquity = totalEquity
	}
	drawdown := 0.0
	if e.peakEquity > 0 {
		drawdown = (e.peakEquity - totalEquity) / e.peakEquity * 100
	}

	dailyReturnAmt := totalEquity - e.prevDayEquity
	e.prevDayEquity = totalEquity

	return SnapshotResult{
		Date:             date,
		Cash:             math.Round(cash*100) / 100,
		PositionValue:    math.Round(posValue*100) / 100,
		TotalEquity:      math.Round(totalEquity*100) / 100,
		DailyReturn:      math.Round(dailyRet*100) / 100,
		DailyReturnAmt:   math.Round(dailyReturnAmt*100) / 100,
		CumulativeReturn: math.Round(cumRet*100) / 100,
		MaxDrawdownPct:   math.Round(drawdown*100) / 100,
		PositionCount:    len(positions),
	}
}

// SnapshotPosition is a lightweight position for snapshot calculation.
type SnapshotPosition struct {
	Code        string
	Name        string
	Quantity    int
	BuyPrice    float64
	MarketPrice float64
	MarketValue float64
	ProfitPct   float64
}
