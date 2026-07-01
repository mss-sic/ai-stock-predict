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
	MIN_REDUCE_QTY = 100
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
// DipLot tracks a dip-buy sub-position (抄底批次).
type DipLot struct {
	Qty      int
	BuyPrice float64
	BuyDate  string
}

type dcPosition struct {
	Code        string
	Name        string
	BuyPrice    float64
	Quantity    int
	BuyDate     string
	dangerDays  int
	dangerAccum float64
	LastReduceDate string
	HighestPrice   float64 // 持仓期间最高收盘价（移动止盈用）
	DipLot          *DipLot // 抄底批次（活跃时非nil）
	LastDipDate     string  // 上次抄底日期（冷却期用）
	GridActive      bool     // 网格是否激活
	GridBase        float64  // 网格基准价（布林中轨）
	GridLots        []GridLot // 网格持仓批次
}

// GridLot tracks a single grid layer position.
type GridLot struct {
	Qty      int
	BuyPrice float64
	Level    int
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
