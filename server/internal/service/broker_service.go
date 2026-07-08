package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/db"
)

// Broker interface defines operations a broker execution channel must support.
type Broker interface {
	SyncPositions(account *model.TradingAccount) (*BrokerPortfolio, error)
	GetBalance(account *model.TradingAccount) (*BrokerBalance, error)
	PlaceOrder(account *model.TradingAccount, req *BrokerOrderRequest) (*BrokerOrderResult, error)
	CancelOrder(account *model.TradingAccount, orderID string, stockCode string) error
	QueryOrders(account *model.TradingAccount) ([]BrokerOrder, error)
}

// BrokerPortfolio represents a broker's portfolio snapshot.
type BrokerPortfolio struct {
	TotalAssets  float64          `json:"totalAssets"`
	AvailBalance float64          `json:"availBalance"`
	TotalPosVal  float64          `json:"totalPosValue"`
	PosCount     int              `json:"posCount"`
	TotalProfit  float64          `json:"totalProfit"`
	Positions    []BrokerPosition `json:"positions"`
}

// BrokerPosition represents a single position from broker.
type BrokerPosition struct {
	SecCode      string  `json:"secCode"`
	SecName      string  `json:"secName"`
	Count        int     `json:"count"`
	AvailCount   int     `json:"availCount"`
	Value        float64 `json:"value"`
	CostPrice    float64 `json:"costPrice"`
	Price        float64 `json:"price"`
	DayProfit    float64 `json:"dayProfit"`
	DayProfitPct float64 `json:"dayProfitPct"`
	Profit       float64 `json:"profit"`
	ProfitPct    float64 `json:"profitPct"`
	PosPct       float64 `json:"posPct"`
}

// BrokerBalance represents broker account balance.
type BrokerBalance struct {
	TotalAssets  float64 `json:"totalAssets"`
	AvailBalance float64 `json:"availBalance"`
	FrozenFunds  float64 `json:"frozenFunds"`
	TotalProfit  float64 `json:"totalProfit"`
}

// BrokerOrderRequest is the request to place an order.
type BrokerOrderRequest struct {
	StockCode      string  `json:"stockCode"`
	OrderType      string  `json:"type"`
	Price          float64 `json:"price"`
	Quantity       int     `json:"quantity"`
	UseMarketPrice bool    `json:"useMarketPrice"`
}

// BrokerOrderResult is the result of placing an order.
type BrokerOrderResult struct {
	OrderID string `json:"orderId"`
	Status  string `json:"status"`
}

// BrokerOrder represents a single order from broker.
type BrokerOrder struct {
	OrderID    string  `json:"orderId"`
	StockCode  string  `json:"stockCode"`
	StockName  string  `json:"stockName"`
	OrderType  string  `json:"orderType"`
	Price      float64 `json:"price"`
	TradePrice float64 `json:"tradePrice"`
	Quantity   int     `json:"quantity"`
	FilledQty  int     `json:"filledQty"`
	Status     int     `json:"status"`
	CreateTime string  `json:"createTime"`
}

// ── MxMoniBroker ──

type MxMoniBroker struct {
	apiURL string
	client *http.Client
}

func NewMxMoniBroker() *MxMoniBroker {
	apiURL := os.Getenv("MX_API_URL")
	if apiURL == "" {
		apiURL = "https://mkapi2.dfcfs.com/finskillshub"
	}
	return &MxMoniBroker{
		apiURL: apiURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *MxMoniBroker) apiKey(account *model.TradingAccount) string {
	if account.MxAPIKey != "" {
		return account.MxAPIKey
	}
	return os.Getenv("MX_APIKEY")
}

// ── API request helpers ──

type mxAPIResponse struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// codeOK checks if the response code indicates success (string "200" or number 200).
func (r *mxAPIResponse) codeOK() bool {
	var s string
	if err := json.Unmarshal(r.Code, &s); err == nil {
		return s == "200" || s == "0"
	}
	var n int
	if err := json.Unmarshal(r.Code, &n); err == nil {
		return n == 200 || n == 0
	}
	return false
}

func (b *MxMoniBroker) doRequest(account *model.TradingAccount, endpoint string, payload interface{}) (*mxAPIResponse, error) {
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	url := b.apiURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("apikey", b.apiKey(account))

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp mxAPIResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (raw: %s)", err, string(respBytes[:minInt(len(respBytes), 200)]))
	}

	if !apiResp.codeOK() {
		return &apiResp, fmt.Errorf("mx-moni API error: code=%s, msg=%s", string(apiResp.Code), apiResp.Message)
	}

	return &apiResp, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Position Sync ──

type mxPositionsData struct {
	TotalAssets  float64 `json:"totalAssets"`
	AvailBalance float64 `json:"availBalance"`
	TotalPosVal  float64 `json:"totalPosValue"`
	PosCount     int     `json:"posCount"`
	TotalProfit  float64 `json:"totalProfit"`
	PosList      []struct {
		SecCode      string  `json:"secCode"`
		SecMkt       int     `json:"secMkt"`
		SecName      string  `json:"secName"`
		Count        int     `json:"count"`
		AvailCount   int     `json:"availCount"`
		Value        float64 `json:"value"`
		CostPrice    float64 `json:"costPrice"`
		CostPriceDec int     `json:"costPriceDec"`
		Price        float64 `json:"price"`
		PriceDec     int     `json:"priceDec"`
		DayProfit    float64 `json:"dayProfit"`
		DayProfitPct float64 `json:"dayProfitPct"`
		Profit       float64 `json:"profit"`
		ProfitPct    float64 `json:"profitPct"`
		PosPct       float64 `json:"posPct"`
	} `json:"posList"`
}

func (b *MxMoniBroker) SyncPositions(account *model.TradingAccount) (*BrokerPortfolio, error) {
	apiResp, err := b.doRequest(account, "/api/claw/mockTrading/positions", map[string]int{"moneyUnit": 1})
	if err != nil {
		return nil, err
	}

	var raw mxPositionsData
	if err := json.Unmarshal(apiResp.Data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal positions data: %w", err)
	}

	portfolio := &BrokerPortfolio{
		TotalAssets:  raw.TotalAssets,
		AvailBalance: raw.AvailBalance,
		TotalPosVal:  raw.TotalPosVal,
		PosCount:     raw.PosCount,
		TotalProfit:  raw.TotalProfit,
	}

	for _, p := range raw.PosList {
		var costPrice, price float64
		if p.CostPriceDec > 0 {
			div := 1.0
			for i := 0; i < p.CostPriceDec; i++ {
				div *= 10
			}
			costPrice = p.CostPrice / div
		} else {
			costPrice = p.CostPrice
		}
		if p.PriceDec > 0 {
			div := 1.0
			for i := 0; i < p.PriceDec; i++ {
				div *= 10
			}
			price = p.Price / div
		} else {
			price = p.Price
		}

		pos := BrokerPosition{
			SecCode:      p.SecCode,
			SecName:      p.SecName,
			Count:        p.Count,
			AvailCount:   p.AvailCount,
			Value:        p.Value,
			CostPrice:    costPrice,
			Price:        price,
			DayProfit:    p.DayProfit,
			DayProfitPct: p.DayProfitPct,
			Profit:       p.Profit,
			ProfitPct:    p.ProfitPct,
			PosPct:       p.PosPct,
		}
		portfolio.Positions = append(portfolio.Positions, pos)
	}

	account.AvailableCash = raw.AvailBalance
	now := time.Now()
	db.MySQL.Model(account).Updates(map[string]interface{}{
		"available_cash":      raw.AvailBalance,
		"total_assets":        raw.TotalAssets,
		"total_market_value":  raw.TotalPosVal,
		"total_profit":        raw.TotalProfit,
		"updated_at":          now,
	})

	today := time.Now().Format("2006-01-02")
	for _, pos := range portfolio.Positions {
		b.syncLocalHolding(account, pos.SecCode, pos.SecName, pos.Count, pos.CostPrice, pos.AvailCount, pos.Price, today)
	}

	log.Printf("[broker] Synced %d positions for account %d, total=%.2f avail=%.2f",
		portfolio.PosCount, account.ID, portfolio.TotalAssets, portfolio.AvailBalance)

	return portfolio, nil
}

func (b *MxMoniBroker) syncLocalHolding(account *model.TradingAccount, stockCode, stockName string, quantity int, costPrice float64, availCount int, currentPrice float64, tradeDate string) {
	var holding model.Holding
	err := db.MySQL.Where("user_id = ? AND account_id = ? AND stock_code = ?",
		account.UserID, account.ID, stockCode).First(&holding).Error

	if quantity <= 0 {
		if err == nil {
			db.MySQL.Delete(&holding)
		}
		return
	}

	totalCost := costPrice * float64(quantity)
	if err == nil {
		todayBuyQty := quantity - availCount
		if todayBuyQty < 0 { todayBuyQty = 0 }
		availQty := availCount
		db.MySQL.Model(&holding).Updates(map[string]interface{}{
			"quantity":       quantity,
			"cost_price":     costPrice,
			"total_cost":     totalCost,
			"buy_date":       tradeDate,
			"today_buy_qty":  todayBuyQty,
			"avail_sell_qty": availQty,
			"stock_name":     stockName,
			"current_price":  currentPrice,
		})
	} else {
		todayBuyQty := quantity - availCount
		if todayBuyQty < 0 { todayBuyQty = 0 }
		newH := model.Holding{
			UserID:       account.UserID,
			AccountID:    account.ID,
			StockCode:    stockCode,
			StockName:    stockName,
			CostPrice:    costPrice,
			Quantity:     quantity,
			TodayBuyQty:  todayBuyQty,
			AvailSellQty: availCount,
			CurrentPrice: currentPrice,
			TotalCost:    totalCost,
			BuyDate:      tradeDate,
		}
		db.MySQL.Create(&newH)
	}
}

// ── Balance ──

// mxBalanceData kept for backward compat; superseded by mxBalanceFullData
type mxBalanceData = mxBalanceFullData

// mxBalanceFullData captures the full balance+account response from mx-moni.
type mxBalanceFullData struct {
	TotalAssets   float64 `json:"totalAssets"`
	AvailBalance  float64 `json:"availBalance"`
	BalanceActual float64 `json:"balanceActual"`
	FrozenMoney   float64 `json:"frozenMoney"`
	TotalPosValue float64 `json:"totalPosValue"`
	InitMoney     float64 `json:"initMoney"`
	Nav           float64 `json:"nav"`
	AccID         string  `json:"accID"`
	AccName       string  `json:"accName"`
}

func (b *MxMoniBroker) GetBalance(account *model.TradingAccount) (*BrokerBalance, error) {
	apiResp, err := b.doRequest(account, "/api/claw/mockTrading/balance", map[string]int{"moneyUnit": 1})
	if err != nil {
		return nil, err
	}

	var raw mxBalanceFullData
	if err := json.Unmarshal(apiResp.Data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal balance data: %w", err)
	}

	// Backfill account fields from broker
	now := time.Now()
	if raw.InitMoney > 0 {
		account.InitialCapital = raw.InitMoney
	}
	if raw.AccID != "" {
		account.MxAccountID = raw.AccID
	}
	account.AvailableCash = raw.AvailBalance
	account.TotalAssets = raw.TotalAssets
	account.TotalMarketValue = raw.TotalPosValue
	account.FrozenCash = raw.FrozenMoney
	account.Nav = raw.Nav
	db.MySQL.Model(account).Select("*").Updates(map[string]interface{}{
		"available_cash":     raw.AvailBalance,
		"total_assets":       raw.TotalAssets,
		"total_market_value": raw.TotalPosValue,
		"frozen_cash":        raw.FrozenMoney,
		"nav":                raw.Nav,
		"initial_capital":    account.InitialCapital,
		"mx_account_id":      account.MxAccountID,
		"updated_at":         now,
	})

	return &BrokerBalance{
		TotalAssets:  raw.TotalAssets,
		AvailBalance: raw.AvailBalance,
		FrozenFunds:  raw.FrozenMoney,
		TotalProfit:  0, // totalProfit is synced via SyncPositions
	}, nil
}

// ── Place Order ──

func (b *MxMoniBroker) PlaceOrder(account *model.TradingAccount, req *BrokerOrderRequest) (*BrokerOrderResult, error) {
	lot := BoardLotSize(req.StockCode)
	if req.Quantity%lot != 0 {
		return nil, fmt.Errorf("委托数量必须是%d的整数倍，当前%d", lot, req.Quantity)
	}

	// Format price to correct decimal places
	req.Price = FormatStockPrice(req.StockCode, req.Price)

	apiResp, err := b.doRequest(account, "/api/claw/mockTrading/trade", req)
	if err != nil {
		return nil, err
	}

	type tradeData struct {
		OrderID string `json:"orderId"`
		Status  string `json:"status"`
	}
	var data tradeData
	if err := json.Unmarshal(apiResp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal trade result: %w", err)
	}

	log.Printf("[broker] Order placed: %s %s %d shares, orderID=%s status=%s",
		req.OrderType, req.StockCode, req.Quantity, data.OrderID, data.Status)

	return &BrokerOrderResult{OrderID: data.OrderID, Status: data.Status}, nil
}

// ── Cancel Order ──

func (b *MxMoniBroker) CancelOrder(account *model.TradingAccount, orderID string, stockCode string) error {
	var payload map[string]interface{}
	if orderID == "all" || orderID == "" {
		payload = map[string]interface{}{"type": "all"}
	} else {
		payload = map[string]interface{}{
			"type":      "order",
			"orderId":   orderID,
			"stockCode": stockCode,
		}
	}

	_, err := b.doRequest(account, "/api/claw/mockTrading/cancel", payload)
	if err != nil {
		return err
	}

	log.Printf("[broker] Order cancelled: %s", orderID)
	return nil
}

// ── Query Orders ──

type mxOrdersData struct {
	TotalNum int `json:"totalNum"`
	Orders   []struct {
		OrderID    string `json:"id"`
		StockCode  string `json:"secCode"`
		StockName  string `json:"secName"`
		OrderType  int    `json:"type"`
		Price      int    `json:"price"`
		PriceDec   int    `json:"priceDec"`
		TradePrice int    `json:"tradePrice"`
		Quantity   int    `json:"count"`
		FilledQty  int    `json:"tradeCount"`
		Status     int    `json:"status"`
		CreateTime int64  `json:"time"`
		Drt        int    `json:"drt"` // 1=buy 2=sell
	} `json:"orders"`
}

func (b *MxMoniBroker) QueryOrders(account *model.TradingAccount) ([]BrokerOrder, error) {
	apiResp, err := b.doRequest(account, "/api/claw/mockTrading/orders", map[string]int{
		"fltOrderDrt":    0,
		"fltOrderStatus": 0,
	})
	if err != nil {
		return nil, err
	}

	var raw mxOrdersData
	if err := json.Unmarshal(apiResp.Data, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal orders data: %w", err)
	}

	var orders []BrokerOrder
	for _, o := range raw.Orders {
		price := float64(o.Price)
		if o.PriceDec > 0 {
			divisor := 1.0
			for i := 0; i < o.PriceDec; i++ {
				divisor *= 10
			}
			price = price / divisor
		}
		tradePrice := float64(o.TradePrice)
		if o.PriceDec > 0 {
			divisor := 1.0
			for i := 0; i < o.PriceDec; i++ {
				divisor *= 10
			}
			tradePrice = tradePrice / divisor
		}
		orderType := "buy"
		if o.Drt == 2 {
			orderType = "sell"
		}
		createTime := time.Unix(o.CreateTime, 0).Format("2006-01-02 15:04:05")
		orders = append(orders, BrokerOrder{
			OrderID:    o.OrderID,
			StockCode:  o.StockCode,
			StockName:  o.StockName,
			OrderType:  orderType,
			Price:      price,
			TradePrice: tradePrice,
			Quantity:   o.Quantity,
			FilledQty:  o.FilledQty,
			Status:     o.Status,
			CreateTime: createTime,
		})
	}

	return orders, nil
}

// ── Broker Service (facade) ──

type BrokerService struct{}

func NewBrokerService() *BrokerService {
	return &BrokerService{}
}

// getBroker returns a broker if credentials are available.
// This is used for sync/query; does NOT require mx_moni broker_mode.
func (s *BrokerService) getBroker(account *model.TradingAccount) (Broker, error) {
	b := NewMxMoniBroker()
	if b.apiKey(account) == "" {
		return nil, fmt.Errorf("未配置妙想 API Key，请在环境变量或账户设置中绑定")
	}
	return b, nil
}

// getExecutionBroker returns a broker for automatic trade execution.
// Only returns a broker when broker_mode == "mx_moni".
func (s *BrokerService) getExecutionBroker(account *model.TradingAccount) (Broker, error) {
	switch account.BrokerMode {
	case "mx_moni":
		b := NewMxMoniBroker()
		if b.apiKey(account) == "" {
			return nil, fmt.Errorf("未配置妙想 API Key")
		}
		return b, nil
	case "lobster":
		return NewLobsterBroker(), nil
	default:
		return nil, fmt.Errorf("账户为手动执行模式")
	}
}

func (s *BrokerService) SyncPositionsFromBroker(accountID uint, userID uint) (*BrokerPortfolio, error) {
	var account model.TradingAccount
	if err := db.MySQL.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("账户不存在: %w", err)
	}

	broker, err := s.getBroker(&account)
	if err != nil {
		return nil, err
	}

	// Sync balance first (captures initMoney, nav, frozenFunds)
	if _, err := broker.GetBalance(&account); err != nil {
		log.Printf("[broker] GetBalance failed during sync: %v", err)
	}

	// Then sync positions
	return broker.SyncPositions(&account)
}

func (s *BrokerService) PlaceBrokerOrder(accountID uint, userID uint, req *BrokerOrderRequest) (*BrokerOrderResult, error) {
	var account model.TradingAccount
	if err := db.MySQL.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("账户不存在: %w", err)
	}

	broker, err := s.getExecutionBroker(&account)
	if err != nil {
		return nil, err
	}

	return broker.PlaceOrder(&account, req)
}

func (s *BrokerService) GetBrokerBalance(accountID uint, userID uint) (*BrokerBalance, error) {
	var account model.TradingAccount
	if err := db.MySQL.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("账户不存在: %w", err)
	}

	broker, err := s.getBroker(&account)
	if err != nil {
		return nil, err
	}

	return broker.GetBalance(&account)
}

func (s *BrokerService) GetBrokerOrders(accountID uint, userID uint) ([]BrokerOrder, error) {
	var account model.TradingAccount
	if err := db.MySQL.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, fmt.Errorf("账户不存在: %w", err)
	}

	broker, err := s.getBroker(&account)
	if err != nil {
		return nil, err
	}

	return broker.QueryOrders(&account)
}

// CancelBrokerOrder cancels an order through the account's configured broker.
func (s *BrokerService) CancelBrokerOrder(accountID uint, userID uint, orderID string, stockCode string) error {
	var account model.TradingAccount
	if err := db.MySQL.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return fmt.Errorf("账户不存在: %w", err)
	}

	broker, err := s.getExecutionBroker(&account)
	if err != nil {
		return err
	}

	return broker.CancelOrder(&account, orderID, stockCode)
}

// ── Stock code helpers ──

func StockPriceDecimals(code string) int {
	if strings.HasPrefix(code, "6") || strings.HasPrefix(code, "9") {
		return 2
	}
	return 3
}

func FormatStockPrice(code string, price float64) float64 {
	decimals := StockPriceDecimals(code)
	format := fmt.Sprintf("%%.%df", decimals)
	formatted, _ := strconv.ParseFloat(fmt.Sprintf(format, price), 64)
	return formatted
}

func ParseStockCode(text string) string {
	for i := 0; i <= len(text)-6; i++ {
		if i+6 > len(text) {
			break
		}
		sub := text[i : i+6]
		if (sub[0] == '0' || sub[0] == '3' || sub[0] == '6' || sub[0] == '9') && isAllDigitsStr(sub) {
			return sub
		}
	}
	return ""
}

func isAllDigitsStr(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ── LobsterBroker (龙虾自动交易) ──
// Lobster 是一家本地券商代理，通过本地客户端操作实际账户。
// 信号状态设置为 pending_auto，由龙虾检查该状态后本地执行并回调。
// 该 broker 实现了 Broker 接口的骨架，实际集成待龙虾 SDK 接入后完成。

type LobsterBroker struct {
	client *http.Client
}

func NewLobsterBroker() *LobsterBroker {
	return &LobsterBroker{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *LobsterBroker) SyncPositions(account *model.TradingAccount) (*BrokerPortfolio, error) {
	return nil, fmt.Errorf("龙虾代理: 暂未实现")
}

func (b *LobsterBroker) GetBalance(account *model.TradingAccount) (*BrokerBalance, error) {
	return nil, fmt.Errorf("龙虾代理: 暂未实现")
}

func (b *LobsterBroker) PlaceOrder(account *model.TradingAccount, req *BrokerOrderRequest) (*BrokerOrderResult, error) {
	return nil, fmt.Errorf("龙虾代理: 请通过本地客户端下单，信号已设为 pending_auto")
}

func (b *LobsterBroker) CancelOrder(account *model.TradingAccount, orderID string, stockCode string) error {
	return fmt.Errorf("龙虾代理: 暂未实现")
}

func (b *LobsterBroker) QueryOrders(account *model.TradingAccount) ([]BrokerOrder, error) {
	return nil, fmt.Errorf("龙虾代理: 暂未实现")
}
