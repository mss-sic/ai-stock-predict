package handler

import (
	"encoding/json"
	"os/exec"
	"os"
	"path/filepath"
	"runtime"
	"log"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	svc *service.AIService
}

// safeSlice truncates s to n chars, returns empty if too short.
func safeSlice(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s
}

func NewAIHandler() *AIHandler {
	return &AIHandler{svc: service.NewAIService()}
}

// GetHistory returns conversation history for a stock
func (h *AIHandler) GetHistory(c *gin.Context) {
	code := c.Param("code")
	msgs := make([]model.AIConversation, 0)
	db.PG.Where("code = ?", code).Order("created_at ASC").Find(&msgs)
	response.Success(c, gin.H{"messages": msgs, "code": code})
}

// ClearHistory deletes all conversation history for a stock
func (h *AIHandler) ClearHistory(c *gin.Context) {
	code := c.Param("code")
	db.PG.Where("code = ?", code).Delete(&model.AIConversation{})
	response.SuccessMsg(c, "ok")
}

func (h *AIHandler) Analyze(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if body.Question == "" {
		response.BadRequest(c, "问题不能为空")
		return
	}

	// Save user message
	db.PG.Create(&model.AIConversation{Code: body.Code, Role: "user", Content: body.Question})

	// Build system context
	sysMsg := h.buildStockContext(body.Code)

	// Build history context from recent messages (last 20)
	var history []model.AIConversation
	db.PG.Where("code = ?", body.Code).Order("created_at DESC").Limit(20).Find(&history)
	var chronHistory []model.AIConversation
	for i := len(history) - 1; i >= 0; i-- {
		chronHistory = append(chronHistory, history[i])
	}

	messages := []map[string]string{
		{"role": "system", "content": sysMsg},
	}
	for _, h := range chronHistory {
		if h.Content == "" {
			continue
		}
		role := h.Role
		if role == "ai" { role = "assistant" }
		messages = append(messages, map[string]string{"role": role, "content": h.Content})
	}

	uid, _ := c.Get("userId")
	reply, err := h.svc.ChatCompletion(uid.(uint), body.Question, messages)
	if err != nil {
		handleAIError(c, err)
		return
	}

	// Save AI reply
	db.PG.Create(&model.AIConversation{Code: body.Code, Role: "ai", Content: reply})

	response.Success(c, gin.H{"reply": reply, "code": body.Code})
}

func (h *AIHandler) AnalyzeStream(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "请求参数错误")
		return
	}

	if body.Question == "" {
		response.BadRequest(c, "问题不能为空")
		return
	}

	// Save user message
	db.PG.Create(&model.AIConversation{Code: body.Code, Role: "user", Content: body.Question})

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	aiCfg := h.loadSystemConfig("chat_analysis")

	// ── Agent mode (tools enabled) ──
	if aiCfg.EnableTools {
		h.analyzeStreamAgent(c, body.Code, body.Question, aiCfg)
		return
	}

	// ── Standard streaming mode ──
	sysMsg := h.buildStockContext(body.Code)

	// Load recent history (last 8 messages, capped at ~2400 chars total)
	var history []model.AIConversation
	db.PG.Where("code = ?", body.Code).Order("created_at DESC").Limit(8).Find(&history)
	messages := []map[string]string{
		{"role": "system", "content": sysMsg},
	}
	total := 0
	const maxCtx = 2400
	for i := len(history) - 1; i >= 0; i-- {
		txt := history[i].Content
		if txt == "" {
			continue
		}
		if total+len(txt) > maxCtx {
			remain := maxCtx - total
			if remain < 20 { break }
			txt = safeSlice(txt, remain) + "…"
		}
		total += len(txt)
		role := history[i].Role
		if role == "ai" { role = "assistant" }
		messages = append(messages, map[string]string{
			"role": role, "content": txt,
		})
		if total >= maxCtx { break }
	}

	var fullReply string
	uid, _ := c.Get("userId")
	err := h.svc.ChatCompletionStreamWithConfig(uid.(uint), body.Question, messages, aiCfg, func(chunk string) {
		fullReply += chunk
		data, _ := json.Marshal(gin.H{"chunk": chunk})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	})
	if err != nil {
		log.Printf("[ai_stream] ERROR uid=%v code=%s: %v", uid, body.Code, err)
		errData, _ := json.Marshal(gin.H{"error": true, "message": err.Error(), "code": response.CodeAIConfigMissing})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(errData))
		c.Writer.Flush()
	} else {
		db.PG.Create(&model.AIConversation{Code: body.Code, Role: "ai", Content: fullReply})
		fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		c.Writer.Flush()
	}
}

// GetScore returns the latest AI comprehensive score for a stock
func (h *AIHandler) GetScore(c *gin.Context) {
	code := c.Param("code")
	var score model.AIStockScore
	err := db.PG.Where("code = ?", code).Order("analyzed_at DESC").First(&score).Error
	if err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, score)
}


// ScoreStockAgent uses agent tools to fetch real data before scoring.
func (h *AIHandler) ScoreStockAgent(code string, uid uint) error {
	sysMsg := h.buildScoringAgentPrompt(code)
	tools := h.buildAgentTools()
	
	var fullReply string
	aiCfg := h.loadSystemConfig("stock_score")
	err := h.svc.ChatCompletionAgentWithModule(uid, []map[string]string{
		{"role": "system", "content": sysMsg},
		{"role": "user", "content": fmt.Sprintf("请对股票 %s 进行全面六维评分。先调用工具获取各维度数据，再输出JSON结果。", code)},
	
	}, tools, aiCfg,
		func(name string, args map[string]interface{}) string {
			return h.executeAgentTool(name, args, code, uid)
		},
		func(chunk string) {
			fullReply += chunk
		},
		"stock_score")
	if err != nil {
		return err
	}
	
	reply := strings.TrimSpace(fullReply)
	if idx := strings.Index(reply, "{"); idx >= 0 {
		reply = reply[idx:]
	}
	if idx := strings.LastIndex(reply, "}"); idx >= 0 {
		reply = reply[:idx+1]
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return err
	}
	
	score := model.AIStockScore{Code: code, AnalyzedAt: time.Now()}
	if v, ok := result["compositeScore"].(float64); ok { score.CompositeScore = v }
	if v, ok := result["fundamentalScore"].(float64); ok { score.FundamentalScore = v }
	if v, ok := result["growthScore"].(float64); ok { score.GrowthScore = v }
	if v, ok := result["valuationScore"].(float64); ok { score.ValuationScore = v }
	if v, ok := result["capitalScore"].(float64); ok { score.CapitalScore = v }
	if v, ok := result["technicalScore"].(float64); ok { score.TechnicalScore = v }
	if v, ok := result["industryScore"].(float64); ok { score.IndustryScore = v }
	if v, ok := result["riskLevel"].(string); ok { score.RiskLevel = v }
	if v, ok := result["suggestion"].(string); ok { score.Suggestion = v }
	if v, ok := result["summary"].(string); ok { score.Summary = v }
	if warnings, ok := result["riskWarnings"].([]interface{}); ok {
		for _, w := range warnings {
			if s, ok := w.(string); ok { score.RiskWarnings = append(score.RiskWarnings, s) }
		}
	}
	return db.PG.Create(&score).Error
}


// toolGetShareholders returns shareholder trends for a stock.
func (h *AIHandler) toolGetShareholders(code string) string {
	type ShRow struct {
		ReportDate         string
		TotalShareholders  float64
		InstitutionRatio   *float64
	}
	var rows []ShRow
	db.PG.Raw(`SELECT TO_CHAR(report_date,'YYYY-MM-DD') as report_date, total_shareholders, institution_ratio FROM stock_shareholders WHERE code=? ORDER BY report_date DESC LIMIT 4`, code).Scan(&rows)
	if len(rows) == 0 {
		return `{"shareholders":[],"trend":"无数据"}`
	}
	b, _ := json.Marshal(map[string]interface{}{"code": code, "shareholders": rows, "count": len(rows)})
	return string(b)
}

// toolGetMyHoldings returns user's current holdings with cost, quantity, and P&L.
func (h *AIHandler) toolGetMyHoldings(userID uint) string {
	var holdings []model.Holding
	log.Printf("[tool_get_holdings] querying for userID=%d", userID)
	result := db.MySQL.Where("user_id = ?", userID).Find(&holdings)
	if result.Error != nil {
		log.Printf("[tool_get_holdings] query error for userID=%d: %v", userID, result.Error)
		return fmt.Sprintf(`{"holdings":[],"totalValue":0,"totalCost":0,"totalPnl":0,"totalPnlPct":0,"message":"查询出错: %s"}`, result.Error.Error())
	}
	log.Printf("[tool_get_holdings] found %d holdings for userID=%d (rows affected: %d)", len(holdings), userID, result.RowsAffected)

	if len(holdings) == 0 {
		return `{"holdings":[],"totalValue":0,"totalCost":0,"totalPnl":0,"totalPnlPct":0,"message":"暂无持仓数据"}`
	}

	// Enrich with current prices
	type PriceRow struct {
		Code  string
		Close float64
		Name  string
	}
	codes := make([]string, len(holdings))
	for i, h := range holdings {
		codes[i] = h.StockCode
	}
	inClause := "'" + strings.Join(codes, "','") + "'"

	var prices []PriceRow
	db.PG.Raw(fmt.Sprintf(`SELECT k.code, k.close, b.name
		FROM stocks_daily_k k
		JOIN stocks_basic b ON b.code = k.code
		WHERE k.code IN (%s)
		AND k.trade_date = (SELECT MAX(trade_date) FROM stocks_daily_k WHERE code = k.code)`, inClause)).Scan(&prices)

	priceMap := make(map[string]PriceRow)
	for _, p := range prices {
		priceMap[p.Code] = p
	}

	// Calculate P&L
	type HoldingResult struct {
		StockCode     string  `json:"stockCode"`
		StockName     string  `json:"stockName"`
		CostPrice     float64 `json:"costPrice"`
		Quantity      int     `json:"quantity"`
		TotalCost     float64 `json:"totalCost"`
		CurrentPrice  float64 `json:"currentPrice"`
		CurrentValue  float64 `json:"currentValue"`
		Pnl           float64 `json:"pnl"`
		PnlPct        float64 `json:"pnlPct"`
		BuyDate       string  `json:"buyDate"`
	}

	results := make([]HoldingResult, 0, len(holdings))
	totalCost := 0.0
	totalValue := 0.0
	for _, h := range holdings {
		tc := h.CostPrice * float64(h.Quantity)
		totalCost += tc
		p, ok := priceMap[h.StockCode]
		cv := tc // default to cost
		if ok && p.Close > 0 {
			cv = p.Close * float64(h.Quantity)
			totalValue += cv
		} else {
			totalValue += tc
		}
		name := h.StockCode
		if ok && p.Name != "" {
			name = p.Name
		}
		pnl := cv - tc
		pnlPct := 0.0
		if tc > 0 {
			pnlPct = pnl / tc * 100
		}
		results = append(results, HoldingResult{
			StockCode:    h.StockCode,
			StockName:    name,
			CostPrice:    h.CostPrice,
			Quantity:     h.Quantity,
			TotalCost:    tc,
			CurrentPrice: p.Close,
			CurrentValue: cv,
			Pnl:          pnl,
			PnlPct:       pnlPct,
			BuyDate:      h.BuyDate,
		})
	}

	totalPnl := totalValue - totalCost
	totalPnlPct := 0.0
	if totalCost > 0 {
		totalPnlPct = totalPnl / totalCost * 100
	}

	b, _ := json.Marshal(map[string]interface{}{
		"holdings":     results,
		"totalValue":   totalValue,
		"totalCost":    totalCost,
		"totalPnl":     totalPnl,
		"totalPnlPct":  totalPnlPct,
		"count":        len(results),
	})
	return string(b)
}

// buildScoringAgentPrompt builds

// buildScoringAgentPrompt builds the system prompt for agent-based AI scoring.
func (h *AIHandler) buildScoringAgentPrompt(code string) string {
	var stock struct{ Name, Industry string }
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)

	cfg := h.loadSystemConfig("stock_score")
	basePrompt := cfg.SystemPrompt
	if basePrompt == "" {
		basePrompt = `你是专业A股量化评分系统。请对股票 __STOCK_CODE__（__STOCK_NAME__，行业：__STOCK_INDUSTRY__）进行六维综合评分。

六维评分标准（每维1-10分，取工具返回的精确数据）：
- fundamentalScore(基本面): 财务健康度（ROE/EPS/利润率/现金流）
- growthScore(成长性): 营收增速/利润增速
- valuationScore(估值): PE/PB分位数与行业对比
- capitalScore(资金面): 成交量变化/量比/换手率
- technicalScore(技术面): 均线趋势/MACD/KDJ/RSI信号
- industryScore(行业景气): 行业政策/景气度/板块表现

综合评分 = 基本面*0.20 + 成长性*0.20 + 估值*0.20 + 资金面*0.15 + 技术面*0.15 + 行业景气*0.10

输出严格JSON（不要代码块标记）：
{"compositeScore":7.2,"fundamentalScore":7.5,"growthScore":6.8,"valuationScore":7.0,"capitalScore":6.5,"technicalScore":7.8,"industryScore":8.0,"riskLevel":"中风险","suggestion":"增持","summary":"...","riskWarnings":["...","..."]}`
	}
	
	toolGuide := `你拥有以下工具可实时查询数据库精确数据，评分前必须先调用工具获取各维度数据：
- get_stock_price: 获取最新价格、PE、PB、市值
- get_kline_summary: 获取近期K线走势（均线、涨跌幅、量价关系）
- get_technical: 获取MACD/KDJ/RSI等技术指标
- get_financials: 获取财务数据（ROE/EPS/营收利润/现金流等）
- get_news: 获取近期新闻和公告
- get_my_holdings: 获取你的持仓数据（成本、数量、盈亏）
- get_shareholders: 获取股东户数和机构持仓比例变化趋势`

	vars := map[string]string{
		"STOCK_CODE": code,
		"STOCK_NAME": stock.Name,
		"STOCK_INDUSTRY": stock.Industry,
	}
	return renderPrompt(basePrompt+"\n\n"+toolGuide, vars)
}

// ScoreStock runs AI scoring for a single stock (reusable, no gin context)
func (h *AIHandler) ScoreStock(code string, uid uint) error {
	// Try agent-based scoring first
	if agentErr := h.ScoreStockAgent(code, uid); agentErr != nil {
		log.Printf("[ai] agent scoring failed for %s, fallback to classic: %v", code, agentErr)
	} else {
		return nil
	}
	
	stockCtx, _ := h.buildScoringContext(code)

	aiCfg := h.loadSystemConfig("stock_score")
	sysPrompt := aiCfg.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = `你是一位资深A股分析师。请全面分析以下股票，从六个维度打分（1-10分），并返回严格JSON格式（不要markdown代码块）：
__STOCK_DATA__

六维评分标准：
- fundamentalScore(基本面): 营收/利润/ROE/现金流等财务健康度
- growthScore(成长性): 营收增速/利润增速/行业空间
- valuationScore(估值): PE/PB分位数/与行业对比
- capitalScore(资金面): 成交量/北向资金/主力资金流向
- technicalScore(技术面): 趋势/均线/MACD/KDJ等指标
- industryScore(行业景气): 行业周期/政策/景气度

综合评分compositeScore为六维加权平均（基本面20%/成长性20%/估值20%/资金面15%/技术面15%/行业景气10%）

额外要求：
- riskWarnings: 列出3-5条风险提示
- riskLevel: 高风险/中高风险/中风险/中低风险/低风险
- suggestion: 强烈买入/买入/增持/持有/减持/卖出/强烈卖出
- summary: 50字以内综合总结

返回格式（严格JSON，不要代码块标记）：
{"compositeScore":7.2,"fundamentalScore":7.5,"growthScore":6.8,"valuationScore":7.0,"capitalScore":6.5,"technicalScore":7.8,"industryScore":8.0,"riskLevel":"中风险","suggestion":"增持","summary":"...","riskWarnings":["...","..."]}`
	}
	sysPrompt = renderPrompt(sysPrompt, map[string]string{"STOCK_DATA": stockCtx})

	reply, err := h.svc.ChatCompletionWithModule(uid, "", []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "请输出JSON格式的六维评分结果。"},
	}, "stock_score")
	if err != nil {
		return err
	}

	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	reply = strings.TrimSpace(reply)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return fmt.Errorf("AI返回格式异常: %%w", err)
	}

	score := model.AIStockScore{
		Code:       code,
		AnalyzedAt: time.Now(),
	}
	if v, ok := result["compositeScore"].(float64); ok { score.CompositeScore = v }
	if v, ok := result["fundamentalScore"].(float64); ok { score.FundamentalScore = v }
	if v, ok := result["growthScore"].(float64); ok { score.GrowthScore = v }
	if v, ok := result["valuationScore"].(float64); ok { score.ValuationScore = v }
	if v, ok := result["capitalScore"].(float64); ok { score.CapitalScore = v }
	if v, ok := result["technicalScore"].(float64); ok { score.TechnicalScore = v }
	if v, ok := result["industryScore"].(float64); ok { score.IndustryScore = v }
	if v, ok := result["riskLevel"].(string); ok { score.RiskLevel = v }
	if v, ok := result["suggestion"].(string); ok { score.Suggestion = v }
	if v, ok := result["summary"].(string); ok { score.Summary = v }
	if warnings, ok := result["riskWarnings"].([]interface{}); ok {
		for _, w := range warnings {
			if s, ok := w.(string); ok { score.RiskWarnings = append(score.RiskWarnings, s) }
		}
	}

	return db.PG.Create(&score).Error
}

// BatchScoreStocks runs AI scoring for multiple stocks asynchronously
func (h *AIHandler) BatchScoreStocks(codes []string, uid uint) {
	go func() {
		log.Printf("[AI批量评分] 开始分析 %%d 只股票", len(codes))
		for i, code := range codes {
			log.Printf("[AI批量评分] %%d/%%d: %%s", i+1, len(codes), code)
			if err := h.ScoreStock(code, uid); err != nil {
				log.Printf("[AI批量评分] %%s 失败: %%v", code, err)
			}
			// Rate limit: sleep between calls
			time.Sleep(2 * time.Second)
		}
		log.Printf("[AI批量评分] 完成，共 %%d 只股票", len(codes))
	}()
}

// RunScore triggers a comprehensive AI scoring analysis
func (h *AIHandler) RunScore(c *gin.Context) {
	code := c.Param("code")

	stockCtx, _ := h.buildScoringContext(code)

	sysPrompt := fmt.Sprintf(`你是一位资深A股分析师。请全面分析以下股票，从六个维度打分（1-10分），并返回严格JSON格式（不要markdown代码块）：
%s

六维评分标准：
- fundamentalScore(基本面): 营收/利润/ROE/现金流等财务健康度
- growthScore(成长性): 营收增速/利润增速/行业空间
- valuationScore(估值): PE/PB分位数/与行业对比
- capitalScore(资金面): 成交量/北向资金/主力资金流向
- technicalScore(技术面): 趋势/均线/MACD/KDJ等指标
- industryScore(行业景气): 行业周期/政策/景气度

综合评分compositeScore为六维加权平均（基本面20%%/成长性20%%/估值20%%/资金面15%%/技术面15%%/行业景气10%%）

额外要求：
- riskWarnings: 列出3-5条风险提示，如估值偏高、解禁压力、政策风险等
- riskLevel: 高风险/中高风险/中风险/中低风险/低风险
- suggestion: 强烈买入/买入/增持/持有/减持/卖出/强烈卖出
- summary: 50字以内综合总结

返回格式（严格JSON，不要代码块标记）：
{"compositeScore":7.2,"fundamentalScore":7.5,"growthScore":6.8,"valuationScore":7.0,"capitalScore":6.5,"technicalScore":7.8,"industryScore":8.0,"riskLevel":"中风险","suggestion":"增持","summary":"...","riskWarnings":["...","..."]}`, stockCtx)

	uid, _ := c.Get("userId")
	reply, err := h.svc.ChatCompletionWithModule(uid.(uint), "", []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "请输出JSON格式的六维评分结果。"},
	}, "stock_score")
	if err != nil {
		handleAIError(c, err)
		return
	}

	// Parse JSON response
	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	reply = strings.TrimSpace(reply)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		response.Error(c, http.StatusOK, response.CodeAIModelError, "AI返回格式异常，请重试")
		return
	}

	// Save to DB
	score := model.AIStockScore{
		Code:       code,
		AnalyzedAt: time.Now(),
	}
	if v, ok := result["compositeScore"].(float64); ok {
		score.CompositeScore = v
	}
	if v, ok := result["fundamentalScore"].(float64); ok {
		score.FundamentalScore = v
	}
	if v, ok := result["growthScore"].(float64); ok {
		score.GrowthScore = v
	}
	if v, ok := result["valuationScore"].(float64); ok {
		score.ValuationScore = v
	}
	if v, ok := result["capitalScore"].(float64); ok {
		score.CapitalScore = v
	}
	if v, ok := result["technicalScore"].(float64); ok {
		score.TechnicalScore = v
	}
	if v, ok := result["industryScore"].(float64); ok {
		score.IndustryScore = v
	}
	if v, ok := result["riskLevel"].(string); ok {
		score.RiskLevel = v
	}
	if v, ok := result["suggestion"].(string); ok {
		score.Suggestion = v
	}
	if v, ok := result["summary"].(string); ok {
		score.Summary = v
	}
	if warnings, ok := result["riskWarnings"].([]interface{}); ok {
		for _, w := range warnings {
			if s, ok := w.(string); ok {
				score.RiskWarnings = append(score.RiskWarnings, s)
			}
		}
	}

	db.PG.Create(&score)

	// Also save as analysis record
	analysis := model.AIAnalysis{
		Code:       code,
		PickDate:   time.Now().Format("2006-01-02"),
		Model:      "ai-scoring",
		RiskLevel:  score.RiskLevel,
		Suggestion: score.Suggestion,
		Summary:    score.Summary,
	}
	if warnings, ok := result["riskWarnings"].([]interface{}); ok {
		for _, w := range warnings {
			if s, ok := w.(string); ok {
				analysis.Signals = append(analysis.Signals, s)
			}
		}
	}
	db.PG.Create(&analysis)

	response.Success(c, score)
}

// handleAIError maps AI service errors to appropriate response codes
func handleAIError(c *gin.Context, err error) {
	msg := err.Error()
	if strings.Contains(msg, "AI配置") || strings.Contains(msg, "AI 配置") || strings.Contains(msg, "未配置") {
		response.Error(c, http.StatusOK, response.CodeAIConfigMissing, msg)
	} else if strings.Contains(msg, "model") || strings.Contains(msg, "api") || strings.Contains(msg, "API") {
		response.Error(c, http.StatusOK, response.CodeAIModelError, msg)
	} else {
		response.InternalError(c, msg)
	}
}

// buildScoringContext gathers richer stock data for scoring analysis
func (h *AIHandler) buildScoringContext(code string) (string, map[string]interface{}) {
	type StockInfo struct {
		Name     string
		Industry string
	}
	type KLineInfo struct {
		Close float64
		High  float64
		Low   float64
		Vol   float64
	}
	type IndicatorInfo struct {
		PE float64
		PB float64
	}

	var stock StockInfo
	if err := db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock).Error; err != nil {
		log.Printf("[ai_handler] stock info query failed for %s: %v", code, err)
	}

	var kline KLineInfo
	if err := db.PG.Raw("SELECT close, high, low, volume FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&kline).Error; err != nil {
		log.Printf("[ai_handler] kline query failed for %s: %v", code, err)
	}

	var ind IndicatorInfo
	if err := db.PG.Raw("SELECT pe, pb FROM stocks_daily_indicator WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&ind).Error; err != nil {
		log.Printf("[ai_handler] indicator query failed for %s: %v", code, err)
	}

	var klines []struct {
		TradeDate string
		Open      float64
		Close     float64
		High      float64
		Low       float64
		Volume    float64
	}
	if err := db.PG.Raw("SELECT trade_date, open, close, high, low, volume FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 20", code).Scan(&klines).Error; err != nil {
		log.Printf("[ai_handler] klines history query failed for %s: %v", code, err)
	}

	klineSummary := ""
	for i := len(klines) - 1; i >= 0; i-- {
		k := klines[i]
		klineSummary += fmt.Sprintf("%s O:%.2f C:%.2f H:%.2f L:%.2f V:%.0f; ", safeSlice(k.TradeDate, 10), k.Open, k.Close, k.High, k.Low, k.Volume)
	}

	extra := map[string]interface{}{
		"name":     stock.Name,
		"industry": stock.Industry,
		"close":    kline.Close,
		"pe":       ind.PE,
		"pb":       ind.PB,
	}

	ctx := fmt.Sprintf(`股票信息：
- 代码：%s
- 名称：%s
- 行业：%s
- 最新收盘价：%.2f
- 最高价：%.2f
- 最低价：%.2f
- 成交量：%.0f
- 市盈率PE：%.2f
- 市净率PB：%.2f

近20日K线走势摘要：
%s`, code, stock.Name, stock.Industry, kline.Close, kline.High, kline.Low, kline.Vol, ind.PE, ind.PB, klineSummary)

	return ctx, extra
}


// GetSystemConfigVars returns available template variables for each scene
func (h *AIHandler) GetSystemConfigVars(c *gin.Context) {
    response.Success(c, ScenePromptVars)
}

// GetSystemConfigs returns all AI system configs (admin)
func (h *AIHandler) GetSystemConfigs(c *gin.Context) {
	var configs []model.AISystemConfig
	db.PG.Order("id ASC").Find(&configs)
	if configs == nil { configs = []model.AISystemConfig{} }
	response.Success(c, configs)
}

// GetSystemConfig returns a single scene config
func (h *AIHandler) GetSystemConfig(c *gin.Context) {
	scene := c.Param("scene")
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", scene).First(&cfg).Error; err != nil {
		response.NotFound(c, "配置不存在")
		return
	}
	response.Success(c, cfg)
}

// UpdateSystemConfig updates an AI system config
func (h *AIHandler) UpdateSystemConfig(c *gin.Context) {
	scene := c.Param("scene")
	var body struct {
		Name         *string  `json:"name"`
		SystemPrompt *string  `json:"systemPrompt"`
		ModelName    *string  `json:"modelName"`
		Temperature  *float64 `json:"temperature"`
		MaxTokens    *int     `json:"maxTokens"`
		EnableSearch *bool    `json:"enableSearch"`
		EnableTools  *bool    `json:"enableTools"`
		AgentModelName *string  `json:"agentModelName"`
		AgentBaseURL   *string  `json:"agentBaseURL"`
		AgentAPIKey    *string  `json:"agentAPIKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", scene).First(&cfg).Error; err != nil {
		response.NotFound(c, "配置不存在")
		return
	}
	updates := map[string]interface{}{}
	if body.Name != nil { updates["name"] = *body.Name }
	if body.SystemPrompt != nil { updates["system_prompt"] = *body.SystemPrompt }
	if body.ModelName != nil { updates["model_name"] = *body.ModelName }
	if body.Temperature != nil { updates["temperature"] = *body.Temperature }
	if body.MaxTokens != nil { updates["max_tokens"] = *body.MaxTokens }
	if body.EnableSearch != nil { updates["enable_search"] = *body.EnableSearch }
	if body.EnableTools != nil { updates["enable_tools"] = *body.EnableTools }
	if body.AgentModelName != nil { updates["agent_model_name"] = *body.AgentModelName }
	if body.AgentBaseURL != nil { updates["agent_base_url"] = *body.AgentBaseURL }
	if body.AgentAPIKey != nil { updates["agent_api_key"] = *body.AgentAPIKey }
	db.PG.Model(&cfg).Updates(updates)
	response.SuccessMsg(c, "ok")
}

// ScenePromptVars defines the available template variables for each scene.
// Admins can insert __VAR_NAME__ in system prompts, replaced at runtime.
var ScenePromptVars = map[string][]struct{ Name, Desc string }{
	"chat_analysis": {
		{"STOCK_CODE", "股票代码"},
		{"STOCK_NAME", "股票名称"},
		{"STOCK_INDUSTRY", "所属行业"},
		{"CURRENT_DATE", "当前日期"},
	},
	"stock_score": {
		{"STOCK_CODE", "股票代码"},
		{"STOCK_NAME", "股票名称"},
		{"STOCK_INDUSTRY", "所属行业"},
		{"STOCK_DATA", "股票分析数据（K线/财务/技术指标等）"},
	},
	"stock_profile": {
		{"STOCK_CODE", "股票代码"},
		{"STOCK_NAME", "股票名称"},
		{"STOCK_DATA", "股票基本面数据"},
	},
	"strategy_gen": {
		{"INDICATORS", "可用指标列表"},
		{"STRATEGY_NAME", "策略名称"},
		{"STRATEGY_DESC", "策略描述"},
		{"STRATEGY_STYLE", "投资风格"},
	},
	"strategy_opt": {
		{"USER_PROMPT", "用户原始描述"},
		{"STRATEGY_STYLE", "风险偏好"},
	},
	"concept_analysis": {
		{"CONCEPT_NAME", "概念板块名称"},
		{"STOCK_COUNT", "成分股数量"},
	},
}

// renderPrompt replaces __VAR__ placeholders in template with values from vars.
func renderPrompt(template string, vars map[string]string) string {
	result := template
	for key, value := range vars {
		result = strings.ReplaceAll(result, "__"+key+"__", value)
	}
	return result
}

// loadSystemConfig loads AI system config for a scene, returning defaults if not found
func (h *AIHandler) loadSystemConfig(scene string) model.AISystemConfig {
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", scene).First(&cfg).Error; err != nil {
		// chat_analysis gets real data injected, no need for web search by default
		enableSearch := scene != "chat_analysis"
		return model.AISystemConfig{
			Scene: scene, Temperature: 0.7, MaxTokens: 2048, EnableSearch: enableSearch,
		}
	}
	return cfg
}

func (h *AIHandler) buildStockContext(code string) string {
	type StockInfo struct {
		Name     string
		Industry string
	}
	type KLineInfo struct {
		Close float64
		High  float64
		Low   float64
		Vol   float64
		TradeDate string
	}
	type IndicatorInfo struct {
		PE float64
		PB float64
	}

	var stock StockInfo
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)

	var kline KLineInfo
	db.PG.Raw("SELECT close, high, low, volume, trade_date FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&kline)

	var ind IndicatorInfo
	db.PG.Raw("SELECT pe, pb FROM stocks_daily_indicator WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&ind)

	now := time.Now()
	cfg := h.loadSystemConfig("chat_analysis")
	prompt := cfg.SystemPrompt
	if prompt == "" {
		// Default prompt with real-time stock data
		prompt = `你是一个专业的A股分析助手。你已拥有该股票的最新精确数据，请直接基于以下数据进行分析回答。

【已注入的股票数据 — 请直接使用，无需联网搜索】
标的：%s（%s）| 行业：%s
收盘价：%.2f | 最高：%.2f | 最低：%.2f | 成交量：%.0f
PE：%.2f | PB：%.2f
数据日期：%s | 分析截止：%s

重要规则：
1. 直接引用上述价格数据，不要说"无法获取实时数据"或"价格会变动"
2. 避免使用"建议您通过交易软件"等推脱话术
3. 数据已注入你的上下文，请当成已知信息使用`
		return fmt.Sprintf(prompt,
			code, stock.Name, stock.Industry,
			kline.Close, safeSlice(kline.TradeDate, 10),
			kline.High, kline.Low, kline.Vol,
			ind.PE, ind.PB,
			now.Format("2006年1月"))
	}
	// Custom prompt: use old 4-arg format for backward compatibility
	if strings.Contains(prompt, "%.2f") {
		// New-style custom prompt with price placeholders
		return fmt.Sprintf(prompt,
			code, stock.Name, stock.Industry,
			kline.Close, safeSlice(kline.TradeDate, 10),
			kline.High, kline.Low, kline.Vol,
			ind.PE, ind.PB,
			now.Format("2006年1月"))
	}
	// Legacy custom prompt with only code/name/industry/date
	return fmt.Sprintf(prompt,
		code, stock.Name, stock.Industry, now.Format("2006年1月"))
}

// ═══════════════════════════════════════════════════════════════
// Agent Mode — Function Calling
// ═══════════════════════════════════════════════════════════════

// analyzeStreamAgent handles the agent loop with tool calling via SSE streaming.
func (h *AIHandler) analyzeStreamAgent(c *gin.Context, code, question string, aiCfg model.AISystemConfig) {
	uid, _ := c.Get("userId")
	w := c.Writer

	// Build system prompt for agent mode (without injected data, agent queries itself)
	sysMsg := h.buildAgentSystemPrompt(code)
	// Load recent history
	var history []model.AIConversation
	db.PG.Where("code = ?", code).Order("created_at DESC").Limit(12).Find(&history)
	messages := []map[string]string{
		{"role": "system", "content": sysMsg},
	}
	total := 0
	const maxCtx = 3000
	for i := len(history) - 1; i >= 0; i-- {
		txt := history[i].Content
		if txt == "" {
			continue
		}
		if total+len(txt) > maxCtx {
			remain := maxCtx - total
			if remain < 20 { break }
			txt = safeSlice(txt, remain) + "…"
		}
		total += len(txt)
		role := history[i].Role
		if role == "ai" { role = "assistant" }
		messages = append(messages, map[string]string{
			"role": role, "content": txt,
		})
		if total >= maxCtx { break }
	}
	// Append current question
	messages = append(messages, map[string]string{"role": "user", "content": question})

	tools := h.buildAgentTools()

	var fullReply string

	err := h.svc.ChatCompletionAgentStream(uid.(uint), messages, aiCfg, tools,
		func(name string, args map[string]interface{}) string {
			return h.executeAgentTool(name, args, code, uid.(uint))
		},
		func(eventType string, data map[string]string) {
			// Send tool status event to frontend
			evt := map[string]interface{}{"status": eventType}
			for k, v := range data { evt[k] = v }
			b, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", string(b))
			w.Flush()
		},
		func(chunk string) {
			fullReply += chunk
			b, _ := json.Marshal(gin.H{"chunk": chunk})
			fmt.Fprintf(w, "data: %s\n\n", string(b))
			w.Flush()
		},
	)
	if err != nil {
		log.Printf("[ai_agent] ERROR uid=%v code=%s: %v", uid, code, err)
		errData, _ := json.Marshal(gin.H{"error": true, "message": err.Error(), "code": response.CodeAIConfigMissing})
		fmt.Fprintf(w, "data: %s\n\n", string(errData))
		w.Flush()
	} else {
		if fullReply != "" {
			db.PG.Create(&model.AIConversation{Code: code, Role: "ai", Content: fullReply})
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		w.Flush()
	}
}

// buildAgentSystemPrompt builds system prompt for agent mode.
// Loads from DB ai_system_configs (scene=chat_analysis), falls back to built-in default.
func (h *AIHandler) buildAgentSystemPrompt(code string) string {
	var stock struct{ Name, Industry string }
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)
	now := time.Now()

	// Try DB custom prompt first
	cfg := h.loadSystemConfig("chat_analysis")
	vars := map[string]string{
		"STOCK_CODE": code,
		"STOCK_NAME": stock.Name,
		"STOCK_INDUSTRY": stock.Industry,
		"CURRENT_DATE": now.Format("2006年1月"),
	}
	if cfg.SystemPrompt != "" && cfg.EnableTools {
		// Agent mode: render template with vars
		return renderPrompt(cfg.SystemPrompt, vars)
	}

	// Fallback built-in agent prompt
	fallback := `你是专业A股分析助手。当前分析标的：__STOCK_CODE__ __STOCK_NAME__（行业：__STOCK_INDUSTRY__）

你拥有以下工具可以实时查询数据库中的精确数据：
- get_stock_price: 获取最新价格、PE/PB、成交量
- get_kline_summary: 获取近期K线走势摘要（均线、涨跌幅）
- get_technical: 获取MACD/KDJ/RSI等技术指标
- get_financials: 获取财务数据（ROE/EPS/营收/利润等）
- get_news: 获取近期新闻和公告
- get_my_holdings: 获取你的持仓数据（成本、数量、盈亏）
- get_shareholders: 获取股东户数和机构持仓比例变化趋势

使用规则：
1. 按问题需求调用工具，只调用回答问题必需的工具
2. 涉及多只股票时最多深入分析 3 只，其余简要带过
3. 优先用自然语言回答，贴合用户问题。仅在需要结构化展示时使用 Widget
4. Widget 格式（可选，按需使用，w字段必填）：
{"w":"summary","label":"短线看多","text":"综合判断≤80字"}
{"w":"signal","u":true,"h":"信号≤10字","d":"说明≤30字"}
{"w":"risk","h":"风险≤10字","d":"说明≤30字"}
{"w":"list","t":"标题≤8字","items":["条目1","条目2","条目3"]}
{"w":"alert","level":"warning","title":"注意","body":"说明"}
{"w":"panel","t":"标题","rows":[{"k":"指标","v":"数值"}]}
{"w":"plan","s":支撑价,"r":压力价,"tip":"建议≤20字","pos":30}
严禁自创格式，必须使用 w 字段。不要用代码块包裹JSON。
5. 分析截止时间：__CURRENT_DATE__`
	return renderPrompt(fallback, vars)
}

// buildAgentTools returns the tool definitions for DeepSeek Function Calling.
func (h *AIHandler) buildAgentTools() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_stock_price",
				"description": "获取股票最新价格、PE、PB、成交量和市值数据",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{"type": "string", "description": "股票代码，如 300059"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_kline_summary",
				"description": "获取近期K线走势摘要，包含均线、涨跌幅、最高最低等信息",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{"type": "string", "description": "股票代码"},
						"days": map[string]interface{}{"type": "integer", "description": "回溯天数，默认20"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_technical",
				"description": "获取技术指标数据：MACD、KDJ、RSI、布林带等",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{"type": "string", "description": "股票代码"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_financials",
				"description": "获取最新财务报表数据：ROE、EPS、营收、利润、毛利率等",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{"type": "string", "description": "股票代码"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_news",
				"description": "获取股票近期重要新闻、公告和研报标题",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code":  map[string]interface{}{"type": "string", "description": "股票代码"},
						"limit": map[string]interface{}{"type": "integer", "description": "返回条数，默认5"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_shareholders",
				"description": "获取股票近期股东户数变化和机构持仓比例趋势，用于分析筹码集中度和机构动向",
				"parameters": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"code": map[string]interface{}{"type": "string", "description": "股票代码"},
					},
					"required": []string{"code"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "get_my_holdings",
				"description": "获取用户持仓概览（成本/数量/盈亏）。收到持仓后只选盈亏最大或仓位最重的2-3只深入分析，其余简要带过即可。严禁逐个分析所有持仓股票。",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
	}
}

// executeAgentTool executes a single tool call against the database.
func (h *AIHandler) executeAgentTool(name string, args map[string]interface{}, defaultCode string, userID uint) string {
	code, ok := args["code"].(string)
	if !ok || code == "" {
		code = defaultCode
	}

	switch name {
	case "get_stock_price":
		return h.toolGetStockPrice(code)

	case "get_kline_summary":
		days := 20
		if d, ok := args["days"].(float64); ok {
			days = int(d)
		}
		return h.toolGetKlineSummary(code, days)

	case "get_technical":
		return h.toolGetTechnical(code)

	case "get_financials":
		return h.toolGetFinancials(code)

	case "get_news":
		limit := 5
		if l, ok := args["limit"].(float64); ok {
			limit = int(l)
		}
		return h.toolGetNews(code, limit)

	case "get_my_holdings":
		return h.toolGetMyHoldings(userID)

	case "get_shareholders":
		return h.toolGetShareholders(code)

	default:
		return `{"error": "unknown tool: ` + name + `"}`
	}
}

// ── Tool implementations ──

func (h *AIHandler) toolGetStockPrice(code string) string {
	type Row struct {
		Close, High, Low, Volume float64
		PE, PB, TotalMV, CircMV  float64
		TradeDate                string
		Name, Industry           string
	}
	var r Row
	db.PG.Raw(`SELECT k.close, k.high, k.low, k.volume, TO_CHAR(k.trade_date,'YYYY-MM-DD') as trade_date,
		i.pe, i.pb, i.total_market_cap, i.circulating_market_cap,
		b.name, b.industry
		FROM stocks_daily_k k
		LEFT JOIN stocks_daily_indicator i ON i.code=k.code AND i.trade_date=k.trade_date
		LEFT JOIN stocks_basic b ON b.code=k.code
		WHERE k.code=? ORDER BY k.trade_date DESC LIMIT 1`, code).Scan(&r)

	return fmt.Sprintf(`{"code":"%s","name":"%s","industry":"%s","close":%.2f,"high":%.2f,"low":%.2f,"volume":%.0f,"pe":%.2f,"pb":%.2f,"totalMV":%.0f,"circMV":%.0f,"tradeDate":"%s"}`,
		code, r.Name, r.Industry, r.Close, r.High, r.Low, r.Volume, r.PE, r.PB, r.TotalMV, r.CircMV, r.TradeDate)
}

func (h *AIHandler) toolGetKlineSummary(code string, days int) string {
	type Row struct {
		Close, High, Low, Volume float64
		TradeDate                string
	}
	var rows []Row
	db.PG.Raw(`SELECT close, high, low, volume, TO_CHAR(trade_date,'YYYY-MM-DD') as trade_date
		FROM stocks_daily_k WHERE code=? ORDER BY trade_date DESC LIMIT ?`, code, days).Scan(&rows)

	if len(rows) == 0 {
		return `{"error":"no kline data"}`
	}

	// Calculate summary: MA5, MA10, MA20, change%, amplitude
	n := len(rows)
	first := rows[n-1] // earliest
	last := rows[0]    // latest
	chgPct := (last.Close - first.Close) / first.Close * 100

	ma5, ma10 := 0.0, 0.0
	c5, c10 := 0, 0
	highAll, lowAll := last.High, last.Low
	for i, r := range rows {
		if r.High > highAll { highAll = r.High }
		if r.Low < lowAll { lowAll = r.Low }
		if i < 5 { ma5 += r.Close; c5++ }
		if i < 10 { ma10 += r.Close; c10++ }
	}
	if c5 > 0 { ma5 /= float64(c5) }
	if c10 > 0 { ma10 /= float64(c10) }

	amp := (highAll - lowAll) / first.Close * 100

	return fmt.Sprintf(`{"code":"%s","days":%d,"startDate":"%s","endDate":"%s","startPrice":%.2f,"endPrice":%.2f,"changePct":%.2f,"ma5":%.2f,"ma10":%.2f,"highAll":%.2f,"lowAll":%.2f,"amplitude":%.2f}`,
		code, n, first.TradeDate, last.TradeDate, first.Close, last.Close, chgPct, ma5, ma10, highAll, lowAll, amp)
}


func (h *AIHandler) toolGetTechnical(code string) string {
	type KRow struct {
		Close, High, Low, Volume, TurnoverRate float64
		TradeDate                              string
	}
	var krows []KRow
	db.PG.Raw(`SELECT close, high, low, volume, turnover_rate, TO_CHAR(trade_date,'YYYY-MM-DD') as trade_date FROM stocks_daily_k WHERE code=? ORDER BY trade_date DESC LIMIT 20`, code).Scan(&krows)
	if len(krows) < 5 { return `{"error":"insufficient data for technical analysis"}` }
	n := len(krows); latest := krows[0]
	ma5, ma10, ma20 := 0.0, 0.0, 0.0
	for i := 0; i < n && i < 5; i++ { ma5 += krows[i].Close }
	for i := 0; i < n && i < 10; i++ { ma10 += krows[i].Close }
	for i := 0; i < n && i < 20; i++ { ma20 += krows[i].Close }
	c5 := float64(min(5, n)); ma5 /= c5
	c10 := float64(min(10, n)); ma10 /= c10
	c20 := float64(min(20, n)); ma20 /= c20
	trend := "震荡整理"
	if ma5 > ma10 && ma10 > ma20 { trend = "多头排列" } else if ma5 < ma10 && ma10 < ma20 { trend = "空头排列" } else if ma5 > ma10 { trend = "短期偏多" } else { trend = "短期偏空" }
	chg5 := (latest.Close - krows[min(4, n-1)].Close) / krows[min(4, n-1)].Close * 100
	vol5, volPrev5 := 0.0, 0.0
	for i := 0; i < min(5, n); i++ { vol5 += krows[i].Volume }
	for i := 5; i < min(10, n); i++ { volPrev5 += krows[i].Volume }
	vol5 /= float64(min(5, n))
	volTrend := "持平"
	if min(10, n) > 5 { volPrev5 /= float64(min(10, n)-5); if volPrev5 > 0 { r := vol5/volPrev5*100 - 100; if r > 20 { volTrend = "放量" } else if r < -20 { volTrend = "缩量" } } }
	high20, low20 := latest.High, latest.Low
	for _, r := range krows { if r.High > high20 { high20 = r.High }; if r.Low < low20 { low20 = r.Low } }
	posInRange := (latest.Close - low20) / (high20 - low20) * 100
	return fmt.Sprintf(`{"code":"%s","tradeDate":"%s","close":%.2f,"ma5":%.2f,"ma10":%.2f,"ma20":%.2f,"trend":"%s","chg5d":%.2f,"volTrend":"%s","posIn20dRange":%.1f,"high20d":%.2f,"low20d":%.2f}`, code, latest.TradeDate, latest.Close, ma5, ma10, ma20, trend, chg5, volTrend, posInRange, high20, low20)
}
func (h *AIHandler) toolGetFinancials(code string) string {
	type Row struct {
		ReportDate, ReportType                string
		TotalRevenue, NetProfit               float64
		RevenueGrowth, ProfitGrowth           float64
		ROE, EPS, BPS, GrossMargin, NetMargin float64
		DebtRatio                             float64
	}
	var rows []Row
	db.PG.Raw(`SELECT report_date, report_type, total_revenue, net_profit,
		revenue_growth, profit_growth, roe, eps, bps, gross_margin, net_margin, debt_ratio
		FROM stock_financials WHERE code=? ORDER BY report_date DESC LIMIT 3`, code).Scan(&rows)

	if len(rows) == 0 {
		return `{"error":"no financial data"}`
	}

	r := rows[0]
	return fmt.Sprintf(`{"code":"%s","latestReport":"%s","reportType":"%s","revenue":%.2f,"netProfit":%.2f,"revenueGrowth":%.2f,"profitGrowth":%.2f,"roe":%.2f,"eps":%.2f,"bps":%.2f,"grossMargin":%.2f,"netMargin":%.2f,"debtRatio":%.2f,"note":"单位：万元(营收/利润)，%%(增长率/比率)"}`,
		code, r.ReportDate, r.ReportType, r.TotalRevenue, r.NetProfit,
		r.RevenueGrowth, r.ProfitGrowth, r.ROE, r.EPS, r.BPS,
		r.GrossMargin, r.NetMargin, r.DebtRatio)
}

func (h *AIHandler) toolGetNews(code string, limit int) string {
	type Row struct {
		Title, Summary, Source, PublishDate, NewsType string
	}
	var rows []Row
	db.PG.Raw(`SELECT title, summary, source, publish_date, news_type
		FROM stock_news WHERE code=? ORDER BY publish_date DESC LIMIT ?`, code, limit).Scan(&rows)

	if len(rows) == 0 {
		return `{"error":"no news data"}`
	}

	var items []string
	for _, r := range rows {
		summary := r.Summary
		if len(summary) > 80 { summary = summary[:80] + "..." }
		items = append(items, fmt.Sprintf(`{"title":"%s","summary":"%s","source":"%s","date":"%s"}`,
			r.Title, summary, r.Source, r.PublishDate))
	}
	return fmt.Sprintf(`{"code":"%s","count":%d,"items":[%s]}`, code, len(rows), strings.Join(items, ","))
}

// ── Stock Profile (Markdown + 6-dim scores) ──

// GetProfile returns the latest AI-generated stock profile
func (h *AIHandler) GetProfile(c *gin.Context) {
	code := c.Param("code")
	var profile model.StockProfile
	if err := db.PG.Where("code = ?", code).Order("analyzed_at DESC").First(&profile).Error; err != nil {
		response.Success(c, nil)
		return
	}
	response.Success(c, profile)
}

// RunProfile generates AI stock profile synchronously and returns the result
func (h *AIHandler) RunProfile(c *gin.Context) {
	code := c.Param("code")
	uid, _ := c.Get("userId")

	// Build context data
	dataCtx, _ := h.buildProfileDataContext(code)
	if dataCtx == nil {
		response.Error(c, http.StatusOK, response.CodeNotFound, "股票数据不存在")
		return
	}

	// Load system prompt
	cfg := h.loadSystemConfig("stock_profile")
	sysPrompt := cfg.SystemPrompt
	if sysPrompt == "" {
		sysPrompt = h.defaultProfilePrompt()
	}

	// Call AI
	reply, err := h.svc.ChatCompletionWithModule(uid.(uint), string(dataCtx), []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "请输出JSON格式的公司简介。"},
	}, "stock_profile")
	if err != nil {
		handleAIError(c, err)
		return
	}

	// Parse response
	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	reply = strings.TrimSpace(reply)

	var result struct {
		ProfileMarkdown string `json:"profileMarkdown"`
	}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		response.Error(c, http.StatusOK, response.CodeAIModelError, "AI返回格式异常，请重试")
		return
	}

	// Save to DB
	db.PG.Where("code = ?", code).Assign(map[string]interface{}{
		"profile_markdown": result.ProfileMarkdown,
		"analyzed_at":      time.Now(),
		"updated_at":        time.Now(),
	}).FirstOrCreate(&model.StockProfile{Code: code})

	response.Success(c, gin.H{"code": code, "profileMarkdown": result.ProfileMarkdown, "analyzedAt": time.Now()})
}

// buildProfileDataContext gathers stock data for profile generation
func (h *AIHandler) buildProfileDataContext(code string) ([]byte, *model.StockBasic) {
	var stock model.StockBasic
	if err := db.PG.Where("code = ?", code).First(&stock).Error; err != nil {
		return nil, nil
	}

	type KLineRow struct {
		TradeDate   string
		Close, Volume, TurnoverRate float64
	}
	var klines []KLineRow
	db.PG.Raw("SELECT TO_CHAR(trade_date,'YYYY-MM-DD') as trade_date, close, volume, turnover_rate FROM stocks_daily_k WHERE code=? ORDER BY trade_date DESC LIMIT 30", code).Scan(&klines)

	type FinRow struct {
		ReportDate, ReportType string
		TotalRevenue, NetProfit, RevenueGrowth, ProfitGrowth, ROE, EPS, GrossMargin, NetMargin, DebtRatio float64
	}
	var fins []FinRow
	db.PG.Raw("SELECT TO_CHAR(report_date,'YYYY-MM-DD') as report_date, report_type, total_revenue, net_profit, revenue_growth, profit_growth, roe, eps, gross_margin, net_margin, debt_ratio FROM stock_financials WHERE code=? ORDER BY report_date DESC LIMIT 4", code).Scan(&fins)

	type IndRow struct{ PE, TotalMarketCap float64 }
	var ind IndRow
	db.PG.Raw("SELECT pe, total_market_cap FROM stocks_daily_indicator WHERE code=? ORDER BY trade_date DESC LIMIT 1", code).Scan(&ind)

	type NewsRow struct{ Title, PublishDate string }
	var news []NewsRow
	db.PG.Raw("SELECT title, TO_CHAR(publish_date,'YYYY-MM-DD') as publish_date FROM stock_news WHERE code=? ORDER BY publish_date DESC LIMIT 10", code).Scan(&news)

	type ShRow struct{ ReportDate string; TotalShareholders, InstitutionRatio float64 }
	var shs []ShRow
	db.PG.Raw("SELECT TO_CHAR(report_date,'YYYY-MM-DD') as report_date, total_shareholders, institution_ratio FROM stock_shareholders WHERE code=? ORDER BY report_date DESC LIMIT 3", code).Scan(&shs)

	data := map[string]interface{}{
		"code": code, "name": stock.Name, "industry": stock.Industry,
		"conceptTags": stock.ConceptTags,
		"klines": klines, "financials": fins, "indicator": ind,
		"news": news, "shareholders": shs,
	}
	b, _ := json.Marshal(data)
	return b, &stock
}

// defaultProfilePrompt returns the built-in stock profile prompt
func (h *AIHandler) defaultProfilePrompt() string {
	return `你是一位专业、客观、严谨的金融投资分析师，精通A股市场。
你的任务是对给定的股票进行深度分析，生成一份精美的结构化 Markdown 公司简介。

## 简介结构（严格按此顺序）
1. **🏢 核心特征** — 一句话概括公司定位、盈利模式和当前经营状态
2. **💼 主营业务** — 业务结构、护城河来源、行业地位
3. **📊 最新财报** — 表格展示关键财务数据，分析变化原因
4. **🚀 成长驱动** — 短期和长期增长因素
5. **⚠️ 风险提示** — 3-5条具体风险
6. **🔮 未来展望** — 至少2个前瞻方向

## 格式：Markdown 表格/引用/标题，每部分200字

输出严格JSON：{"profileMarkdown":"..."}`
}

// RunProfileBatch triggers batch AI profile generation (async, for collector)
func (h *AIHandler) RunProfileBatch(c *gin.Context) {
	go func() {
		if err := exec.Command("python3",
			filepath.Join(scriptsRoot(), "stock_profile_collect.py"),
			"--batch",
		).Run(); err != nil {
			log.Printf("[profile] batch failed: %v", err)
		}
	}()
	response.SuccessMsg(c, "已触发批量简介采集，请稍后刷新查看")
}

var scriptsRoot = func() string {
	if root := os.Getenv("APP_ROOT"); root != "" {
		return filepath.Join(root, "scripts/collector")
	}
	_, f, _, _ := runtime.Caller(0)
	base := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(f))))
	return filepath.Join(base, "scripts/collector")
}
