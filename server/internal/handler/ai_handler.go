package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

type AIHandler struct {
	svc *service.AIService
}

func NewAIHandler() *AIHandler {
	return &AIHandler{svc: service.NewAIService()}
}

// GetHistory returns conversation history for a stock
func (h *AIHandler) GetHistory(c *gin.Context) {
	code := c.Param("code")
	msgs := make([]model.AIConversation, 0)
	db.PG.Where("code = ?", code).Order("created_at ASC").Find(&msgs)
	c.JSON(http.StatusOK, gin.H{"data": msgs, "code": code})
}

// ClearHistory deletes all conversation history for a stock
func (h *AIHandler) ClearHistory(c *gin.Context) {
	code := c.Param("code")
	db.PG.Where("code = ?", code).Delete(&model.AIConversation{})
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *AIHandler) Analyze(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	// Save user message
	db.PG.Create(&model.AIConversation{Code: body.Code, Role: "user", Content: body.Question})

	// Build system context
	sysMsg := h.buildStockContext(body.Code)

	// Build history context from recent messages (last 20)
	var history []model.AIConversation
	db.PG.Where("code = ?", body.Code).Order("created_at DESC").Limit(20).Find(&history)
	// Reverse to chronological order
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

	reply, err := h.svc.ChatCompletion(body.Question, messages)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"reply": "AI服务暂不可用: " + err.Error(), "code": body.Code}})
		return
	}

	// Save AI reply
	db.PG.Create(&model.AIConversation{Code: body.Code, Role: "ai", Content: reply})

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"reply": reply, "code": body.Code}})
}

func (h *AIHandler) AnalyzeStream(c *gin.Context) {
	var body struct {
		Code     string `json:"code"`
		Question string `json:"question"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
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
	err := h.svc.ChatCompletionStream(body.Question, []map[string]string{
		{"role": "system", "content": sysMsg},
	}, func(chunk string) {
		fullReply += chunk
		data, _ := json.Marshal(gin.H{"chunk": chunk})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	})
	if err != nil {
		data, _ := json.Marshal(gin.H{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
		c.Writer.Flush()
	} else {
		// Save AI reply
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
		c.JSON(http.StatusOK, gin.H{"data": nil, "code": code})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": score, "code": code})
}

// RunScore triggers a comprehensive AI scoring analysis
func (h *AIHandler) RunScore(c *gin.Context) {
	code := c.Param("code")

	// Gather stock data
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
- riskWarnings: 列出3-5条具体风险提示（如：估值偏高，PE位于历史XX%%分位；限售股解禁；行业受政策影响敏感等）
- riskLevel: 高风险/中高风险/中风险/中低风险/低风险
- suggestion: 强烈买入/买入/增持/持有/减持/卖出/强烈卖出
- summary: 200字以内综合分析

返回JSON格式：
{
  "compositeScore": 7.5,
  "fundamentalScore": 8,
  "growthScore": 7,
  "valuationScore": 6,
  "capitalScore": 7,
  "technicalScore": 8,
  "industryScore": 7,
  "riskLevel": "中风险",
  "suggestion": "增持",
  "riskWarnings": ["风险1", "风险2"],
  "summary": "综合分析..."
}`, stockCtx)

	messages := []map[string]string{
		{"role": "system", "content": sysPrompt},
		{"role": "user", "content": fmt.Sprintf("请对%s进行全面六维分析打分", code)},
	}

	reply, err := h.svc.ChatCompletion("", messages)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"error": err.Error()})
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
		c.JSON(http.StatusOK, gin.H{"error": "AI返回格式异常，请重试", "raw": reply})
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

	c.JSON(http.StatusOK, gin.H{"data": score})
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
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)

	var kline KLineInfo
	db.PG.Raw("SELECT close, high, low, volume FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&kline)

	var ind IndicatorInfo
	db.PG.Raw("SELECT pe, pb FROM stocks_daily_indicator WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&ind)

	// Get recent K-line summary (last 20 days)
	var klines []struct {
		TradeDate string
		Open      float64
		Close     float64
		High      float64
		Low       float64
		Volume    float64
	}
	db.PG.Raw("SELECT trade_date, open, close, high, low, volume FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 20", code).Scan(&klines)

	klineSummary := ""
	for i := len(klines) - 1; i >= 0; i-- {
		k := klines[i]
		klineSummary += fmt.Sprintf("%s O:%.2f C:%.2f H:%.2f L:%.2f V:%.0f; ", k.TradeDate[:10], k.Open, k.Close, k.High, k.Low, k.Volume)
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
	type KLineInfo struct {
		Close float64
	}
	type IndicatorInfo struct {
		PE float64
		PB float64
	}

	var stock StockInfo
	db.PG.Raw("SELECT name, industry FROM stocks_basic WHERE code = ?", code).Scan(&stock)
	var kline KLineInfo
	db.PG.Raw("SELECT close FROM stocks_daily_k WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&kline)
	var ind IndicatorInfo
	db.PG.Raw("SELECT pe, pb FROM stocks_daily_indicator WHERE code = ? ORDER BY trade_date DESC LIMIT 1", code).Scan(&ind)

	return fmt.Sprintf(`你是一个专业的A股投资分析助手。当前分析的股票信息：
- 代码：%s
- 名称：%s
- 行业：%s
- 最新收盘价：%.2f
- 市盈率PE：%.2f
- 市净率PB：%.2f

请基于以上信息，用专业、简洁的语言回答用户的问题。涉及建议时请强调风险，不构成投资建议。`,
		code, stock.Name, stock.Industry, kline.Close, ind.PE, ind.PB)
}
