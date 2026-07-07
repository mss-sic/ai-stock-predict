package service

import (
	"math"
	"sort"
)

// BacktestTrade represents a single executed trade during backtest.
type BacktestTrade struct {
	Date       string
	SignalDate string
	Action     string
	Code       string
	Name       string
	Price      float64
	Quantity   int
	Reason     string
	Pnl        float64
	PnlPct     float64
}

// BacktestPerformance aggregates all performance metrics after a backtest run.
type BacktestPerformance struct {
	InitialCapital   float64
	FinalEquity      float64
	TotalReturn      float64
	TotalReturnPct   float64
	AnnualizedReturn float64
	SharpeRatio      float64
	MaxDrawdown      float64
	MaxDrawdownPct   float64
	WinRate          float64
	WinRatePct       float64
	TotalTrades      int
	WinningTrades    int
	AvgWin           float64
	AvgLoss          float64
	ProfitFactor     float64
	DailyReturns     []float64
	EquityCurve      []EquityPoint
}

// EquityPoint is a single point on the equity curve.
type EquityPoint struct {
	Date   string
	Equity float64
	Return float64 // cumulative return %
}

// BacktestService provides backtest execution and performance calculation.
type BacktestService struct {
	positionSvc *PositionService
}

// NewBacktestService creates a new BacktestService.
func NewBacktestService() *BacktestService {
	return &BacktestService{
		positionSvc: NewPositionService(),
	}
}

// CalculatePerformance computes all performance metrics from trade history.
func (s *BacktestService) CalculatePerformance(
	initialCapital float64,
	finalEquity float64,
	trades []BacktestTrade,
	dailyReturns []float64,
	equityCurve []EquityPoint,
	tradingDays int,
) BacktestPerformance {
	totalReturn := finalEquity - initialCapital
	totalReturnPct := 0.0
	if initialCapital > 0 {
		totalReturnPct = totalReturn / initialCapital * 100
	}

	// Annualized return (assuming 252 trading days)
	annualizedReturn := 0.0
	if tradingDays > 0 && initialCapital > 0 {
		years := float64(tradingDays) / 252.0
		if years > 0 {
			annualizedReturn = (math.Pow(finalEquity/initialCapital, 1.0/years) - 1) * 100
		}
	}

	// Sharpe ratio
	sharpe := s.computeSharpe(dailyReturns)

	// Max drawdown
	maxDD, maxDDPct := s.computeMaxDrawdown(equityCurve, initialCapital)

	// Win rate
	winRate, winningTrades := s.computeWinRate(trades)

	// Average win/loss
	avgWin, avgLoss := s.computeAvgWinLoss(trades)

	// Profit factor
	profitFactor := s.computeProfitFactor(trades)

	return BacktestPerformance{
		InitialCapital:   initialCapital,
		FinalEquity:      math.Round(finalEquity*100) / 100,
		TotalReturn:      math.Round(totalReturn*100) / 100,
		TotalReturnPct:   math.Round(totalReturnPct*100) / 100,
		AnnualizedReturn: math.Round(annualizedReturn*100) / 100,
		SharpeRatio:      math.Round(sharpe*10000) / 10000,
		MaxDrawdown:      math.Round(maxDD*100) / 100,
		MaxDrawdownPct:   math.Round(maxDDPct*100) / 100,
		WinRate:          math.Round(winRate*10000) / 10000,
		WinRatePct:       math.Round(winRate*10000) / 100,
		TotalTrades:      len(trades),
		WinningTrades:    winningTrades,
		AvgWin:           math.Round(avgWin*100) / 100,
		AvgLoss:          math.Round(avgLoss*100) / 100,
		ProfitFactor:     math.Round(profitFactor*100) / 100,
		DailyReturns:     dailyReturns,
		EquityCurve:      equityCurve,
	}
}

// computeSharpe calculates annualized Sharpe ratio from daily returns.
func (s *BacktestService) computeSharpe(dailyReturns []float64) float64 {
	if len(dailyReturns) < 2 {
		return 0
	}

	// Mean
	var sum float64
	for _, r := range dailyReturns {
		sum += r
	}
	mean := sum / float64(len(dailyReturns))

	// Std dev
	var sumSq float64
	for _, r := range dailyReturns {
		diff := r - mean
		sumSq += diff * diff
	}
	stdDev := math.Sqrt(sumSq / float64(len(dailyReturns)-1))

	if stdDev == 0 {
		return 0
	}
	// Annualize: Sharpe = (mean / stdDev) * sqrt(252)
	return (mean / stdDev) * math.Sqrt(252)
}

// computeMaxDrawdown calculates maximum drawdown from equity curve.
func (s *BacktestService) computeMaxDrawdown(curve []EquityPoint, initialCapital float64) (amount, pct float64) {
	if len(curve) == 0 {
		return 0, 0
	}

	peak := curve[0].Equity
	maxDD := 0.0

	for _, p := range curve {
		if p.Equity > peak {
			peak = p.Equity
		}
		dd := (peak - p.Equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
			amount = peak - p.Equity
		}
	}

	return amount, maxDD
}

// computeWinRate calculates win rate from completed trades.
func (s *BacktestService) computeWinRate(trades []BacktestTrade) (rate float64, wins int) {
	completed := 0
	for _, t := range trades {
		// Only count sell/stop/reduce/dip_sell as completed trades
		if t.Action == "sell" || t.Action == "stop" || t.Action == "reduce" || t.Action == "dip_sell" || t.Action == "grid_sell" {
			completed++
			if t.Pnl > 0 {
				wins++
			}
		}
	}
	if completed == 0 {
		return 0, 0
	}
	return float64(wins) / float64(completed), wins
}

// computeAvgWinLoss calculates average winning and losing trade P&L.
func (s *BacktestService) computeAvgWinLoss(trades []BacktestTrade) (avgWin, avgLoss float64) {
	var totalWin, totalLoss float64
	var winCount, lossCount int

	for _, t := range trades {
		if t.Action == "sell" || t.Action == "stop" || t.Action == "reduce" || t.Action == "dip_sell" || t.Action == "grid_sell" {
			if t.Pnl > 0 {
				totalWin += t.Pnl
				winCount++
			} else if t.Pnl < 0 {
				totalLoss += math.Abs(t.Pnl)
				lossCount++
			}
		}
	}

	if winCount > 0 {
		avgWin = totalWin / float64(winCount)
	}
	if lossCount > 0 {
		avgLoss = totalLoss / float64(lossCount)
	}
	return
}

// computeProfitFactor calculates ratio of gross profit to gross loss.
func (s *BacktestService) computeProfitFactor(trades []BacktestTrade) float64 {
	var totalWin, totalLoss float64

	for _, t := range trades {
		if t.Action == "sell" || t.Action == "stop" || t.Action == "reduce" || t.Action == "dip_sell" || t.Action == "grid_sell" {
			if t.Pnl > 0 {
				totalWin += t.Pnl
			} else if t.Pnl < 0 {
				totalLoss += math.Abs(t.Pnl)
			}
		}
	}

	if totalLoss == 0 {
		if totalWin > 0 {
			return 999 // effectively infinite
		}
		return 0
	}
	return totalWin / totalLoss
}

// TopN returns the top N results from a sorted list.
func (s *BacktestService) TopN(results []ScoreResult, n int, minScore float64) []ScoreResult {
	if n <= 0 || len(results) == 0 {
		return nil
	}

	// Sort by TotalScore descending
	sorted := make([]ScoreResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalScore > sorted[j].TotalScore
	})

	var out []ScoreResult
	for _, r := range sorted {
		if r.TotalScore < minScore {
			continue
		}
		out = append(out, r)
		if len(out) >= n {
			break
		}
	}
	return out
}
