package service

import (
	"fmt"
	"log"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// OrderSyncService handles periodic synchronization of order status from brokers.
// It scans signals with pending_order / partial_filled status, queries the broker
// for actual order status, and updates signal state + positions accordingly.
type OrderSyncService struct {
	brokerSvc *BrokerService
	liveSvc   *LiveTradingService
}

// NewOrderSyncService creates a new order sync service.
func NewOrderSyncService() *OrderSyncService {
	return &OrderSyncService{
		brokerSvc: NewBrokerService(),
		liveSvc:   NewLiveTradingService(),
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
	Skipped       int      `json:"skipped"`
	Logs          []string `json:"logs"`
}

// SyncAllPendingOrders scans pending_order / partial_filled signals for today & yesterday
// with non-empty broker_order_id, queries broker for actual status, and completes filled orders.
// Called by the scheduler every 30 minutes during trading hours.
func (s *OrderSyncService) SyncAllPendingOrders() (*SyncResult, error) {
	result := &SyncResult{}

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// Find signals that need syncing: today/yesterday, pending_order/partial_filled, has broker_order_id
	var signals []model.BacktestSignal
	db.MySQL.Where("exec_date IN ? AND status IN ? AND broker_order_id != ''",
		[]string{today, yesterday},
		[]string{"pending_order", "partial_filled"}).
		Order("run_id ASC, id ASC").
		Find(&signals)

	result.TotalScanned = len(signals)
	if len(signals) == 0 {
		result.Logs = append(result.Logs, fmt.Sprintf("无需同步的委托订单 (日期: %s ~ %s)", yesterday, today))
		return result, nil
	}

	result.Logs = append(result.Logs, fmt.Sprintf("扫描到 %d 条待同步委托 (日期: %s ~ %s)", len(signals), yesterday, today))

	// Group by run to get account info efficiently
	type runKey struct {
		RunID     uint
		AccountID uint
		UserID    uint
	}
	runAccounts := make(map[uint]*runKey)

	for i := range signals {
		sig := &signals[i]

		// Resolve account for this run (cached)
		rk, ok := runAccounts[sig.RunID]
		if !ok {
			var run model.StrategyRun
			if err := db.MySQL.Where("id = ?", sig.RunID).First(&run).Error; err != nil {
				result.Logs = append(result.Logs, fmt.Sprintf("⚠️ 信号%d: run %d 不存在, 跳过", sig.ID, sig.RunID))
				result.Skipped++
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
			result.Skipped++
			continue
		}

		orderID := sig.BrokerOrderID
		// Query broker for this order
		orders, err := s.brokerSvc.GetBrokerOrders(rk.AccountID, rk.UserID)
		if err != nil {
			result.Logs = append(result.Logs, fmt.Sprintf("❌ 查询账户%d委托失败: %v", rk.AccountID, err))
			result.Skipped++
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
			result.Logs = append(result.Logs, fmt.Sprintf("❓ %s %s orderID=%s 未在委托列表中找到",
				sig.StockCode, sig.StockName, orderID))
			result.Skipped++
			continue
		}

		// Map broker status to our signal status
		newStatus, ok := mxStatusMap[matched.Status]
		if !ok {
			result.Logs = append(result.Logs, fmt.Sprintf("❓ %s %s 未知委托状态%d: orderID=%s",
				sig.StockCode, sig.StockName, matched.Status, orderID))
			result.Skipped++
			continue
		}

		oldStatus := sig.Status

		// If order is fully filled (已成), complete the signal using the same logic as manual completion
		if matched.Status == 4 && newStatus == "executed" {
			execPrice := matched.Price
			if execPrice <= 0 && sig.OrderPrice > 0 {
				execPrice = sig.OrderPrice
			}
			filledQty := matched.FilledQty
			if filledQty <= 0 {
				filledQty = matched.Quantity
			}
			if execPrice <= 0 || filledQty <= 0 {
				result.Logs = append(result.Logs, fmt.Sprintf("⚠️ %s %s 成交但价格/数量无效: price=%.2f qty=%d orderID=%s",
					sig.StockCode, sig.StockName, matched.Price, matched.FilledQty, orderID))
				result.Skipped++
				continue
			}

			log.Printf("[order_sync] ✅ %s %s 已成: price=%.2f qty=%d orderID=%s → completing via executeSignal",
				sig.StockCode, sig.StockName, execPrice, filledQty, orderID)

			// Call the same completion logic as manual "执行" button on the frontend
			// This handles: position, trade record, fund update, holding sync, daily snapshot
			if err := s.liveSvc.ExecuteSignalByIDWithPrice(sig.ID, sig.UserID, execPrice, filledQty); err != nil {
				result.Logs = append(result.Logs, fmt.Sprintf("❌ %s %s 信号完成失败: %v orderID=%s",
					sig.StockCode, sig.StockName, err, orderID))
				result.Failed++
				continue
			}

			result.Executed++
			result.Logs = append(result.Logs, fmt.Sprintf("✅ %s %s 已成: %.2f×%d orderID=%s",
				sig.StockCode, sig.StockName, execPrice, filledQty, orderID))

		} else if newStatus != oldStatus {
			// Status change but not fully filled — just update signal status
			sig.Status = newStatus
			db.MySQL.Save(sig)

			switch newStatus {
			case "partial_filled":
				result.PartialFilled++
				result.Logs = append(result.Logs, fmt.Sprintf("📊 %s %s 部成: %d/%d orderID=%s",
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
		}

		log.Printf("[order_sync] %s %s: %s → %s orderID=%s",
			sig.StockCode, sig.StockName, oldStatus, newStatus, orderID)
	}

	result.Logs = append(result.Logs, fmt.Sprintf("同步完成: 扫描%d 更新%d 已成%d 部成%d 已撤%d 废单%d 跳过%d",
		result.TotalScanned, result.Updated, result.Executed,
		result.PartialFilled, result.Cancelled, result.Failed, result.Skipped))

	return result, nil
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
	_, err := s.SyncAllPendingOrders()
	return sig.Status, err
}
