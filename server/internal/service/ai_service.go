package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type AIService struct{}

func NewAIService() *AIService { return &AIService{} }

func getUsername(userID uint) string {
	var u model.User
	if err := db.MySQL.Where("id = ?", userID).Select("username").First(&u).Error; err == nil {
		return u.Username
	}
	return ""
}



func (s *AIService) GetConfig(userID uint) (*model.AIConfig, error) {
	var cfg model.AIConfig
	if err := db.MySQL.Where("user_id = ?", userID).First(&cfg).Error; err != nil {
		return &cfg, err
	}
	return &cfg, nil
}

// ── Unified Chat Completion Layer ──

type chatRequest struct {
	UserID  uint
	APIKey  string
	BaseURL string
	Body    map[string]interface{}
	Module  string // "chat" / "stock_score" / "stock_profile" / "strategy_gen" / "strategy_opt"
	Timeout time.Duration
}

type chatResponse struct {
	Content string
	Usage   *model.UsageInfo
	Model   string
}

// doChatCompletion sends a non-streaming request and returns content + usage.
// It also records the cost log automatically.
func (s *AIService) doChatCompletion(req chatRequest) (*chatResponse, error) {
	start := time.Now()

	b, _ := json.Marshal(req.Body)
	httpReq, err := http.NewRequest("POST", req.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, "", start, err, "", "")
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	client := &http.Client{Timeout: req.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, "", start, fmt.Errorf("AI请求失败: %w", err), "", "")
		return nil, fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, "", start, err, "", "")
		return nil, err
	}

	var result struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage model.UsageInfo `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, "", start, err, "", "")
		return nil, err
	}
	if len(result.Choices) == 0 {
		err := fmt.Errorf("AI返回空结果")
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, result.Model, start, err, "", "")
		return nil, err
	}
	content := result.Choices[0].Message.Content
	if content == "" {
		err := fmt.Errorf("AI返回空内容，模型可能因提示词过长被截断")
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", nil, result.Model, start, err, "", "")
		return nil, err
	}

	recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "NonStream", &result.Usage, result.Model, start, nil, "", "")
	return &chatResponse{Content: content, Usage: &result.Usage, Model: result.Model}, nil
}

// doChatCompletionStream sends a streaming request, calling onChunk for each text delta.
// It extracts usage from the last SSE chunk and records the cost log automatically.
func (s *AIService) doChatCompletionStream(req chatRequest, onChunk func(string)) (*model.UsageInfo, error) {
	start := time.Now()

	b, _ := json.Marshal(req.Body)
	httpReq, err := http.NewRequest("POST", req.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Stream", nil, "", start, err, "", "")
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	client := &http.Client{Timeout: req.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Stream", nil, "", start, fmt.Errorf("AI请求失败: %w", err), "", "")
		return nil, fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Stream", nil, "", start, err, "", "")
		return nil, err
	}

	var lastUsage *model.UsageInfo
	var modelName string

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk struct {
			Model   string `json:"model"`
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage model.UsageInfo `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Model != "" {
			modelName = chunk.Model
		}
		// Capture usage from the last chunk (the one with finish_reason)
		if chunk.Usage.TotalTokens > 0 {
			u := chunk.Usage
			lastUsage = &u
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}

	recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Stream", lastUsage, modelName, start, nil, "", "")
	return lastUsage, nil
}

// doChatCompletionRaw sends a non-streaming request returning full parsed JSON.
// Used by agent methods to inspect tool_calls. Also records cost log.
func (s *AIService) doChatCompletionRaw(req chatRequest, result interface{}) error {
	start := time.Now()

	b, _ := json.Marshal(req.Body)
	httpReq, err := http.NewRequest("POST", req.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)

	client := &http.Client{Timeout: req.Timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Agent", nil, "", start, fmt.Errorf("AI请求失败: %w", err), "", "")
		return fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		err := fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Agent", nil, "", start, err, "", "")
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		recordCostLog(req.UserID, getUsername(req.UserID), req.Module, "Agent", nil, "", start, err, "", "")
		return fmt.Errorf("AI响应解析失败: %w", err)
	}

	return nil
}

// recordCostLog inserts an AI call record into MySQL ai_cost_logs.
// Uses a goroutine to avoid blocking the caller.
func recordCostLog(userID uint, username, module, function string, usage *model.UsageInfo, modelName string, startTime time.Time, callErr error, requestBody, responseContent string) {
	go func() {
		if db.MySQL == nil {
			return
		}
		durationMs := int(time.Since(startTime).Milliseconds())

		entry := model.AICostLog{
			UserID:   userID,
			Username: username,
			Module:   module,
			ModelName: modelName,
			Function: function,
			DurationMs: durationMs,
			Success:  callErr == nil,
			RequestContent:  requestBody,
			ResponseContent: responseContent,
		}
		if callErr != nil {
			entry.ErrorMsg = callErr.Error()
		}
		if usage != nil {
			entry.PromptTokens = usage.PromptTokens
			entry.CompletionTokens = usage.CompletionTokens
			entry.TotalTokens = usage.TotalTokens
			entry.PromptCacheHit = usage.PromptCacheHit
			entry.PromptCacheMiss = usage.PromptCacheMiss
			// Calculate cost from model_prices
			var price model.ModelPrice
			if err := db.MySQL.Where("model_name = ?", modelName).First(&price).Error; err == nil {
				cost := float64(usage.PromptCacheMiss)/1e6*price.InputPrice +
					float64(usage.PromptCacheHit)/1e6*price.CacheHitPrice +
					float64(usage.CompletionTokens)/1e6*price.OutputPrice
				entry.CostAmount = cost
			}
		}
		if err := db.MySQL.Create(&entry).Error; err != nil {
			log.Printf("[cost] record failed: %v", err)
		}
	}()
}

// ── Public API Methods (thin wrappers) ──

// ChatCompletion sends a non-streaming chat completion request (user-scoped)
func (s *AIService) ChatCompletion(userID uint, prompt string, history []map[string]string) (string, error) {
	return s.ChatCompletionWithModule(userID, prompt, history, "chat")
}

// ChatCompletionWithModule sends a non-streaming chat completion request with module tracking
func (s *AIService) ChatCompletionWithModule(userID uint, prompt string, history []map[string]string, module string) (string, error) {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return "", err
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("AI API Key未配置，请在设置页面配置")
	}

	messages := make([]map[string]string, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	resp, err := s.doChatCompletion(chatRequest{
		UserID:  userID,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Body: map[string]interface{}{
			"model":         cfg.ModelName,
			"messages":      messages,
			"temperature":   0.7,
			"max_tokens":    2048,
			"enable_search": true,
		},
		Module:  module,
		Timeout: 180 * time.Second,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatCompletionWithTokens is like ChatCompletion but with configurable max_tokens.
func (s *AIService) ChatCompletionWithTokens(userID uint, prompt string, history []map[string]string, maxTokens int) (string, error) {
	return s.ChatCompletionWithTokensModule(userID, prompt, history, maxTokens, "chat")
}

// ChatCompletionWithTokensModule is like ChatCompletionWithTokens but with module tracking
func (s *AIService) ChatCompletionWithTokensModule(userID uint, prompt string, history []map[string]string, maxTokens int, module string) (string, error) {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return "", err
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("AI API Key未配置，请在设置页面配置")
	}

	messages := make([]map[string]string, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	resp, err := s.doChatCompletion(chatRequest{
		UserID:  userID,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Body: map[string]interface{}{
			"model":         cfg.ModelName,
			"messages":      messages,
			"temperature":   0.3,
			"max_tokens":    maxTokens,
			"enable_search": false,
		},
		Module:  module,
		Timeout: 120 * time.Second,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// ChatCompletionStream sends a streaming chat completion (user-scoped)
func (s *AIService) ChatCompletionStream(userID uint, prompt string, history []map[string]string, onChunk func(chunk string)) error {
	return s.ChatCompletionStreamWithModule(userID, prompt, history, onChunk, "chat")
}

// ChatCompletionStreamWithModule sends a streaming chat completion with module tracking
func (s *AIService) ChatCompletionStreamWithModule(userID uint, prompt string, history []map[string]string, onChunk func(chunk string), module string) error {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("AI API Key未配置")
	}

	messages := make([]map[string]string, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	_, err = s.doChatCompletionStream(chatRequest{
		UserID:  userID,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Body: map[string]interface{}{
			"model":         cfg.ModelName,
			"messages":      messages,
			"temperature":   0.7,
			"max_tokens":    2048,
			"stream":        true,
			"enable_search": true,
		},
		Module:  module,
		Timeout: 60 * time.Second,
	}, onChunk)
	return err
}

// ChatCompletionStreamWithConfig sends a streaming request with system config overrides
func (s *AIService) ChatCompletionStreamWithConfig(userID uint, prompt string, history []map[string]string, sysCfg model.AISystemConfig, onChunk func(chunk string)) error {
	return s.ChatCompletionStreamWithConfigModule(userID, prompt, history, sysCfg, onChunk, "chat")
}

// ChatCompletionStreamWithConfigModule sends a streaming request with config and module tracking
func (s *AIService) ChatCompletionStreamWithConfigModule(userID uint, prompt string, history []map[string]string, sysCfg model.AISystemConfig, onChunk func(chunk string), module string) error {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return err
	}
	if cfg.APIKey == "" {
		return fmt.Errorf("AI API Key未配置")
	}

	messages := make([]map[string]string, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	modelName := cfg.ModelName
	if sysCfg.ModelName != "" {
		modelName = sysCfg.ModelName
	}

	body := map[string]interface{}{
		"model":       modelName,
		"messages":    messages,
		"temperature": sysCfg.Temperature,
		"max_tokens":  sysCfg.MaxTokens,
		"stream":      true,
	}
	if sysCfg.EnableSearch {
		body["enable_search"] = true
	}

	_, err = s.doChatCompletionStream(chatRequest{
		UserID:  userID,
		APIKey:  cfg.APIKey,
		BaseURL: cfg.BaseURL,
		Body:    body,
		Module:  module,
		Timeout: 120 * time.Second,
	}, onChunk)
	return err
}

// ChatCompletionAgent runs the agent loop with tools.
func (s *AIService) ChatCompletionAgent(userID uint, history []map[string]string, tools []map[string]interface{}, sysCfg model.AISystemConfig, toolExecutor func(name string, args map[string]interface{}) string, onChunk func(chunk string)) error {
	return s.ChatCompletionAgentWithModule(userID, history, tools, sysCfg, toolExecutor, onChunk, "chat")
}

// ChatCompletionAgentWithModule runs the agent loop with tools and module tracking
func (s *AIService) ChatCompletionAgentWithModule(userID uint, history []map[string]string, tools []map[string]interface{}, sysCfg model.AISystemConfig, toolExecutor func(name string, args map[string]interface{}) string, onChunk func(chunk string), module string) error {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return err
	}
	apiKey := cfg.APIKey
	baseURL := cfg.BaseURL
	modelName := cfg.ModelName
	if sysCfg.AgentAPIKey != "" {
		apiKey = sysCfg.AgentAPIKey
	}
	if sysCfg.AgentBaseURL != "" {
		baseURL = sysCfg.AgentBaseURL
	}
	if sysCfg.AgentModelName != "" {
		modelName = sysCfg.AgentModelName
	}
	if apiKey == "" {
		return fmt.Errorf("AI API Key未配置")
	}

	msgs := buildAgentMessages(history)

	// Accumulate usage across turns
	var totalUsage model.UsageInfo
	agentStart := time.Now()

	const maxTurns = 8
	for turn := 0; turn < maxTurns; turn++ {
		body := map[string]interface{}{
			"model":       modelName,
			"messages":    msgs,
			"temperature": 0.7,
			"max_tokens":  2048,
			"tools":       tools,
			"tool_choice": "auto",
		}

		var result struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Role      string    `json:"role"`
					Content   string    `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage model.UsageInfo `json:"usage"`
		}
		if err := s.doChatCompletionRaw(chatRequest{
			UserID:  userID,
			APIKey:  apiKey,
			BaseURL: baseURL,
			Body:    body,
			Module:  module,
			Timeout: 120 * time.Second,
		}, &result); err != nil {
			return err
		}

		// Accumulate usage
		totalUsage.PromptTokens += result.Usage.PromptTokens
		totalUsage.CompletionTokens += result.Usage.CompletionTokens
		totalUsage.TotalTokens += result.Usage.TotalTokens
		totalUsage.PromptCacheHit += result.Usage.PromptCacheHit
		totalUsage.PromptCacheMiss += result.Usage.PromptCacheMiss

		if len(result.Choices) == 0 {
			return fmt.Errorf("AI返回空结果")
		}

		choice := result.Choices[0]

		if len(choice.Message.ToolCalls) > 0 {
			msgs = append(msgs, AgentMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})
			for _, tc := range choice.Message.ToolCalls {
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				toolResult := toolExecutor(tc.Function.Name, args)
				msgs = append(msgs, AgentMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    toolResult,
				})
			}
			continue
		}

		if choice.Message.Content != "" {
			onChunk(choice.Message.Content)
		}
		recordCostLog(userID, getUsername(userID), module, "Agent", &totalUsage, result.Model, agentStart, nil, "", "")
		return nil
	}
	onChunk("\n\n> ⚠️ 分析需要的数据较多，已自动截断。您可以换一种方式提问。")
	recordCostLog(userID, getUsername(userID), module, "Agent", &totalUsage, modelName, agentStart, nil, "", "")
	return nil
}

// ChatCompletionAgentStream runs the agent loop with SSE event streaming.
func (s *AIService) ChatCompletionAgentStream(userID uint, history []map[string]string, sysCfg model.AISystemConfig, tools []map[string]interface{}, toolExecutor func(name string, args map[string]interface{}) string, onEvent func(eventType string, data map[string]string), onChunk func(chunk string)) error {
	return s.ChatCompletionAgentStreamWithModule(userID, history, sysCfg, tools, toolExecutor, onEvent, onChunk, "chat")
}

// ChatCompletionAgentStreamWithModule runs the agent loop with SSE streaming and module tracking
func (s *AIService) ChatCompletionAgentStreamWithModule(userID uint, history []map[string]string, sysCfg model.AISystemConfig, tools []map[string]interface{}, toolExecutor func(name string, args map[string]interface{}) string, onEvent func(eventType string, data map[string]string), onChunk func(chunk string), module string) error {
	cfg, err := s.GetConfig(userID)
	if err != nil {
		return err
	}
	apiKey := cfg.APIKey
	baseURL := cfg.BaseURL
	modelName := cfg.ModelName
	if sysCfg.AgentAPIKey != "" {
		apiKey = sysCfg.AgentAPIKey
	}
	if sysCfg.AgentBaseURL != "" {
		baseURL = sysCfg.AgentBaseURL
	}
	if sysCfg.AgentModelName != "" {
		modelName = sysCfg.AgentModelName
	}
	if sysCfg.ModelName != "" && modelName == cfg.ModelName {
		modelName = sysCfg.ModelName
	}
	if apiKey == "" {
		return fmt.Errorf("AI API Key未配置")
	}

	msgs := buildAgentMessages(history)

	var totalUsage model.UsageInfo
	var finalModel string
	agentStart := time.Now()

	const maxTurns = 8
	for turn := 0; turn < maxTurns; turn++ {
		if turn == 0 {
			onEvent("agent_phase", map[string]string{
				"phase": "analyzing", "label": "AI 正在分析问题...",
			})
		}
		body := map[string]interface{}{
			"model":       modelName,
			"messages":    msgs,
			"temperature": sysCfg.Temperature,
			"max_tokens":  sysCfg.MaxTokens,
			"tools":       tools,
			"tool_choice": "auto",
		}

		if turn == maxTurns-1 {
			body["tool_choice"] = "none"
			delete(body, "tools")
			onEvent("agent_phase", map[string]string{
				"phase": "finalizing", "label": "AI 正在生成最终回答...",
			})
		}

		var result struct {
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Role      string    `json:"role"`
					Content   string    `json:"content"`
					ToolCalls []ToolCall `json:"tool_calls"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage model.UsageInfo `json:"usage"`
		}
		if err := s.doChatCompletionRaw(chatRequest{
			UserID:  userID,
			APIKey:  apiKey,
			BaseURL: baseURL,
			Body:    body,
			Module:  module,
			Timeout: 120 * time.Second,
		}, &result); err != nil {
			return err
		}

		totalUsage.PromptTokens += result.Usage.PromptTokens
		totalUsage.CompletionTokens += result.Usage.CompletionTokens
		totalUsage.TotalTokens += result.Usage.TotalTokens
		totalUsage.PromptCacheHit += result.Usage.PromptCacheHit
		totalUsage.PromptCacheMiss += result.Usage.PromptCacheMiss
		if result.Model != "" {
			finalModel = result.Model
		}

		if len(result.Choices) == 0 {
			return fmt.Errorf("AI返回空结果")
		}

		choice := result.Choices[0]

		if len(choice.Message.ToolCalls) > 0 {
			msgs = append(msgs, AgentMessage{
				Role:      "assistant",
				Content:   choice.Message.Content,
				ToolCalls: choice.Message.ToolCalls,
			})
			for idx, tc := range choice.Message.ToolCalls {
				toolName := tc.Function.Name
				toolLabel := toolName
				switch toolName {
				case "get_stock_price":
					toolLabel = "查询行情估值"
				case "get_kline_summary":
					toolLabel = "分析K线走势"
				case "get_technical":
					toolLabel = "计算技术指标"
				case "get_financials":
					toolLabel = "读取财务数据"
				case "get_news":
					toolLabel = "检索最新资讯"
				case "get_shareholders":
					toolLabel = "股东户数查询"
				}
				onEvent("tool_start", map[string]string{
					"tool": toolName, "label": toolLabel,
					"turn": fmt.Sprintf("%d", turn+1),
					"index": fmt.Sprintf("%d", idx+1),
					"total": fmt.Sprintf("%d", len(choice.Message.ToolCalls)),
				})
				var args map[string]interface{}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				toolResult := toolExecutor(tc.Function.Name, args)
				onEvent("tool_end", map[string]string{
					"tool": toolName, "label": toolLabel,
				})
				msgs = append(msgs, AgentMessage{
					Role:       "tool",
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
					Content:    toolResult,
				})
			}
			continue
		}

		if choice.Message.Content != "" {
			text := choice.Message.Content
			runes := []rune(text)
			chunkSize := 15
			for i := 0; i < len(runes); i += chunkSize {
				end := i + chunkSize
				if end > len(runes) {
					end = len(runes)
				}
				onChunk(string(runes[i:end]))
				time.Sleep(30 * time.Millisecond)
			}
		}
		recordCostLog(userID, getUsername(userID), module, "AgentStream", &totalUsage, finalModel, agentStart, nil, "", "")
		return nil
	}
	onChunk("\n\n> ⚠️ 分析需要的数据较多，已自动截断。您可以换一种方式提问。")
	recordCostLog(userID, getUsername(userID), module, "AgentStream", &totalUsage, finalModel, agentStart, nil, "", "")
	return nil
}

// ── Helper ──

func buildAgentMessages(history []map[string]string) []AgentMessage {
	msgs := make([]AgentMessage, 0, len(history))
	for _, h := range history {
		if h["content"] == "" {
			continue
		}
		role := h["role"]
		if role == "ai" {
			role = "assistant"
		}
		msgs = append(msgs, AgentMessage{Role: role, Content: h["content"]})
	}
	return msgs
}

// ── Legacy (kept for backward compatibility) ──

func (s *AIService) TestConnection(provider, apiKey, modelName, baseURL string) error {
	body := map[string]interface{}{
		"model":    modelName,
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
		"max_tokens": 10,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("AI连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI返回 %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// AnalyzeStock builds a structured prompt and returns AI analysis
func (s *AIService) AnalyzeStock(userID uint, code, name, industry string, close float64, pe, pb float64, klineSummary string, predSummary string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`你是一位资深A股分析师。请分析以下股票并返回JSON结果。

股票: %s (%s) 行业: %s
收盘价: %.2f PE: %.2f PB: %.2f
K线: %s
预测: %s

返回纯JSON:
{"suggestion":"买入/持有/卖出","confidence":0.8,"reason":"..."}`, name, code, industry, close, pe, pb, klineSummary, predSummary)

	reply, err := s.ChatCompletion(userID, prompt, nil)
	if err != nil {
		return nil, err
	}
	reply = cleanJSON(reply)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return nil, fmt.Errorf("AI返回解析失败: %w | 原文: %.200s", err, reply)
	}
	return result, nil
}

func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// ── Agent types ──

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type AgentMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}
