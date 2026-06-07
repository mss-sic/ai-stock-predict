package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/ai-stock-predict/server/internal/service"
	"github.com/ai-stock-predict/server/pkg/response"
	"github.com/gin-gonic/gin"
)

type SettingsHandler struct {
	aiSvc *service.AIService
}

func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{aiSvc: service.NewAIService()}
}

func getUIDFromContext(c *gin.Context) uint {
	uid, exists := c.Get("userId")
	if !exists {
		return 0
	}
	switch v := uid.(type) {
	case uint:
		return v
	case float64:
		return uint(v)
	case int:
		return uint(v)
	case int64:
		return uint(v)
	default:
		return 0
	}
}

func (h *SettingsHandler) GetAIConfig(c *gin.Context) {
	uid := getUIDFromContext(c)
	var cfg model.AIConfig
	if err := db.MySQL.Where("user_id = ?", uid).First(&cfg).Error; err != nil {
		response.Success(c, model.AIConfig{
			Provider:  "deepseek",
			ModelName: "deepseek-chat",
			BaseURL:   "https://api.deepseek.com",
		})
		return
	}
	if len(cfg.APIKey) > 8 {
		cfg.APIKey = cfg.APIKey[:4] + "****" + cfg.APIKey[len(cfg.APIKey)-4:]
	}
	response.Success(c, cfg)
}

func (h *SettingsHandler) SaveAIConfig(c *gin.Context) {
	uid := getUIDFromContext(c)
	if uid == 0 {
		response.Unauthorized(c, "未登录")
		return
	}
	var body struct {
		Provider  string `json:"provider"`
		APIKey    string `json:"apiKey"`
		ModelName string `json:"modelName"`
		BaseURL   string `json:"baseUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	var cfg model.AIConfig
	if err := db.MySQL.Where("user_id = ?", uid).First(&cfg).Error; err != nil {
		// Create new config for this user
		cfg = model.AIConfig{UserID: uid}
	}
	cfg.UserID = uid
	if body.APIKey != "" && !containsWildcard(body.APIKey) {
		cfg.APIKey = body.APIKey
	}
	cfg.Provider = body.Provider
	cfg.ModelName = body.ModelName
	cfg.BaseURL = body.BaseURL

	if err := db.MySQL.Save(&cfg).Error; err != nil {
		response.InternalError(c, "保存失败: "+err.Error())
		return
	}
	response.SuccessMsg(c, "配置已保存")
}

func (h *SettingsHandler) TestAIConnection(c *gin.Context) {
	uid := getUIDFromContext(c)
	var body struct {
		Provider  string `json:"provider"`
		APIKey    string `json:"apiKey"`
		ModelName string `json:"modelName"`
		BaseURL   string `json:"baseUrl"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	err := h.aiSvc.TestConnection(body.Provider, body.APIKey, body.ModelName, body.BaseURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if uid > 0 {
		var cfg model.AIConfig
		if db.MySQL.Where("user_id = ?", uid).First(&cfg).Error != nil {
			cfg = model.AIConfig{UserID: uid}
		}
		cfg.IsActive = true
		db.MySQL.Save(&cfg)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "连接成功，AI分析已启用"})
}

func (h *SettingsHandler) ListModels(c *gin.Context) {
	var body struct {
		BaseURL string `json:"baseUrl"`
		APIKey  string `json:"apiKey"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if body.BaseURL == "" || body.APIKey == "" {
		response.BadRequest(c, "API地址和Key不能为空")
		return
	}

	req, err := http.NewRequest("GET", body.BaseURL+"/v1/models", nil)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+body.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		response.Error(c, http.StatusOK, response.CodeAIModelError, fmt.Sprintf("API返回 %d: %s", resp.StatusCode, string(respBody)))
		return
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		response.InternalError(c, "解析模型列表失败: "+err.Error())
		return
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	response.Success(c, models)
}

func containsWildcard(s string) bool {
	return len(s) > 4 && s[0:4] == "****"
}
