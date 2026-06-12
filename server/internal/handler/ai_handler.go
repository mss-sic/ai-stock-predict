package handler

import (
	"encoding/json"
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
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
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

	sysMsg := h.buildStockContext(body.Code)

	var fullReply string
	uid, _ := c.Get("userId")
	err := h.svc.ChatCompletionStream(uid.(uint), body.Question, []map[string]string{
		{"role": "system", "content": sysMsg},
	}, func(chunk string) {
		fullReply += chunk
		data, _ := json.Marshal(gin.H{"chunk": chunk})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	})
	if err != nil {
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

// ScoreStock runs AI scoring for a single stock (reusable, no gin context)
func (h *AIHandler) ScoreStock(code string, uid uint) error {
	stockCtx, _ := h.buildScoringContext(code)

	sysPrompt := fmt.Sprintf(`你是一位资深A股分析师。请全面分析以下股票，从六个维度打分（1-10分），并返回严格JSON格式（不要markdown代码块）：
%%s

六维评分标准：
- fundamentalScore(基本面): 营收/利润/ROE/现金流等财务健康度
- growthScore(成长性): 营收增速/利润增速/行业空间
- valuationScore(估值): PE/PB分位数/与行业对比
- capitalScore(资金面): 成交量/北向资金/主力资金流向
- technicalScore(技术面): 趋势/均线/MACD/KDJ等指标
- industryScore(行业景气): 行业周期/政策/景气度

综合评分compositeScore为六维加权平均（基本面20%%%%/成长性20%%%%/估值20%%%%/资金面15%%%%/技术面15%%%%/行业景气10%%%%）

额外要求：
- riskWarnings: 列出3-5条风险提示
- riskLevel: 高风险/中高风险/中风险/中低风险/低风险
- suggestion: 强烈买入/买入/增持/持有/减持/卖出/强烈卖出
- summary: 50字以内综合总结

返回格式（严格JSON，不要代码块标记）：
{"compositeScore":7.2,"fundamentalScore":7.5,"growthScore":6.8,"valuationScore":7.0,"capitalScore":6.5,"technicalScore":7.8,"industryScore":8.0,"riskLevel":"中风险","suggestion":"增持","summary":"...","riskWarnings":["...","..."]}`, stockCtx)

	reply, err := h.svc.ChatCompletion(uid, "", []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "请输出JSON格式的六维评分结果。"},
	})
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
	reply, err := h.svc.ChatCompletion(uid.(uint), "", []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": "请输出JSON格式的六维评分结果。"},
	})
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

func (h *AIHandler) buildStockContext(code string) string {
	type StockInfo struct {
		Name     string
		Industry string
	}
	var stock StockInfo
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)

	now := time.Now()
	return fmt.Sprintf(`你是一个专业、严谨、深度的金融分析助手。

当前分析的股票：%s（%s），行业：%s。

回答问题时，必须遵循以下步骤：
1. 明确分析的时间范围和依据。
2. 分财务数据、战略动向、风险挑战三个维度展开。
3. 每个维度至少列出3个要点，并使用数据支撑。
4. 最终给出明确的总结与展望。
5. 输出格式使用紧凑型 Markdown，关键数据用表格呈现。
   格式约束：
   - 段落间不留空行，标题上下最多空一行。
   - 表格列数 ≤4，超出拆分为多个小表格，每个表格 ≤6 行。
   - 不使用代码块包裹正文，仅代码/公式使用行内代码。
   - 列表用短横线开头，紧凑排列无需额外空行。
   - 数字右对齐、文字左对齐，百分比保留 1 位小数。
   - 避免使用 > 引用块和 HTML。

重要要求：
- 请务必基于联网搜索的最新信息回答，确保数据时效性截止至%s。
- 涉及操作建议时请强调风险，声明"不构成投资建议"。
- 对不确定的信息请注明"待核实"。`,
		code, stock.Name, stock.Industry, now.Format("2006年1月"))
}
