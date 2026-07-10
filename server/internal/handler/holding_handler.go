package handler

import (
	"log"
	"fmt"
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
	ID           uint    `json:"id"`
	AccountID    uint    `json:"accountId"`
	StockCode    string  `json:"stockCode"`
	StockName    string  `json:"stockName"`
	CostPrice    float64 `json:"costPrice"`
	Quantity     int     `json:"quantity"`
	TotalCost    float64 `json:"totalCost"`
	BuyDate      string  `json:"buyDate"`
	CurPrice     float64 `json:"curPrice"`
	PriceDate    string  `json:"priceDate"`
	PrevClose    float64 `json:"prevClose"`
	DailyChg     float64 `json:"dailyChg"`
	DailyChgPct  float64 `json:"dailyChgPct"`
	DailyPnl     float64 `json:"dailyPnl"`
	DailyPnlPct  float64 `json:"dailyPnlPct"`
	MarketVal    float64 `json:"marketVal"`
	Pnl          float64 `json:"pnl"`
	PnlPct       float64 `json:"pnlPct"`
	HoldDays     int     `json:"holdDays"`
	TodayBuyQty  int     `json:"todayBuyQty"`
	AvailSellQty int     `json:"availSellQty"`
	UpdatedAt    string  `json:"updatedAt"`
}

type AccountOverview struct {
	AccountID      uint    `json:"accountId"`
	AccountName    string  `json:"accountName"`
	Broker         string  `json:"broker"`
	AccountType    string  `json:"accountType"`
	InitialCapital float64 `json:"initialCapital"`
	AvailableCash  float64 `json:"availableCash"`
	PositionValue  float64 `json:"positionValue"`
	TotalEquity    float64 `json:"totalEquity"`
	CommittedToRuns float64 `json:"committedToRuns"`
	FreeCash        float64 `json:"freeCash"`
	TotalPnl       float64 `json:"totalPnl"`
	TotalPnlPct    float64 `json:"totalPnlPct"`
	DailyPnl       float64 `json:"dailyPnl"`
	PositionCount  int     `json:"positionCount"`
}

// AccountsOverview returns per-account breakdown for the holdings page.
func (h *HoldingHandler) AccountsOverview(c *gin.Context) {
	uid := getUID(c)
	accountType := c.Query("accountType") // real / simulated / ""=all

	var accounts []model.TradingAccount
	q := db.MySQL.Where("user_id = ? AND status = ?", uid, "active")
	if accountType != "" {
		q = q.Where("account_type = ?", accountType)
	}
	q.Find(&accounts)

	result := make([]AccountOverview, 0, len(accounts))
	for _, acc := range accounts {
		ov := AccountOverview{
			AccountID: acc.ID, AccountName: acc.Name, Broker: acc.Broker,
			AccountType: acc.AccountType, InitialCapital: acc.InitialCapital,
			AvailableCash: acc.AvailableCash,
		}

		// Compute committed capital from strategy runs
		var committed float64
		db.MySQL.Raw(`SELECT COALESCE(SUM(available_cash), 0) FROM strategy_runs WHERE account_id = ? AND status IN ("active", "paused")`, acc.ID).Scan(&committed)
		ov.CommittedToRuns = math.Round(committed*100) / 100
		ov.FreeCash = math.Round((acc.AvailableCash-committed)*100) / 100

		var holdings []model.Holding
		db.MySQL.Where("user_id = ? AND account_id = ?", uid, acc.ID).Find(&holdings)

		if len(holdings) == 0 {
			ov.TotalEquity = acc.AvailableCash
			result = append(result, ov)
			continue
		}

		codes := make([]string, len(holdings))
		for i, h := range holdings {
			codes[i] = h.StockCode
		}

		// Fetch prices from PG
		type PriceInfo struct {
			Code      string
			Close     float64
			PrevClose float64
		}
		var infos []PriceInfo
		infoMap := make(map[string]PriceInfo)
		db.PG.Raw(fmt.Sprintf(`SELECT s.code,
			COALESCE(k.close, 0) AS close,
			COALESCE(k2.close, 0) AS prev_close
			FROM stocks_basic s
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1 OFFSET 1) k2 ON true
			WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos)
		for _, info := range infos {
			infoMap[info.Code] = info
		}

		totalCost := 0.0
		for _, h := range holdings {
			pi := infoMap[h.StockCode]
			mv := pi.Close * float64(h.Quantity)
			pnl := (pi.Close - h.CostPrice) * float64(h.Quantity)
			dailyPnl := (pi.Close - pi.PrevClose) * float64(h.Quantity)
			ov.PositionValue += mv
			ov.TotalPnl += pnl
			ov.DailyPnl += dailyPnl
			totalCost += h.CostPrice * float64(h.Quantity)
			ov.PositionCount++
		}
		ov.PositionValue = math.Round(ov.PositionValue*100) / 100
		ov.TotalPnl = math.Round(ov.TotalPnl*100) / 100
		ov.DailyPnl = math.Round(ov.DailyPnl*100) / 100
		ov.TotalEquity = acc.AvailableCash + ov.PositionValue
		if totalCost > 0 {
			ov.TotalPnlPct = math.Round(ov.TotalPnl/totalCost*10000) / 100
		}
		result = append(result, ov)
	}

	response.Success(c, result)
}

// Summary returns aggregated portfolio overview across selected account type.
func (h *HoldingHandler) Summary(c *gin.Context) {
	uid := getUID(c)
	accountType := c.Query("accountType")

	var accounts []model.TradingAccount
	q := db.MySQL.Where("user_id = ? AND status = ?", uid, "active")
	if accountType != "" {
		q = q.Where("account_type = ?", accountType)
	}
	q.Find(&accounts)



	if len(accounts) == 0 {
		response.Success(c, map[string]interface{}{
			"initialCapital": 0, "availableCash": 0,
			"totalMarketValue": 0, "totalCost": 0, "totalEquity": 0,
			"totalPnl": 0, "totalPnlPct": 0, "totalDailyPnl": 0,
			"positionCount": 0, "upCount": 0, "downCount": 0,
		})
		return
	}

	totalCash := 0.0
	totalDeposit := 0.0
	totalCommitted := 0.0
	accountIDs := make([]uint, len(accounts))
	for i, acc := range accounts {
		totalCash += acc.AvailableCash
		totalDeposit += acc.InitialCapital
		accountIDs[i] = acc.ID
		// Sum committed capital from active/paused strategy runs
		var committed float64
		db.MySQL.Raw(`SELECT COALESCE(SUM(available_cash), 0) FROM strategy_runs WHERE account_id = ? AND status IN ("active", "paused")`, acc.ID).Scan(&committed)
		totalCommitted += committed
	}

	var holdings []model.Holding
	db.MySQL.Where("user_id = ? AND account_id IN ?", uid, accountIDs).Find(&holdings)

	totalMV, totalCost, totalPnl, totalDailyPnl, upCount, downCount := 0.0, 0.0, 0.0, 0.0, 0, 0

	if len(holdings) > 0 {
		codes := make([]string, len(holdings))
		for i, h := range holdings {
			codes[i] = h.StockCode
			totalCost += h.CostPrice * float64(h.Quantity)
		}

		type PriceInfo2 struct { Code string; Close float64; PrevClose float64 }
		var infos []PriceInfo2
		infoMap := make(map[string]PriceInfo2)
		db.PG.Raw(fmt.Sprintf(`SELECT s.code,
			COALESCE(k.close, 0) AS close,
			COALESCE(k2.close, 0) AS prev_close
			FROM stocks_basic s
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
			LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1 OFFSET 1) k2 ON true
			WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos)
		for _, info := range infos {
			infoMap[info.Code] = info
		}

		for _, h := range holdings {
			pi := infoMap[h.StockCode]
			curPrice := h.CurrentPrice
			if curPrice == 0 {
				curPrice = pi.Close
			}
			// Round to fen (2dp) to eliminate float64 drift from (price * quantity) arithmetic.
			mv := math.Round(curPrice*float64(h.Quantity)*100) / 100
			pnl := math.Round((curPrice-h.CostPrice)*float64(h.Quantity)*100) / 100
			dailyPnl := math.Round((pi.Close-pi.PrevClose)*float64(h.Quantity)*100) / 100
			totalMV += mv
			totalPnl += pnl
			totalDailyPnl += dailyPnl
			if pnl >= 0 { upCount++ } else { downCount++ }
		}
	}

	totalEquity := totalCash + totalMV
	totalPnlPct := 0.0
	if totalCost > 0 {
		totalPnlPct = totalPnl / totalCost * 100
	}

	totalFreeCash := totalCash - totalCommitted
	response.Success(c, map[string]interface{}{
		"initialCapital":   math.Round(totalDeposit*100) / 100,
		"availableCash":    math.Round(totalCash*100) / 100,
		"freeCash":         math.Round(totalFreeCash*100) / 100,
		"committedToRuns":  math.Round(totalCommitted*100) / 100,
		"totalMarketValue": math.Round(totalMV*100) / 100,
		"totalCost":        math.Round(totalCost*100) / 100,
		"totalEquity":      math.Round(totalEquity*100) / 100,
		"totalPnl":         math.Round(totalPnl*100) / 100,
		"totalPnlPct":      math.Round(totalPnlPct*100) / 100,
		"totalDailyPnl":    math.Round(totalDailyPnl*100) / 100,
		"positionCount":    len(holdings),
		"upCount":          upCount,
		"downCount":        downCount,
		"accountCount":     len(accounts),
	})
}

// List returns user's holdings, optionally filtered by account or account type.
func (h *HoldingHandler) List(c *gin.Context) {
	uid := getUID(c)
	accountIDStr := c.Query("accountId")
	accountType := c.Query("accountType")

	var holdings []model.Holding
	q := db.MySQL.Where("user_id = ?", uid)

	// Filter by account type (via JOIN with trading_accounts)
	if accountType != "" {
		q = q.Where("account_id IN (SELECT id FROM trading_accounts WHERE user_id = ? AND account_type = ? AND status = 'active')", uid, accountType)
	} else if accountIDStr != "" {
		if aid, err := strconv.Atoi(accountIDStr); err == nil && aid > 0 {
			q = q.Where("account_id = ?", aid)
		} else if accountIDStr == "0" {
			q = q.Where("account_id = 0")
		}
	}
	q.Order("created_at DESC").Find(&holdings)

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
	db.PG.Raw(fmt.Sprintf(`SELECT s.code, s.name,
		COALESCE(k.close, 0) AS close,
		TO_CHAR(k.trade_date, 'YYYY-MM-DD') AS price_date,
		COALESCE(k2.close, 0) AS prev_close
		FROM stocks_basic s
		LEFT JOIN LATERAL (SELECT close, trade_date FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1) k ON true
		LEFT JOIN LATERAL (SELECT close FROM stocks_daily_k WHERE code = s.code ORDER BY trade_date DESC LIMIT 1 OFFSET 1) k2 ON true
		WHERE s.code IN (%s)`, db.CodesToInClause(codes))).Scan(&infos)
	for _, info := range infos {
		infoMap[info.Code] = info
	}

	out := make([]HoldingOut, 0, len(holdings))
	now := time.Now()
	for _, h := range holdings {
		info := infoMap[h.StockCode]
		// Use holding's broker-synced CurrentPrice if available; fallback to PG close.
		curPrice := h.CurrentPrice
		if curPrice == 0 {
			curPrice = info.Close
		}
		mv := curPrice * float64(h.Quantity)
		pnl := (curPrice - h.CostPrice) * float64(h.Quantity)
		pnlPct := 0.0
		if h.CostPrice > 0 {
			pnlPct = (curPrice - h.CostPrice) / h.CostPrice * 100
		}
		dailyChg := curPrice - info.PrevClose
		dailyChgPct := 0.0
		if info.PrevClose > 0 {
			dailyChgPct = (curPrice - info.PrevClose) / info.PrevClose * 100
		}
		holdDays := 0
		if h.BuyDate != "" {
			if buyT, err := time.Parse("2006-01-02", h.BuyDate); err == nil {
				holdDays = int(now.Sub(buyT).Hours()/24) + 1
			}
		}

		out = append(out, HoldingOut{
			ID: h.ID, AccountID: h.AccountID,
			StockCode: h.StockCode, StockName: info.Name,
			CostPrice: h.CostPrice, Quantity: h.Quantity, TotalCost: h.TotalCost,
			BuyDate: h.BuyDate, CurPrice: curPrice, PriceDate: info.PriceDate,
			PrevClose: info.PrevClose, DailyChg: math.Round(dailyChg*100)/100, DailyChgPct: math.Round(dailyChgPct*100)/100,
			DailyPnl: math.Round((curPrice-info.PrevClose)*float64(h.Quantity)*100)/100,
			DailyPnlPct: math.Round((curPrice-info.PrevClose)/info.PrevClose*10000)/100,
			MarketVal: math.Round(mv*100)/100,
			Pnl: math.Round(pnl*100)/100, PnlPct: math.Round(pnlPct*100)/100,
			HoldDays: holdDays,
			TodayBuyQty: h.TodayBuyQty,
			AvailSellQty: h.AvailSellQty,
			UpdatedAt: h.UpdatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	response.Success(c, out)
}

// Create adds a new holding
func (h *HoldingHandler) Create(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		StockCode string  `json:"stockCode"`
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
		BuyDate   string  `json:"buyDate"`
		AccountID uint    `json:"accountId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.StockCode == "" || body.CostPrice <= 0 || body.Quantity <= 0 {
		response.BadRequest(c, "参数错误：stockCode/costPrice/quantity 必填且大于0")
		return
	}
	if body.BuyDate == "" {
		body.BuyDate = time.Now().Format("2006-01-02")
	}
	totalAmount := body.CostPrice * float64(body.Quantity)

	// Get account for balance check
	if body.AccountID > 0 {
		var acc model.TradingAccount
		db.MySQL.Where("id = ? AND user_id = ?", body.AccountID, uid).First(&acc)
		if acc.ID > 0 && acc.AvailableCash < totalAmount {
			response.BadRequest(c, fmt.Sprintf("账户余额不足: 需要 ¥%.0f, 可用 ¥%.0f", totalAmount, acc.AvailableCash))
			return
		}
		// Deduct from account
		acc.AvailableCash -= totalAmount
		db.MySQL.Save(&acc)
	}

	// Get stock name from PG
	var stockName string
	db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", body.StockCode).Scan(&stockName)

	holding := model.Holding{
		UserID: uid, AccountID: body.AccountID,
		StockCode: body.StockCode, CostPrice: body.CostPrice,
		Quantity: body.Quantity, TotalCost: totalAmount, BuyDate: body.BuyDate,
	}
	db.MySQL.Create(&holding)

	// Record trade
	db.MySQL.Create(&model.TradeRecord{
		UserID: uid, StockCode: body.StockCode, StockName: stockName,
		TradeType: "buy", TradeDate: body.BuyDate,
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
		response.BadRequest(c, "参数错误"); return
	}
	var body struct {
		CostPrice float64 `json:"costPrice"`
		Quantity  int     `json:"quantity"`
		BuyDate   string  `json:"buyDate"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Quantity <= 0 || body.CostPrice <= 0 {
		response.BadRequest(c, "参数错误：costPrice/quantity 必填且大于0"); return
	}

	totalCost := body.CostPrice * float64(body.Quantity)
	updates := map[string]interface{}{
		"cost_price": body.CostPrice, "quantity": body.Quantity, "total_cost": totalCost,
	}
	if body.BuyDate != "" { updates["buy_date"] = body.BuyDate }

	result := db.MySQL.Model(&model.Holding{}).Where("id = ? AND user_id = ?", id, uid).Updates(updates)
	if result.RowsAffected == 0 {
		response.NotFound(c, "持仓记录不存在"); return
	}
	response.SuccessMsg(c, "更新成功")
}

// Delete removes a holding (sell)
func (h *HoldingHandler) Delete(c *gin.Context) {
	uid := getUID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil { response.BadRequest(c, "参数错误"); return }

	var holding model.Holding
	if err := db.MySQL.Where("id = ? AND user_id = ?", id, uid).First(&holding).Error; err != nil {
		response.NotFound(c, "持仓记录不存在"); return
	}

	var curPrice float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", holding.StockCode).Scan(&curPrice)

	sellAmount := curPrice * float64(holding.Quantity)
	pnl := (curPrice - holding.CostPrice) * float64(holding.Quantity)
	pnlPct := 0.0
	if holding.CostPrice > 0 { pnlPct = (curPrice - holding.CostPrice) / holding.CostPrice * 100 }
	holdDays := 0
	if holding.BuyDate != "" {
		if buyT, err := time.Parse("2006-01-02", holding.BuyDate); err == nil { holdDays = int(time.Since(buyT).Hours()/24) + 1 }
	}

	// Sync with strategy live_positions: reduce quantity for managed stocks
	var affectedRuns []model.StrategyRun
	db.MySQL.Where("account_id = ? AND status IN ?", holding.AccountID, []string{"active", "paused"}).Find(&affectedRuns)
	for _, run := range affectedRuns {
		var lp model.LivePosition
		if err := db.MySQL.Where("strategy_run_id = ? AND stock_code = ? AND quantity > 0", run.ID, holding.StockCode).First(&lp).Error; err == nil {
			sellQty := holding.Quantity
			if sellQty > lp.Quantity {
				sellQty = lp.Quantity
			}
			lp.Quantity -= sellQty
			lp.AvailSellQty = lp.Quantity - lp.TodayBuyQty
			if lp.AvailSellQty < 0 { lp.AvailSellQty = 0 }
			run.AvailableCash += curPrice * float64(sellQty)
			db.MySQL.Save(&lp)
			db.MySQL.Save(&run)
			// Record cash flow
			db.MySQL.Create(&model.StrategyCashFlow{
				StrategyRunID: run.ID, AccountID: run.AccountID, UserID: run.UserID,
				FlowType: "manual_sell", Amount: curPrice * float64(sellQty),
				BeforeCash: run.AvailableCash - curPrice*float64(sellQty),
				AfterCash:  run.AvailableCash,
				Reason:     fmt.Sprintf("手动卖出 %s(%s) %d股 @¥%.2f", holding.StockName, holding.StockCode, sellQty, curPrice),
			})
			log.Printf("[holding] manual sell synced to strategy run=%d code=%s qty=%d", run.ID, holding.StockCode, sellQty)
		}
	}

	// Return cash to account
	if holding.AccountID > 0 {
		var acc model.TradingAccount
		db.MySQL.Where("id = ? AND user_id = ?", holding.AccountID, uid).First(&acc)
		if acc.ID > 0 {
			acc.AvailableCash += sellAmount
			db.MySQL.Save(&acc)
		}
	}

	var stockName string
	db.PG.Raw("SELECT name FROM stocks_basic WHERE code = ?", holding.StockCode).Scan(&stockName)
	db.MySQL.Create(&model.TradeRecord{
		UserID: uid, StockCode: holding.StockCode, StockName: stockName,
		TradeType: "sell", TradeDate: time.Now().Format("2006-01-02"),
		Price: curPrice, Quantity: holding.Quantity, Amount: sellAmount,
		Pnl: math.Round(pnl*100)/100, PnlPct: math.Round(pnlPct*100)/100,
		HoldDays: holdDays,
	})

	db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Holding{})
	response.SuccessMsg(c, fmt.Sprintf("已卖出，成交价 ¥%.2f，盈亏 %+.2f (%.2f%%)", curPrice, pnl, pnlPct))
}

// Account returns all active accounts for the holdings page account selector.
func (h *HoldingHandler) Account(c *gin.Context) {
	uid := getUID(c)
	var accounts []model.TradingAccount
	db.MySQL.Where("user_id = ? AND status = ?", uid, "active").Find(&accounts)
	response.Success(c, accounts)
}

// UpdateAccount updates account capital/cash
func (h *HoldingHandler) UpdateAccount(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Action    string  `json:"action"`
		Amount    float64 `json:"amount"`
		AccountID uint    `json:"accountId"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Amount <= 0 {
		response.BadRequest(c, "参数错误：amount 必须大于0"); return
	}

	// Use specified account or first active
	var acc model.TradingAccount
	if body.AccountID > 0 {
		db.MySQL.Where("id = ? AND user_id = ? AND status = ?", body.AccountID, uid, "active").First(&acc)
	} else {
		db.MySQL.Where("user_id = ? AND status = ?", uid, "active").First(&acc)
	}
	if acc.ID == 0 {
		response.BadRequest(c, "未找到活跃账户，请先创建资金账户")
		return
	}

	switch body.Action {
	case "deposit":
		acc.AvailableCash += body.Amount
		acc.TotalDeposit += body.Amount
	case "withdraw":
		// Check free cash (account cash - committed to runs)
		var committed float64
		db.MySQL.Raw("SELECT COALESCE(SUM(available_cash), 0) FROM strategy_runs WHERE account_id = ? AND status IN ('active', 'paused')", acc.ID).Scan(&committed)
		freeCash := acc.AvailableCash - committed
		if freeCash < body.Amount {
			response.BadRequest(c, fmt.Sprintf("可用余额不足: 需要 ¥%.2f, 可用 ¥%.2f (总 ¥%.2f - 已分配 ¥%.2f)", body.Amount, freeCash, acc.AvailableCash, committed))
			return
		}
		acc.AvailableCash -= body.Amount
		acc.TotalWithdraw += body.Amount
	default:
		response.BadRequest(c, "action 必须是 deposit 或 withdraw"); return
	}
	db.MySQL.Save(&acc)
	response.Success(c, acc)
}

func (h *HoldingHandler) TradeRecords(c *gin.Context) {
	uid := getUID(c)
	var records []model.TradeRecord
	db.MySQL.Where("user_id = ?", uid).Order("trade_date DESC, created_at DESC").Limit(500).Find(&records)
	response.Success(c, records)
}
