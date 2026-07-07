package service

import (
	"math"
)

// Position represents a single stock holding during backtest.
type Position struct {
	Code           string
	Name           string
	BuyPrice       float64
	Quantity       int
	BuyDate        string
	LastReduceDate string // for cooldown tracking
}

// TotalCost returns the total cost basis of this position.
func (p *Position) TotalCost() float64 {
	return p.BuyPrice * float64(p.Quantity)
}

// MarketValue returns the current market value at the given price.
func (p *Position) MarketValue(currentPrice float64) float64 {
	return currentPrice * float64(p.Quantity)
}

// UnrealizedPnl returns unrealized profit/loss at the given price.
func (p *Position) UnrealizedPnl(currentPrice float64) float64 {
	return (currentPrice - p.BuyPrice) * float64(p.Quantity)
}

// UnrealizedPnlPct returns unrealized P&L as percentage.
func (p *Position) UnrealizedPnlPct(currentPrice float64) float64 {
	if p.BuyPrice <= 0 {
		return 0
	}
	return (currentPrice - p.BuyPrice) / p.BuyPrice * 100
}

// PositionService manages portfolio positions during backtest.
type PositionService struct{}

// NewPositionService creates a new PositionService.
func NewPositionService() *PositionService { return &PositionService{} }

// Buy executes a buy order and returns the new position.
// Deducts amount + commission from cash pointer; returns nil if insufficient funds.
func (s *PositionService) Buy(
	cash *float64,
	code, name, date string,
	openPrice float64,
	plannedAmount float64,
	commissionRate, minCommission float64,
) *Position {
	actualQty := int(plannedAmount / openPrice / 100) * 100
	if actualQty <= 0 {
		return nil
	}
	actualAmount := openPrice * float64(actualQty)
	commission := math.Max(actualAmount*commissionRate, minCommission)

	// Ensure actualAmount + commission fits within cash
	if actualAmount+commission > *cash {
		// Reduce qty iteratively until (amount + commission) <= cash
		for actualQty >= 100 {
			actualQty -= 100
			actualAmount = openPrice * float64(actualQty)
			commission = math.Max(actualAmount*commissionRate, minCommission)
			if actualAmount+commission <= *cash {
				break
			}
		}
	}
	if actualQty <= 0 || actualAmount+commission > *cash {
		return nil
	}

	*cash -= actualAmount + commission

	return &Position{
		Code:     code,
		Name:     name,
		BuyPrice: openPrice,
		Quantity: actualQty,
		BuyDate:  date,
	}
}

// Add increases an existing position's quantity.
func (s *PositionService) Add(
	cash *float64,
	pos *Position,
	openPrice float64,
	plannedAmount float64,
	commissionRate, minCommission float64,
) (addedQty int) {
	actualQty := int(plannedAmount / openPrice / 100) * 100
	if actualQty <= 0 {
		return 0
	}
	actualAmount := openPrice * float64(actualQty)
	commission := math.Max(actualAmount*commissionRate, minCommission)

	// Ensure actualAmount + commission fits within cash
	if actualAmount+commission > *cash {
		for actualQty >= 100 {
			actualQty -= 100
			actualAmount = openPrice * float64(actualQty)
			commission = math.Max(actualAmount*commissionRate, minCommission)
			if actualAmount+commission <= *cash {
				break
			}
		}
	}
	if actualQty <= 0 || actualAmount+commission > *cash {
		return 0
	}

	*cash -= actualAmount + commission

	// Weighted average cost
	totalCost := pos.BuyPrice*float64(pos.Quantity) + actualAmount
	pos.Quantity += actualQty
	if pos.Quantity > 0 {
		pos.BuyPrice = totalCost / float64(pos.Quantity)
	}
	return actualQty
}

// Sell fully exits a position and returns the realized P&L.
func (s *PositionService) Sell(
	cash *float64,
	pos *Position,
	openPrice float64,
	commissionRate, minCommission, stampTaxRate float64,
) (pnl, pnlPct float64) {
	sellQty := pos.Quantity
	pnl = (openPrice - pos.BuyPrice) * float64(sellQty)
	if pos.BuyPrice > 0 {
		pnlPct = (openPrice - pos.BuyPrice) / pos.BuyPrice * 100
	}

	sellAmount := openPrice * float64(sellQty)
	*cash += sellAmount
	commission := math.Max(sellAmount*commissionRate, minCommission)
	stampTax := sellAmount * stampTaxRate
	*cash -= commission + stampTax

	// Round for clean display
	pnl = math.Round(pnl*100) / 100
	pnlPct = math.Round(pnlPct*100) / 100
	return
}

// Reduce partially exits a position.
// Returns realized P&L and the actual quantity reduced.
func (s *PositionService) Reduce(
	cash *float64,
	pos *Position,
	openPrice float64,
	reduceQty int,
	execDate string,
	commissionRate, minCommission, stampTaxRate float64,
) (pnl, pnlPct float64, actualReduced int) {
	if reduceQty >= pos.Quantity {
		reduceQty = pos.Quantity
	}
	pnl = (openPrice - pos.BuyPrice) * float64(reduceQty)
	if pos.BuyPrice > 0 {
		pnlPct = (openPrice - pos.BuyPrice) / pos.BuyPrice * 100
	}

	reduceAmount := openPrice * float64(reduceQty)
	*cash += reduceAmount
	commission := math.Max(reduceAmount*commissionRate, minCommission)
	stampTax := reduceAmount * stampTaxRate
	*cash -= commission + stampTax

	pos.Quantity -= reduceQty
	pos.LastReduceDate = execDate

	pnl = math.Round(pnl*100) / 100
	pnlPct = math.Round(pnlPct*100) / 100
	return pnl, pnlPct, reduceQty
}

// CalculatePortfolioValue computes total portfolio value (cash + positions).
func (s *PositionService) CalculatePortfolioValue(
	cash float64,
	positions map[string]*Position,
	priceProvider func(code string) float64,
) float64 {
	equity := cash
	for _, pos := range positions {
		price := priceProvider(pos.Code)
		equity += price * float64(pos.Quantity)
	}
	return equity
}

// TransactionCost calculates total cost for a sell trade.
func TransactionCost(amount float64, commissionRate, minCommission, stampTaxRate float64) float64 {
	commission := math.Max(amount*commissionRate, minCommission)
	stampTax := amount * stampTaxRate
	return commission + stampTax
}

// BuyCost calculates commission for a buy trade (no stamp tax on buy in A-shares).
func BuyCost(amount float64, commissionRate, minCommission float64) float64 {
	return math.Max(amount*commissionRate, minCommission)
}
