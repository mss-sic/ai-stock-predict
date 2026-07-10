package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/internal/ws"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// AgentHandler provides REST API endpoints for local auto-trading agents.
type AgentHandler struct {
	testMgr   *ws.TestManager
	commander *ws.Commander
}

// NewAgentHandler creates a new agent handler.
func NewAgentHandler(testMgr *ws.TestManager, commander *ws.Commander) *AgentHandler {
	return &AgentHandler{testMgr: testMgr, commander: commander}
}

// ── Auth ──

func authAgent(c *gin.Context) (*model.TradingAccount, error) {
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("X-Agent-Token")
	}
	if token == "" {
		return nil, nil
	}
	var account model.TradingAccount
	if err := db.MySQL.Where("agent_token = ?", token).
		First(&account).Error; err != nil {
		return nil, nil
	}
	if !model.IsAgentMode(account.BrokerMode) {
		return nil, nil
	}
	return &account, nil
}

// ── Account Info ──

// GetAccount returns full account information for the connected agent.
// GET /api/v1/agent/account?token=xxx
func (h *AgentHandler) GetAccount(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	type AccountInfo struct {
		ID               uint    `json:"id"`
		Name             string  `json:"name"`
		Broker           string  `json:"broker"`
		AccountType      string  `json:"accountType"`
		AccountNumber    string  `json:"accountNumber"`
		Status           string  `json:"status"`
		BrokerMode       string  `json:"brokerMode"`
		InitialCapital   float64 `json:"initialCapital"`
		AvailableCash    float64 `json:"availableCash"`
		FrozenCash       float64 `json:"frozenCash"`
		TotalAssets      float64 `json:"totalAssets"`
		TotalMarketValue float64 `json:"totalMarketValue"`
		TotalProfit      float64 `json:"totalProfit"`
		Nav              float64 `json:"nav"`
	}

	response.Success(c, AccountInfo{
		ID:               account.ID,
		Name:             account.Name,
		Broker:           account.Broker,
		AccountType:      account.AccountType,
		AccountNumber:    account.AccountNumber,
		Status:           account.Status,
		BrokerMode:       account.BrokerMode,
		InitialCapital:   account.InitialCapital,
		AvailableCash:    account.AvailableCash,
		FrozenCash:       account.FrozenCash,
		TotalAssets:      account.TotalAssets,
		TotalMarketValue: account.TotalMarketValue,
		TotalProfit:      account.TotalProfit,
		Nav:              account.Nav,
	})
}

// ── Command Response ──

// PostCommandResponse receives the agent's response to a dispatched command.
// POST /api/v1/agent/commands/:requestId/response
func (h *AgentHandler) PostCommandResponse(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	requestID := c.Param("requestId")
	if requestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing requestId"})
		return
	}

	var body ws.CommandResponse
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	body.RequestID = requestID

	if h.commander != nil {
		ok := h.commander.ReceiveResponse(requestID, body)
		if !ok {
			c.JSON(http.StatusGone, gin.H{"error": "request expired or not found"})
			return
		}
	}

	log.Printf("[agent] command response account=%d requestID=%s status=%s", account.ID, requestID, body.Status)
	response.Success(c, gin.H{"received": true})
}

// ── Position Sync ──

// SyncPositions receives position data from the agent and updates the database.
// POST /api/v1/agent/positions/sync
func (h *AgentHandler) SyncPositions(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var body struct {
		Positions []AgentPosition `json:"positions"`
		Balance   *AgentBalance   `json:"balance,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Update account balance if provided
	if body.Balance != nil {
		db.MySQL.Model(account).Updates(map[string]interface{}{
			"available_cash":     body.Balance.AvailableCash,
			"frozen_cash":        body.Balance.FrozenCash,
			"total_assets":       body.Balance.TotalAssets,
			"total_market_value": body.Balance.TotalMarketValue,
			"total_profit":       body.Balance.TotalProfit,
			"nav":                body.Balance.Nav,
		})
	}

	// Upsert positions
	liveSvc := service.NewLiveTradingService()
	synced := 0
	for _, pos := range body.Positions {
		if pos.StockCode == "" {
			continue
		}
		liveSvc.UpsertPosition(account.UserID, account.ID, &model.LivePosition{
			StockCode:    pos.StockCode,
			StockName:    pos.StockName,
			Quantity:     pos.Quantity,
			AvailSellQty: pos.AvailQty,
			AvgCost:      pos.AvgCost,
			CurrentPrice: pos.CurrentPrice,
		})
		synced++
	}

	log.Printf("[agent] positions sync account=%d: %d positions, balance=%v", account.ID, synced, body.Balance != nil)
	response.Success(c, gin.H{"synced": synced})
}

// ── Order Sync ──

// SyncOrders receives order/entrust data from the agent.
// POST /api/v1/agent/orders/sync
func (h *AgentHandler) SyncOrders(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var body struct {
		Orders []AgentOrder `json:"orders"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Update signal statuses based on order state
	updated := 0
	for _, order := range body.Orders {
		if order.BrokerOrderID == "" {
			continue
		}
		var sig model.BacktestSignal
		if err := db.MySQL.Where("broker_order_id = ?", order.BrokerOrderID).First(&sig).Error; err != nil {
			continue
		}
		newStatus := mapOrderStatus(order.Status)
		if newStatus != "" && newStatus != sig.Status {
			db.MySQL.Model(&sig).Updates(map[string]interface{}{
				"status":      newStatus,
				"exec_price":  order.ExecPrice,
				"exec_qty":    order.ExecQty,
				"skip_reason": order.StatusText,
				"updated_at":  time.Now(),
			})
			// Record trade if executed/partial
			if newStatus == "executed" || newStatus == "partial_filled" {
				svc := service.NewLiveTradingService()
				svc.RecordTradeFromSignal(&sig, order.ExecPrice, order.ExecQty)
			}
			updated++
		}
	}

	log.Printf("[agent] orders sync account=%d: %d orders, %d updated", account.ID, len(body.Orders), updated)
	response.Success(c, gin.H{"synced": len(body.Orders), "updated": updated})
}

// mapOrderStatus converts agent order status to signal status.
func mapOrderStatus(s string) string {
	switch s {
	case "pending", "submitted", "accepted":
		return "pending_order"
	case "partial_filled", "partially_filled":
		return "partial_filled"
	case "filled", "executed", "done":
		return "executed"
	case "cancelled", "canceled", "rejected":
		return "cancelled"
	case "failed", "error":
		return "order_failed"
	default:
		return ""
	}
}

// ── Data Types for Agent API ──

type AgentPosition struct {
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	Quantity    int     `json:"quantity"`
	AvailQty    int     `json:"availQty"`
	AvgCost     float64 `json:"avgCost"`
	CurrentPrice float64 `json:"currentPrice"`
}

type AgentBalance struct {
	AvailableCash    float64 `json:"availableCash"`
	FrozenCash       float64 `json:"frozenCash"`
	TotalAssets      float64 `json:"totalAssets"`
	TotalMarketValue float64 `json:"totalMarketValue"`
	TotalProfit      float64 `json:"totalProfit"`
	Nav              float64 `json:"nav"`
}

type AgentOrder struct {
	BrokerOrderID string  `json:"brokerOrderId"`
	StockCode     string  `json:"stockCode"`
	StockName     string  `json:"stockName"`
	OrderType     string  `json:"orderType"`
	Price         float64 `json:"price"`
	Quantity      int     `json:"quantity"`
	ExecPrice     float64 `json:"execPrice"`
	ExecQty       int     `json:"execQty"`
	Status        string  `json:"status"`
	StatusText    string  `json:"statusText"`
}

// ── Existing: Signal Management (unchanged below, but we include them) ──

// GetPendingAutoSignals returns signals with pending_auto status for the agent's account.
func (h *AgentHandler) GetPendingAutoSignals(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var runIDs []uint
	db.MySQL.Model(&model.StrategyRun{}).
		Where("account_id = ? AND status IN ?", account.ID, []string{"active", "paused"}).
		Pluck("id", &runIDs)

	if len(runIDs) == 0 {
		response.Success(c, []interface{}{})
		return
	}

	today := time.Now().Format("2006-01-02")
	var signals []model.BacktestSignal
	db.MySQL.Where("run_id IN ? AND exec_date = ? AND status IN ?",
		runIDs, today, []string{"pending_auto", "claimed"}).
		Order("id ASC").
		Find(&signals)

	type SignalSummary struct {
		SignalID     uint    `json:"signalId"`
		RunID        uint    `json:"runId"`
		StockCode    string  `json:"stockCode"`
		StockName    string  `json:"stockName"`
		ActionType   string  `json:"actionType"`
		OrderPrice   float64 `json:"orderPrice"`
		PlannedQty   int     `json:"plannedQty"`
		PlannedAmount float64 `json:"plannedAmount"`
		ExecDate     string  `json:"execDate"`
		Status       string  `json:"status"`
		Reason       string  `json:"reason"`
		CreatedAt    string  `json:"createdAt"`
	}

	result := make([]SignalSummary, 0, len(signals))
	for _, s := range signals {
		result = append(result, SignalSummary{
			SignalID:     s.ID,
			RunID:        s.RunID,
			StockCode:    s.StockCode,
			StockName:    s.StockName,
			ActionType:   s.ActionType,
			OrderPrice:   s.OrderPrice,
			PlannedQty:   s.PlannedQty,
			PlannedAmount: s.PlannedAmount,
			ExecDate:     s.ExecDate,
			Status:       s.Status,
			Reason:       s.Reason,
			CreatedAt:    s.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	response.Success(c, result)
}

// ClaimSignal claims a signal for execution, preventing duplicate processing.
func (h *AgentHandler) ClaimSignal(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	sid, _ := strconv.Atoi(c.Param("id"))
	if sid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signal id"})
		return
	}

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ?", sid).First(&sig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "signal not found"})
		return
	}

	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND account_id = ?", sig.RunID, account.ID).First(&run).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "signal does not belong to this account"})
		return
	}

	if sig.Status != "pending_auto" {
		c.JSON(http.StatusConflict, gin.H{"error": "signal status is not pending_auto", "currentStatus": sig.Status})
		return
	}

	result := db.MySQL.Model(&sig).Where("id = ? AND status = ?", sid, "pending_auto").
		Updates(map[string]interface{}{
			"status":      "claimed",
			"skip_reason": "agent claimed",
			"updated_at":  time.Now(),
		})

	if result.RowsAffected == 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "signal already claimed by another agent"})
		return
	}

	log.Printf("[agent] signal %d claimed by account %d (%s %s %s)",
		sig.ID, account.ID, sig.StockCode, sig.StockName, sig.ActionType)

	response.Success(c, map[string]interface{}{
		"claimed":   true,
		"signalId":  sig.ID,
		"stockCode": sig.StockCode,
		"stockName": sig.StockName,
		"action":    sig.ActionType,
		"price":     sig.OrderPrice,
		"quantity":  sig.PlannedQty,
		"amount":    sig.PlannedAmount,
	})
}

// ReportResultRequest is the payload for reporting execution results.
type ReportResultRequest struct {
	Status    string  `json:"status"`    // "executed" / "order_failed"
	OrderID   string  `json:"orderId"`   // broker-assigned order id
	ErrorMsg  string  `json:"errorMsg"`  // error message if failed
	ExecPrice float64 `json:"execPrice"` // actual execution price
	ExecQty   int     `json:"execQty"`   // actual executed quantity
}

// ReportResult records the execution result for a claimed signal.
func (h *AgentHandler) ReportResult(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	sid, _ := strconv.Atoi(c.Param("id"))
	if sid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signal id"})
		return
	}

	var body ReportResultRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if body.Status != "executed" && body.Status != "order_failed" && body.Status != "submitted" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be 'executed', 'order_failed' or 'submitted'"})
		return
	}

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ?", sid).First(&sig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "signal not found"})
		return
	}

	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND account_id = ?", sig.RunID, account.ID).First(&run).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "signal does not belong to this account"})
		return
	}

	if sig.Status != "claimed" {
		c.JSON(http.StatusConflict, gin.H{"error": "signal is not in claimed state", "currentStatus": sig.Status})
		return
	}

	if body.Status == "executed" && (body.ExecPrice <= 0 || body.ExecQty <= 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "executed status requires execPrice > 0 and execQty > 0"})
		return
	}
	if body.Status == "submitted" && (body.ExecPrice <= 0 || body.ExecQty <= 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "submitted status requires execPrice > 0 and execQty > 0"})
		return
	}

	log.Printf("[agent] signal %d result: %s, order=%s, price=%.2f, qty=%d, error=%s",
		sig.ID, body.Status, body.OrderID, body.ExecPrice, body.ExecQty, body.ErrorMsg)

	// submitted: 委托已提交但尚未成交，信号流转为 pending_order（委托中）。
	// 不调用 FinalizeSignalExecution（不更新持仓/资金），后续由 OrderSync 或
	// agent 轮询成交状态后再上报 executed 完成结算。
	if body.Status == "submitted" {
		updates := map[string]interface{}{
			"status":           "pending_order",
			"exec_price":       body.ExecPrice,
			"exec_qty":         body.ExecQty,
			"skip_reason":      "委托已提交，等待成交",
			"updated_at":       time.Now(),
		}
		if body.OrderID != "" {
			updates["broker_order_id"] = body.OrderID
		}
		db.MySQL.Model(&sig).Updates(updates)
		log.Printf("[agent] signal %d submitted, pending_order: %s x %d @ %.2f order=%s",
			sig.ID, sig.StockCode, int(body.ExecQty), body.ExecPrice, body.OrderID)
	}

	// order_failed: agent 内部已按 max_retries 重试仍失败，此处置为终态 order_failed。
	// 不再重置为 pending_auto，避免信号被反复下发导致无限重试/重复下单。
	if body.Status == "order_failed" {
		updates := map[string]interface{}{
			"status":       "order_failed",
			"skip_reason":  body.ErrorMsg,
			"exec_price":   0,
			"exec_qty":     0,
			"updated_at":   time.Now(),
		}
		if body.OrderID != "" {
			updates["broker_order_id"] = body.OrderID
		}
		db.MySQL.Model(&sig).Updates(updates)
		log.Printf("[agent] signal %d order_failed (terminal, no retry): %s", sig.ID, body.ErrorMsg)
	}

	// executed: delegate entirely to FinalizeSignalExecution for complete balance update.
	// This single call handles: signal status→executed, exec_price/qty, LiveTrade record,
	// run.AvailableCash, run.PositionValue, live_positions, holdings sync.
	// We skip the initial DB write to avoid double-updating the signal.
	if body.Status == "executed" && body.ExecPrice > 0 && body.ExecQty > 0 {
		liveSvc := service.NewLiveTradingService()
		if err := liveSvc.FinalizeSignalExecution(sig.RunID, sig.ID, body.ExecPrice, body.ExecQty); err != nil {
			log.Printf("[agent] FinalizeSignalExecution failed for signal %d: %v, resetting to order_failed", sig.ID, err)
			db.MySQL.Model(&sig).Updates(map[string]interface{}{
				"status":      "order_failed",
				"skip_reason": "balance update failed: " + err.Error(),
				"exec_price":  0,
				"exec_qty":    0,
				"updated_at":  time.Now(),
			})
			response.InternalError(c, "资金更新失败: "+err.Error())
			return
		}
		// Update broker_order_id after FinalizeSignalExecution completes
		if body.OrderID != "" {
			db.MySQL.Model(&sig).Update("broker_order_id", body.OrderID)
		}
		log.Printf("[agent] signal %d finalized: run=%d availableCash updated, positions synced", sig.ID, sig.RunID)
	}

	response.Success(c, map[string]interface{}{
		"reported": true,
		"signalId": sig.ID,
		"status":   body.Status,
	})
}

// GetAccountSummary returns account balance and active positions for the agent.
func (h *AgentHandler) GetAccountSummary(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	today := time.Now().Format("2006-01-02")
	var pendingCount int64
	var runIDs []uint
	db.MySQL.Model(&model.StrategyRun{}).
		Where("account_id = ? AND status IN ?", account.ID, []string{"active", "paused"}).
		Pluck("id", &runIDs)
	if len(runIDs) > 0 {
		db.MySQL.Model(&model.BacktestSignal{}).
			Where("run_id IN ? AND exec_date = ? AND status IN ?",
				runIDs, today, []string{"pending_auto", "claimed"}).
			Count(&pendingCount)
	}

	var positions []model.LivePosition
	if len(runIDs) > 0 {
		db.MySQL.Where("strategy_run_id IN ?", runIDs).
			Order("stock_code ASC").Find(&positions)
	}

	type PositionSummary struct {
		StockCode string  `json:"stockCode"`
		StockName string  `json:"stockName"`
		Quantity  int     `json:"quantity"`
		AvgCost   float64 `json:"avgCost"`
		CurPrice  float64 `json:"currentPrice"`
		Pnl       float64 `json:"pnl"`
		PnlPct    float64 `json:"pnlPct"`
	}

	posList := make([]PositionSummary, 0, len(positions))
	for _, p := range positions {
		posList = append(posList, PositionSummary{
			StockCode: p.StockCode,
			StockName: p.StockName,
			Quantity:  p.Quantity,
			AvgCost:   p.AvgCost,
			CurPrice:  p.CurrentPrice,
			Pnl:       p.UnrealizedPnl,
			PnlPct:    p.UnrealizedPnlPct,
		})
	}

	response.Success(c, map[string]interface{}{
		"accountId":     account.ID,
		"accountName":   account.Name,
		"totalAssets":   account.TotalAssets,
		"availBalance":  account.AvailableCash,
		"totalProfit":   account.TotalProfit,
		"pendingCount":  pendingCount,
		"positionCount": len(positions),
		"positions":     posList,
	})
}

// GetSignalDetail returns full details for a single signal.
func (h *AgentHandler) GetSignalDetail(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	sid, _ := strconv.Atoi(c.Param("id"))
	if sid <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signal id"})
		return
	}

	var sig model.BacktestSignal
	if err := db.MySQL.Where("id = ?", sid).First(&sig).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "signal not found"})
		return
	}

	var run model.StrategyRun
	if err := db.MySQL.Where("id = ? AND account_id = ?", sig.RunID, account.ID).First(&run).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "signal does not belong to this account"})
		return
	}

	detail, _ := json.Marshal(sig)
	var result map[string]interface{}
	json.Unmarshal(detail, &result)
	result["runName"] = run.Name

	response.Success(c, result)
}

// ── Agent Connectivity Test ──

func (h *AgentHandler) TestAgent(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}
	if h.testMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "test manager not initialized"})
		return
	}

	err = h.testMgr.SendTest(account.ID, 30)
	if err != nil {
		response.Success(c, map[string]interface{}{
			"passed":  false,
			"message": err.Error(),
		})
		return
	}

	response.Success(c, map[string]interface{}{
		"passed":  true,
		"message": "agent 连接正常，可以启用自动交易",
	})
}

func (h *AgentHandler) AgentTestResponse(c *gin.Context) {
	var body struct {
		RequestID string `json:"requestId"`
		Success   bool   `json:"success"`
		Message   string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.RequestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing requestId"})
		return
	}
	if h.testMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "test manager not initialized"})
		return
	}

	ok := h.testMgr.ReceiveResponse(body.RequestID, body.Success, body.Message)
	if !ok {
		c.JSON(http.StatusGone, gin.H{"error": "test request expired or not found"})
		return
	}

	response.Success(c, gin.H{"status": "received"})
}

func (h *AgentHandler) CheckAgentStatus(c *gin.Context) {
	account, err := authAgent(c)
	if err != nil || account == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	connected := h.testMgr != nil

	response.Success(c, map[string]interface{}{
		"connected": connected,
		"accountId": account.ID,
		"message":   "agent status retrieved",
	})
}
