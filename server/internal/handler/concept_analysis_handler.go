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

// conceptAnalysisDefaultPrompt is the fallback system prompt when DB has no config.
const conceptAnalysisDefaultPrompt = "你是一位资深证券分析师，请对以下概念板块进行全面分析。\n\n" +
	"## 要求\n" +
	"1. **概念概述**：用一段话简要介绍该概念的核心定义、行业背景\n" +
	"2. **龙头股票**：列出3-5只核心龙头股（代码+名称+简要逻辑）\n" +
	"3. **商业模式**：分析该概念的典型商业模式和盈利模式\n" +
	"4. **利润拆分**：拆解产业链各环节的利润分配（上游/中游/下游）\n" +
	"5. **上下游产业链**：详细分析上游供应商、中游制造/服务、下游应用\n" +
	"6. **投资逻辑**：核心投资逻辑和关键跟踪指标\n" +
	"7. **风险提示**：行业面临的主要风险\n\n" +
	"请使用专业的Markdown格式输出，适当使用表格和列表，语言精炼专业。"

// loadConceptAnalysisPrompt returns the system prompt from DB or default.
func loadConceptAnalysisPrompt() string {
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", "concept_analysis").First(&cfg).Error; err != nil {
		return conceptAnalysisDefaultPrompt
	}
	if cfg.SystemPrompt == "" {
		return conceptAnalysisDefaultPrompt
	}
	return cfg.SystemPrompt
}

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

	// refresh=false 且有缓存 → 直接返回缓存
	if cached {
		if !forceRefresh {
			response.Success(c, gin.H{
				"conceptCode": conceptCode,
				"conceptName": board.ConceptName,
				"content":     analysis.Content,
				"generatedAt": analysis.GeneratedAt,
				"cached":      true,
			})
			return
		}
	}

	// refresh=false 且无缓存 → 不调用AI，直接返回空
	if !forceRefresh {
		response.Success(c, gin.H{
			"conceptCode": conceptCode,
			"conceptName": board.ConceptName,
			"content":     nil,
			"generatedAt": nil,
			"cached":      false,
			"empty":       true,
		})
		return
	}

	// Build prompt from DB system config
	sysPrompt := loadConceptAnalysisPrompt()
	prompt := fmt.Sprintf("请分析以下概念板块：\n\n概念名称：%s\n成分股数量：%d只\n\n%s",
		board.ConceptName, board.StockCount, sysPrompt)

	// Call AI
	uidVal, _ := c.Get("userId"); userID := uidVal.(uint)
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

// UpdateConceptAnalysisPrompt updates the analysis prompt in DB system configs
func (h *BoardHandler) UpdateConceptAnalysisPrompt(c *gin.Context) {
	var body struct {
		Prompt string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Prompt == "" {
		response.BadRequest(c, "请提供prompt")
		return
	}

	// Upsert into ai_system_configs
	var cfg model.AISystemConfig
	if err := db.PG.Where("scene = ?", "concept_analysis").First(&cfg).Error; err != nil {
		// Create new
		db.PG.Create(&model.AISystemConfig{
			Scene:        "concept_analysis",
			Name:         "概念分析",
			SystemPrompt: body.Prompt,
			Temperature:  0.7,
			MaxTokens:    4096,
			EnableSearch: false,
		})
	} else {
		db.PG.Model(&cfg).Updates(map[string]interface{}{
			"system_prompt": body.Prompt,
			"updated_at":    time.Now(),
		})
	}

	response.SuccessMsg(c, "提示词已更新")
}
