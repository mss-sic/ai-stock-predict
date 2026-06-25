package handler

import (
	"fmt"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

// ConceptAnalysisPrompt is the system prompt template for concept board AI analysis
var ConceptAnalysisPrompt = "你是一位资深证券分析师，请对以下概念板块进行全面分析。\n\n" +
	"## 要求\n" +
	"1. **概念概述**：用一段话简要介绍该概念的核心定义、行业背景\n" +
	"2. **龙头股票**：列出3-5只核心龙头股（代码+名称+简要逻辑）\n" +
	"3. **商业模式**：分析该概念的典型商业模式和盈利模式\n" +
	"4. **利润拆分**：拆解产业链各环节的利润分配（上游/中游/下游）\n" +
	"5. **上下游产业链**：详细分析上游供应商、中游制造/服务、下游应用\n" +
	"6. **投资逻辑**：核心投资逻辑和关键跟踪指标\n" +
	"7. **风险提示**：行业面临的主要风险\n\n" +
	"请使用专业的Markdown格式输出，适当使用表格和列表，语言精炼专业。"

// GetConceptAnalysis returns or generates AI analysis for a concept board
func (h *BoardHandler) GetConceptAnalysis(c *gin.Context) {
	conceptCode := c.Param("code")
	forceRefresh := c.Query("refresh") == "1"

	var board model.ConceptBoard
	if err := db.PG.Where("concept_code = ?", conceptCode).First(&board).Error; err != nil {
		response.NotFound(c, "板块不存在")
		return
	}

	// Check cache
	var analysis model.ConceptAnalysis
	cached := db.PG.Where("concept_code = ?", conceptCode).First(&analysis).Error == nil

	if cached && !forceRefresh {
		response.Success(c, gin.H{
			"conceptCode": conceptCode,
			"conceptName": board.ConceptName,
			"content":     analysis.Content,
			"generatedAt": analysis.GeneratedAt,
			"cached":      true,
		})
		return
	}

	// Build prompt
	prompt := fmt.Sprintf("请分析以下概念板块：\n\n概念名称：%s\n成分股数量：%d只\n\n%s",
		board.ConceptName, board.StockCount, ConceptAnalysisPrompt)

	// Call AI
	userID := c.GetUint("userID")
	aiSvc := service.NewAIService()
	aiContent, err := aiSvc.ChatCompletionWithTokensModule(userID, prompt, nil, 4096, "concept_analysis")
	if err != nil {
		response.InternalError(c, "AI分析失败: "+err.Error())
		return
	}

	// Save to DB
	now := time.Now()
	if cached {
		db.PG.Model(&analysis).Updates(map[string]interface{}{
			"content":      aiContent,
			"generated_at": now,
			"updated_at":   now,
		})
	} else {
		db.PG.Create(&model.ConceptAnalysis{
			ConceptCode: conceptCode,
			Content:     aiContent,
			GeneratedAt: now,
		})
	}

	response.Success(c, gin.H{
		"conceptCode": conceptCode,
		"conceptName": board.ConceptName,
		"content":     aiContent,
		"generatedAt": now,
		"cached":      false,
	})
}

// UpdateConceptAnalysisPrompt updates the analysis prompt
func (h *BoardHandler) UpdateConceptAnalysisPrompt(c *gin.Context) {
	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Prompt == "" {
		response.BadRequest(c, "请提供prompt")
		return
	}
	ConceptAnalysisPrompt = body.Prompt
	response.SuccessMsg(c, "提示词已更新")
}
