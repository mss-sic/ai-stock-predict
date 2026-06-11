package handler

// ═══════════════════════════════════════════════════════════════
// Position Sizer — 动态仓位分配
// ═══════════════════════════════════════════════════════════════

// SizingMethod defines the position sizing algorithm.
type SizingMethod string

const (
	SizingFixedPct    SizingMethod = "fixed_pct"    // 固定% (当前逻辑, 默认)
	SizingEqualWeight SizingMethod = "equal_weight"  // 等权分配
	SizingKelly       SizingMethod = "kelly"         // Kelly公式
)

// PositionSizer computes buy/add size dynamically.
type PositionSizer struct {
	Method SizingMethod

	// Kelly parameters (populated from backtest stats in v3)
	winRate    float64
	avgWinPct  float64
	avgLossPct float64
	fraction   float64 // conservative factor, default 0.25
}

// NewPositionSizer creates a position sizer with the given method.
func NewPositionSizer(method SizingMethod) *PositionSizer {
	return &PositionSizer{
		Method:     method,
		winRate:    0.45,
		avgWinPct:  8.0,
		avgLossPct: 4.0,
		fraction:   0.25,
	}
}

// ComputeBuySize returns (quantity, cost) for a new buy.
func (ps *PositionSizer) ComputeBuySize(
	code string,
	price float64,
	remainingCash float64,
	positions map[string]*dcPosition,
	maxHoldings int,
	buyPct float64,
) (int, float64) {
	availableSlots := maxHoldings - len(positions)
	if availableSlots <= 0 {
		return 0, 0
	}

	switch ps.Method {

	case SizingEqualWeight:
		// Equal weight: remaining cash / remaining slots
		budget := remainingCash / float64(availableSlots)
		qty := (int(budget/price) / 100) * 100
		if qty < 100 {
			qty = 100
		}
		cost := price * float64(qty)
		if cost > remainingCash {
			qty = (int(remainingCash/price) / 100) * 100
			cost = price * float64(qty)
		}
		return qty, cost

	case SizingKelly:
		kellyFrac := ps.kellyFraction()
		budget := remainingCash * kellyFrac
		qty := (int(budget/price) / 100) * 100
		if qty < 100 {
			qty = 100
		}
		cost := price * float64(qty)
		if cost > remainingCash {
			qty = (int(remainingCash/price) / 100) * 100
			cost = price * float64(qty)
		}
		return qty, cost

	default: // SizingFixedPct
		budget := remainingCash * buyPct / 100
		qty := (int(budget/price) / 100) * 100
		if qty < 100 {
			qty = 100
		}
		cost := price * float64(qty)
		if cost > remainingCash {
			qty = (int(remainingCash/price) / 100) * 100
			cost = price * float64(qty)
		}
		return qty, cost
	}
}

// ComputeAddSize returns (quantity, cost) for adding to an existing position.
func (ps *PositionSizer) ComputeAddSize(
	code string,
	price float64,
	currentQty int,
	remainingCash float64,
	addPct float64,
) (int, float64) {
	// Add amount cap = addPct% of remaining cash
	budget := remainingCash * addPct / 100
	qty := (int(budget/price) / 100) * 100
	if qty < 100 {
		return 0, 0
	}
	cost := price * float64(qty)
	if cost > remainingCash {
		qty = (int(remainingCash/price) / 100) * 100
		cost = price * float64(qty)
	}
	return qty, cost
}

// kellyFraction computes the Kelly-optimal allocation fraction.
// f* = (p*b - q) / b, where p=winRate, q=1-p, b=avgWin/avgLoss.
// Returns conservative fraction (clamped to [0.05, 0.25]).
func (ps *PositionSizer) kellyFraction() float64 {
	b := ps.avgWinPct / ps.avgLossPct
	q := 1 - ps.winRate
	kelly := (ps.winRate*b - q) / b
	kelly = kelly * ps.fraction
	if kelly < 0.05 {
		kelly = 0.05
	}
	if kelly > 0.25 {
		kelly = 0.25
	}
	return kelly
}
