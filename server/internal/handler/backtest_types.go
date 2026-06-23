package handler

// Shared types used across backtest engine, scoring engine, AI agent, and strategy handler.

type ActionType string

const (
	ActionStop   ActionType = "stop"
	ActionSell   ActionType = "sell"
	ActionReduce ActionType = "reduce"
	ActionBuy    ActionType = "buy"
	ActionAdd    ActionType = "add"

	// REDUCE_COOLDOWN_DAYS prevents repeated reduce signals within N trading days
	REDUCE_COOLDOWN_DAYS = 5
	// MIN_REDUCE_QTY is the minimum shares for a reduce; below this, skip or sell all
	MIN_REDUCE_QTY = 10
)

func (a ActionType) Label() string {
	switch a {
	case ActionStop:
		return "止损/止盈"
	case ActionSell:
		return "卖出"
	case ActionReduce:
		return "减仓"
	case ActionBuy:
		return "买入"
	case ActionAdd:
		return "加仓"
	}
	return string(a)
}

// ActionTarget is a candidate stock for an action.
type ActionTarget struct {
	Code       string
	Name       string
	CurrentMV  float64
	CurrentQty int
	Price      float64
	Reason     string
}

// dcPosition represents a position held during backtest simulation.
type dcPosition struct {
	Code        string
	Name        string
	BuyPrice    float64
	Quantity    int
	BuyDate     string
	dangerDays  int
	dangerAccum float64
	LastReduceDate string
}

// dcStockInfo is a minimal stock entry used in scoring/decision universes.
type dcStockInfo struct {
	Code string
	Name string
}

// joinActions concatenates action descriptions with "→" separator.
func joinActions(items []string) string {
	s := ""
	for i, item := range items {
		if i > 0 {
			s += "→"
		}
		s += item
	}
	return s
}
