package handler

import (
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type HoldingHandler struct{}

func NewHoldingHandler() *HoldingHandler { return &HoldingHandler{} }

type HoldingOut struct {
	ID          uint    `json:"id"`
	StockCode   string  `json:"stockCode"`
	StockName   string  `json:"stockName"`
	CostPrice   float64 `json:"costPrice"`
	Quantity    int     `json:"quantity"`
	TotalCost   float64 `json:"totalCost"`
	BuyDate     string  `json:"buyDate"`
	CurPrice    float64 `json:"curPrice"`
	PriceDate   string  `json:"priceDate"`
	PrevClose   float64 `json:"prevClose"`
	DailyChg    float64 `json:"dailyChg"`
	DailyChgPct float64 `json:"dailyChgPct"`
	DailyPnl    float64 `json:"dailyPnl"`
	DailyPnlPct float64 `json:"dailyPnlPct"`
	MarketVal   float64 `json:"marketVal"`
	Pnl         float64 `json:"pnl"`
	PnlPct      float64 `json:"pnlPct"`
	HoldDays    int     `json:"holdDays"`
}

// Summary returns portfolio overview: total assets, available cash, positions summary.
func (h *HoldingHandler) Summary(c *gin.Context) {
	uid := getUID(c)

	// Get account
	var acc model.TradingAccount
	db.MySQL.Where("user_id = ?", uid).First(&acc)

	var holdings []model.Holding
	db.MySQL.Where("user_id = ?", uid).Find(&holdings)

	// Total market value
	totalMV := 0.0
	totalCost := 0.0
	totalPnl := 0.0
	upCount := 0
	downCount := 0

	var totalDailyPnl float64
	if len(holdings) > 0 {
		codes := make([]string, len(holdings))
		for i, h := range holdings {
			codes[i] = h.StockCode
			totalCost += h.CostPrice * float64(h.Quantity)
		}

		type PriceInfo2 struct {
			Code      string
			Close     float64
			PrevClose float64
		}
		var infos []PriceInfo2
		infoMap := make(map[string]PriceInfo2)
		if err := db.PG.Raw(fmt.Sprintf(`SELECT s.code,
			COALESCE(k.close, 0) AS close,
			COALESCE(k2.close, 0) AS prev_close
			FROM stocks_basic s
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1 OFFSET 1) k2 ON true
			WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos).Error; err != nil {
			log.Printf("[holding] summary price query failed: %v", err)
		}
		for _, info := range infos {
			infoMap[info.Code] = info
		}

		for _, h := range holdings {
			pi := infoMap[h.StockCode]
			cp := pi.Close
			mv := cp * float64(h.Quantity)
			pnl := (cp - h.CostPrice) * float64(h.Quantity)
			dailyPnl := (cp - pi.PrevClose) * float64(h.Quantity)
			totalMV += mv
			totalPnl += pnl
			totalDailyPnl += dailyPnl
			if pnl >= 0 {
				upCount++
			} else {
				downCount++
			}
		}
	}

	totalEquity := acc.AvailableCash + totalMV
	totalPnlPct := 0.0
	if totalCost > 0 {
		totalPnlPct = totalPnl / totalCost * 100
	}

	response.Success(c, map[string]interface{}{
		"initialCapital":   math.Round(acc.InitialCapital*100) / 100,
		"availableCash":    math.Round(acc.AvailableCash*100) / 100,
		"totalMarketValue": math.Round(totalMV*100) / 100,
		"totalCost":        math.Round(totalCost*100) / 100,
		"totalEquity":      math.Round(totalEquity*100) / 100,
		"totalPnl":         math.Round(totalPnl*100) / 100,
		"totalPnlPct":      math.Round(totalPnlPct*100) / 100,
		"totalDailyPnl":    math.Round(totalDailyPnl*100) / 100,
		"positionCount":    len(holdings),
		"upCount":          upCount,
		"downCount":        downCount,
	})
}

// List returns user's holdings enriched with latest price from PG
func (h *HoldingHandler) List(c *gin.Context) {
	uid := getUID(c)
	var holdings []model.Holding
	db.MySQL.Where("user_id = ?", uid).Order("created_at DESC").Find(&holdings)

	if len(holdings) == 0 {
		response.Success(c, []HoldingOut{})
		return
	}

	codes := make([]string, len(holdings))
	for i, h := range holdings {
		codes[i] = h.StockCode
	}

	type PriceInfo struct {
		Code      string
		Name      string
		Close     float64
		PriceDate string
		PrevClose float64
	}
	var infos []PriceInfo
	infoMap := make(map[string]PriceInfo)
	if err := db.PG.Raw(fmt.Sprintf(`SELECT s.code, s.name,
		COALESCE(k.close, 0) AS close,
		TO_CHAR(k.trade_date, 'YYYY-MM-DD') AS price_date,
		COALESCE(k2.close, 0) AS prev_close
		FROM stocks_basic s
		LEFT JOIN LATERAL (SELECT close, trade_date FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
		LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1 OFFSET 1) k2 ON true
		WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos).Error; err != nil {
		log.Printf("[holding] price info query failed: %v", err)
	}
	for _, info := range infos {
		infoMap[info.Code] = info
	}

	out := make([]HoldingOut, 0, len(holdings))
	for _, h := range holdings {
		info := infoMap[h.StockCode]
		curPrice := info.Close
		marketVal := curPrice * float64(h.Quantity)
		pnl := (curPrice - h.CostPrice) * float64(h.Quantity)
		pnlPct := 0.0
		if h.CostPrice > 0 {
			pnlPct = (curPrice - h.CostPrice) / h.CostPrice * 100
		}
		holdDays := 0
		if h.BuyDate != "" {
			buyT, err := time.Parse("2006-01-02", h.BuyDate)
			if err == nil {
				holdDays = int(time.Since(buyT).Hours() / 24)
			}
		}
		// Daily P&L: use prevClose, or costPrice for stocks bought today
		var dailyChg, dailyChgPct, dailyPnl, dailyPnlPct float64
		if curPrice > 0 {
			refPrice := info.PrevClose
			todayStr := time.Now().Format("2006-01-02")
			if refPrice <= 0 || h.BuyDate == todayStr {
				refPrice = h.CostPrice
			}
			dailyChg = curPrice - refPrice
			if refPrice > 0 {
				dailyChgPct = dailyChg / refPrice * 100
			}
			dailyPnl = dailyChg * float64(h.Quantity)
			dailyPnlPct = dailyChgPct
		}

		out = append(out, HoldingOut{
			ID:          h.ID,
			StockCode:   h.StockCode,
			StockName:   info.Name,
			CostPrice:   h.CostPrice,
			Quantity:    h.Quantity,
			TotalCost:   h.CostPrice * float64(h.Quantity),
			BuyDate:     h.BuyDate,
			CurPrice:    curPrice,
			PriceDate:   info.PriceDate,
			PrevClose:   info.PrevClose,
			DailyChg:    math.Round(dailyChg*100) / 100,
			DailyChgPct: math.Round(dailyChgPct*100) / 100,
			DailyPnl:    math.Round(dailyPnl*100) / 100,
			DailyPnlPct: math.Round(dailyPnlPct*100) / 100,
			MarketVal:   marketVal,
			Pnl:         pnl,
			PnlPct:      pnlPct,
			HoldDays:    holdDays,
		})
	}
	response.Success(c, out)
}

// Create adds a new holding — acts as a "buy" trade.
// If the same stock is already held, merges quantities and recalculates weighted average cost.
func (h *HoldingHandler) Create(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		StockCode string  `json:"stockCode"`
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
		BuyDate   string  `json:"buyDate"` // optional
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.StockCode == "" || body.Quantity <= 0 || body.CostPrice <= 0 {
		response.BadRequest(c, "参数错误：stockCode/costPrice/quantity 必填且大于0")
		return
	}

	var count int64
	if err := db.PG.Raw("SELECT COUNT(*) FROM stocks_basic WHERE code = ?", body.StockCode).Scan(&count).Error; err != nil || count == 0 {
		response.BadRequest(c, "股票代码不存在: "+body.StockCode)
		return
	}

	buyDate := body.BuyDate
	todayStr := time.Now().Format("2006-01-02")
	if buyDate == "" {
		buyDate = todayStr
	}
	totalAmount := body.CostPrice * float64(body.Quantity)

	// Check account balance
	var acc model.TradingAccount
	db.MySQL.Where("user_id = ?", uid).First(&acc)
	if acc.ID == 0 {
		acc = model.TradingAccount{
			UserID: uid, InitialCapital: 100000, AvailableCash: 100000,
		}
		db.MySQL.Create(&acc)
	}
	if acc.AvailableCash < totalAmount {
		response.BadRequest(c, fmt.Sprintf("可用余额不足：需要 ¥%.2f，当前余额 ¥%.2f", totalAmount, acc.AvailableCash))
		return
	}

	// Deduct cash
	acc.AvailableCash -= totalAmount
	db.MySQL.Save(&acc)

	// Check if same stock already held → merge
	var existing model.Holding
	result := db.MySQL.Where("user_id = ? AND stock_code = ?", uid, body.StockCode).First(&existing)
	if result.Error == nil {
		// Merge: weighted average cost
		newTotalQty := existing.Quantity + body.Quantity
		newTotalCost := existing.CostPrice*float64(existing.Quantity) + body.CostPrice*float64(body.Quantity)
		newAvgCost := newTotalCost / float64(newTotalQty)

		db.MySQL.Model(&existing).Updates(map[string]interface{}{
			"cost_price": math.Round(newAvgCost*10000) / 10000,
			"quantity":   newTotalQty,
			"total_cost": math.Round(newTotalCost*100) / 100,
			"buy_date":   buyDate, // update to latest buy date
		})

		// Record trade (no holdingID since merged)
		var stockName string
		db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", body.StockCode).Scan(&stockName)
		db.MySQL.Create(&model.TradeRecord{
			UserID: uid, StockCode: body.StockCode, StockName: stockName,
			TradeType: "buy", TradeDate: buyDate,
			Price: body.CostPrice, Quantity: body.Quantity, Amount: totalAmount,
		})

		response.SuccessMsg(c, fmt.Sprintf("加仓成功！均价 ¥%.2f，总持仓 %d 股", newAvgCost, newTotalQty))
		return
	}

	// New holding
	var stockName string
	db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", body.StockCode).Scan(&stockName)

	holding := model.Holding{
		UserID:    uid,
		StockCode: body.StockCode,
		CostPrice: body.CostPrice,
		Quantity:  body.Quantity,
		TotalCost: totalAmount,
		BuyDate:   buyDate,
	}
	if err := db.MySQL.Create(&holding).Error; err != nil {
		response.Error(c, 500, 5001, "创建持仓失败: "+err.Error())
		return
	}

	// Record trade
	db.MySQL.Create(&model.TradeRecord{
		UserID: uid, StockCode: body.StockCode, StockName: stockName,
		TradeType: "buy", TradeDate: buyDate,
		Price: body.CostPrice, Quantity: body.Quantity, Amount: totalAmount,
		HoldingID: &holding.ID,
	})

	response.Created(c, holding)
}

// Update modifies cost & quantity
func (h *HoldingHandler) Update(c *gin.Context) {
	uid := getUID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var body struct {
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
		BuyDate   string  `json:"buyDate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Quantity <= 0 || body.CostPrice <= 0 {
		response.BadRequest(c, "参数错误：costPrice/quantity 必填且大于0")
		return
	}

	totalCost := body.CostPrice * float64(body.Quantity)
	updates := map[string]interface{}{
		"cost_price": body.CostPrice,
		"quantity":   body.Quantity,
		"total_cost": totalCost,
	}
	if body.BuyDate != "" {
		updates["buy_date"] = body.BuyDate
	}

	result := db.MySQL.Model(&model.Holding{}).
		Where("id = ? AND user_id = ?", id, uid).
		Updates(updates)
	if result.RowsAffected == 0 {
		response.NotFound(c, "持仓记录不存在")
		return
	}
	response.SuccessMsg(c, "更新成功")
}

// Delete removes a holding — acts as a "sell" trade (全部卖出)
func (h *HoldingHandler) Delete(c *gin.Context) {
	uid := getUID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Find the holding first
	var holding model.Holding
	if err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&holding).Error; err != nil {
		response.NotFound(c, "持仓记录不存在")
		return
	}

	// Get current price from PG
	var curPrice float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1",
		holding.StockCode).Scan(&curPrice)

	// Calculate P&L
	sellAmount := curPrice * float64(holding.Quantity)
	pnl := (curPrice - holding.CostPrice) * float64(holding.Quantity)
	pnlPct := 0.0
	if holding.CostPrice > 0 {
		pnlPct = (curPrice - holding.CostPrice) / holding.CostPrice * 100
	}
	holdDays := 0
	if holding.BuyDate != "" {
		buyT, err := time.Parse("2006-01-02", holding.BuyDate)
		if err == nil {
			holdDays = int(time.Since(buyT).Hours() / 24)
		}
	}

	// Update account balance
	var acc model.TradingAccount
	db.MySQL.Where("user_id = ?", uid).First(&acc)
	acc.AvailableCash += sellAmount
	db.MySQL.Save(&acc)

	// Record sell trade
	var stockName string
	db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", holding.StockCode).Scan(&stockName)
	db.MySQL.Create(&model.TradeRecord{
		UserID: uid, StockCode: holding.StockCode, StockName: stockName,
		TradeType: "sell", TradeDate: time.Now().Format("2006-01-02"),
		Price: curPrice, Quantity: holding.Quantity, Amount: sellAmount,
		Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
		HoldDays: holdDays,
	})

	// Delete holding
	db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Holding{})

	response.SuccessMsg(c, fmt.Sprintf("已卖出，成交价 ¥%.2f，盈亏 %+.2f (%.2f%%)", curPrice, pnl, pnlPct))
}

// Account returns or creates the user's trading account
func (h *HoldingHandler) Account(c *gin.Context) {
	uid := getUID(c)

	var acc model.TradingAccount
	db.MySQL.Where("user_id = ?", uid).First(&acc)
	if acc.ID == 0 {
		acc = model.TradingAccount{
			UserID: uid, InitialCapital: 100000, AvailableCash: 100000,
		}
		db.MySQL.Create(&acc)
	}
	response.Success(c, acc)
}

// UpdateAccount updates account capital/cash
func (h *HoldingHandler) UpdateAccount(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Action  string  `json:"action"`  // deposit / withdraw
		Amount  float64 `json:"amount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		response.BadRequest(c, "参数错误：amount 必须大于0")
		return
	}

	var acc model.TradingAccount
	db.MySQL.Where("user_id = ?", uid).First(&acc)
	if acc.ID == 0 {
		acc = model.TradingAccount{
			UserID: uid, InitialCapital: 100000, AvailableCash: 100000,
		}
		db.MySQL.Create(&acc)
	}

	switch body.Action {
	case "deposit":
		acc.AvailableCash += body.Amount
		acc.TotalDeposit += body.Amount
	case "withdraw":
		if acc.AvailableCash < body.Amount {
			response.BadRequest(c, fmt.Sprintf("可用余额不足：需要 ¥%.2f，当前 ¥%.2f", body.Amount, acc.AvailableCash))
			return
		}
		acc.AvailableCash -= body.Amount
		acc.TotalWithdraw += body.Amount
	default:
		response.BadRequest(c, "action 必须是 deposit 或 withdraw")
		return
	}
	db.MySQL.Save(&acc)
	response.Success(c, acc)
}

// TradeRecords returns all buy/sell trade history
func (h *HoldingHandler) TradeRecords(c *gin.Context) {
	uid := getUID(c)
	var records []model.TradeRecord
	db.MySQL.Where("user_id = ?", uid).Order("trade_date DESC, created_at DESC").Limit(500).Find(&records)
	response.Success(c, records)
}
