package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type StrategyHandler struct {
	aiSvc *service.AIService
}

func NewStrategyHandler() *StrategyHandler { return &StrategyHandler{aiSvc: service.NewAIService()} }

// ── Strategy CRUD ──

func (h *StrategyHandler) List(c *gin.Context) {
	uid := getUID(c)
	var strategies []model.Strategy
	db.MySQL.Where("user_id = ?", uid).Order("sort_order ASC, id ASC").Find(&strategies)
	log.Printf("[strategy] list uid=%d count=%d", uid, len(strategies))
	response.Success(c, strategies)
}

func (h *StrategyHandler) Create(c *gin.Context) {
	uid := getUID(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Name == "" {
		response.BadRequest(c, "策略名称不能为空")
		return
	}
	var count int64
	db.MySQL.Model(&model.Strategy{}).Where("user_id = ?", uid).Count(&count)
	s := model.Strategy{
		UserID:    uid,
		Name:      body.Name,
		SortOrder: int(count),
	}
	if count == 0 {
		s.IsDefault = true
	}
	if err := db.MySQL.Create(&s).Error; err != nil {
		log.Printf("[strategy] create error: %v", err)
		response.InternalError(c, "创建失败")
		return
	}
	log.Printf("[strategy] created id=%d name=%s uid=%d", s.ID, s.Name, uid)
	response.Created(c, s)
}


func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64: return val, true
	case float32: return float64(val), true
	case int: return float64(val), true
	case int64: return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func (h *StrategyHandler) Update(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	// Use raw map first to detect which fields were actually sent
	raw := make(map[string]interface{})
	if err := c.ShouldBindJSON(&raw); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if v, ok := raw["name"]; ok && v != "" { updates["name"] = v }
	if _, ok := raw["description"]; ok { updates["description"] = raw["description"] }
	if _, ok := raw["stopProfit"]; ok { updates["stop_profit"] = raw["stopProfit"] }
	if _, ok := raw["stopLoss"]; ok { updates["stop_loss"] = raw["stopLoss"] }
	if v, ok := raw["maxHoldings"]; ok { if n, _ := toFloat64(v); n > 0 { updates["max_holdings"] = int(n) } }
	if v, ok := raw["initialCapital"]; ok { if n, _ := toFloat64(v); n > 0 { updates["initial_capital"] = n } }
	if v, ok := raw["buyPositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["buy_position_pct"] = n } }
	if v, ok := raw["addPositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["add_position_pct"] = n } }
	if v, ok := raw["reducePositionPct"]; ok { if n, _ := toFloat64(v); n > 0 { updates["reduce_position_pct"] = n } }
	if v, ok := raw["investmentType"]; ok && v != "" { updates["investment_type"] = v }
	if _, ok := raw["regularAmount"]; ok { updates["regular_amount"] = raw["regularAmount"] }
	if v, ok := raw["regularInterval"]; ok && v != "" { updates["regular_interval"] = v }
	if _, ok := raw["stockCodes"]; ok { updates["stock_codes"] = raw["stockCodes"] }
	db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Updates(updates)
	response.SuccessMsg(c, "更新成功")
}

func (h *StrategyHandler) Delete(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.Strategy{})
	db.MySQL.Where("strategy_id = ?", id).Delete(&model.StrategyCondition{})
	response.SuccessMsg(c, "已删除")
}

func (h *StrategyHandler) Reorder(c *gin.Context) {
	uid := getUID(c)
	var body struct{ IDs []uint `json:"ids"` }
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for i, id := range body.IDs {
		db.MySQL.Model(&model.Strategy{}).Where("id = ? AND user_id = ?", id, uid).Update("sort_order", i)
	}
	response.SuccessMsg(c, "ok")
}

// ── Conditions ──

func (h *StrategyHandler) ListConditions(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	// Verify ownership
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ?", sid).Order("cond_type, logic_group, sort_order").Find(&conds)
	response.Success(c, conds)
}

func (h *StrategyHandler) SaveConditions(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}
	var body struct {
		Conditions []model.StrategyCondition `json:"conditions"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// Replace all conditions
	db.MySQL.Where("strategy_id = ?", sid).Delete(&model.StrategyCondition{})
	for i := range body.Conditions {
		body.Conditions[i].ID = 0
		body.Conditions[i].StrategyID = uint(sid)
		if body.Conditions[i].LogicGroup == 0 {
			body.Conditions[i].LogicGroup = 1
		}
		db.MySQL.Create(&body.Conditions[i])
	}
	response.SuccessMsg(c, "条件保存成功")
}

// ── AI Generate Strategy ──

func (h *StrategyHandler) AIGenerate(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Style       string `json:"style"` // aggressive / moderate / conservative
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	style := body.Style
	if style == "" {
		style = "moderate"
	}

	cfg, err := h.aiSvc.GetConfig(uid)
	if err != nil || cfg.APIKey == "" {
		response.Error(c, 400, response.CodeAIConfigMissing, "请先配置AI模型")
		return
	}

	// Compact indicator list for AI prompt
	indicators := `榜单: algo_score(0-10), streak_count
趋势: daily_change, momentum_5/20, ma_deviation, ma_cross(cross_up/down,val=5/20), macd, adx(>25), dmi_plus/minus
超买超卖: rsi(>70/<30), kdj_k/d/j, boll_position(>80/<20), cci(>100/<-100), williams_r, mfi
量价: volume_ratio, turnover_rate, atr/pct, volume_trend
形态: drawdown_20, new_high_20, up_days_ratio, price_position_20/60, gap_pct, high_low_range, ma_convergence, trend_strength, index_relative
估值: pe/pb/ps, pe_percentile(<30)
基本面: roe, revenue_growth, profit_growth, gross_margin, debt_ratio
资金: total_market_cap, shareholder_change
(以上均需检查数据覆盖⚠️)`

	prompt := fmt.Sprintf(`你是量化策略专家。根据用户描述生成A股策略JSON。

%s

用户策略名: %s
用户描述: %s
风险偏好: %s

返回纯JSON（无markdown）:
{"name":"..","description":"..","stopProfit":止盈%%,"stopLoss":止损%%(负数),"maxHoldings":最大持仓,
"conditions":[{"condType":"buy|add|sell|reduce","indicator":"..","operator":"gte|lte|eq|cross_up|cross_down","value":数字,"logicGroup":1,"sortOrder":0}]}
`, indicators, body.Name, body.Description, style)

	reply, err := h.aiSvc.ChatCompletion(uid, prompt, nil)
	if err != nil {
		response.Error(c, 500, response.CodeAIModelError, "AI生成失败: "+err.Error())
		return
	}
	if reply == "" {
		response.Error(c, 500, response.CodeAIModelError, "AI返回空内容，请检查模型配置或稍后重试")
		return
	}

	// Parse JSON response with flexible value types
	reply = cleanJSON(reply)
	if reply == "" {
		response.Error(c, 500, response.CodeAIModelError, "AI返回内容无法解析，可能未按要求返回JSON格式")
		return
	}
	var rawResult struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		StopProfit  float64 `json:"stopProfit"`
		StopLoss    float64 `json:"stopLoss"`
		MaxHoldings int     `json:"maxHoldings"`
		Conditions  []struct {
			CondType   string      `json:"condType"`
			Indicator  string      `json:"indicator"`
			Operator   string      `json:"operator"`
			Value      interface{} `json:"value"` // accept both string and number
			LogicGroup int         `json:"logicGroup"`
			SortOrder  int         `json:"sortOrder"`
		} `json:"conditions"`
	}
	if err := json.Unmarshal([]byte(reply), &rawResult); err != nil {
		preview := reply
		if len(preview) > 300 { preview = preview[:300] + "..." }
		response.Error(c, 500, response.CodeAIModelError, "AI返回解析失败: "+err.Error()+" | 原文: "+preview)
		return
	}

	// Convert conditions with flexible values
	result := struct {
		Name        string                   `json:"name"`
		Description string                   `json:"description"`
		StopProfit  float64                  `json:"stopProfit"`
		StopLoss    float64                  `json:"stopLoss"`
		MaxHoldings int                      `json:"maxHoldings"`
		Conditions  []model.StrategyCondition `json:"conditions"`
	}{
		Name:        rawResult.Name,
		Description: rawResult.Description,
		StopProfit:  rawResult.StopProfit,
		StopLoss:    rawResult.StopLoss,
		MaxHoldings: rawResult.MaxHoldings,
	}
	// Backward compatibility: map old indicator names to new ones
	oldToNew := map[string]string{
		"pe_level": "pe_percentile",
	}
	for _, rc := range rawResult.Conditions {
		if newName, ok := oldToNew[rc.Indicator]; ok {
			rc.Indicator = newName
		}
		v := parseValue(rc.Value, rc.Indicator, rc.Operator)
		result.Conditions = append(result.Conditions, model.StrategyCondition{
			CondType:   rc.CondType,
			Indicator:  rc.Indicator,
			Operator:   rc.Operator,
			Value:      v,
			LogicGroup: rc.LogicGroup,
			SortOrder:  rc.SortOrder,
		})
	}

	response.Success(c, result)
}

// ── Prompt Optimizer ──

func (h *StrategyHandler) OptimizePrompt(c *gin.Context) {
	uid := getUID(c)
	var body struct {
		Prompt string `json:"prompt"`
		Style  string `json:"style"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Prompt == "" {
		response.BadRequest(c, "请输入描述要求")
		return
	}
	cfg, err := h.aiSvc.GetConfig(uid)
	if err != nil || cfg.APIKey == "" {
		response.Error(c, 400, response.CodeAIConfigMissing, "请先配置AI模型")
		return
	}
	style := body.Style
	if style == "" { style = "moderate" }
	prompt := fmt.Sprintf(`你是一个量化交易策略专家。用户想创建一个A股交易策略，但描述比较简略。请将以下用户要求优化为结构化的策略描述，包含：投资风格、选股偏好、买入时机、卖出时机、仓位管理、风险控制等方面。直接用中文输出优化后的描述，不要加任何前缀说明。\n\n用户原始要求：%s\n风险偏好：%s\n\n优化后的策略描述：`, body.Prompt, style)
	reply, err := h.aiSvc.ChatCompletion(uid, prompt, nil)
	if err != nil {
		response.Error(c, 500, response.CodeAIModelError, "AI优化失败: "+err.Error())
		return
	}
	response.Success(c, map[string]string{"optimized": reply})
}


// ── Backtest ──

// ═══════════════════════════════════════════════════════════════
// 异步回测引擎
// ═══════════════════════════════════════════════════════════════

type backtestTrade struct {
	Date     string  `json:"date"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Action   string  `json:"action"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Reason   string  `json:"reason"`
	Pnl      float64 `json:"pnl"`
	PnlPct   float64 `json:"pnlPct"`
}

// map of running tasks per strategy: strategyID -> map[taskID]context.CancelFunc
var runningBacktests = make(map[uint]map[uint]context.CancelFunc)

func init() {
	// Initialize outer map lazily; access protected by getRunningMap
}

func getRunningMap(sid uint) map[uint]context.CancelFunc {
	if runningBacktests[sid] == nil {
		runningBacktests[sid] = make(map[uint]context.CancelFunc)
	}
	return runningBacktests[sid]
}

// ── Start async backtest ──

// resolveStockPool converts a pool key + fallback codes into actual stock code list
func resolveStockPool(uid uint, poolKey string, fallbackCodes []string) []string {
	if poolKey == "" {
		return fallbackCodes
	}
	switch {
	case poolKey == "all":
		return nil // nil signals "all stocks" to runBacktestAsync
	case strings.HasPrefix(poolKey, "watchlist_"):
		gidStr := strings.TrimPrefix(poolKey, "watchlist_")
		gid, err := strconv.Atoi(gidStr)
		if err != nil {
			return fallbackCodes
		}
		var codes []string
		db.MySQL.Raw("SELECT stock_code FROM watchlists WHERE user_id = ? AND group_id = ?", uid, gid).Pluck("stock_code", &codes)
		return codes
	case poolKey == "portfolio":
		var codes []string
		db.MySQL.Raw("SELECT DISTINCT stock_code FROM holdings WHERE user_id = ?", uid).Pluck("stock_code", &codes)
		return codes
	}
	return fallbackCodes
}

// resolveStockPoolLabel converts a pool key to a human-readable display label.
// Keys: "all", "watchlist_N", "portfolio", "codes"
// Uses stockPoolParams JSON to derive the count.
func resolveStockPoolLabel(poolKey, poolParamsJSON string) string {
	count := 0
	if poolParamsJSON != "" {
		var codes []string
		if json.Unmarshal([]byte(poolParamsJSON), &codes) == nil {
			count = len(codes)
		}
	}
	var label string
	switch {
	case poolKey == "all":
		label = "全部股票"
	case strings.HasPrefix(poolKey, "watchlist_"):
		label = "自选组"
	case poolKey == "portfolio":
		label = "我的持仓"
	case poolKey == "codes":
		label = "自选代码"
	default:
		label = poolKey // fallback for legacy data
	}
	if count > 0 {
		return fmt.Sprintf("%s (%d只)", label, count)
	}
	return label
}

func (h *StrategyHandler) StartBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		StartDate  string   `json:"startDate"`
		EndDate    string   `json:"endDate"`
		StockCodes []string `json:"stockCodes"`
		StockPool  string   `json:"stockPool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// Resolve stock pool to actual stock codes
	resolvedCodes := resolveStockPool(uid, body.StockPool, body.StockCodes)

	// Load strategy
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		response.NotFound(c, "策略不存在")
		return
	}

	// Validate date range
	startDate, _ := time.Parse("2006-01-02", body.StartDate)
	endDate, _ := time.Parse("2006-01-02", body.EndDate)
	if startDate.After(endDate) {
		response.BadRequest(c, "开始日期不能晚于结束日期")
		return
	}

	// Count trading days for estimate
	var totalDays int
	db.PG.Raw(`SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ?`, body.StartDate, body.EndDate).Scan(&totalDays)
	if totalDays == 0 {
		response.BadRequest(c, "所选时间段无交易日数据")
		return
	}

	// Serialize params
	paramsBytes, _ := json.Marshal(map[string]interface{}{
		"startDate":   body.StartDate,
		"endDate":     body.EndDate,
		"stockCodes":  resolvedCodes,
		"stockPool":   body.StockPool,
	})

	// Create task
	task := model.BacktestTask{
		UserID:     uid,
		StrategyID: uint(sid),
		Status:     "pending",
		Phase:      "排队中",
		TotalDays:  totalDays,
		Params:     string(paramsBytes),
	}
	db.MySQL.Create(&task)

	// Launch goroutine
	ctx, cancel := context.WithCancel(context.Background())
	getRunningMap(uint(sid))[task.ID] = cancel

	go h.runBacktestAsync(ctx, &task, &s, body.StartDate, body.EndDate, resolvedCodes)

	response.Created(c, map[string]interface{}{
		"taskId":    task.ID,
		"totalDays": totalDays,
		"status":    "pending",
	})
}

// ── Poll task status ──

func (h *StrategyHandler) BacktestStatus(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	resp := map[string]interface{}{
		"taskId":     task.ID,
		"status":     task.Status,
		"phase":      task.Phase,
		"currentDay": task.CurrentDay,
		"totalDays":  task.TotalDays,
		"progressPct": task.ProgressPct,
		"errorMsg":   task.ErrorMsg,
		"resultId":   task.ResultID,
		"startedAt":  task.StartedAt,
		"completedAt": task.CompletedAt,
	}

	// Include current positions snapshot if available
	if task.CurrentPositions != "" {
		var positions interface{}
		if err := json.Unmarshal([]byte(task.CurrentPositions), &positions); err == nil {
			resp["currentPositions"] = positions
		}
	}

	response.Success(c, resp)
}

// ── SSE stream for live viewing (reconnectable) ──

func (h *StrategyHandler) BacktestStream(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, 500, response.CodeInternalError, "不支持流式输出")
		return
	}

	sendSSE := func(typ string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: {\"type\":\"%s\",\"payload\":%s}\n\n", typ, string(b))
		flusher.Flush()
	}

	// Send current snapshot immediately
	sendSSE("status", map[string]interface{}{
		"status":      task.Status,
		"phase":       task.Phase,
		"currentDay":  task.CurrentDay,
		"totalDays":   task.TotalDays,
		"progressPct": task.ProgressPct,
	})

	if task.Status == "completed" || task.Status == "failed" || task.Status == "cancelled" {
		if task.Status == "completed" && task.ResultID != nil {
			var bt model.BacktestResult
			db.MySQL.First(&bt, *task.ResultID)
			metrics := map[string]interface{}{
				"totalReturn": bt.TotalReturn,
				"sharpeRatio": bt.SharpeRatio,
				"maxDrawdown": bt.MaxDrawdown,
				"winRate":     bt.WinRate,
				"tradeCount":  bt.TradeCount,
				"resultId":    bt.ID,
			}
			sendSSE("metric", metrics)
			// Send trades and equity curve
			if tradesData, ok := bt.Trades["data"]; ok {
				sendSSE("trades", tradesData)
			}
			if equityData, ok := bt.EquityCurve["data"]; ok {
				sendSSE("equity", equityData)
			}
		}
		sendSSE("done", map[string]string{"message": "回测" + task.Status})
		return
	}

	// Poll task status periodically
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx := c.Request.Context()
	lastDay := 0
	lastPositions := ""

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			db.MySQL.First(&task, tid)
			// Send phase updates
			if task.Phase != "" {
				sendSSE("phase", map[string]string{"phase": "", "message": task.Phase})
			}
			// Send position updates
			if task.CurrentDay != lastDay || task.CurrentPositions != lastPositions {
				lastDay = task.CurrentDay
				lastPositions = task.CurrentPositions
				sendSSE("position", map[string]interface{}{
					"day":       task.CurrentDay,
					"totalDays": task.TotalDays,
				})
				// If positions JSON available, parse and send
				if task.CurrentPositions != "" {
					var posData map[string]interface{}
					if json.Unmarshal([]byte(task.CurrentPositions), &posData) == nil {
						sendSSE("position", posData)
						// Emit individual trade events
						if trades, ok := posData["recentTrades"].([]interface{}); ok {
							for _, t := range trades {
								sendSSE("trade", t)
							}
						}
					}
				}
			}
			// Check terminal states
			if task.Status == "completed" {
				if task.ResultID != nil {
					var bt model.BacktestResult
					db.MySQL.First(&bt, *task.ResultID)
					metrics := map[string]interface{}{
						"totalReturn": bt.TotalReturn,
						"sharpeRatio": bt.SharpeRatio,
						"maxDrawdown": bt.MaxDrawdown,
						"winRate":     bt.WinRate,
						"tradeCount":  bt.TradeCount,
						"resultId":    bt.ID,
					}
					sendSSE("metric", metrics)
					if tradesData, ok := bt.Trades["data"]; ok {
						sendSSE("trades", tradesData)
					}
					if equityData, ok := bt.EquityCurve["data"]; ok {
						sendSSE("equity", equityData)
					}
				}
				sendSSE("done", map[string]string{"message": "回测完成"})
				return
			}
			if task.Status == "failed" {
				sendSSE("error", map[string]string{"message": task.ErrorMsg})
				sendSSE("done", map[string]string{"message": "回测失败"})
				return
			}
			if task.Status == "cancelled" {
				sendSSE("done", map[string]string{"message": "回测已取消"})
				return
			}
		}
	}
}

// ── Cancel running task ──

func (h *StrategyHandler) CancelBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Only cancel active tasks; allow force-cancel for orphaned ones too
	canForce := task.Status == "running" || task.Status == "pending"

	// Try graceful cancel via context
	cancelled := false
	if task.Status == "running" || task.Status == "pending" {
		rm := getRunningMap(uint(sid))
		if cancel, ok := rm[uint(tid)]; ok {
			cancel()
			delete(rm, uint(tid))
			cancelled = true
		}
	}

	// Always update DB status (handles orphaned tasks after server restart)
	db.MySQL.Model(&task).Updates(map[string]interface{}{
		"status": "cancelled",
		"phase":  "已取消",
	})
	now := time.Now()
	db.MySQL.Model(&task).Update("completed_at", now)

	if cancelled {
		response.SuccessMsg(c, "已取消运行中的任务")
	} else if canForce {
		response.SuccessMsg(c, "已强制取消（任务可能已丢失上下文）")
	} else {
		response.SuccessMsg(c, "已取消")
	}
}

// ── List backtest tasks ──

func (h *StrategyHandler) BacktestTasks(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))

	var tasks []model.BacktestTask
	db.MySQL.Where("strategy_id = ? AND user_id = ? AND status != ?", sid, uid, "completed").
		Order("created_at DESC").Limit(50).Find(&tasks)
	response.Success(c, tasks)
}

// DeleteBacktestTask deletes a backtest task and all related child records.
// Cascades: execution_logs, daily_snapshots, and the linked backtest_result.
func (h *StrategyHandler) DeleteBacktestTask(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	// Cancel if running
	if task.Status == "running" {
		rm := getRunningMap(uint(sid))
		if cancel, ok := rm[uint(tid)]; ok {
			cancel()
			delete(rm, uint(tid))
		}
	}

	// Cascade delete child tables
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestExecutionLog{})
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestDailySnapshot{})
	db.MySQL.Where("task_id = ?", tid).Delete(&model.BacktestResult{})

	// Delete the task itself
	db.MySQL.Delete(&task)

	log.Printf("[backtest] task %d deleted by user %d", tid, uid)
	response.SuccessMsg(c, "已删除")
}

// BacktestTaskLogs returns execution logs for a task.
// Supports incremental polling: ?afterSeq=N returns only logs with seq > N.
func (h *StrategyHandler) BacktestTaskLogs(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	afterSeq := c.Query("afterSeq")
	q := db.MySQL.Where("task_id = ?", tid).Order("date ASC, seq ASC")
	if afterSeq != "" {
		q = q.Where("id > ?", afterSeq)
	}

	var logs []model.BacktestExecutionLog
	q.Find(&logs)

	// Return the max log id as cursor for next poll
	maxID := uint(0)
	if len(logs) > 0 {
		maxID = logs[len(logs)-1].ID
	}

	response.Success(c, map[string]interface{}{
		"logs":   logs,
		"cursor": maxID,
		"total":  len(logs),
	})
}

// BacktestTaskSnapshots returns daily snapshots for a task.
// Supports ?limit=N for latest N snapshots (default all).
func (h *StrategyHandler) BacktestTaskSnapshots(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	tid, _ := strconv.Atoi(c.Param("taskId"))

	var task model.BacktestTask
	if db.MySQL.Where("id = ? AND strategy_id = ? AND user_id = ?", tid, sid, uid).First(&task).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}

	limitStr := c.DefaultQuery("limit", "0")
	limit, _ := strconv.Atoi(limitStr)

	q := db.MySQL.Where("task_id = ?", tid).Order("date ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}

	var snapshots []model.BacktestDailySnapshot
	q.Find(&snapshots)

	// Parse positions JSON for each snapshot
	type SnapshotOut struct {
		model.BacktestDailySnapshot
		PositionsData []map[string]interface{} `json:"positionsData"`
	}

	out := make([]SnapshotOut, len(snapshots))
	for i, s := range snapshots {
		out[i] = SnapshotOut{BacktestDailySnapshot: s}
		if s.Positions != "" {
			var pd []map[string]interface{}
			if json.Unmarshal([]byte(s.Positions), &pd) == nil {
				out[i].PositionsData = pd
			}
		}
	}

	response.Success(c, out)
}

// ═══════════════════════════════════════════════════════════════
// K-line cache — preloads close prices to avoid N+1 DB queries
// ═══════════════════════════════════════════════════════════════

type KlineCache struct {
	dates    []string
	dateIdx  map[string]int
	closeMap map[string][]float64 // code -> []close per date (forward-filled)
}

// preloadKline loads all close prices for the given stock codes within date range.
// Forward-fills gaps (suspended stocks keep previous close).
func preloadKline(codes []string, startDate, endDate string) *KlineCache {
	kc := &KlineCache{
		dateIdx:  make(map[string]int),
		closeMap: make(map[string][]float64, len(codes)),
	}

	// 1. Get all trading days
	db.PG.Raw(`SELECT DISTINCT trade_date::text FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ? ORDER BY trade_date`,
		startDate, endDate).Scan(&kc.dates)

	for i, d := range kc.dates {
		kc.dateIdx[d] = i
	}

	if len(kc.dates) == 0 {
		return kc
	}

	// 2. Bulk load close prices in ONE query
	type KCRow struct {
		Code  string
		Date  string
		Close float64
	}
	var rows []KCRow
	db.PG.Raw(`SELECT code, trade_date::text, close FROM stocks_daily_k 
		WHERE code = ANY($1) AND trade_date >= $2 AND trade_date <= $3
		ORDER BY code, trade_date`,
		codes, startDate, endDate).Scan(&rows)

	// 3. Initialize arrays with zeros
	nDays := len(kc.dates)
	for _, c := range codes {
		kc.closeMap[c] = make([]float64, nDays)
	}

	// 4. Fill prices
	for _, r := range rows {
		if idx, ok := kc.dateIdx[r.Date]; ok {
			kc.closeMap[r.Code][idx] = r.Close
		}
	}

	// 5. Forward-fill: if a stock has 0 close on day i, use previous non-zero close
	for _, c := range codes {
		arr := kc.closeMap[c]
		var last float64
		for i := 0; i < nDays; i++ {
			if arr[i] > 0 {
				last = arr[i]
			} else {
				arr[i] = last
			}
		}
	}

	return kc
}

// GetClose returns the close price for a stock on a given date (O(1) lookup).
func (kc *KlineCache) GetClose(code, date string) float64 {
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	idx, ok := kc.dateIdx[date]
	if !ok {
		return 0
	}
	return arr[idx]
}

// checkOp evaluates a comparison between a float value and threshold.
func checkOp(val float64, op string, threshold float64) bool {
	switch op {
	case "gte": return val >= threshold
	case "lte": return val <= threshold
	case "gt": return val > threshold
	case "lt": return val < threshold
	case "eq": return val == threshold
	case "cross_up": return val > 0
	case "cross_down": return val < 0
	}
	return false
}

// getPrevClose returns the close price one trading day before the given date.
func getPrevClose(kc *KlineCache, code, date string) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx == 0 {
		return 0
	}
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	return arr[idx-1]
}

// getCloseNDaysAgo returns the close price N trading days before the given date.
func getCloseNDaysAgo(kc *KlineCache, code, date string, n int) float64 {
	idx, ok := kc.dateIdx[date]
	if !ok || idx < n {
		return 0
	}
	arr, ok := kc.closeMap[code]
	if !ok {
		return 0
	}
	return arr[idx-n]
}

// ── Indicator batch preloader ──


// IndicatorValue holds a preloaded indicator value for a stock on a date.
type IndicatorValue struct {
	Code  string
	Date  string
	Value float64
}

// IndicatorCache stores preloaded indicator values: map[indicatorName]map[code|date]value
type IndicatorCache struct {
	data map[string]map[string]float64 // key: indicator, inner key: "code|date"
}

func newIndicatorCache() *IndicatorCache {
	return &IndicatorCache{data: make(map[string]map[string]float64)}
}

func (ic *IndicatorCache) set(indicator, code, date string, val float64) {
	key := indicator
	if _, ok := ic.data[key]; !ok {
		ic.data[key] = make(map[string]float64)
	}
	ic.data[key][code+"|"+date] = val
}

func (ic *IndicatorCache) get(indicator, code, date string) (float64, bool) {
	m, ok := ic.data[indicator]
	if !ok {
		return 0, false
	}
	v, ok := m[code+"|"+date]
	return v, ok
}

// batchScan runs a query and populates the cache for a given indicator.
func (ic *IndicatorCache) batchScan(indicator string, query string, args ...interface{}) {
	var rows []IndicatorValue
	if err := db.PG.Raw(query, args...).Scan(&rows).Error; err != nil {
		log.Printf("[backtest] batch scan %s failed: %v", indicator, err)
		return
	}
	for _, r := range rows {
		ic.set(indicator, r.Code, r.Date, r.Value)
	}
}

// preloadIndicators batch-loads all indicator values needed by the strategy for the given universe.
func preloadIndicators(conds []model.StrategyCondition, codes []string, startDate, endDate string, kcache *KlineCache) *IndicatorCache {
	cache := newIndicatorCache()

	// Collect unique indicator names to preload
	needPreload := make(map[string]bool)
	for _, c := range conds {
		ind := c.Indicator
		// Skip indicators we can compute from close prices in Go
		switch ind {
		case "daily_change", "momentum_5", "momentum_20":
			continue // computed from kcache.GetClose
		}
		// Skip indicators from other tables (they're fast enough with simple queries)
		switch {
		case strings.HasPrefix(ind, "ai_"):
			continue
		case ind == "streak_count", ind == "algo_score", ind == "signal_value":
			continue
		case ind == "pe", ind == "pb", ind == "ps", ind == "pe_percentile", ind == "pb_percentile", ind == "total_market_cap":
			continue
		case ind == "roe", ind == "revenue_growth", ind == "profit_growth", ind == "gross_margin", ind == "net_margin", ind == "debt_ratio", ind == "eps":
			continue
		case ind == "shareholder_change", ind == "inst_hold_ratio":
			continue
		}
		needPreload[ind] = true
	}

	// Preload volume data for volume-related indicators
	needVolume := false
	for ind := range needPreload {
		if strings.HasPrefix(ind, "volume") || ind == "turnover_rate" || ind == "mfi" {
			needVolume = true
			break
		}
	}
	_ = needVolume // we'll handle volume inline

	// Batch preload: DMI/ADX (most expensive, preload all three at once)
	if needPreload["dmi_plus"] || needPreload["dmi_minus"] || needPreload["adx"] {
		log.Printf("[backtest] batch preloading DMI/ADX for %d stocks...", len(codes))
		cache.batchScan("dmi_plus",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k
				WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low,
						ABS(high-prev_close),
						ABS(low-prev_close)
					) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(up_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(down_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			), dmi AS (
				SELECT code, date,
					CASE WHEN avg_tr > 0 THEN avg_up/avg_tr*100 ELSE 0 END as pdi,
					CASE WHEN avg_tr > 0 THEN avg_down/avg_tr*100 ELSE 0 END as mdi
				FROM dmi14
			), adx_calc AS (
				SELECT code, date, pdi, mdi,
					CASE WHEN pdi+mdi > 0 THEN ABS(pdi-mdi)/(pdi+mdi)*100 ELSE 0 END as dx
				FROM dmi
			)
			SELECT code, date, 
				AVG(dx) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as value
			FROM adx_calc`,
			codes, startDate, endDate)
		delete(needPreload, "adx")

		cache.batchScan("dmi_minus",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(up_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(down_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			)
			SELECT code, date,
				CASE WHEN avg_tr > 0 THEN avg_down/avg_tr*100 ELSE 0 END as value
			FROM dmi14`,
			codes, startDate, endDate)
		delete(needPreload, "dmi_minus")

		cache.batchScan("dmi_plus",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), tr_calc AS (
				SELECT code, date,
					GREATEST(high - LAG(high) OVER (PARTITION BY code ORDER BY date), 0) as up_move,
					GREATEST(LAG(low) OVER (PARTITION BY code ORDER BY date) - low, 0) as down_move,
					GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)) as tr
				FROM klines
			), dmi14 AS (
				SELECT code, date,
					AVG(up_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_up,
					AVG(down_move) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_down,
					AVG(tr) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_tr
				FROM tr_calc
			)
			SELECT code, date,
				CASE WHEN avg_tr > 0 THEN avg_up/avg_tr*100 ELSE 0 END as value
			FROM dmi14`,
			codes, startDate, endDate)
		delete(needPreload, "dmi_plus")
	}

	// Batch preload: RSI
	if needPreload["rsi"] {
		log.Printf("[backtest] batch preloading RSI for %d stocks...", len(codes))
		cache.batchScan("rsi",
			`WITH klines AS (
				SELECT code, trade_date::text as date, close,
					close - LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as chg
				FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), gains AS (
				SELECT code, date,
					AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_gain,
					AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as avg_loss
				FROM klines
			)
			SELECT code, date,
				CASE WHEN avg_loss > 0 THEN 100 - 100/(1 + avg_gain/NULLIF(avg_loss,0)) ELSE 100 END as value
			FROM gains`,
			codes, startDate, endDate)
		delete(needPreload, "rsi")
	}

	// Batch preload: MACD
	if needPreload["macd"] {
		log.Printf("[backtest] batch preloading MACD for %d stocks...", len(codes))
		cache.batchScan("macd",
			`WITH klines AS (
				SELECT code, trade_date::text as date, close FROM stocks_daily_k
				WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), ema AS (
				SELECT code, date,
					AVG(close) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) as ema12,
					AVG(close) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 25 PRECEDING AND CURRENT ROW) as ema26
				FROM klines
			), macd_calc AS (
				SELECT code, date,
					ema12 - ema26 as dif,
					AVG(ema12 - ema26) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as dea
				FROM ema
			)
			SELECT code, date, dif - dea as value FROM macd_calc`,
			codes, startDate, endDate)
		delete(needPreload, "macd")
	}

	// Batch preload: KDJ (preload K, D, J separately)
	if needPreload["kdj_k"] || needPreload["kdj_d"] || needPreload["kdj_j"] {
		log.Printf("[backtest] batch preloading KDJ for %d stocks...", len(codes))
		cache.batchScan("kdj_k",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close FROM stocks_daily_k
				WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), rsv AS (
				SELECT code, date,
					CASE WHEN MAX(high) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) -
						MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) > 0
					THEN (close - MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW)) /
						(MAX(high) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) -
						MIN(low) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 8 PRECEDING AND CURRENT ROW)) * 100
					ELSE 50 END as rsv_val
				FROM klines
			)
			SELECT code, date,
				(0.6667 * COALESCE(LAG(rsv_val) OVER (PARTITION BY code ORDER BY date), 50) + 0.3333 * rsv_val) + 
				COALESCE(LAG((0.6667 * COALESCE(LAG(rsv_val) OVER (PARTITION BY code ORDER BY date), 50) + 0.3333 * rsv_val)) OVER (PARTITION BY code ORDER BY date), 50) * 0.3333
				as value FROM rsv`,
			codes, startDate, endDate)
		// For simplicity, preload K only for now; D/J are similar
		delete(needPreload, "kdj_k")
		delete(needPreload, "kdj_d")
		delete(needPreload, "kdj_j")
	}

	// Batch preload: simple volume-related from stocks_daily_k
	if needPreload["volume_ratio"] || needPreload["volume_ma_ratio"] {
		log.Printf("[backtest] batch preloading volume data for %d stocks...", len(codes))
		cache.batchScan("volume_ratio",
			`SELECT code, trade_date::text as date, 
				COALESCE(volume / NULLIF(AVG(volume) OVER (PARTITION BY code ORDER BY trade_date ROWS BETWEEN 4 PRECEDING AND 1 PRECEDING), 0), 0) as value
			FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3`,
			codes, startDate, endDate)
		delete(needPreload, "volume_ratio")
	}

	// Batch preload: turnover_rate if available
	if needPreload["turnover_rate"] {
		log.Printf("[backtest] batch preloading turnover_rate for %d stocks...", len(codes))
		cache.batchScan("turnover_rate",
			`SELECT code, trade_date::text as date, COALESCE(turnover_rate, 0) as value
			FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3`,
			codes, startDate, endDate)
		delete(needPreload, "turnover_rate")
	}

	// Batch preload: ATR
	if needPreload["atr"] || needPreload["atr_pct"] {
		log.Printf("[backtest] batch preloading ATR for %d stocks...", len(codes))
		cache.batchScan("atr",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close,
					LAG(close) OVER (PARTITION BY code ORDER BY trade_date) as prev_close
				FROM stocks_daily_k WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			)
			SELECT code, date,
				AVG(GREATEST(high-low, ABS(high-prev_close), ABS(low-prev_close)))
					OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 13 PRECEDING AND CURRENT ROW) as value
			FROM klines`,
			codes, startDate, endDate)
		delete(needPreload, "atr")
	}

	// Batch preload: CCI
	if needPreload["cci"] {
		log.Printf("[backtest] batch preloading CCI for %d stocks...", len(codes))
		cache.batchScan("cci",
			`WITH klines AS (
				SELECT code, trade_date::text as date, high, low, close FROM stocks_daily_k
				WHERE code = ANY($1) AND trade_date BETWEEN $2 AND $3
			), tp AS (
				SELECT code, date, (high+low+close)/3 as typical FROM klines
			)
			SELECT code, date,
				(typical - AVG(typical) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)) /
				NULLIF(0.015 * AVG(ABS(typical - AVG(typical) OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW)))
					OVER (PARTITION BY code ORDER BY date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW), 0) as value
			FROM tp`,
			codes, startDate, endDate)
		delete(needPreload, "cci")
	}

	if len(needPreload) > 0 {
		keys := make([]string, 0, len(needPreload))
		for k := range needPreload {
			keys = append(keys, k)
		}
		log.Printf("[backtest] unbatched indicators (fallback to per-stock): %v", keys)
	}

	return cache
}


// ── The async backtest runner (runs in goroutine) ──

func (h *StrategyHandler) runBacktestAsync(ctx context.Context, task *model.BacktestTask, s *model.Strategy, startDate, endDate string, stockCodes []string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[backtest] PANIC in task %d: %v", task.ID, r)
			db.MySQL.Model(task).Updates(map[string]interface{}{
				"status": "failed",
				"phase":  "回测异常",
				"error_msg": fmt.Sprintf("panic: %v", r),
			})
		}
	}()
	// Mark as running
	now := time.Now()
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"status":     "running",
		"phase":      "正在初始化...",
		"started_at": now,
	})

	updateProgress := func(day, total int, phase string, positions string) {
		pct := 0.0
		if total > 0 {
			pct = float64(day) / float64(total) * 100
		}
		updates := map[string]interface{}{
			"current_day": day,
			"progress_pct": pct,
		}
		if phase != "" {
			updates["phase"] = phase
		}
		if positions != "" {
			updates["current_positions"] = positions
		}
		db.MySQL.Model(task).Updates(updates)
	}

	// Load conditions
	var conds []model.StrategyCondition
	db.MySQL.Where("strategy_id = ?", task.StrategyID).Find(&conds)
	buyConds := filterConds(conds, "buy")
	addConds := filterConds(conds, "add")
	sellConds := filterConds(conds, "sell")
	reduceConds := filterConds(conds, "reduce")

	buyPct := s.BuyPositionPct
	if buyPct <= 0 { buyPct = 15 }
	addPct := s.AddPositionPct
	if addPct <= 0 { addPct = 10 }
	reducePct := s.ReducePositionPct
	if reducePct <= 0 { reducePct = 50 }

	capital := s.InitialCapital
	if capital <= 0 { capital = 100000 }
	remainingCash := capital
	maxHold := s.MaxHoldings

	// Record initial capital on task
	db.MySQL.Model(task).Update("initial_capital", capital)
	if maxHold <= 0 { maxHold = 20 }

	updateProgress(0, task.TotalDays, fmt.Sprintf("初始资金: ¥%.0f | 最大持股: %d", capital, maxHold), "")

	// Determine stock universe
	type StockInfo struct {
		Code string
		Name string
	}
	var universe []StockInfo
	if len(stockCodes) > 0 {
		db.PG.Raw("SELECT code, COALESCE(name,'') as name FROM stocks_basic WHERE code = ANY($1)", stockCodes).Scan(&universe)
	} else {
		// Stock pool "all" — sample up to 3000 stocks for performance
		db.PG.Raw(`SELECT DISTINCT k.code, COALESCE(s.name, k.code) as name 
			FROM stocks_daily_k k
			LEFT JOIN stocks_basic s ON s.code = k.code
			WHERE k.trade_date >= ? AND k.trade_date <= ? ORDER BY RANDOM() LIMIT 3000`,
			startDate, endDate).Scan(&universe)
	}

	if len(universe) == 0 {
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":  "failed",
			"phase":   "无可用股票数据",
			"error_msg": "所选时间段无可用股票数据",
		})
		return
	}

	// Get all stock codes for preload
	universeCodes := make([]string, len(universe))
	for i, s := range universe {
		universeCodes[i] = s.Code
	}

	// Preload K-line cache (one query instead of N+1)
	kcache := preloadKline(universeCodes, startDate, endDate)
	allDates := kcache.dates

	// Preload indicator values in batch (one query per indicator instead of N+1 per stock)
	icache := preloadIndicators(conds, universeCodes, startDate, endDate, kcache)

	// Local evaluateSingleCondition that uses the preloaded cache
	evalSingle := func(cond model.StrategyCondition, code, date string) bool {
		ind := cond.Indicator
		// Try cache first
		if val, ok := icache.get(ind, code, date); ok {
			return checkOp(val, cond.Operator, cond.Value)
		}
		// Compute from close cache for simple momentum indicators
		switch ind {
		case "daily_change":
			cur := kcache.GetClose(code, date)
			prev := getPrevClose(kcache, code, date)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		case "momentum_5":
			cur := kcache.GetClose(code, date)
			prev := getCloseNDaysAgo(kcache, code, date, 5)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		case "momentum_20":
			cur := kcache.GetClose(code, date)
			prev := getCloseNDaysAgo(kcache, code, date, 20)
			if prev > 0 {
				return checkOp((cur-prev)/prev*100, cond.Operator, cond.Value)
			}
			return false
		}
		// Fallback to original per-stock SQL query
		return evaluateSingleCondition(cond, code, date)
	}

	// Local evaluateConditions that uses the cached evalSingle
	evalConds := func(conds_ []model.StrategyCondition, code, date string) bool {
		if len(conds_) == 0 { return false }
		groups := make(map[int][]model.StrategyCondition)
		for _, c := range conds_ {
			groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
		}
		for _, groupConds := range groups {
			allMet := true
			for _, c := range groupConds {
				if !evalSingle(c, code, date) {
					allMet = false
					break
				}
			}
			if allMet { return true }
		}
		return false
	}
	// evalSingleWithDetail returns (passed bool, detail string) for diagnostic logging.
	evalSingleWithDetail := func(cond model.StrategyCondition, code, date string) (bool, string) {
		ind := cond.Indicator
		var val float64
		found := false
		if v, ok := icache.get(ind, code, date); ok {
			val = v
			found = true
		} else {
			switch ind {
			case "daily_change":
				cur := kcache.GetClose(code, date)
				prev := getPrevClose(kcache, code, date)
				if prev > 0 { val = (cur - prev) / prev * 100; found = true }
			case "momentum_5":
				cur := kcache.GetClose(code, date)
				prev := getCloseNDaysAgo(kcache, code, date, 5)
				if prev > 0 { val = (cur - prev) / prev * 100; found = true }
			case "momentum_20":
				cur := kcache.GetClose(code, date)
				prev := getCloseNDaysAgo(kcache, code, date, 20)
				if prev > 0 { val = (cur - prev) / prev * 100; found = true }
			default:
				val = getIndicatorValue(cond, code, date)
				found = true
			}
		}
		_ = found
		passed := checkOp(val, cond.Operator, cond.Value)
		status := "✓"
		if !passed { status = "✗" }
		return passed, fmt.Sprintf("%s %s=%.2f op=%s thr=%.2f", status, ind, val, cond.Operator, cond.Value)
	}

	_ = evalSingle
	_ = evalConds

	if len(allDates) == 0 {
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":  "failed",
			"phase":   "无交易日数据",
			"error_msg": "所选时间段无交易日数据",
		})
		return
	}

	totalDays := len(allDates)
	db.MySQL.Model(task).Update("total_days", totalDays)
	updateProgress(0, totalDays, fmt.Sprintf("回测区间: %s ~ %s | 共 %d 个交易日", startDate, endDate, totalDays), "")

	// Regular investment schedule
	regDates := make(map[string]bool)
	if s.InvestmentType == "regular" && s.RegularAmount > 0 && s.RegularInterval != "" {
		lastReg := ""
		for _, d := range allDates {
			add := false
			switch s.RegularInterval {
			case "daily": add = true
			case "weekly": add = lastReg == "" || d > lastReg
			case "monthly": add = lastReg == "" || d[:7] != lastReg[:7]
			}
			if add { regDates[d] = true; lastReg = d }
		}
	}

	type Position struct {
		Code     string  `json:"code"`
		Name     string  `json:"name"`
		BuyPrice float64 `json:"buyPrice"`
		Quantity int     `json:"quantity"`
		BuyDate  string  `json:"buyDate"`
	}
	positions := make(map[string]*Position)
	var allTrades []backtestTrade
	var equityPoints []map[string]interface{}
	prevDayEquity := capital

	for di, date := range allDates {
		// Day start log
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 0,
			"system", "info", "", "",
			fmt.Sprintf("━━━ 第%d天 %s 持仓%d只 现金¥%.0f ━━━", di+1, date, len(positions), remainingCash),
			nil)

		// Check cancellation
		select {
		case <-ctx.Done():
			db.MySQL.Model(task).Updates(map[string]interface{}{
				"status": "cancelled",
				"phase":  "已取消",
			})
			return
		default:
		}

		if regDates[date] {
			remainingCash += s.RegularAmount
		}

		// Check sell/reduce + stop
		sellTriggered := 0
		reduceTriggered := 0
		origPosCount := len(positions)
		// Sort position codes for deterministic iteration
		sortedPosCodes := make([]string, 0, len(positions))
		for code := range positions {
			sortedPosCodes = append(sortedPosCodes, code)
		}
		sort.Strings(sortedPosCodes)
		for _, code := range sortedPosCodes {
			pos, exists := positions[code]
			if !exists { continue }
			price := kcache.GetClose(code, date)
			if price <= 0 { continue }

			triggered := ""
			if s.StopLoss > 0 && price <= pos.BuyPrice*(1-s.StopLoss/100) {
				triggered = "止损"
			} else if s.StopProfit > 0 && price >= pos.BuyPrice*(1+s.StopProfit/100) {
				triggered = "止盈"
			} else if evalConds(sellConds, code, date) {
				triggered = "卖出条件"
			}

			if triggered != "" {
				pnl := (price - pos.BuyPrice) * float64(pos.Quantity)
				pnlPct := (price - pos.BuyPrice) / pos.BuyPrice * 100
				remainingCash += price * float64(pos.Quantity)
				t := backtestTrade{
					Date: date, Code: code, Name: pos.Name, Action: "sell",
					Price: price, Quantity: pos.Quantity, Reason: triggered,
					Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
				}
				allTrades = append(allTrades, t)
				sellTriggered++
				delete(positions, code)
				continue
			}

			if evalConds(reduceConds, code, date) {
				reduceQty := int(float64(pos.Quantity) * reducePct / 100)
				if reduceQty >= 100 && reduceQty < pos.Quantity {
					pnl := (price - pos.BuyPrice) * float64(reduceQty)
					pnlPct := (price - pos.BuyPrice) / pos.BuyPrice * 100
					remainingCash += price * float64(reduceQty)
					pos.Quantity -= reduceQty
					t := backtestTrade{
						Date: date, Code: code, Name: pos.Name, Action: "reduce",
						Price: price, Quantity: reduceQty, Reason: "减仓条件触发",
						Pnl: math.Round(pnl*100) / 100, PnlPct: math.Round(pnlPct*100) / 100,
					}
					allTrades = append(allTrades, t)
					reduceTriggered++
				}
			}
		}

		// Sell/reduce scan summary (seq=1, before trades)
		hasSellConds := len(sellConds) > 0
		hasReduceConds := len(reduceConds) > 0
		hasStop := s.StopProfit > 0 || s.StopLoss < 0
		if origPosCount > 0 {
			parts := []string{}
			if hasStop { parts = append(parts, fmt.Sprintf("止盈%.0f%%/止损%.0f%%", s.StopProfit, s.StopLoss)) }
			if hasSellConds { parts = append(parts, "卖出条件") }
			if hasReduceConds { parts = append(parts, "减仓条件") }
			if sellTriggered > 0 || reduceTriggered > 0 {
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 11,
					"condition_eval", "info", "", "",
					fmt.Sprintf("卖出检查: %d只持仓 → 卖出%d只, 减仓%d只", origPosCount, sellTriggered, reduceTriggered),
					nil)
			} else {
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 11,
					"condition_eval", "info", "", "",
					fmt.Sprintf("卖出检查: %d只持仓, %s → 无触发", origPosCount, strings.Join(parts, "+")),
					nil)
			}
		} else {
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 11,
				"condition_eval", "info", "", "",
				"卖出检查: 无持仓, 跳过",
				nil)
		}

		// Check buy + add (respect max holdings)
		buyHitCount := 0
		addHitCount := 0
		if len(positions) < maxHold {
			for _, stock := range universe {
				code := stock.Code
				price := kcache.GetClose(code, date)
				if price <= 0 { continue }

				if pos, exists := positions[code]; exists {
					if len(addConds) > 0 && evalConds(addConds, code, date) {
						addQty := int(remainingCash * addPct / 100 / price)
						// Round to 100-share lots (A-share rule)
						addQty = (addQty / 100) * 100
						if addQty >= 100 {
							cost := price * float64(addQty)
							if cost > remainingCash {
								addQty = (int(remainingCash/price) / 100) * 100
								cost = price * float64(addQty)
							}
							if addQty >= 100 {
								remainingCash -= cost
								pos.Quantity += addQty
								t := backtestTrade{
									Date: date, Code: code, Name: pos.Name, Action: "add",
									Price: price, Quantity: addQty, Reason: "加仓条件触发",
								}
								allTrades = append(allTrades, t)
								addHitCount++
							}
						}
					}
					continue
				}

				if len(buyConds) > 0 && evalConds(buyConds, code, date) {
					buyQty := int(remainingCash * buyPct / 100 / price)
					// Round to 100-share lots (A-share minimum trading unit)
					buyQty = (buyQty / 100) * 100
					if buyQty < 100 { buyQty = 100 }
					cost := price * float64(buyQty)
					if cost > remainingCash {
						buyQty = (int(remainingCash/price) / 100) * 100
						cost = price * float64(buyQty)
					}
					if buyQty < 100 { continue }

					remainingCash -= cost
					name := stock.Name
					if name == "" { name = code }
					positions[code] = &Position{
						Code: code, Name: name,
						BuyPrice: price, Quantity: buyQty, BuyDate: date,
					}
					t := backtestTrade{
						Date: date, Code: code, Name: name, Action: "buy",
						Price: price, Quantity: buyQty, Reason: "买入条件触发",
					}
					allTrades = append(allTrades, t)
					buyHitCount++

					// Re-check max holdings after each buy
					if len(positions) >= maxHold {
						break
					}
				}
			}
		}

		// Buy/add scan summary
		if len(buyConds) > 0 || len(addConds) > 0 {
			// Build condition descriptions
			condParts := []string{}
			for _, c := range buyConds {
				condParts = append(condParts, fmt.Sprintf("%s %s %.0f", c.Indicator, c.Operator, c.Value))
			}
			for _, c := range addConds {
				condParts = append(condParts, fmt.Sprintf("加仓:%s %s %.0f", c.Indicator, c.Operator, c.Value))
			}
			condDesc := strings.Join(condParts, ", ")
			if len(positions) >= maxHold && maxHold > 0 {
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 21,
					"condition_eval", "info", "", "",
					fmt.Sprintf("买入扫描: 已达最大持仓%d/%d, 跳过扫描", len(positions), maxHold),
					nil)
			} else if buyHitCount > 0 || addHitCount > 0 {
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 21,
					"condition_eval", "info", "", "",
					fmt.Sprintf("买入扫描: 遍历%d只股票, 条件[%s] → 命中买入%d只, 加仓%d只, 当前持仓%d/%d",
						len(universe), condDesc, buyHitCount, addHitCount, len(positions), maxHold),
					nil)
			} else {
				// Summary log
				insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 21,
					"condition_eval", "warn", "", "",
					fmt.Sprintf("买入扫描: 遍历%d只股票, 条件[%s] → 无满足买入条件的股票, 当前持仓%d/%d",
						len(universe), condDesc, len(positions), maxHold),
					nil)
				// Per-stock diagnostic: emit individual condition_eval lines for small universes
				maxDetail := 8
				if len(universe) <= 10 { maxDetail = len(universe) }
				log.Printf("[backtest] task=%d date=%s emitting per-stock diag for %d stocks", task.ID, date, maxDetail)
				diagSeq := 30
				for si, stock := range universe {
					if si >= maxDetail { break }
					code := stock.Code
					price := kcache.GetClose(code, date)
					if price <= 0 { continue }
					condResults := []string{}
					allCondResults := []map[string]interface{}{}
					for _, c := range buyConds {
						passed, reason := evalSingleWithDetail(c, code, date)
						condResults = append(condResults, reason)
						allCondResults = append(allCondResults, map[string]interface{}{
							"indicator": c.Indicator, "op": c.Operator,
							"threshold": c.Value, "passed": passed, "detail": reason,
						})
					}
					insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, diagSeq,
						"condition_eval", "info", code, stock.Name,
						fmt.Sprintf("  %s ¥%.2f → %s", code, price, strings.Join(condResults, " | ")),
						map[string]interface{}{"conditions": allCondResults})
					diagSeq++
				}
			}
		} else {
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 21,
				"condition_eval", "info", "", "",
				"买入扫描: 无买入/加仓条件, 跳过",
				nil)
		}

		// Update progress every trading day
		posList := make([]map[string]interface{}, 0)
		totalEquity := remainingCash
		for _, pos := range positions {
			cp := kcache.GetClose(pos.Code, date)
			mv := cp * float64(pos.Quantity)
			pnl := (cp - pos.BuyPrice) * float64(pos.Quantity)
			pnlPct := 0.0
			if pos.BuyPrice > 0 { pnlPct = (cp - pos.BuyPrice) / pos.BuyPrice * 100 }
			posList = append(posList, map[string]interface{}{
				"code": pos.Code, "name": pos.Name, "qty": pos.Quantity,
				"price": cp, "costPrice": pos.BuyPrice,
				"marketVal": math.Round(mv*100)/100,
				"pnl": math.Round(pnl*100)/100, "pnlPct": math.Round(pnlPct*100)/100,
			})
			totalEquity += mv
		}

		// Collect trades that happened today (since last snapshot)
		todayTrades := make([]backtestTrade, 0)
		tradeCountSoFar := 0
		for _, t := range allTrades {
			if t.Date == date {
				todayTrades = append(todayTrades, t)
			}
			tradeCountSoFar++
		}

		posData := map[string]interface{}{
			"date": date, "day": di+1, "totalDays": totalDays,
			"cash": math.Round(remainingCash*100)/100,
			"totalEquity": math.Round(totalEquity*100)/100,
			"totalReturn": math.Round((totalEquity-capital)/capital*10000)/100,
			"positions": posList,
			"positionCount": len(positions),
			"recentTrades": todayTrades,
		}
		posBytes, _ := json.Marshal(posData)
		updateProgress(di+1, totalDays, "", string(posBytes))

		// Equity point
		equityPoints = append(equityPoints, map[string]interface{}{
			"date": date, "equity": math.Round(totalEquity*100) / 100,
		})

		// ── Insert daily snapshot ──
		dailyRet := 0.0
		if di > 0 {
			prevEquity := capital
			if len(equityPoints) >= 2 {
				prevEquity = equityPoints[len(equityPoints)-2]["equity"].(float64)
			}
			if prevEquity > 0 {
				dailyRet = (totalEquity - prevEquity) / prevEquity * 100
			}
		}
		cumRet := (totalEquity - capital) / capital * 100
		peak := capital
		for _, eq := range equityPoints {
			e := eq["equity"].(float64)
			if e > peak { peak = e }
		}
		currentDD := 0.0
		if peak > 0 { currentDD = (peak - totalEquity) / peak * 100 }
		insertDailySnapshot(task.ID, task.StrategyID, task.UserID, date, di+1,
			remainingCash, totalEquity, dailyRet, cumRet, currentDD, len(positions), posList)

		// ── Log today's trades ──
		logSeq := 100
		for _, t := range todayTrades {
			detail := map[string]interface{}{
				"action": t.Action, "price": t.Price, "quantity": t.Quantity,
				"reason": t.Reason, "pnl": t.Pnl, "pnlPct": t.PnlPct,
			}
			msg := fmt.Sprintf("📌 [%s] %s %s %d股 @¥%.2f %s",
				t.Action, t.Code, t.Name, t.Quantity, t.Price, t.Reason)
			insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, logSeq,
				"trade", "info", t.Code, t.Name, msg, detail)
			logSeq++
		}

		// Day end summary
		dailyPnl := totalEquity - prevDayEquity
		prevDayEquity = totalEquity
		insertBacktestLog(task.ID, task.StrategyID, task.UserID, date, 999,
			"system", "info", "", "",
			fmt.Sprintf("第%d天结束: 权益¥%.0f 日盈亏%+.0f 累计%+.1f%% 持仓%d只", di+1, totalEquity, dailyPnl, cumRet, len(positions)),
			nil)
	}

	// Calculate final metrics
	winCount := 0
	for _, t := range allTrades {
		if (t.Action == "sell" || t.Action == "reduce") && t.Pnl > 0 { winCount++ }
	}
	sellCount := 0
	for _, t := range allTrades {
		if t.Action == "sell" || t.Action == "reduce" { sellCount++ }
	}
	winRate := 0.0
	if sellCount > 0 { winRate = float64(winCount) / float64(sellCount) * 100 }

	finalEquity := equityPoints[len(equityPoints)-1]["equity"].(float64)
	totalReturn := (finalEquity - capital) / capital * 100

	peak := capital
	maxDD := 0.0
	for _, eq := range equityPoints {
		e := eq["equity"].(float64)
		if e > peak { peak = e }
		dd := (peak - e) / peak * 100
		if dd > maxDD { maxDD = dd }
	}

	sharpe := 0.0
	if totalReturn > 0 { sharpe = (totalReturn / 100) / (math.Sqrt(float64(totalDays)) * 0.15) * math.Sqrt(252) }

	// Compute indicator coverage
	usedIndicators := make(map[string]bool)
	allConds := append(append(append(buyConds, addConds...), sellConds...), reduceConds...)
	for _, c := range allConds {
		usedIndicators[c.Indicator] = true
	}
	var coverageStats []map[string]interface{}
	klineSafe := 0
	klineUnsafe := 0
	for ind := range usedIndicators {
		isKlineDerived := strings.HasPrefix(ind, "daily_change") || strings.HasPrefix(ind, "momentum") ||
			ind == "ma_deviation" || ind == "ma_cross" || ind == "macd" || ind == "ema_cross" ||
			ind == "rsi" || strings.HasPrefix(ind, "kdj") || strings.HasPrefix(ind, "boll") ||
			strings.HasPrefix(ind, "volume") || ind == "turnover_rate" || ind == "atr" ||
			strings.HasPrefix(ind, "drawdown") || strings.HasPrefix(ind, "new_high") ||
			ind == "up_days_ratio" || strings.HasPrefix(ind, "price_position") ||
			ind == "adx" || strings.HasPrefix(ind, "dmi_") ||
			ind == "cci" || ind == "williams_r" || ind == "mfi" ||
			ind == "atr_pct" || ind == "ma_convergence" || ind == "trend_strength" ||
			ind == "consecutive_days" || ind == "gap_pct" || ind == "high_low_range" ||
			ind == "vwap_deviation" || ind == "index_relative"
		if isKlineDerived {
			klineSafe++
		} else {
			klineUnsafe++
		}
		coverageStats = append(coverageStats, map[string]interface{}{
			"indicator": ind,
			"klineSafe": isKlineDerived,
		})
	}
	coveragePct := 0.0
	if len(usedIndicators) > 0 {
		coveragePct = float64(klineSafe) / float64(len(usedIndicators)) * 100
	}

	// Determine pool key (short enum) from task params
	poolKey := "all"
	if task.Params != "" {
		var params map[string]interface{}
		if json.Unmarshal([]byte(task.Params), &params) == nil {
			if pool, ok := params["stockPool"].(string); ok && pool != "" {
				poolKey = pool
			}
		}
	}

	// Build stock pool params for replay
	poolParamsBytes, _ := json.Marshal(universeCodes)
	poolParamsJSON := string(poolParamsBytes)

	// Save result
	bt := model.BacktestResult{
		TaskID:          task.ID,
		UserID:          task.UserID,
		StrategyID:      task.StrategyID,
		StockPool:       poolKey,
		StockPoolParams: poolParamsJSON,
		StartDate:       parseDate(startDate),
		EndDate:         parseDate(endDate),
		InitialCapital:  capital,
		FinalEquity:     math.Round(finalEquity*100) / 100,
		TotalReturn:     math.Round(totalReturn*100) / 100,
		SharpeRatio:     math.Round(sharpe*100) / 100,
		MaxDrawdown:     math.Round(maxDD*100) / 100,
		WinRate:         math.Round(winRate*100) / 100,
		TradeCount:      sellCount, // completed (sell+reduce), not all actions
		Trades:          model.JSONMap{"data": allTrades},
		EquityCurve:     model.JSONMap{"data": equityPoints},
		Coverage:        model.JSONMap{"stats": coverageStats, "klineSafe": klineSafe, "klineUnsafe": klineUnsafe, "coveragePct": coveragePct},
	}
	if err := db.MySQL.Create(&bt).Error; err != nil {
		log.Printf("[backtest] failed to save result for task %d: %v", task.ID, err)
		db.MySQL.Model(task).Updates(map[string]interface{}{
			"status":    "failed",
			"phase":     "保存结果失败",
			"error_msg": fmt.Sprintf("保存结果失败: %v", err),
		})
		rm := getRunningMap(task.StrategyID)
		delete(rm, task.ID)
		return
	}

	// Final system log
	insertBacktestLog(task.ID, task.StrategyID, task.UserID, "", 0,
		"system", "info", "", "",
		fmt.Sprintf("回测完成: 收益率%.2f%%, 夏普%.2f, 最大回撤%.2f%%, 胜率%.2f%%, 交易%d次",
			totalReturn, sharpe, maxDD, winRate, len(allTrades)),
		map[string]interface{}{
			"totalReturn": math.Round(totalReturn*100)/100,
			"sharpe": math.Round(sharpe*100)/100,
			"maxDrawdown": math.Round(maxDD*100)/100,
			"winRate": math.Round(winRate*100)/100,
			"tradeCount": len(allTrades),
		})

	now2 := time.Now()
	db.MySQL.Model(task).Updates(map[string]interface{}{
		"status":        "completed",
		"phase":         "回测完成",
		"result_id":     bt.ID,
		"final_equity":  math.Round(finalEquity*100) / 100,
		"total_return":  math.Round(totalReturn*100) / 100,
		"progress_pct":  100,
		"completed_at":  now2,
	})

	// Clean up running map
	rm := getRunningMap(task.StrategyID)
	delete(rm, task.ID)
}

// ── Legacy SSE backtest (kept for backward compat, delegates to async) ──

func (h *StrategyHandler) RunBacktest(c *gin.Context) {
	uid := getUID(c)
	sid, _ := strconv.Atoi(c.Param("id"))
	var body struct {
		StartDate  string   `json:"startDate"`
		EndDate    string   `json:"endDate"`
		StockCodes []string `json:"stockCodes"`
		StockPool  string   `json:"stockPool"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// SSE headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, 500, response.CodeInternalError, "不支持流式输出")
		return
	}

	sendSSE := func(typ string, data interface{}) {
		b, _ := json.Marshal(data)
		fmt.Fprintf(c.Writer, "data: {\"type\":\"%s\",\"payload\":%s}\n\n", typ, string(b))
		flusher.Flush()
	}

	// Load strategy
	var s model.Strategy
	if db.MySQL.Where("id = ? AND user_id = ?", sid, uid).First(&s).Error != nil {
		sendSSE("error", map[string]string{"message": "策略不存在"})
		return
	}

	// Count trading days
	var totalDays int
	db.PG.Raw(`SELECT COUNT(DISTINCT trade_date) FROM stocks_daily_k 
		WHERE trade_date >= ? AND trade_date <= ?`, body.StartDate, body.EndDate).Scan(&totalDays)
	if totalDays == 0 {
		sendSSE("error", map[string]string{"message": "所选时间段无交易日数据"})
		return
	}

	resolvedCodes := resolveStockPool(uid, body.StockPool, body.StockCodes)

	// Create task
	paramsBytes, _ := json.Marshal(map[string]interface{}{
		"startDate":  body.StartDate,
		"endDate":    body.EndDate,
		"stockCodes": resolvedCodes,
		"stockPool":  body.StockPool,
	})
	task := model.BacktestTask{
		UserID:     uid,
		StrategyID: uint(sid),
		Status:     "pending",
		Phase:      "初始化",
		TotalDays:  totalDays,
		Params:     string(paramsBytes),
	}
	db.MySQL.Create(&task)

	sendSSE("taskId", map[string]interface{}{"taskId": task.ID})

	// Run inline synchronously
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	getRunningMap(uint(sid))[task.ID] = cancel

	h.runBacktestAsync(ctx, &task, &s, body.StartDate, body.EndDate, resolvedCodes)

	// Reload task for final status
	db.MySQL.First(&task, task.ID)
	if task.Status == "completed" && task.ResultID != nil {
		var bt model.BacktestResult
		db.MySQL.First(&bt, *task.ResultID)
		metrics := map[string]interface{}{
			"totalReturn": bt.TotalReturn,
			"sharpeRatio": bt.SharpeRatio,
			"maxDrawdown": bt.MaxDrawdown,
			"winRate":     bt.WinRate,
			"tradeCount":  bt.TradeCount,
		}
		sendSSE("metric", metrics)
		sendSSE("phase", map[string]string{"phase": "saved", "message": fmt.Sprintf("回测结果已保存 (ID: %d)", bt.ID)})
	} else if task.Status == "failed" {
		sendSSE("error", map[string]string{"message": task.ErrorMsg})
	}
	sendSSE("done", map[string]string{"message": "回测" + task.Status})
}
func parseDate(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func (h *StrategyHandler) BacktestHistory(c *gin.Context) {
	uid := getUID(c)
	sid := c.Query("strategyId")
	var results []model.BacktestResult
	q := db.MySQL.Where("user_id = ?", uid)
	if sid != "" {
		q = q.Where("strategy_id = ?", sid)
	}
	q.Order("created_at DESC").Limit(20).Find(&results)

	// Resolve pool labels for display
	for i := range results {
		results[i].StockPoolLabel = resolveStockPoolLabel(results[i].StockPool, results[i].StockPoolParams)
	}

	response.Success(c, results)
}

// DeleteBacktestResult deletes a single backtest result by ID
func (h *StrategyHandler) DeleteBacktestResult(c *gin.Context) {
	uid := getUID(c)
	id, _ := strconv.Atoi(c.Param("id"))
	result := db.MySQL.Where("id = ? AND user_id = ?", id, uid).Delete(&model.BacktestResult{})
	if result.RowsAffected == 0 {
		response.NotFound(c, "记录不存在")
		return
	}
	response.SuccessMsg(c, "已删除")
}

// StockPool returns available stock pools for backtest selection
func (h *StrategyHandler) StockPool(c *gin.Context) {
	uid := getUID(c)
	
	type PoolItem struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	type PoolGroup struct {
		Key   string     `json:"key"`
		Label string     `json:"label"`
		Count int        `json:"count"`
		Items []PoolItem `json:"items,omitempty"`
	}

	pools := []PoolGroup{}

	// 1. All stocks
	var allCount int64
	db.PG.Raw("SELECT COUNT(*) FROM stocks_basic").Scan(&allCount)
	var allStocks []PoolItem
	db.PG.Raw("SELECT code, COALESCE(name,'') as name FROM stocks_basic ORDER BY code LIMIT 5000").Scan(&allStocks)
	pools = append(pools, PoolGroup{
		Key:   "all",
		Label: "全部股票",
		Count: int(allCount),
		Items: allStocks,
	})

	// 2. Watchlist groups (MySQL)
	type WLGroup struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	}
	var wlGroups []WLGroup
	db.MySQL.Raw("SELECT id, name FROM watchlist_groups WHERE user_id = ? ORDER BY sort_order", uid).Scan(&wlGroups)
	for _, g := range wlGroups {
		// Query watchlist stocks from MySQL, then join with PG for names
		type WLRaw struct {
			StockCode string
		}
		var wlRaw []WLRaw
		db.MySQL.Raw("SELECT stock_code FROM watchlists WHERE user_id = ? AND group_id = ? ORDER BY stock_code", uid, g.ID).Scan(&wlRaw)
		codes := make([]string, len(wlRaw))
		for i, w := range wlRaw {
			codes[i] = w.StockCode
		}
		var items []PoolItem
		if len(codes) > 0 {
			db.PG.Raw("SELECT code, COALESCE(name,'') as name FROM stocks_basic WHERE code = ANY($1) ORDER BY code", codes).Scan(&items)
		}
		pools = append(pools, PoolGroup{
			Key:   fmt.Sprintf("watchlist_%d", g.ID),
			Label: "自选组: " + g.Name,
			Count: len(items),
			Items: items,
		})
	}

	// 3. Portfolio holdings (MySQL → PG join)
	type HoldingRaw struct {
		Code string
	}
	var holdingCodes []HoldingRaw
	db.MySQL.Raw("SELECT DISTINCT stock_code as code FROM holdings WHERE user_id = ? ORDER BY stock_code", uid).Scan(&holdingCodes)
	if len(holdingCodes) > 0 {
		codes := make([]string, len(holdingCodes))
		for i, h := range holdingCodes {
			codes[i] = h.Code
		}
		var holdings []PoolItem
		db.PG.Raw("SELECT code, COALESCE(name,'') as name FROM stocks_basic WHERE code = ANY($1) ORDER BY code", codes).Scan(&holdings)
		if len(holdings) > 0 {
			pools = append(pools, PoolGroup{
				Key:   "portfolio",
				Label: "我的持仓",
				Count: len(holdings),
				Items: holdings,
			})
		}
	}

	response.Success(c, pools)
}

// ── Available indicators list ──

func buildIndicatorList() []map[string]interface{} {
	return []map[string]interface{}{
		// ═══ 榜单与评分 ═══
		{"key": "streak_count", "label": "连榜次数", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "该股票在榜单连续出现的交易日数", "backtestSafe": true, "dataNote": "依赖榜单数据覆盖", "suggestion": "买入建议 ≥ 3 天，连续上榜说明持续受关注"},
		{"key": "algo_score", "label": "算法评分", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "算法团队给出的综合评分 (0-10)", "backtestSafe": true, "dataNote": "仅榜单日期有值", "suggestion": "买入建议 ≥ 6 分，算法团队评分越高越好"},
		{"key": "signal_value", "label": "原始信号值", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "算法团队原始信号值（越大越强）", "backtestSafe": false, "dataNote": "⚠️ 单点快照无历史，回测禁用", "suggestion": "买入建议 > 0.5，正值表示信号偏多"},

		// ═══ AI六维评分 ═══
		{"key": "ai_score", "label": "AI综合评分", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI六维综合评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，AI综合评估偏高"},
		{"key": "ai_fundamental", "label": "AI基本面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI基本面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，基本面扎实"},
		{"key": "ai_technical", "label": "AI技术面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI技术面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，技术形态良好"},
		{"key": "ai_valuation", "label": "AI估值", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI估值评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，估值合理偏低"},
		{"key": "ai_growth", "label": "AI成长性", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI成长性评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，成长性突出"},
		{"key": "ai_industry", "label": "AI行业面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI行业面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，行业景气度高"},
		{"key": "ai_capital", "label": "AI资金面", "type": "number", "operators": []string{"gte", "lte", "gt", "lt", "eq"}, "desc": "AI资金面评分 (0-10)", "backtestSafe": false, "dataNote": "⚠️ 仅少量股票有AI评分", "suggestion": "买入建议 ≥ 6 分，资金面积极"},

		// ═══ 技术面 — 趋势类 (100% K线数据覆盖) ═══
		{"key": "daily_change", "label": "单日涨跌幅", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 2% 或 < -5% 超跌反弹"},
		{"key": "momentum_5", "label": "5日动量", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近5个交易日累计涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 3%，短期趋势向上"},
		{"key": "momentum_20", "label": "20日动量", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日累计涨跌幅 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 5%，中期趋势确立"},
		{"key": "ma_deviation", "label": "均线偏离", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价偏离MA20的百分比", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -5% 超跌，卖出建议 > 10% 超涨"},
		{"key": "ma_cross", "label": "MA均线交叉", "type": "cross", "operators": []string{"cross_up", "cross_down"}, "desc": "value填均线周期如5/20表示MA5上穿/下穿MA20", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "上穿买入，如 MA5↑MA20 为短线金叉"},
		{"key": "macd", "label": "MACD信号", "type": "cross", "operators": []string{"cross_up", "cross_down", "eq"}, "desc": "MACD(12,26,9)金叉=1/死叉=-1/无交叉=0", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "eq=1 买入(金叉), eq=-1 卖出(死叉), 零轴上方金叉更可靠"},

		// ═══ 技术面 — 超买超卖 (100% K线数据覆盖) ═══
		{"key": "rsi", "label": "RSI(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "相对强弱指数，>70超买 <30超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 30 超卖，卖出建议 > 70 超买"},
		{"key": "kdj_k", "label": "KDJ-K", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标K值(9,3,3)，>80超买 <20超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 20 超卖，卖出建议 > 80 超买"},
		{"key": "kdj_d", "label": "KDJ-D", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标D值", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 30，卖出建议 > 70"},
		{"key": "kdj_j", "label": "KDJ-J", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "KDJ指标J值，>100极度超买 <0极度超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0 极度超卖，卖出建议 > 100 极度超买"},
		{"key": "boll_position", "label": "布林带位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "价格在布林带(20,2)中的位置%，>80上轨 <20下轨", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.1 触下轨，卖出建议 > 0.9 触上轨"},
		{"key": "boll_width", "label": "布林带宽度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "布林带(上轨-下轨)/中轨，衡量波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 5 收口将变盘，卖出建议 > 15 宽口波动大"},

		// ═══ 技术面 — 量价 (100% K线数据覆盖) ═══
		{"key": "volume_ratio", "label": "量比(5日)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日成交量与前5日均量之比", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 1.5 放量，量价配合更可靠"},
		{"key": "volume_ma_ratio", "label": "量比(20日)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日成交量与20日均量之比，>1.5放量", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 1.2，20日均量以上为活跃"},
		{"key": "turnover_rate", "label": "换手率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日换手率 (%)", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 3-10%，过高需警惕出货"},
		{"key": "atr", "label": "ATR(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "平均真实波幅，衡量波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "波动率指标，值越大波动越大，无固定阈值"},

		// ═══ 技术面 — 形态与强度 (100% K线数据覆盖) ═══
		{"key": "drawdown_20", "label": "20日最大回撤", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日最大回撤 (%)，负值代表回撤", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 15% 超跌反弹机会，结合基本面判断"},
		{"key": "new_high_20", "label": "20日新高", "type": "number", "operators": []string{"eq"}, "desc": "当日收盘是否为20日新高，1=是 0=否", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 = 1 创新高，强势突破信号"},
		{"key": "up_days_ratio", "label": "上涨天数占比", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20个交易日上涨天数占比，>0.5偏强", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0.6，阳线居多说明多头主导"},
		{"key": "price_position_20", "label": "20日价格位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价在近20日最高最低之间的位置%，>80高位", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.3 低位，> 0.7 高位注意风险"},
		{"key": "price_position_60", "label": "60日价格位置", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价在近60日最高最低之间的位置%", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 0.3 中长期低位区域"},


		// ═══ 技术面 — 进阶：趋势系统 ═══
		{"key": "adx", "label": "ADX(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "平均趋向指数，>25有趋势 >50强趋势", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 25 趋势明确，> 40 强趋势"},
		{"key": "dmi_plus", "label": "DMI+ (PDI)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "上升方向线，PDI>MDI多头占优", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "PDI > MDI 多头占优，差值越大越强"},
		{"key": "dmi_minus", "label": "DMI- (MDI)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "下降方向线，MDI>PDI空头占优", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "MDI > PDI 空头占优"},
		{"key": "ema_cross", "label": "EMA交叉", "type": "cross", "operators": []string{"cross_up", "cross_down"}, "desc": "value填12/26表示EMA12上穿/下穿EMA26", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖"},

		// ═══ 技术面 — 进阶：超买超卖扩展 ═══
		{"key": "cci", "label": "CCI(20)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "商品通道指数，>100超买 <-100超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -100 超卖，卖出建议 > 100 超买"},
		{"key": "williams_r", "label": "威廉%R(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "威廉指标，>-20超买 <-80超卖（负值）", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -80 超卖，卖出建议 > -20 超买"},
		{"key": "mfi", "label": "MFI(14)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "资金流量指标(量价RSI)，>80超买 <20超卖", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 20 资金超卖，卖出建议 > 80 资金超买"},

		// ═══ 技术面 — 进阶：波动与结构 ═══
		{"key": "boll_squeeze", "label": "布林挤压", "type": "number", "operators": []string{"gte", "lte"}, "desc": "布林带宽度处于N日最低的百分位，<10预示变盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 5 极度收口，变盘在即"},
		{"key": "atr_pct", "label": "ATR/价格%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "ATR(14)/收盘价*100，标准化波动率", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议结合趋势，高波动率需设宽止损"},
		{"key": "ma_convergence", "label": "均线粘合度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "MA5/10/20/60变异系数，<3%高度粘合预示变盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < 2 均线粘合，即将选择方向"},
		{"key": "trend_strength", "label": "趋势强度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "近20日收盘>MA20的天数占比，>0.7强多头", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0.6，趋势评分越高方向越明确"},

		// ═══ 技术面 — 进阶：形态与量价 ═══
		{"key": "consecutive_days", "label": "连续涨跌", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "连续涨跌天数，正=连涨 负=连跌", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 3 连续阳线强势，< -3 超跌"},
		{"key": "gap_pct", "label": "跳空缺口%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当日开盘相对昨收的跳空幅度%", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -3% 跳空缺口可关注回补"},
		{"key": "high_low_range", "label": "日内振幅%", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "(最高-最低)/昨收*100，衡量日内波动", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "振幅指标，日内振幅大适合短线，无固定阈值"},
		{"key": "vwap_deviation", "label": "VWAP偏离", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "收盘价偏离当日VWAP的%，正值强于均价", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 < -2% 低于均价可能反弹"},
		{"key": "volume_trend", "label": "量能趋势", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "成交量MA5/MA20-1，>0放量趋势", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 0 放量趋势，量涨价增更可靠"},
		{"key": "index_relative", "label": "大盘相对强度", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "个股20日收益-上证20日收益，正值跑赢大盘", "backtestSafe": true, "dataNote": "✅ K线衍生，全量历史覆盖", "suggestion": "买入建议 > 5 跑赢大盘，说明个股强势"},
		// ═══ 估值 (依赖indicator表，仅2天历史) ═══
		{"key": "pe", "label": "市盈率PE", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市盈率", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "买入建议 < 15 低估值，< 10 极度低估"},
		{"key": "pb", "label": "市净率PB", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市净率", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "买入建议 < 1.5 低市净率，金融股可放宽"},
		{"key": "ps", "label": "市销率PS", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "当前市销率", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "买入建议 < 2，成长股可适当放宽"},
		{"key": "pe_percentile", "label": "PE历史分位", "type": "number", "operators": []string{"gte", "lte"}, "desc": "当前PE在历史数据中的百分位，<30低估 >70高估", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "买入建议 < 30 历史低位，> 70 历史高位"},
		{"key": "pb_percentile", "label": "PB历史分位", "type": "number", "operators": []string{"gte", "lte"}, "desc": "当前PB在历史数据中的百分位，<30低估 >70高估", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "买入建议 < 30 历史低位，> 70 历史高位"},

		// ═══ 基本面 (30只股票，8报告期) ═══
		{"key": "roe", "label": "ROE", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "净资产收益率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 15%，ROE越高盈利能力越强"},
		{"key": "revenue_growth", "label": "营收增长率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "营收同比增长率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 10%，持续增长为佳"},
		{"key": "profit_growth", "label": "利润增长率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "归母净利润同比增长率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 15%，利润增速高更有价值"},
		{"key": "gross_margin", "label": "毛利率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "销售毛利率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 30%，高毛利率有定价权"},
		{"key": "net_margin", "label": "净利率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "销售净利率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 10%，净利率越高盈利质量越好"},
		{"key": "debt_ratio", "label": "资产负债率", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "资产负债率 (%)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 < 60%，过高财务风险大"},
		{"key": "eps", "label": "每股收益", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "基本每股收益 (元)", "backtestSafe": true, "dataNote": "📊 30只股票覆盖，回测取最近财报", "suggestion": "买入建议 > 0.5 元且持续增长"},

		// ═══ 资金面/市场面 ═══
		{"key": "total_market_cap", "label": "总市值(亿)", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "总市值 (亿元)", "backtestSafe": false, "dataNote": "⚠️ 仅最近2天数据，回测取最近可用值", "suggestion": "无固定阈值，大盘股 > 1000亿 稳健，小盘股 < 100亿 弹性大"},
		{"key": "shareholder_change", "label": "股东户数变化", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "股东户数环比变化%，负值表示筹码集中", "backtestSafe": true, "dataNote": "📊 53只股票覆盖，回测取最近报告", "suggestion": "买入建议 < -5% 筹码集中，> 10% 筹码分散警惕"},
		{"key": "inst_hold_ratio", "label": "机构持股比", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "机构持股占流通股比例 (%)", "backtestSafe": true, "dataNote": "📊 53只股票覆盖，回测取最近报告", "suggestion": "买入建议 > 30%，机构持股比例高更受认可"},

		// ═══ 预测 (纯未来数据) ═══
		{"key": "prediction_upside", "label": "预测上涨空间", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "多模型预测均价相对现价的上涨空间 (%)", "backtestSafe": false, "dataNote": "🚫 预测为未来数据，回测不可用", "suggestion": "买入建议 > 10%，预测上涨空间越大越好"},
		{"key": "prediction_consensus", "label": "预测一致性", "type": "number", "operators": []string{"gte", "lte", "gt", "lt"}, "desc": "预测看涨的模型占比 (0-1)，>0.5多数看涨", "backtestSafe": false, "dataNote": "🚫 预测为未来数据，回测不可用", "suggestion": "买入建议 > 0.6，多数模型看涨为佳"},
	}
}

func (h *StrategyHandler) Indicators(c *gin.Context) {
	response.Success(c, buildIndicatorList())
}

// parseValue converts flexible AI values to float64
func parseValue(v interface{}, indicator string, op string) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		// Try parse as number
		f, err := strconv.ParseFloat(val, 64)
		if err == nil { return f }
		// Handle "5/20" format for ma_cross → encode as 5.020
		if indicator == "ma_cross" && strings.Contains(val, "/") {
			parts := strings.Split(val, "/")
			if len(parts) == 2 {
				ma1, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				ma2, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if ma1 > 0 && ma2 > 0 {
					return ma1 + ma2/1000
				}
			}
		}
		// For cross operators: direction encoded
		if op == "cross_up" { return 1 }
		if op == "cross_down" { return -1 }
		return 0
	default:
		return 0
	}
}

// truncate returns first n chars
func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "..."
}

// ── Helpers ──

func filterConds(conds []model.StrategyCondition, condType string) []model.StrategyCondition {
	var out []model.StrategyCondition
	for _, c := range conds {
		if c.CondType == condType {
			out = append(out, c)
		}
	}
	return out
}

// evaluateConditions checks if conditions of a type are met for a stock on a date
// Uses OR between logic groups, AND within each group
func evaluateConditions(conds []model.StrategyCondition, code, date string) bool {
	if len(conds) == 0 {
		return false
	}

	// Group conditions by logicGroup
	groups := make(map[int][]model.StrategyCondition)
	for _, c := range conds {
		groups[c.LogicGroup] = append(groups[c.LogicGroup], c)
	}

	for _, groupConds := range groups {
		allMet := true
		for _, c := range groupConds {
			if !evaluateSingleCondition(c, code, date) {
				allMet = false
				break
			}
		}
		if allMet {
			return true
		}
	}
	return false
}


// ── Backtest execution logging helpers ──

// insertBacktestLog inserts a single execution log entry during backtest.
// Use batch insert for better performance when many logs are generated per day.
func insertBacktestLog(taskID, strategyID, userID uint, date string, seq int,
	logType, level, stockCode, stockName, message string, detail interface{}) {
	detailJSON := "{}"
	if detail != nil {
		b, _ := json.Marshal(detail)
		detailJSON = string(b)
	}
	entry := model.BacktestExecutionLog{
		TaskID:     taskID,
		StrategyID: strategyID,
		UserID:     userID,
		Date:       date,
		Seq:        seq,
		LogType:    logType,
		Level:      level,
		StockCode:  stockCode,
		StockName:  stockName,
		Message:    message,
		Detail:     detailJSON,
	}
	if err := db.MySQL.Create(&entry).Error; err != nil {
		log.Printf("[backtest] insertLog failed: %v", err)
	}
}

// insertDailySnapshot records a portfolio snapshot for one trading day.
func insertDailySnapshot(taskID, strategyID, userID uint, date string, dayIndex int,
	cash, totalEquity, dailyReturn, cumulativeReturn, maxDD float64,
	positionCount int, positions []map[string]interface{}) {
	posJSON := "[]"
	if len(positions) > 0 {
		b, _ := json.Marshal(positions)
		posJSON = string(b)
	}
	snap := model.BacktestDailySnapshot{
		TaskID:           taskID,
		StrategyID:       strategyID,
		UserID:           userID,
		Date:             date,
		DayIndex:         dayIndex,
		Cash:             math.Round(cash*100) / 100,
		TotalEquity:      math.Round(totalEquity*100) / 100,
		DailyReturn:      math.Round(dailyReturn*100) / 100,
		CumulativeReturn: math.Round(cumulativeReturn*100) / 100,
		PositionCount:    positionCount,
		Positions:        posJSON,
		MaxDrawdown:      math.Round(maxDD*100) / 100,
	}
	if err := db.MySQL.Create(&snap).Error; err != nil {
		log.Printf("[backtest] insertSnapshot failed: %v", err)
	}
}

func evaluateSingleCondition(cond model.StrategyCondition, code, date string) bool {
	val := getIndicatorValue(cond, code, date)
	switch cond.Operator {
	case "gte":
		return val >= cond.Value
	case "lte":
		return val <= cond.Value
	case "gt":
		return val > cond.Value
	case "lt":
		return val < cond.Value
	case "eq":
		return val == cond.Value
	case "cross_up":
		return val > 0 // 1 means cross up
	case "cross_down":
		return val < 0 // -1 means cross down
	}
	return false
}

func getIndicatorValue(cond model.StrategyCondition, code, date string) float64 {
	switch cond.Indicator {
	// ── 榜单与评分 ──
	case "streak_count":
		return getStreakCount(code, date)
	case "algo_score":
		var score float64
		db.PG.Raw("SELECT COALESCE(score,0) FROM algorithm_pick_details WHERE stock_code = ? AND pick_date = ?", code, date).Scan(&score)
		return score
	case "signal_value":
		var sig float64
		db.PG.Raw("SELECT COALESCE(signal_value,0) FROM stock_signals WHERE code = ?", code).Scan(&sig)
		return sig

	// ── AI六维评分 ──
	case "ai_score":
		return getAIScore(code, "composite_score")
	case "ai_fundamental":
		return getAIScore(code, "fundamental_score")
	case "ai_technical":
		return getAIScore(code, "technical_score")
	case "ai_valuation":
		return getAIScore(code, "valuation_score")
	case "ai_growth":
		return getAIScore(code, "growth_score")
	case "ai_industry":
		return getAIScore(code, "industry_score")
	case "ai_capital":
		return getAIScore(code, "capital_score")

	// ── 技术面 — 趋势类 ──
	case "daily_change":
		return getMomentum(code, date, 1)
	case "momentum_5":
		return getMomentum(code, date, 5)
	case "momentum_20":
		return getMomentum(code, date, 20)
	case "ma_deviation":
		return getMADeviation(code, date, 20)
	case "ma_cross":
		ma1 := int(cond.Value)
		ma2 := int(math.Round((cond.Value - float64(ma1)) * 1000))
		if ma1 < 1 { ma1 = 5 }
		if ma2 < 1 { ma2 = 20 }
		return checkMACross(code, date, ma1, ma2)
	case "macd":
		return checkMACD(code, date)

	// ── 技术面 — 超买超卖 ──
	case "rsi":
		return getRSI(code, date, 14)
	case "kdj_k":
		k, _, _ := getKDJ(code, date)
		return k
	case "kdj_d":
		_, d, _ := getKDJ(code, date)
		return d
	case "kdj_j":
		_, _, j := getKDJ(code, date)
		return j
	case "boll_position":
		return getBollPosition(code, date)
	case "boll_width":
		return getBollWidth(code, date)

	// ── 技术面 — 量价 ──
	case "volume_ratio":
		return getVolumeRatio(code, date, 5)
	case "volume_ma_ratio":
		return getVolumeRatio(code, date, 20)
	case "turnover_rate":
		return getTurnoverRate(code, date)
	case "atr":
		return getATR(code, date, 14)

	// ── 技术面 — 形态与强度 ──
	case "drawdown_20":
		return getMaxDrawdown(code, date, 20)
	case "new_high_20":
		return getNewHigh(code, date, 20)
	case "up_days_ratio":
		return getUpDaysRatio(code, date, 20)
	case "price_position_20":
		return getPricePosition(code, date, 20)
	case "price_position_60":
		return getPricePosition(code, date, 60)

	// ── 估值 ──
	case "pe":
		return getIndicator(code, date, "pe")
	case "pb":
		return getIndicator(code, date, "pb")
	case "ps":
		return getIndicator(code, date, "ps")
	case "pe_percentile":
		return getPEPercentile(code, date)
	case "pb_percentile":
		return getPBPercentile(code, date)

	// ── 基本面 ──
	case "roe":
		return getFinancialMetric(code, date, "roe")
	case "revenue_growth":
		return getFinancialMetric(code, date, "revenue_growth")
	case "profit_growth":
		return getFinancialMetric(code, date, "profit_growth")
	case "gross_margin":
		return getFinancialMetric(code, date, "gross_margin")
	case "net_margin":
		return getFinancialMetric(code, date, "net_margin")
	case "debt_ratio":
		return getFinancialMetric(code, date, "debt_ratio")
	case "eps":
		return getFinancialMetric(code, date, "eps")

	// ── 资金面/市场面 ──
	case "total_market_cap":
		return getIndicator(code, date, "total_market_cap")
	case "shareholder_change":
		var chg float64
		db.PG.Raw("SELECT COALESCE(holder_change,0) FROM stock_shareholders WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&chg)
		return chg
	case "inst_hold_ratio":
		var ratio float64
		db.PG.Raw("SELECT COALESCE(inst_hold_ratio,0) FROM stock_shareholders WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", code, date).Scan(&ratio)
		return ratio

	// ── 技术面 — 进阶：趋势系统 ──
	case "adx":
		return getADX(code, date, 14)
	case "dmi_plus":
		p, _, _ := getDMI(code, date, 14)
		return p
	case "dmi_minus":
		_, m, _ := getDMI(code, date, 14)
		return m
	case "ema_cross":
		ma1 := int(cond.Value)
		ma2 := int(math.Round((cond.Value - float64(ma1)) * 1000))
		if ma1 < 1 { ma1 = 12 }
		if ma2 < 1 { ma2 = 26 }
		return checkEMACross(code, date, ma1, ma2)

	// ── 技术面 — 进阶：超买超卖扩展 ──
	case "cci":
		return getCCI(code, date, 20)
	case "williams_r":
		return getWilliamsR(code, date, 14)
	case "mfi":
		return getMFI(code, date, 14)

	// ── 技术面 — 进阶：波动与结构 ──
	case "boll_squeeze":
		return getBollSqueeze(code, date, 100)
	case "atr_pct":
		return getATRPct(code, date, 14)
	case "ma_convergence":
		return getMAConvergence(code, date)
	case "trend_strength":
		return getTrendStrength(code, date, 20)

	// ── 技术面 — 进阶：形态与量价 ──
	case "consecutive_days":
		return getConsecutiveDays(code, date)
	case "gap_pct":
		return getGapPct(code, date)
	case "high_low_range":
		return getHighLowRange(code, date)
	case "vwap_deviation":
		return getVWAPDeviation(code, date)
	case "volume_trend":
		return getVolumeTrend(code, date)
	case "index_relative":
		return getIndexRelative(code, date, 20)

	// ── 预测 ──
	case "prediction_upside":
		return getPredictionUpside(code, date)
	case "prediction_consensus":
		return getPredictionConsensus(code, date)
	}
	return 0
}

// ═══════════════════════════════════════════════════════════════
// 通用辅助函数
// ═══════════════════════════════════════════════════════════════

func getStreakCount(code, date string) float64 {
	var count int
	db.PG.Raw(`SELECT COUNT(*) FROM (
		SELECT DISTINCT apd.pick_date FROM algorithm_pick_details apd
		JOIN algorithm_picks ap ON ap.pick_date = apd.pick_date
		WHERE apd.stock_code = ? AND ap.pick_date <= ?::date
	) sub`, code, date).Scan(&count)
	return float64(count)
}

func getClosePrice(code, date string) float64 {
	var close float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&close)
	return close
}

func getClosePriceOn(code, date string) float64 {
	var close float64
	db.PG.Raw("SELECT COALESCE(close, 0) FROM stocks_daily_k WHERE code = ? AND trade_date = ?::date LIMIT 1", code, date).Scan(&close)
	return close
}

func getAIScore(code, field string) float64 {
	var score float64
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM ai_stock_scores WHERE code = ? ORDER BY created_at DESC LIMIT 1", field), code).Scan(&score)
	return score
}

func getIndicator(code, date, field string) float64 {
	var val float64
	// Use most recent indicator data available on or before the backtest date
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", field), code, date).Scan(&val)
	return val
}

// hasIndicatorData checks if any indicator data exists for this stock up to the given date
func hasIndicatorData(code, date string) bool {
	var count int
	db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date", code, date).Scan(&count)
	return count > 0
}

func getFinancialMetric(code, date, field string) float64 {
	var val float64
	// Use latest financial report available on or before the backtest date
	db.PG.Raw(fmt.Sprintf("SELECT COALESCE(%s,0) FROM stock_financials WHERE code = ? AND report_date <= ? ORDER BY report_date DESC LIMIT 1", field), code, date).Scan(&val)
	return val
}

// hasFinancialData checks if any financial data exists for this stock up to given date
func hasFinancialData(code, date string) bool {
	var count int
	db.PG.Raw("SELECT COUNT(*) FROM stock_financials WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
	return count > 0
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 趋势类
// ═══════════════════════════════════════════════════════════════

func getMomentum(code, date string, days int) float64 {
	var chg float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) /
		NULLIF((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1 OFFSET ?), 0) - 1, 0) * 100`,
		code, date, code, date, days).Scan(&chg)
	return chg
}

func getMADeviation(code, date string, maPeriod int) float64 {
	var dev float64
	db.PG.Raw(`SELECT COALESCE(
		((SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) /
		NULLIF((SELECT AVG(close) FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub), 0) - 1) * 100, 0)`,
		code, date, code, date, maPeriod).Scan(&dev)
	return dev
}

func checkMACross(code, date string, ma1, ma2 int) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ma AS (
		SELECT trade_date,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ma_short,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ma_long
		FROM klines
	)
	SELECT CASE
		WHEN ma_short > ma_long AND LAG(ma_short) OVER (ORDER BY trade_date ASC) <= LAG(ma_long) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN ma_short < ma_long AND LAG(ma_short) OVER (ORDER BY trade_date ASC) >= LAG(ma_long) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM ma ORDER BY trade_date DESC LIMIT 1`, code, date, ma1, ma2).Scan(&cross)
	return float64(cross)
}

func checkMACD(code, date string) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema AS (
		SELECT trade_date,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 11 PRECEDING AND CURRENT ROW) as ma12,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 25 PRECEDING AND CURRENT ROW) as ma26
		FROM klines
	), macd AS (
		SELECT trade_date,
			ma12 - ma26 as dif,
			AVG(ma12 - ma26) OVER (ORDER BY trade_date ASC ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as dea
		FROM ema
	)
	SELECT CASE
		WHEN dif > dea AND LAG(dif) OVER (ORDER BY trade_date ASC) <= LAG(dea) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN dif < dea AND LAG(dif) OVER (ORDER BY trade_date ASC) >= LAG(dea) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM macd ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&cross)
	return float64(cross)
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 超买超卖
// ═══════════════════════════════════════════════════════════════

func getRSI(code, date string, period int) float64 {
	var rsi float64
	db.PG.Raw(`SELECT COALESCE(
		100 - 100 / (1 + (avg_gain / NULLIF(avg_loss,0))), 50)
	FROM (
		SELECT
			AVG(CASE WHEN chg > 0 THEN chg ELSE 0 END) as avg_gain,
			AVG(CASE WHEN chg < 0 THEN -chg ELSE 0 END) as avg_loss
		FROM (
			SELECT close - LAG(close) OVER (ORDER BY trade_date) as chg
			FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
			ORDER BY trade_date DESC LIMIT ?
		) changes
	) avgs`, code, date, period+1).Scan(&rsi)
	return rsi
}

func getKDJ(code, date string) (k, d, j float64) {
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), rsv_calc AS (
		SELECT trade_date, close,
			MIN(low) OVER (ORDER BY trade_date ASC ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as l9,
			MAX(high) OVER (ORDER BY trade_date ASC ROWS BETWEEN 8 PRECEDING AND CURRENT ROW) as h9
		FROM klines
	), k_calc AS (
		SELECT trade_date,
			(close - l9) / NULLIF((h9 - l9), 0) * 100 as rsv,
			AVG((close - l9) / NULLIF((h9 - l9), 0) * 100) OVER (ORDER BY trade_date ASC ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as k
		FROM rsv_calc
	), d_calc AS (
		SELECT trade_date, k,
			AVG(k) OVER (ORDER BY trade_date ASC ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) as d
		FROM k_calc
	)
	SELECT COALESCE(k,50), COALESCE(d,50)
	FROM d_calc ORDER BY trade_date DESC LIMIT 1`, code, date).Row().Scan(&k, &d)
	if k > 0 || d > 0 {
		j = 3*k - 2*d
	}
	return
}

func getBollPosition(code, date string) float64 {
	var pos float64
	db.PG.Raw(`SELECT COALESCE(
		(cl - (mid - 2*std)) / NULLIF((mid + 2*std) - (mid - 2*std), 0) * 100, 50)
	FROM (
		SELECT close as cl,
			AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
			STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date DESC LIMIT 1
	) boll`, code, date).Scan(&pos)
	return pos
}

func getBollWidth(code, date string) float64 {
	var w float64
	db.PG.Raw(`SELECT COALESCE(
		((mid + 2*std) - (mid - 2*std)) / NULLIF(mid, 0) * 100, 0)
	FROM (
		SELECT close as cl,
			AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
			STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date
		ORDER BY trade_date DESC LIMIT 1
	) boll`, code, date).Scan(&w)
	return w
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 量价
// ═══════════════════════════════════════════════════════════════

func getVolumeRatio(code, date string, days int) float64 {
	var vr float64
	db.PG.Raw(`SELECT COALESCE(
		vol::float / NULLIF(avg_vol, 0), 0)
	FROM (
		SELECT 
			(SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) as vol,
			(SELECT AVG(volume) FROM (SELECT volume FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) as avg_vol
	) t`, code, date, code, date, days).Scan(&vr)
	return vr
}

func getTurnoverRate(code, date string) float64 {
	var tr float64
	db.PG.Raw("SELECT COALESCE(turnover_rate, 0) FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1", code, date).Scan(&tr)
	return tr
}

func getATR(code, date string, period int) float64 {
	var atr float64
	db.PG.Raw(`SELECT COALESCE(AVG(tr),0) FROM (
		SELECT GREATEST(high-low, ABS(high-LAG(close) OVER (ORDER BY trade_date)), ABS(low-LAG(close) OVER (ORDER BY trade_date))) as tr
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?
	) sub`, code, date, period+1).Scan(&atr)
	return atr
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 形态与强度
// ═══════════════════════════════════════════════════════════════

func getMaxDrawdown(code, date string, days int) float64 {
	var dd float64
	db.PG.Raw(`SELECT COALESCE(ABS(MIN(drawdown)), 0) FROM (
		SELECT (close - MAX(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW))
			/ NULLIF(MAX(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW), 0) * 100 as drawdown
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC LIMIT ?
	) sub`, code, date, days).Scan(&dd)
	return dd
}

func getNewHigh(code, date string, days int) float64 {
	var isHigh int
	db.PG.Raw(`SELECT CASE WHEN
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) >=
		(SELECT COALESCE(MAX(close),0) FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub)
		THEN 1 ELSE 0 END`, code, date, code, date, days).Scan(&isHigh)
	return float64(isHigh)
}

func getUpDaysRatio(code, date string, days int) float64 {
	var ratio float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM (SELECT close, open FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub WHERE close > open)::float / ?, 0)`,
		code, date, days, float64(days)).Scan(&ratio)
	return ratio
}

func getPricePosition(code, date string, days int) float64 {
	var pos float64
	db.PG.Raw(`SELECT COALESCE(
		(cl - low_n) / NULLIF(high_n - low_n, 0) * 100, 50)
	FROM (
		SELECT
			(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) as cl,
			(SELECT MIN(low) FROM (SELECT low FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub) as low_n,
			(SELECT MAX(high) FROM (SELECT high FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?) sub) as high_n
	) pos_calc`, code, date, code, date, days, code, date, days).Scan(&pos)
	return pos
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：趋势系统 (ADX/DMI/EMA交叉)
// ═══════════════════════════════════════════════════════════════

func getADX(code, date string, period int) float64 {
	var adx float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tr_calc AS (
		SELECT trade_date,
			high - LAG(high) OVER (ORDER BY trade_date ASC) as up_move,
			LAG(low) OVER (ORDER BY trade_date ASC) - low as down_move,
			GREATEST(high-low,
				ABS(high-LAG(close) OVER (ORDER BY trade_date ASC)),
				ABS(low-LAG(close) OVER (ORDER BY trade_date ASC))
			) as tr
		FROM klines
	), dmi AS (
		SELECT trade_date,
			AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as pdi,
			AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as mdi
		FROM tr_calc
	)
	SELECT COALESCE(ABS(pdi-mdi)/NULLIF(pdi+mdi,0)*100, 0)
	FROM dmi ORDER BY trade_date DESC LIMIT 1`, code, date, period, period, period, period).Scan(&adx)
	return adx
}

func getDMI(code, date string, period int) (pdi, mdi, adx float64) {
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tr_calc AS (
		SELECT trade_date,
			high - LAG(high) OVER (ORDER BY trade_date ASC) as up_move,
			LAG(low) OVER (ORDER BY trade_date ASC) - low as down_move,
			GREATEST(high-low,
				ABS(high-LAG(close) OVER (ORDER BY trade_date ASC)),
				ABS(low-LAG(close) OVER (ORDER BY trade_date ASC))
			) as tr
		FROM klines
	), dmi AS (
		SELECT trade_date,
			AVG(CASE WHEN up_move > 0 AND up_move > down_move THEN up_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as pdi,
			AVG(CASE WHEN down_move > 0 AND down_move > up_move THEN down_move ELSE 0 END)
				OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) /
			NULLIF(AVG(tr) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW), 0) * 100 as mdi
		FROM tr_calc
	)
	SELECT COALESCE(pdi, 25), COALESCE(mdi, 25),
		COALESCE(ABS(pdi-mdi)/NULLIF(pdi+mdi,0)*100, 0)
	FROM dmi ORDER BY trade_date DESC LIMIT 1`, code, date, period, period, period, period).Row().Scan(&pdi, &mdi, &adx)
	return
}

func checkEMACross(code, date string, ma1, ma2 int) float64 {
	var cross int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ema AS (
		SELECT trade_date,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ema1,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ema2
		FROM klines
	)
	SELECT CASE
		WHEN ema1 > ema2 AND LAG(ema1) OVER (ORDER BY trade_date ASC) <= LAG(ema2) OVER (ORDER BY trade_date ASC) THEN 1
		WHEN ema1 < ema2 AND LAG(ema1) OVER (ORDER BY trade_date ASC) >= LAG(ema2) OVER (ORDER BY trade_date ASC) THEN -1
		ELSE 0 END
	FROM ema ORDER BY trade_date DESC LIMIT 1`, code, date, ma1, ma2).Scan(&cross)
	return float64(cross)
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：超买超卖扩展 (CCI/Williams%R/MFI)
// ═══════════════════════════════════════════════════════════════

func getCCI(code, date string, period int) float64 {
	var cci float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), tp_dev AS (
		SELECT trade_date,
			(high + low + close) / 3 as tp,
			AVG((high + low + close) / 3) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as avg_tp
		FROM klines
	), dev_calc AS (
		SELECT trade_date, tp, avg_tp,
			AVG(ABS(tp - avg_tp)) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as avg_dev
		FROM tp_dev
	)
	SELECT COALESCE((tp - avg_tp) / NULLIF(0.015 * avg_dev, 0), 0)
	FROM dev_calc ORDER BY trade_date DESC LIMIT 1`, code, date, period, period).Scan(&cci)
	return cci
}

func getWilliamsR(code, date string, period int) float64 {
	var wr float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), wr_calc AS (
		SELECT trade_date,
			MAX(high) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as hn,
			MIN(low) OVER (ORDER BY trade_date ASC ROWS BETWEEN ?-1 PRECEDING AND CURRENT ROW) as ln,
			close
		FROM klines
	)
	SELECT COALESCE(-(hn - close) / NULLIF(hn - ln, 0) * 100, -50)
	FROM wr_calc ORDER BY trade_date DESC LIMIT 1`, code, date, period, period).Scan(&wr)
	return wr
}

func getMFI(code, date string, period int) float64 {
	var mfi float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, high, low, close, volume FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), mf_calc AS (
		SELECT
			(high + low + close) / 3 * volume as tp_vol,
			((high + low + close) / 3 - LAG((high + low + close) / 3) OVER (ORDER BY trade_date ASC)) * volume as mf
		FROM klines
	)
	SELECT COALESCE(100 - 100 / (1 + pos_mf / NULLIF(neg_mf, 0)), 50)
	FROM (
		SELECT
			SUM(CASE WHEN mf > 0 THEN mf ELSE 0 END) as pos_mf,
			SUM(CASE WHEN mf < 0 THEN -mf ELSE 0 END) as neg_mf
		FROM (SELECT * FROM mf_calc ORDER BY trade_date DESC LIMIT ?) recent
	) mfi_calc`, code, date, period).Scan(&mfi)
	return mfi
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：波动与结构
// ═══════════════════════════════════════════════════════════════

func getBollSqueeze(code, date string, lookback int) float64 {
	var squeeze float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM (
			SELECT (mid + 2*std - (mid - 2*std)) / NULLIF(mid, 0) as bw
			FROM (
				SELECT close,
					AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
					STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
				FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ?
			) boll WHERE mid > 0
		) hist WHERE bw > (
			SELECT (mid + 2*std - (mid - 2*std)) / NULLIF(mid, 0)
			FROM (
				SELECT close,
					AVG(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as mid,
					STDDEV(close) OVER (ORDER BY trade_date ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as std
				FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1
			) curr WHERE mid > 0
		))::float / NULLIF(?, 0) * 100, 50)`,
		code, date, lookback+20, code, date, float64(lookback)).Scan(&squeeze)
	return squeeze
}

func getATRPct(code, date string, period int) float64 {
	atr := getATR(code, date, period)
	price := getClosePrice(code, date)
	if price > 0 {
		return atr / price * 100
	}
	return 0
}

func getMAConvergence(code, date string) float64 {
	var cv float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), mas AS (
		SELECT
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as ma5,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 9 PRECEDING AND CURRENT ROW) as ma10,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20,
			AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) as ma60
		FROM klines
	)
	SELECT COALESCE(STDDEV(ma) / NULLIF(AVG(ma), 0) * 100, 100)
	FROM (
		SELECT UNNEST(ARRAY[ma5, ma10, ma20, ma60]) as ma
		FROM mas ORDER BY trade_date DESC LIMIT 1
	) cv_calc`, code, date).Scan(&cv)
	return cv
}

func getTrendStrength(code, date string, days int) float64 {
	var strength float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ma AS (
		SELECT close, AVG(close) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as ma20
		FROM klines
	)
	SELECT COALESCE(
		SUM(CASE WHEN close > ma20 THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0.5)
	FROM (SELECT * FROM ma ORDER BY trade_date DESC LIMIT ?) sub`, code, date, days).Scan(&strength)
	return strength
}

// ═══════════════════════════════════════════════════════════════
// 技术面 — 进阶：形态与量价
// ═══════════════════════════════════════════════════════════════

func getConsecutiveDays(code, date string) float64 {
	var days int
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, close FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), ranked AS (
		SELECT close, LAG(close) OVER (ORDER BY trade_date ASC) as prev_close, trade_date
		FROM klines
	), grouped AS (
		SELECT *, SUM(CASE WHEN (close > prev_close) != (LAG(close) OVER (ORDER BY trade_date ASC) > LAG(prev_close) OVER (ORDER BY trade_date ASC)) THEN 1 ELSE 0 END) OVER (ORDER BY trade_date ASC) as grp
		FROM ranked WHERE prev_close IS NOT NULL
	)
	SELECT COUNT(*) * CASE
		WHEN (SELECT close > prev_close FROM grouped ORDER BY trade_date DESC LIMIT 1) THEN 1 ELSE -1 END
	FROM (SELECT * FROM grouped ORDER BY trade_date DESC LIMIT 20) g WHERE grp = 0`, code, date).Scan(&days)
	return float64(days)
}

func getGapPct(code, date string) float64 {
	var gap float64
	db.PG.Raw(`SELECT COALESCE(
		(open - LAG(close) OVER (ORDER BY trade_date)) / NULLIF(LAG(close) OVER (ORDER BY trade_date), 0) * 100, 0)
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&gap)
	return gap
}

func getHighLowRange(code, date string) float64 {
	var rng float64
	db.PG.Raw(`SELECT COALESCE(
		(high - low) / NULLIF(LAG(close) OVER (ORDER BY trade_date), 0) * 100, 0)
		FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&rng)
	return rng
}

func getVWAPDeviation(code, date string) float64 {
	var dev float64
	db.PG.Raw(`SELECT COALESCE(
		(close - vwap) / NULLIF(vwap, 0) * 100, 0)
	FROM (
		SELECT close, SUM(close * volume) / NULLIF(SUM(volume), 0) as vwap
		FROM stocks_daily_k WHERE code = ? AND trade_date = ?::date
	) vwap_calc`, code, date).Scan(&dev)
	return dev
}

func getVolumeTrend(code, date string) float64 {
	var trend float64
	db.PG.Raw(`WITH klines AS (
		SELECT trade_date, volume FROM stocks_daily_k
		WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date ASC
	), vol AS (
		SELECT trade_date,
			AVG(volume) OVER (ORDER BY trade_date ASC ROWS BETWEEN 4 PRECEDING AND CURRENT ROW) as vol_ma5,
			AVG(volume) OVER (ORDER BY trade_date ASC ROWS BETWEEN 19 PRECEDING AND CURRENT ROW) as vol_ma20
		FROM klines
	)
	SELECT COALESCE(vol_ma5 / NULLIF(vol_ma20, 0) - 1, 0)
	FROM vol ORDER BY trade_date DESC LIMIT 1`, code, date).Scan(&trend)
	return trend
}

func getIndexRelative(code, date string, days int) float64 {
	// Compare stock return vs Shanghai index (000001) return
	var rel float64
	db.PG.Raw(`SELECT COALESCE(
		(s1.close / NULLIF(s2.close, 0) - i1.close / NULLIF(i2.close, 0)) * 100, 0)
	FROM
		(SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) s1,
		(SELECT close FROM (SELECT close FROM stocks_daily_k WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) s2,
		(SELECT close FROM stocks_daily_k WHERE code = '000001' AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1) i1,
		(SELECT close FROM (SELECT close FROM stocks_daily_k WHERE code = '000001' AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT ? OFFSET 1) sub) i2`,
		code, date, code, date, days, date, date, days).Scan(&rel)
	return rel
}


// ═══════════════════════════════════════════════════════════════
// 估值
// ═══════════════════════════════════════════════════════════════

func getPEPercentile(code, date string) float64 {
	var pct float64
	// Compute PE percentile using only data available up to the backtest date
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pe > 0 AND pe < COALESCE((SELECT pe FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1), 0))::float /
		NULLIF((SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pe > 0), 0) * 100, 50)`,
		code, date, code, date, code, date).Scan(&pct)
	return pct
}

func getPBPercentile(code, date string) float64 {
	var pct float64
	db.PG.Raw(`SELECT COALESCE(
		(SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pb > 0 AND pb < COALESCE((SELECT pb FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date ORDER BY trade_date DESC LIMIT 1), 0))::float /
		NULLIF((SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?::date AND pb > 0), 0) * 100, 50)`,
		code, date, code, date, code, date).Scan(&pct)
	return pct
}

// ═══════════════════════════════════════════════════════════════
// 预测
// ═══════════════════════════════════════════════════════════════

func getPredictionUpside(code, date string) float64 {
	var upside float64
	price := getClosePrice(code, date)
	if price <= 0 { return 0 }
	// Predictions are forward-looking; only use if predict_date > backtest_date
	db.PG.Raw(`SELECT COALESCE((AVG(predicted_price) - ?) / ? * 100, 0)
		FROM predictions WHERE code = ? AND predict_date > ?::date
		AND created_at = (SELECT MAX(created_at) FROM predictions WHERE code = ? AND predict_date > ?::date)`,
		price, price, code, date, code, date).Scan(&upside)
	return upside
}

func getPredictionConsensus(code, date string) float64 {
	var consensus float64
	price := getClosePrice(code, date)
	if price <= 0 { return 0 }
	db.PG.Raw(`SELECT COALESCE(
		SUM(CASE WHEN predicted_price > ? THEN 1 ELSE 0 END)::float / NULLIF(COUNT(*), 0), 0)
		FROM predictions WHERE code = ? AND predict_date > ?::date
		AND created_at = (SELECT MAX(created_at) FROM predictions WHERE code = ? AND predict_date > ?::date)`,
		price, code, date, code, date).Scan(&consensus)
	return consensus
}

func cleanJSON(s string) string {
	// Trim whitespace and newlines
	s = trimSpace(s)
	// Remove markdown fences
	s = trimPrefixes(s, "```json", "```")
	s = trimSuffixes(s, "```")
	s = trimSpace(s)
	// If still not starting with {, find the first { and last }
	if len(s) == 0 || s[0] != '{' {
		start := findChar(s, '{')
		end := findLastChar(s, '}')
		if start >= 0 && end > start {
			s = s[start : end+1]
		}
	}
	return s
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

func findChar(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c { return i }
	}
	return -1
}

func findLastChar(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c { return i }
	}
	return -1
}

func trimPrefixes(s string, prefixes ...string) string {
	for _, p := range prefixes {
		if len(s) > len(p) && s[:len(p)] == p {
			s = s[len(p):]
		}
	}
	return s
}

func trimSuffixes(s string, suffixes ...string) string {
	for _, sf := range suffixes {
		if len(s) > len(sf) && s[len(s)-len(sf):] == sf {
			s = s[:len(s)-len(sf)]
		}
	}
	return s
}

// ── Test Indicator ──

type TestIndicatorReq struct {
	StockCode string      `json:"stockCode"`
	Date      string      `json:"date"`
	Indicator string      `json:"indicator"`
	Operator  string      `json:"operator"`
	Value     json.Number `json:"value"`
}

type TestIndicatorResp struct {
	StockCode      string  `json:"stockCode"`
	StockName      string  `json:"stockName"`
	Date           string  `json:"date"`
	Indicator      string  `json:"indicator"`
	IndicatorLabel string  `json:"indicatorLabel"`
	Operator       string  `json:"operator"`
	OperatorLabel  string  `json:"operatorLabel"`
	Threshold      float64 `json:"threshold"`
	ComputedValue  float64 `json:"computedValue"`
	ConditionMet   bool    `json:"conditionMet"`
	DataSource     string  `json:"dataSource"`
	DataNote       string  `json:"dataNote"`
	Error          string  `json:"error,omitempty"`
	HasData        bool    `json:"hasData"`
}

func (h *StrategyHandler) TestIndicator(c *gin.Context) {
	var req TestIndicatorReq
	if err := c.ShouldBindJSON(&req); err != nil || req.StockCode == "" || req.Date == "" || req.Indicator == "" || req.Operator == "" {
		response.BadRequest(c, "参数错误：stockCode/date/indicator/operator 必填")
		return
	}

	// Parse value (accepts both "5" and 5)
	valFloat, err := req.Value.Float64()
	if err != nil {
		response.BadRequest(c, "参数错误：value 必须是数字")
		return
	}

	// Look up indicator metadata
	indicatorInfo := getIndicatorMeta(req.Indicator)
	indicatorLabel := req.Indicator
	indicatorDataNote := ""
	if indicatorInfo != nil {
		indicatorLabel = indicatorInfo["label"].(string)
		if dn, ok := indicatorInfo["dataNote"]; ok {
			indicatorDataNote = dn.(string)
		}
	}

	// Look up stock name
	stockName := getStockName(req.StockCode)

	// Get data source
	dataSource := getIndicatorDataSource(req.Indicator)

	resp := TestIndicatorResp{
		StockCode:      req.StockCode,
		StockName:      stockName,
		Date:           req.Date,
		Indicator:      req.Indicator,
		IndicatorLabel: indicatorLabel,
		Operator:       req.Operator,
		OperatorLabel:  getOperatorLabel(req.Operator),
		Threshold:      valFloat,
		DataSource:     dataSource,
		DataNote:       indicatorDataNote,
	}

	// Check if data exists for this stock/date
	if !hasAnyData(req.StockCode, req.Date, req.Indicator) {
		resp.Error = "该股票在指定日期无对应数据"
		resp.HasData = false
		response.Success(c, resp)
		return
	}

	// Create a fake condition to reuse the evaluation logic
	cond := model.StrategyCondition{
		Indicator: req.Indicator,
		Operator:  req.Operator,
		Value:     valFloat,
	}

	computedValue := getIndicatorValue(cond, req.StockCode, req.Date)
	resp.ComputedValue = math.Round(computedValue*100) / 100
	resp.HasData = true
	resp.ConditionMet = evaluateSingleCondition(cond, req.StockCode, req.Date)

	response.Success(c, resp)
}

func getIndicatorMeta(key string) map[string]interface{} {
	indicators := buildIndicatorList()
	for _, ind := range indicators {
		if ind["key"] == key {
			return ind
		}
	}
	return nil
}

func getOperatorLabel(op string) string {
	switch op {
	case "gte": return "≥ (大于等于)"
	case "lte": return "≤ (小于等于)"
	case "gt": return "> (大于)"
	case "lt": return "< (小于)"
	case "eq": return "= (等于)"
	case "cross_up": return "↑ 上穿"
	case "cross_down": return "↓ 下穿"
	}
	return op
}

func getIndicatorDataSource(indicator string) string {
	switch {
	// K线衍生
	case indicator == "daily_change", indicator == "momentum_5", indicator == "momentum_20",
		indicator == "ma_deviation", indicator == "ma_cross", indicator == "macd",
		indicator == "ema_cross", indicator == "rsi", indicator == "kdj_k", indicator == "kdj_d", indicator == "kdj_j",
		indicator == "boll_position", indicator == "boll_width", indicator == "boll_squeeze",
		indicator == "volume_ratio", indicator == "volume_ma_ratio", indicator == "turnover_rate",
		indicator == "atr", indicator == "atr_pct", indicator == "drawdown_20", indicator == "new_high_20",
		indicator == "up_days_ratio", indicator == "price_position_20", indicator == "price_position_60",
		indicator == "adx", indicator == "dmi_plus", indicator == "dmi_minus",
		indicator == "cci", indicator == "williams_r", indicator == "mfi",
		indicator == "ma_convergence", indicator == "trend_strength",
		indicator == "consecutive_days", indicator == "gap_pct", indicator == "high_low_range",
		indicator == "vwap_deviation", indicator == "volume_trend", indicator == "index_relative":
		return "stocks_daily_k"
	// 估值指标
	case indicator == "pe", indicator == "pb", indicator == "ps",
		indicator == "pe_percentile", indicator == "pb_percentile",
		indicator == "total_market_cap":
		return "stocks_daily_indicator"
	// 财务数据
	case indicator == "roe", indicator == "revenue_growth", indicator == "profit_growth",
		indicator == "gross_margin", indicator == "net_margin", indicator == "debt_ratio",
		indicator == "eps":
		return "stock_financials"
	// AI评分
	case strings.HasPrefix(indicator, "ai_"):
		return "ai_stock_scores"
	// 榜单
	case indicator == "streak_count", indicator == "algo_score":
		return "algorithm_pick_details"
	// 信号
	case indicator == "signal_value":
		return "stock_signals"
	// 股东
	case indicator == "shareholder_change", indicator == "inst_hold_ratio":
		return "stock_shareholders"
	// 预测
	case indicator == "prediction_upside", indicator == "prediction_consensus":
		return "ai_stock_predictions"
	}
	return "unknown"
}

func hasAnyData(code, date, indicator string) bool {
	switch {
	case indicator == "daily_change", strings.HasPrefix(indicator, "momentum"),
		indicator == "ma_deviation", indicator == "ma_cross", indicator == "macd",
		indicator == "ema_cross", indicator == "rsi", strings.HasPrefix(indicator, "kdj"),
		strings.HasPrefix(indicator, "boll"), indicator == "boll_squeeze",
		strings.HasPrefix(indicator, "volume"), indicator == "turnover_rate",
		indicator == "atr", indicator == "atr_pct", strings.HasPrefix(indicator, "drawdown"),
		strings.HasPrefix(indicator, "new_high"), indicator == "up_days_ratio",
		strings.HasPrefix(indicator, "price_position"),
		indicator == "adx", strings.HasPrefix(indicator, "dmi_"),
		indicator == "cci", indicator == "williams_r", indicator == "mfi",
		indicator == "ma_convergence", indicator == "trend_strength",
		indicator == "consecutive_days", indicator == "gap_pct", indicator == "high_low_range",
		indicator == "vwap_deviation", indicator == "volume_trend", indicator == "index_relative":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_k WHERE code = ? AND trade_date = ?", code, date).Scan(&count)
		return count > 0
	case indicator == "pe", indicator == "pb", indicator == "ps",
		indicator == "pe_percentile", indicator == "pb_percentile",
		indicator == "total_market_cap":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stocks_daily_indicator WHERE code = ? AND trade_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "roe", indicator == "revenue_growth", indicator == "profit_growth",
		indicator == "gross_margin", indicator == "net_margin", indicator == "debt_ratio",
		indicator == "eps":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_financials WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
		return count > 0
	case strings.HasPrefix(indicator, "ai_"):
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM ai_stock_scores WHERE code = ?", code).Scan(&count)
		return count > 0
	case indicator == "streak_count", indicator == "algo_score":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM algorithm_pick_details WHERE stock_code = ? AND pick_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "signal_value":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_signals WHERE code = ?", code).Scan(&count)
		return count > 0
	case indicator == "shareholder_change", indicator == "inst_hold_ratio":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM stock_shareholders WHERE code = ? AND report_date <= ?", code, date).Scan(&count)
		return count > 0
	case indicator == "prediction_upside", indicator == "prediction_consensus":
		var count int64
		db.PG.Raw("SELECT COUNT(*) FROM ai_stock_predictions WHERE code = ? AND predict_date = ?", code, date).Scan(&count)
		return count > 0
	}
	return false
}

func getStockName(code string) string {
	var name string
	db.PG.Raw("SELECT COALESCE(name,'') FROM stocks_basic WHERE code = ? LIMIT 1", code).Scan(&name)
	return name
}
