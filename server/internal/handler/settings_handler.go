package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	aiSvc *service.AIService
}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{aiSvc: service.NewAIService()}
}

func (h *SettingsHandler) GetAIConfig(c *gin.Context) {
	var cfg model.AIConfig
	if err := db.MySQL.First(&cfg).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"data": model.AIConfig{
			Provider:  "deepseek",
			ModelName: "deepseek-chat",
			BaseURL:   "https://api.deepseek.com",
		}})
		return
	}
	if len(cfg.APIKey) > 8 {
		cfg.APIKey = cfg.APIKey[:4] + "****" + cfg.APIKey[len(cfg.APIKey)-4:]
	}
	c.JSON(http.StatusOK, gin.H{"data": cfg})
}

func (h *SettingsHandler) SaveAIConfig(c *gin.Context) {
	var body struct {
		Provider  string `json:"provider"`
		APIKey    string `json:"apiKey"`
		ModelName string `json:"modelName"`
		BaseURL   string `json:"baseUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	var cfg model.AIConfig
	db.MySQL.First(&cfg)

	if body.APIKey != "" && !containsWildcard(body.APIKey) {
		cfg.APIKey = body.APIKey
	}
	cfg.Provider = body.Provider
	cfg.ModelName = body.ModelName
	cfg.BaseURL = body.BaseURL

	if err := db.MySQL.Save(&cfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": "ok"})
}

func (h *SettingsHandler) TestAIConnection(c *gin.Context) {
	var body struct {
		Provider  string `json:"provider"`
		APIKey    string `json:"apiKey"`
		ModelName string `json:"modelName"`
		BaseURL   string `json:"baseUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	err := h.aiSvc.TestConnection(body.Provider, body.APIKey, body.ModelName, body.BaseURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	var cfg model.AIConfig
	db.MySQL.First(&cfg)
	cfg.IsActive = true
	db.MySQL.Save(&cfg)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "连接成功，AI分析已启用"})
}

func (h *SettingsHandler) ListModels(c *gin.Context) {
	var body struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	if body.BaseURL == "" || body.APIKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "baseUrl and apiKey required"})
		return
	}

	req, err := http.NewRequest("GET", body.BaseURL+"/v1/models", nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}, "error": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+body.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		c.JSON(http.StatusOK, gin.H{"data": []string{}, "error": fmt.Sprintf("API返回 %d: %s", resp.StatusCode, string(respBody))})
		return
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		c.JSON(http.StatusOK, gin.H{"data": []string{}, "error": err.Error()})
		return
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	c.JSON(http.StatusOK, gin.H{"data": models})
}

func containsWildcard(s string) bool {
	return len(s) > 4 && s[0:4] == "****"
}
