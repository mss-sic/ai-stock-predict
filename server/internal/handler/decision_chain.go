package handler

import (
	"fmt"
	"log"
	"math"
	"sort"

	"github.com/ai-stock-predict/server/internal/model"
)

// ═══════════════════════════════════════════════════════════════
// Decision Chain Engine — 交易决策链引擎
// ═══════════════════════════════════════════════════════════════

// ── Core Types ──

// ActionType represents a decision node type.
type ActionType string

const (
	ActionStop   ActionType = "stop"   // 止损/止盈
	ActionSell   ActionType = "sell"   // 卖出条件触发
	ActionReduce ActionType = "reduce" // 减仓
	ActionBuy    ActionType = "buy"    // 买入
	ActionAdd    ActionType = "add"    // 加仓
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
	CurrentMV  float64 // current market value (for reduce/add)
	CurrentQty int
	Price      float64
	Reason     string
}

// ActionNode bundles an action type with its priority and candidate targets.
type ActionNode struct {
	Type     ActionType
	Priority int
	Targets  []ActionTarget
}

// StateChange records mutations from executing one action node.
type StateChange struct {
	CashDelta        float64
	PositionsAdded   map[string]*dcPosition
	PositionsRemoved []string
	PositionsUpdated map[string]dcPositionUpdate
	NewTrades        []backtestTrade
	Logs             []dcLogEntry
	HasStop          bool // true if stop-loss/profit was triggered this cycle
}

type dcPositionUpdate struct {
	QuantityDelta int
}

func (sc *StateChange) Apply(cash *float64, positions map[string]*dcPosition) {
	*cash += sc.CashDelta
	for _, code := range sc.PositionsRemoved {
		delete(positions, code)
	}
	for code, pos := range sc.PositionsAdded {
		positions[code] = pos
	}
	for code, upd := range sc.PositionsUpdated {
		if p, ok := positions[code]; ok {
			p.Quantity += upd.QuantityDelta
		}
	}
}

func (sc *StateChange) HasChanges() bool {
	return sc.CashDelta != 0 ||
		len(sc.PositionsAdded) > 0 ||
		len(sc.PositionsRemoved) > 0 ||
		len(sc.PositionsUpdated) > 0
}

// dcLogEntry is a structured log produced by the decision chain.
type dcLogEntry struct {
	Seq    int
	Type   string
	Level  string
	Code   string
	Name   string
	Msg    string
	Detail map[string]interface{}
}

// dcPosition is the decision-chain internal position type (mirrors backtest Position).
type dcPosition struct {
	Code     string
	Name     string
	BuyPrice float64
	Quantity int
	BuyDate  string
}

// ── DayAssessment ──

// DayAssessment is the structured snapshot produced by PositionManager.Assess().
type DayAssessment struct {
	Date            string
	Cash            float64
	PositionMV      float64
	TotalEquity     float64
	PositionRatio   float64 // 持仓市值/总权益
	PositionCount   int
	AvailableSlots  int
	StopTriggers    []ActionTarget
	SellCandidates  []ActionTarget
	BuyCandidates   []ActionTarget
	AddCandidates   []ActionTarget
	ReduceCandidates []ActionTarget
}

func (a *DayAssessment) Summary() string {
	risk := "低"
	if a.PositionRatio > 0.5 {
		risk = "高"
	} else if a.PositionRatio > 0.3 {
		risk = "中"
	}
	return fmt.Sprintf("仓位率%.0f%% 现金%.0f%% 可用槽位%d 风险=%s",
		a.PositionRatio*100, (1-a.PositionRatio)*100, a.AvailableSlots, risk)
}

// ── RiskHook interface (extension point for v2) ──

// RiskHook is called before each action execution.
// Return "" to allow, or a non-empty reason string to block.
type RiskHook interface {
	Name() string
	BeforeAction(action ActionNode, a *DayAssessment) string
}

// ── PositionManager ──

// PositionManager is the orchestrator that assesses state and builds action queues.
type PositionManager struct {
	capital     float64
	maxHoldings int
	buyPct      float64
	addPct      float64
	reducePct   float64
	stopProfit  float64
	stopLoss    float64
	riskHooks   []RiskHook
	sizer       *PositionSizer
}

// NewPositionManager creates a position manager with strategy parameters.
func NewPositionManager(
	capital float64, maxHoldings int,
	buyPct, addPct, reducePct float64,
	stopProfit, stopLoss float64,
	sizer *PositionSizer,
) *PositionManager {
	return &PositionManager{
		capital:     capital,
		maxHoldings: maxHoldings,
		buyPct:      buyPct,
		addPct:      addPct,
		reducePct:   reducePct,
		stopProfit:  stopProfit,
		stopLoss:    stopLoss,
		sizer:       sizer,
	}
}

// SetRiskHooks attaches risk hooks (v2 extension).
func (pm *PositionManager) SetRiskHooks(hooks []RiskHook) {
	pm.riskHooks = hooks
}

// Assess evaluates current state and returns a structured DayAssessment.
func (pm *PositionManager) Assess(
	cash float64,
	positions map[string]*dcPosition,
	universe []dcStockInfo,
	date string,
	buyConds, sellConds, addConds, reduceConds []model.StrategyCondition,
	evalConds func([]model.StrategyCondition, string, string) bool,
	getPrice func(string, string) float64,
) *DayAssessment {
	posMV := 0.0
	for _, p := range positions {
		posMV += getPrice(p.Code, date) * float64(p.Quantity)
	}
	totalEquity := cash + posMV
	posRatio := 0.0
	if totalEquity > 0 {
		posRatio = posMV / totalEquity
	}

	a := &DayAssessment{
		Date:           date,
		Cash:           cash,
		PositionMV:     posMV,
		TotalEquity:    totalEquity,
		PositionRatio:  posRatio,
		PositionCount:  len(positions),
		AvailableSlots: pm.maxHoldings - len(positions),
	}

	// 1. Find stop triggers (always highest priority)
	for code, pos := range positions {
		price := getPrice(code, date)
		if price <= 0 {
			continue
		}
		triggered := ""
		if pm.stopProfit > 0 && price >= pos.BuyPrice*(1+pm.stopProfit/100) {
			triggered = "止盈"
		} else if pm.stopLoss < 0 && price <= pos.BuyPrice*(1+pm.stopLoss/100) {
			triggered = "止损"
		}
		if triggered != "" {
			a.StopTriggers = append(a.StopTriggers, ActionTarget{
				Code: code, Name: pos.Name, Price: price,
				CurrentQty: pos.Quantity, Reason: triggered,
			})
		}
	}

	// 2. Find sell candidates
	if len(sellConds) > 0 {
		for code, pos := range positions {
			price := getPrice(code, date)
			if price <= 0 {
				continue
			}
			if evalConds(sellConds, code, date) {
				a.SellCandidates = append(a.SellCandidates, ActionTarget{
					Code: code, Name: pos.Name, Price: price,
					CurrentQty: pos.Quantity, Reason: "卖出条件触发",
				})
			}
		}
	}

	// 3. Find reduce candidates
	if len(reduceConds) > 0 {
		for code, pos := range positions {
			price := getPrice(code, date)
			if price <= 0 {
				continue
			}
			if evalConds(reduceConds, code, date) {
				a.ReduceCandidates = append(a.ReduceCandidates, ActionTarget{
					Code: code, Name: pos.Name, Price: price,
					CurrentQty: pos.Quantity, Reason: "减仓条件触发",
				})
			}
		}
	}

	// 4. Find buy candidates
	if len(buyConds) > 0 && a.AvailableSlots > 0 {
		for _, stock := range universe {
			if _, exists := positions[stock.Code]; exists {
				continue // already held
			}
			price := getPrice(stock.Code, date)
			if price <= 0 {
				continue
			}
			if evalConds(buyConds, stock.Code, date) {
				a.BuyCandidates = append(a.BuyCandidates, ActionTarget{
					Code: stock.Code, Name: stock.Name, Price: price,
					Reason: "买入条件触发",
				})
			}
		}
	}

	// 5. Find add candidates
	if len(addConds) > 0 {
		for code, pos := range positions {
			price := getPrice(code, date)
			if price <= 0 {
				continue
			}
			if evalConds(addConds, code, date) {
				a.AddCandidates = append(a.AddCandidates, ActionTarget{
					Code: code, Name: pos.Name, Price: price,
					CurrentQty: pos.Quantity, Reason: "加仓条件触发",
				})
			}
		}
	}

	return a
}

// BuildActionQueue builds a priority-ordered action queue from an assessment.
func (pm *PositionManager) BuildActionQueue(a *DayAssessment) []ActionNode {
	queue := make([]ActionNode, 0)

	// Priority 0: Stop-loss/profit — always runs first, unconditionally
	if len(a.StopTriggers) > 0 {
		queue = append(queue, ActionNode{Type: ActionStop, Priority: 0, Targets: a.StopTriggers})
	}

	// Priorities 1-4 depend on position ratio
	switch {
	case a.PositionRatio >= 0.50:
		// Heavy position → sell-first
		if len(a.SellCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionSell, Priority: 1, Targets: a.SellCandidates})
		}
		if len(a.ReduceCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionReduce, Priority: 2, Targets: a.ReduceCandidates})
		}
		if len(a.BuyCandidates) > 0 && a.AvailableSlots > 0 {
			queue = append(queue, ActionNode{Type: ActionBuy, Priority: 3, Targets: a.BuyCandidates})
		}
		if len(a.AddCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionAdd, Priority: 4, Targets: a.AddCandidates})
		}

	case a.PositionRatio < 0.30:
		// Light position → buy-first
		if len(a.BuyCandidates) > 0 && a.AvailableSlots > 0 {
			queue = append(queue, ActionNode{Type: ActionBuy, Priority: 1, Targets: a.BuyCandidates})
		}
		if len(a.AddCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionAdd, Priority: 2, Targets: a.AddCandidates})
		}
		if len(a.SellCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionSell, Priority: 3, Targets: a.SellCandidates})
		}
		if len(a.ReduceCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionReduce, Priority: 4, Targets: a.ReduceCandidates})
		}

	default:
		// Neutral (30%-50%) — balanced, sell slightly preferred
		if len(a.SellCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionSell, Priority: 1, Targets: a.SellCandidates})
		}
		if len(a.BuyCandidates) > 0 && a.AvailableSlots > 0 {
			queue = append(queue, ActionNode{Type: ActionBuy, Priority: 2, Targets: a.BuyCandidates})
		}
		if len(a.AddCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionAdd, Priority: 3, Targets: a.AddCandidates})
		}
		if len(a.ReduceCandidates) > 0 {
			queue = append(queue, ActionNode{Type: ActionReduce, Priority: 4, Targets: a.ReduceCandidates})
		}
	}

	return queue
}

// ── Executors (each produces a StateChange) ──

func (pm *PositionManager) ExecuteStop(
	action ActionNode, cash float64, positions map[string]*dcPosition,
	date string, getPrice func(string, string) float64,
) StateChange {
	sc := StateChange{}
	for _, t := range action.Targets {
		pos, ok := positions[t.Code]
		if !ok {
			continue
		}
		price := getPrice(t.Code, date)
		if price <= 0 {
			continue
		}
		pnl := (price - pos.BuyPrice) * float64(pos.Quantity)
		pnlPct := (price - pos.BuyPrice) / pos.BuyPrice * 100
		proceeds := price * float64(pos.Quantity)

		sc.CashDelta += proceeds
		sc.PositionsRemoved = append(sc.PositionsRemoved, t.Code)
		sc.NewTrades = append(sc.NewTrades, backtestTrade{
			Date: date, Code: t.Code, Name: pos.Name, Action: "sell",
			Price: price, Quantity: pos.Quantity, Reason: t.Reason,
			Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
		})
		sc.HasStop = true
		sc.Logs = append(sc.Logs, dcLogEntry{
			Type: "executor", Level: "info", Code: t.Code, Name: pos.Name,
			Msg: fmt.Sprintf("  %s: %s ¥%.2f×%d=¥%.0f 盈亏¥%.0f(%.1f%%)",
				t.Reason, t.Code, price, pos.Quantity, proceeds, pnl, pnlPct),
		})
	}
	return sc
}

func (pm *PositionManager) ExecuteSell(
	action ActionNode, cash float64, positions map[string]*dcPosition,
	date string, getPrice func(string, string) float64,
	evalConds func([]model.StrategyCondition, string, string) bool,
	sellConds []model.StrategyCondition,
) StateChange {
	sc := StateChange{}
	for _, t := range action.Targets {
		pos, ok := positions[t.Code]
		if !ok {
			continue
		}
		price := getPrice(t.Code, date)
		if price <= 0 {
			continue
		}
		// Re-check condition at execution time (state may have changed)
		if len(sellConds) > 0 && !evalConds(sellConds, t.Code, date) {
			continue
		}
		pnl := (price - pos.BuyPrice) * float64(pos.Quantity)
		pnlPct := (price - pos.BuyPrice) / pos.BuyPrice * 100
		proceeds := price * float64(pos.Quantity)

		sc.CashDelta += proceeds
		sc.PositionsRemoved = append(sc.PositionsRemoved, t.Code)
		sc.NewTrades = append(sc.NewTrades, backtestTrade{
			Date: date, Code: t.Code, Name: pos.Name, Action: "sell",
			Price: price, Quantity: pos.Quantity, Reason: t.Reason,
			Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
		})
		sc.Logs = append(sc.Logs, dcLogEntry{
			Type: "executor", Level: "info", Code: t.Code, Name: pos.Name,
			Msg: fmt.Sprintf("  卖出: %s ¥%.2f×%d=¥%.0f 盈亏¥%.0f(%.1f%%)",
				t.Code, price, pos.Quantity, proceeds, pnl, pnlPct),
		})
	}
	return sc
}

func (pm *PositionManager) ExecuteReduce(
	action ActionNode, cash float64, positions map[string]*dcPosition,
	date string, getPrice func(string, string) float64,
	evalConds func([]model.StrategyCondition, string, string) bool,
	reduceConds []model.StrategyCondition,
) StateChange {
	sc := StateChange{}
	for _, t := range action.Targets {
		pos, ok := positions[t.Code]
		if !ok {
			continue
		}
		price := getPrice(t.Code, date)
		if price <= 0 {
			continue
		}
		if len(reduceConds) > 0 && !evalConds(reduceConds, t.Code, date) {
			continue
		}
		reduceQty := int(float64(pos.Quantity) * pm.reducePct / 100)
		if reduceQty < 100 || reduceQty >= pos.Quantity {
			continue // minimum 100 shares, can't reduce to zero
		}
		proceeds := price * float64(reduceQty)
		pnl := (price - pos.BuyPrice) * float64(reduceQty)
		pnlPct := (price - pos.BuyPrice) / pos.BuyPrice * 100

		sc.CashDelta += proceeds
		sc.PositionsUpdated = make(map[string]dcPositionUpdate)
		sc.PositionsUpdated[t.Code] = dcPositionUpdate{QuantityDelta: -reduceQty}
		sc.NewTrades = append(sc.NewTrades, backtestTrade{
			Date: date, Code: t.Code, Name: pos.Name, Action: "reduce",
			Price: price, Quantity: reduceQty, Reason: t.Reason,
			Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
		})
		sc.Logs = append(sc.Logs, dcLogEntry{
			Type: "executor", Level: "info", Code: t.Code, Name: pos.Name,
			Msg: fmt.Sprintf("  减仓: %s ¥%.2f×%d(%.0f%%) 释放¥%.0f",
				t.Code, price, reduceQty, pm.reducePct, proceeds),
		})
	}
	return sc
}

func (pm *PositionManager) ExecuteBuy(
	action ActionNode, cash float64, positions map[string]*dcPosition,
	universe []dcStockInfo, date string, getPrice func(string, string) float64,
	evalConds func([]model.StrategyCondition, string, string) bool,
	buyConds []model.StrategyCondition,
) StateChange {
	sc := StateChange{PositionsAdded: make(map[string]*dcPosition)}
	remainingCash := cash
	bought := 0
	slotLimit := pm.maxHoldings - len(positions)

	// Sort candidates by some criterion (e.g., higher price = more liquidity)
	// For now keep order from Assess
	for _, t := range action.Targets {
		if bought >= slotLimit {
			break
		}
		if _, exists := positions[t.Code]; exists {
			continue // already held (e.g. from a concurrent buy)
		}
		price := getPrice(t.Code, date)
		if price <= 0 {
			continue
		}
		if len(buyConds) > 0 && !evalConds(buyConds, t.Code, date) {
			continue
		}

		// Compute buy size using the sizer
		qty, cost := pm.sizer.ComputeBuySize(t.Code, price, remainingCash, positions, pm.maxHoldings, pm.buyPct)
		if qty < 100 || cost > remainingCash {
			sc.Logs = append(sc.Logs, dcLogEntry{
				Type: "executor", Level: "warn", Code: t.Code, Name: t.Name,
				Msg: fmt.Sprintf("  %s 资金不足: 需¥%.0f 现金¥%.0f", t.Code, cost, remainingCash),
			})
			continue
		}

		remainingCash -= cost
		name := t.Name
		if name == "" {
			name = t.Code
		}
		sc.PositionsAdded[t.Code] = &dcPosition{
			Code: t.Code, Name: name,
			BuyPrice: price, Quantity: qty, BuyDate: date,
		}
		sc.NewTrades = append(sc.NewTrades, backtestTrade{
			Date: date, Code: t.Code, Name: name, Action: "buy",
			Price: price, Quantity: qty, Reason: t.Reason,
		})
		sc.Logs = append(sc.Logs, dcLogEntry{
			Type: "executor", Level: "info", Code: t.Code, Name: name,
			Msg: fmt.Sprintf("  买入: %s ¥%.2f×%d=¥%.0f", t.Code, price, qty, cost),
		})
		bought++
	}
	sc.CashDelta = remainingCash - cash // negative (money spent)
	return sc
}

func (pm *PositionManager) ExecuteAdd(
	action ActionNode, cash float64, positions map[string]*dcPosition,
	date string, getPrice func(string, string) float64,
	evalConds func([]model.StrategyCondition, string, string) bool,
	addConds []model.StrategyCondition,
) StateChange {
	sc := StateChange{PositionsUpdated: make(map[string]dcPositionUpdate)}
	remainingCash := cash

	for _, t := range action.Targets {
		pos, ok := positions[t.Code]
		if !ok {
			continue
		}
		price := getPrice(t.Code, date)
		if price <= 0 {
			continue
		}
		if len(addConds) > 0 && !evalConds(addConds, t.Code, date) {
			continue
		}

		// Use sizer for add size
		qty, cost := pm.sizer.ComputeAddSize(t.Code, price, pos.Quantity, remainingCash, pm.addPct)
		if qty < 100 || cost > remainingCash {
			sc.Logs = append(sc.Logs, dcLogEntry{
				Type: "executor", Level: "warn", Code: t.Code, Name: pos.Name,
				Msg: fmt.Sprintf("  %s 加仓资金不足: 需¥%.0f 现金¥%.0f", t.Code, cost, remainingCash),
			})
			continue
		}

		remainingCash -= cost
		sc.PositionsUpdated[t.Code] = dcPositionUpdate{QuantityDelta: qty}
		sc.NewTrades = append(sc.NewTrades, backtestTrade{
			Date: date, Code: t.Code, Name: pos.Name, Action: "add",
			Price: price, Quantity: qty, Reason: t.Reason,
		})
		sc.Logs = append(sc.Logs, dcLogEntry{
			Type: "executor", Level: "info", Code: t.Code, Name: pos.Name,
			Msg: fmt.Sprintf("  加仓: %s ¥%.2f×%d=¥%.0f", t.Code, price, qty, cost),
		})
	}
	sc.CashDelta = remainingCash - cash // negative
	return sc
}

// ── dcStockInfo (mirrors backtest-local StockInfo) ──

type dcStockInfo struct {
	Code string
	Name string
}

// ── Decision Chain Loop ──

// RunDailyDecisionLoop executes the trading decision chain for one trading day.
// Returns updated cash, today's trades, and structured logs.
func RunDailyDecisionLoop(
	date string,
	cash float64,
	positions map[string]*dcPosition,
	universe []dcStockInfo,
	buyConds, sellConds, addConds, reduceConds []model.StrategyCondition,
	pm *PositionManager,
	getPrice func(string, string) float64,
	evalConds func([]model.StrategyCondition, string, string) bool,
	evalSingleWithDetail func(model.StrategyCondition, string, string) (bool, string),
) (float64, []backtestTrade, []dcLogEntry) {

	var allTrades []backtestTrade
	var allLogs []dcLogEntry
	remainingCash := cash

	maxIter := 10
	logSeq := 10

	for iter := 0; iter < maxIter; iter++ {
		// 1. Assess
		assessment := pm.Assess(remainingCash, positions, universe, date,
			buyConds, sellConds, addConds, reduceConds, evalConds, getPrice)

		// First iteration: emit assessment summary
		if iter == 0 {
			allLogs = append(allLogs, dcLogEntry{
				Seq: 5, Type: "position_mgr", Level: "info",
				Msg: fmt.Sprintf("评估: %s", assessment.Summary()),
			})
		}

		// 2. Build priority queue
		queue := pm.BuildActionQueue(assessment)

		if len(queue) == 0 {
			allLogs = append(allLogs, dcLogEntry{
				Seq: 99, Type: "system", Level: "info",
				Msg: fmt.Sprintf("决策链完成 — 持仓%d只 现金¥%.0f 权益¥%.0f",
					len(positions), remainingCash, assessment.TotalEquity),
			})
			break
		}

		// 3. Take highest-priority action
		action := queue[0]
		logSeq += 10

		// Log the queue decision
		queueTypes := make([]string, len(queue))
		for i, a := range queue {
			queueTypes[i] = string(a.Type)
		}
		allLogs = append(allLogs, dcLogEntry{
			Seq: logSeq - 5, Type: "position_mgr", Level: "info",
			Msg: fmt.Sprintf("决策: %s → 队列[%s]",
				map[bool]string{true: "重仓→卖优先", false: ""}[assessment.PositionRatio >= 0.5],
				joinActions(queueTypes)),
		})

		// Log the scan phase
		switch action.Type {
		case ActionSell:
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq, Type: "sell_scanner", Level: "info",
				Msg: fmt.Sprintf("卖出扫描: %d只持仓, %d只满足卖出条件",
					assessment.PositionCount, len(action.Targets)),
			})
		case ActionBuy:
			condDesc := describeConds(buyConds)
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq, Type: "buy_scanner", Level: "info",
				Msg: fmt.Sprintf("买入扫描: 遍历%d只, 条件[%s] → %d只满足, 可用槽位%d",
					len(universe), condDesc, len(action.Targets), assessment.AvailableSlots),
			})
		case ActionAdd:
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq, Type: "add_scanner", Level: "info",
				Msg: fmt.Sprintf("加仓扫描: %d只持仓, %d只满足加仓条件",
					assessment.PositionCount, len(action.Targets)),
			})
		case ActionReduce:
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq, Type: "reduce_scanner", Level: "info",
				Msg: fmt.Sprintf("减仓扫描: %d只持仓, %d只满足减仓条件",
					assessment.PositionCount, len(action.Targets)),
			})
		case ActionStop:
			stopReasons := make([]string, 0)
			for _, t := range action.Targets {
				stopReasons = append(stopReasons, fmt.Sprintf("%s(%s)", t.Code, t.Reason))
			}
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq, Type: "stop_check", Level: "warn",
				Msg: fmt.Sprintf("止损/止盈触发: %v", stopReasons),
			})
		}

		// 4. Risk check
		blocked := false
		for _, hook := range pm.riskHooks {
			if reason := hook.BeforeAction(action, assessment); reason != "" {
				allLogs = append(allLogs, dcLogEntry{
					Seq: logSeq + 1, Type: "risk", Level: "warn",
					Msg: fmt.Sprintf("[risk:%s] 跳过%s: %s", hook.Name(), action.Type.Label(), reason),
				})
				blocked = true
				break
			}
		}
		if blocked {
			// Remove this action type from consideration, try next priority
			continue
		}

		// 5. Execute
		var sc StateChange
		switch action.Type {
		case ActionStop:
			sc = pm.ExecuteStop(action, remainingCash, positions, date, getPrice)
		case ActionSell:
			sc = pm.ExecuteSell(action, remainingCash, positions, date, getPrice, evalConds, sellConds)
		case ActionReduce:
			sc = pm.ExecuteReduce(action, remainingCash, positions, date, getPrice, evalConds, reduceConds)
		case ActionBuy:
			sc = pm.ExecuteBuy(action, remainingCash, positions, universe, date, getPrice, evalConds, buyConds)
		case ActionAdd:
			sc = pm.ExecuteAdd(action, remainingCash, positions, date, getPrice, evalConds, addConds)
		}

		// 6. Apply changes
		sc.Apply(&remainingCash, positions)
		allTrades = append(allTrades, sc.NewTrades...)
		allLogs = append(allLogs, sc.Logs...)

		// 7. If no changes from this action, move to next priority
		if !sc.HasChanges() {
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq + 2, Type: action.Type.String(), Level: "info",
				Msg: fmt.Sprintf("[%s] 本轮无成交, 尝试下一优先级", action.Type.Label()),
			})
		} else {
			// Changes made — re-assess (continue loop)
			allLogs = append(allLogs, dcLogEntry{
				Seq: logSeq + 2, Type: "position_mgr", Level: "info",
				Msg: fmt.Sprintf("执行完成, 重新评估 — 持仓%d只 现金¥%.0f",
					len(positions), remainingCash),
			})
		}
	}

	return remainingCash, allTrades, allLogs
}

// ── Helpers ──

func (a ActionType) String() string { return string(a) }

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

func describeConds(conds []model.StrategyCondition) string {
	if len(conds) == 0 {
		return "无"
	}
	parts := make([]string, 0, len(conds))
	seen := make(map[string]bool)
	for _, c := range conds {
		key := fmt.Sprintf("%s %s %.0f", c.Indicator, c.Operator, c.Value)
		if !seen[key] {
			parts = append(parts, key)
			seen[key] = true
		}
	}
	sort.Strings(parts)
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += ", "
		}
		s += p
	}
	return s
}

// dcScanLog produces per-stock diagnostic logs for small universes.
func dcScanLog(
	date string,
	buyConds []model.StrategyCondition,
	universe []dcStockInfo,
	positions map[string]*dcPosition,
	getPrice func(string, string) float64,
	evalSingleWithDetail func(model.StrategyCondition, string, string) (bool, string),
) []dcLogEntry {
	if len(buyConds) == 0 || len(universe) > 10 {
		return nil
	}

	var logs []dcLogEntry
	seq := 30
	maxDetail := len(universe)
	if maxDetail > 8 {
		maxDetail = 8
	}
	log.Printf("[backtest] date=%s emitting per-stock diag for %d stocks", date, maxDetail)

	for si, stock := range universe {
		if si >= maxDetail {
			break
		}
		if _, exists := positions[stock.Code]; exists {
			continue
		}
		price := getPrice(stock.Code, date)
		if price <= 0 {
			logs = append(logs, dcLogEntry{
				Seq: seq, Type: "condition_eval", Level: "warn", Code: stock.Code, Name: stock.Name,
				Msg: fmt.Sprintf("  %s 无K线数据, 跳过", stock.Code),
			})
			seq++
			continue
		}
		condResults := make([]string, 0)
		for _, c := range buyConds {
			passed, reason := evalSingleWithDetail(c, stock.Code, date)
			condResults = append(condResults, reason)
			_ = passed
		}
		logs = append(logs, dcLogEntry{
			Seq: seq, Type: "condition_eval", Level: "info", Code: stock.Code, Name: stock.Name,
			Msg: fmt.Sprintf("  %s ¥%.2f → %s", stock.Code, price, joinActions(condResults)),
		})
		seq++
	}

	return logs
}
