package service

import (
	"fmt"
	"log"
	"strings"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// OrderSyncService handles periodic synchronization of order status from brokers.
// It scans signals with pending_order / partial_filled status, queries the broker
// for actual order status, and updates signal state + positions accordingly.
type OrderSyncService struct {
	brokerSvc *BrokerService
}

// NewOrderSyncService creates a new order sync service.
func NewOrderSyncService() *OrderSyncService {
	return &OrderSyncService{
		brokerSvc: NewBrokerService(),
	}
}

// mxStatusMap maps 妙想 order status codes to our signal statuses.
//   1=未报  2=已报  3=部成  4=已成  5=部成待撤  6=已报待撤  7=部撤  8=已撤  9=废单  10=撤单失败
var mxStatusMap = map[int]string{
	1:  "pending_order",   // 未报 — still pending
	2:  "pending_order",   // 已报 — still pending
	3:  "partial_filled",  // 部成
	4:  "executed",        // 已成
	5:  "partial_filled",  // 部成待撤 — treat as partial for now
	6:  "pending_order",   // 已报待撤 — still in progress
	7:  "cancelled",       // 部撤
	8:  "cancelled",       // 已撤
	9:  "order_failed",    // 废单
	10: "pending_order",   // 撤单失败 — still pending
}

// mxStatusLabel returns a human-readable label for a 妙想 status code.
func mxStatusLabel(status int) string {
	labels := map[int]string{
		1: "未报", 2: "已报", 3: "部成", 4: "已成", 5: "部成待撤",
		6: "已报待撤", 7: "部撤", 8: "已撤", 9: "废单", 10: "撤单失败",
	}
	if l, ok := labels[status]; ok {
		return l
	}
	return fmt.Sprintf("未知(%d)", status)
}

// SyncResult summarizes the outcome of a sync run.
type SyncResult struct {
	TotalScanned  int      `json:"totalScanned"`
	Updated       int      `json:"updated"`       // status changed
	Executed      int      `json:"executed"`      // newly filled
	PartialFilled int      `json:"partialFilled"`
	Cancelled     int      `json:"cancelled"`
	Failed        int      `json:"failed"`
	Logs          []string `json:"logs"`
}

// SyncAllPendingOrders scans all pending_order / partial_filled signals across all runs
// and syncs their status from the respective broker.
// Called by the scheduler every 30 minutes during trading hours.
func (s *OrderSyncService) SyncAllPendingOrders() (*SyncResult, error) {
	result := &SyncResult{}

	// Find all signals that need syncing
	var signals []model.BacktestSignal
	db.MySQL.Where("status IN ?", []string{"pending_order", "partial_filled"}).
		Order("run_id ASC, id ASC").
		Find(&signals)

	result.TotalScanned = len(signals)
	if len(signals) == 0 {
		result.Logs = append(result.Logs, "无需同步的委托订单")
		return result, nil
	}

	result.Logs = append(result.Logs, fmt.Sprintf("扫描到 %d 条待同步委托", len(signals)))

	// Group by run to get account info efficiently
	type runKey struct {
		RunID     uint
		AccountID uint
		UserID    uint
	}
	runAccounts := make(map[uint]*runKey) // runID → account info

	// Process each signal
	for i := range signals {
		sig := &signals[i]

		// Resolve account for this run (cached)
		rk, ok := runAccounts[sig.RunID]
		if !ok {
			var run model.StrategyRun
			if err := db.MySQL.Where("id = ?", sig.RunID).First(&run).Error; err != nil {
				result.Logs = append(result.Logs, fmt.Sprintf("⚠️ 信号%d: run %d 不存在, 跳过", sig.ID, sig.RunID))
				continue
			}
			var account model.TradingAccount
			if run.AccountID > 0 {
				db.MySQL.Where("id = ?", run.AccountID).First(&account)
			}
			if account.ID == 0 {
				db.MySQL.Where("user_id = ? AND status = ?", run.UserID, "active").
					Order("id ASC").First(&account)
			}
			rk = &runKey{RunID: sig.RunID, AccountID: account.ID, UserID: account.UserID}
			runAccounts[sig.RunID] = rk
		}
		if rk.AccountID == 0 {
			result.Logs = append(result.Logs, fmt.Sprintf("⚠️ 信号%d: run %d 无关联账户, 跳过", sig.ID, sig.RunID))
			continue
		}

		// Use dedicated broker_order_id field
		orderID := sig.BrokerOrderID
		if orderID == "" {
			result.Logs = append(result.Logs, fmt.Sprintf("⚠️ 信号%d %s: 无 broker_order_id, 跳过", sig.ID, sig.StockCode))
			continue
		}

		// Query broker for this order
		orders, err := s.brokerSvc.GetBrokerOrders(rk.AccountID, rk.UserID)
		if err != nil {
			result.Logs = append(result.Logs, fmt.Sprintf("❌ 查询账户%d委托失败: %v", rk.AccountID, err))
			continue
		}

		// Find matching order
		var matched *BrokerOrder
		for j := range orders {
			if orders[j].OrderID == orderID {
				matched = &orders[j]
				break
			}
		}
		if matched == nil {
			// Order not found in broker list — might have been cancelled externally
			result.Logs = append(result.Logs, fmt.Sprintf("❓ %s %s orderID=%s 未在委托列表中找到",
				sig.StockCode, sig.StockName, orderID))
			continue
		}

		// Map broker status to our signal status
		newStatus, ok := mxStatusMap[matched.Status]
		if !ok {
			result.Logs = append(result.Logs, fmt.Sprintf("⚠️ %s %s: 未知委托状态 %d",
				sig.StockCode, sig.StockName, matched.Status))
			continue
		}

		oldStatus := sig.Status
		if newStatus == oldStatus {
			// Status unchanged — nothing to do (but still log for visibility)
			continue
		}

		// Update signal
		sig.Status = newStatus
		sig.SkipReason = fmt.Sprintf("委托同步: %s orderID=%s", mxStatusLabel(matched.Status), orderID)

		// If filled (executed or partial_filled), update execution details
		if newStatus == "executed" || newStatus == "partial_filled" {
			if matched.FilledQty > 0 {
				sig.ExecQty = matched.FilledQty
			}
			if matched.Price > 0 {
				sig.ExecPrice = matched.Price
			}
			sig.ExecAmount = sig.ExecPrice * float64(sig.ExecQty)
		}

		db.MySQL.Save(sig)

		// Classify the transition
		switch newStatus {
		case "executed":
			result.Executed++
			result.Logs = append(result.Logs, fmt.Sprintf("✅ %s %s 已成: %d股@%.2f orderID=%s",
				sig.StockCode, sig.StockName, sig.ExecQty, sig.ExecPrice, orderID))

			// Create LiveTrade record on execution
			s.createTradeRecord(sig, rk.UserID, rk.RunID)

			// Update position on full execution
			s.updatePositionOnExecution(sig, rk.RunID)

		case "partial_filled":
			result.PartialFilled++
			result.Logs = append(result.Logs, fmt.Sprintf("📊 %s %s 部成: %d/%d股 orderID=%s",
				sig.StockCode, sig.StockName, matched.FilledQty, matched.Quantity, orderID))

		case "cancelled":
			result.Cancelled++
			result.Logs = append(result.Logs, fmt.Sprintf("🚫 %s %s 已撤: orderID=%s",
				sig.StockCode, sig.StockName, orderID))

		case "order_failed":
			result.Failed++
			result.Logs = append(result.Logs, fmt.Sprintf("❌ %s %s 废单: orderID=%s",
				sig.StockCode, sig.StockName, orderID))
		}

		result.Updated++
		log.Printf("[order_sync] %s %s: %s → %s orderID=%s",
			sig.StockCode, sig.StockName, oldStatus, newStatus, orderID)
	}

	result.Logs = append(result.Logs, fmt.Sprintf("同步完成: 扫描%d 更新%d 已成%d 部成%d 已撤%d 废单%d",
		result.TotalScanned, result.Updated, result.Executed,
		result.PartialFilled, result.Cancelled, result.Failed))

	return result, nil
}

// updatePositionOnExecution updates or creates a LivePosition when an order is filled.
func (s *OrderSyncService) updatePositionOnExecution(sig *model.BacktestSignal, runID uint) {
	action := strings.ToLower(sig.ActionType)

	if action == "buy" || action == "add" {
		// Check if position already exists
		var existing model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND stock_code = ?", runID, sig.StockCode).First(&existing)

		if existing.ID > 0 {
			// Update existing position
			totalQty := existing.Quantity + sig.ExecQty
			newAvgCost := (existing.AvgCost*float64(existing.Quantity) + sig.ExecPrice*float64(sig.ExecQty)) / float64(totalQty)
			db.MySQL.Model(&existing).Updates(map[string]interface{}{
				"quantity":      totalQty,
				"avg_cost":      newAvgCost,
				"current_price": sig.ExecPrice,
			})
			log.Printf("[order_sync] position updated: %s %d→%d avg=%.2f", sig.StockCode, existing.Quantity, totalQty, newAvgCost)
		} else {
			// Create new position
			pos := model.LivePosition{
				StrategyRunID: runID,
				StockCode:     sig.StockCode,
				StockName:     sig.StockName,
				Quantity:      sig.ExecQty,
				AvgCost:       sig.ExecPrice,
				CurrentPrice:  sig.ExecPrice,
			}
			db.MySQL.Create(&pos)
			log.Printf("[order_sync] position created: %s qty=%d price=%.2f", sig.StockCode, sig.ExecQty, sig.ExecPrice)
		}
	} else if action == "sell" || action == "reduce" || action == "stop" {
		// Reduce existing position
		var existing model.LivePosition
		db.MySQL.Where("strategy_run_id = ? AND stock_code = ?", runID, sig.StockCode).First(&existing)
		if existing.ID > 0 {
			newQty := existing.Quantity - sig.ExecQty
			if newQty <= 0 {
				db.MySQL.Delete(&existing)
				log.Printf("[order_sync] position closed: %s", sig.StockCode)
			} else {
				db.MySQL.Model(&existing).Updates(map[string]interface{}{
					"quantity":      newQty,
					"current_price": sig.ExecPrice,
				})
				log.Printf("[order_sync] position reduced: %s %d→%d", sig.StockCode, existing.Quantity, newQty)
			}
		}
	}
}

// extractOrderID extracts the order ID from a skip_reason string.

// createTradeRecord creates a LiveTrade record when an order is filled.
func (s *OrderSyncService) createTradeRecord(sig *model.BacktestSignal, userID uint, runID uint) {
	// Avoid duplicate: check if a trade for this signal already exists
	var count int64
	sigID := sig.ID
	db.MySQL.Model(&model.LiveTrade{}).
		Where("strategy_run_id = ? AND signal_id = ? AND trade_date = ?", runID, sigID, sig.ExecDate).
		Count(&count)
	if count > 0 {
		log.Printf("[order_sync] trade already exists for signal %d, skip", sig.ID)
		return
	}

	// Resolve execution mode from run
	var run model.StrategyRun
	db.MySQL.Where("id = ?", runID).First(&run)
	var account model.TradingAccount
	if run.AccountID > 0 {
		db.MySQL.Where("id = ?", run.AccountID).First(&account)
	}
	execMode := "mx_moni"
	if account.BrokerMode != "" {
		execMode = account.BrokerMode
	}

	trade := model.LiveTrade{
		UserID:        userID,
		StrategyRunID: runID,
		SignalID:      &sigID,
		StockCode:     sig.StockCode,
		StockName:     sig.StockName,
		ActionType:    sig.ActionType,
		Quantity:      sig.ExecQty,
		Price:         sig.ExecPrice,
		Amount:        sig.ExecPrice * float64(sig.ExecQty),
		Reason:        fmt.Sprintf("妙想成交 orderID=%s", sig.BrokerOrderID),
		ExecutionMode: execMode,
		TradeDate:     sig.ExecDate,
	}
	db.MySQL.Create(&trade)
	log.Printf("[order_sync] trade created: signal=%d code=%s %s %d@%.2f amount=%.2f",
		sig.ID, sig.StockCode, sig.ActionType, sig.ExecQty, sig.ExecPrice, sig.ExecPrice*float64(sig.ExecQty))
}

// SyncOrderForSignal syncs a single signal's order status (used for manual refresh).
func (s *OrderSyncService) SyncOrderForSignal(signalID uint) (string, error) {
	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ?", signalID).First(&sig).Error; err != nil {
		return "", fmt.Errorf("signal not found: %w", err)
	}
	if sig.Status != "pending_order" && sig.Status != "partial_filled" {
		return sig.Status, nil
	}
	// Delegate to the full sync — it will pick up this signal
	_, err := s.SyncAllPendingOrders()
	return sig.Status, err
}
