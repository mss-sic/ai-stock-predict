package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"strings"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

type AIService struct{}

func NewAIService() *AIService { return &AIService{} }

// GetConfig returns the AI config for a specific user
func (s *AIService) GetConfig(userID uint) (*model.AIConfig, error) {
	var cfg model.AIConfig
	err := db.MySQL.Where("user_id = ?", userID).First(&cfg).Error
	if err != nil {
		return &model.AIConfig{
			UserID:    userID,
			Provider:  "deepseek",
			ModelName: "deepseek-chat",
			BaseURL:   "https://api.deepseek.com",
		}, nil
	}
	return &cfg, nil
}

// TestConnection tests the AI API connectivity
func (s *AIService) TestConnection(provider, apiKey, modelName, baseURL string) error {
	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
		"max_tokens": 10,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", baseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API返回 %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// ChatCompletion sends a non-streaming chat completion request (user-scoped)
func (s *AIService) ChatCompletion(userID uint, prompt string, history []map[string]string) (string, error) {
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

	body := map[string]interface{}{
		"model":         cfg.ModelName,
		"messages":      messages,
		"temperature":   0.7,
		"max_tokens":    2048,
		"enable_search": true,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("AI返回空结果")
	}
	content := result.Choices[0].Message.Content
	if content == "" {
		return "", fmt.Errorf("AI返回空内容，模型可能因提示词过长被截断")
	}
	return content, nil
}

// ChatCompletionStream sends a streaming chat completion (user-scoped)
func (s *AIService) ChatCompletionStream(userID uint, prompt string, history []map[string]string, onChunk func(chunk string)) error {
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

	body := map[string]interface{}{
		"model":         cfg.ModelName,
		"messages":      messages,
		"temperature":   0.7,
		"max_tokens":    2048,
		"stream":        true,
		"enable_search": true,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
	}

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
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	return nil
}


// ChatCompletionStreamWithConfig sends a streaming request with system config overrides
func (s *AIService) ChatCompletionStreamWithConfig(userID uint, prompt string, history []map[string]string, sysCfg model.AISystemConfig, onChunk func(chunk string)) error {
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

	b, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", cfg.BaseURL+"/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("AI请求失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AI API返回 %d: %s", resp.StatusCode, string(respBody))
	}

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
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	return nil
}

// AnalyzeStock builds a structured prompt and returns AI analysis
func (s *AIService) AnalyzeStock(userID uint, code, name, industry string, close float64, pe, pb float64, klineSummary string, predSummary string) (map[string]interface{}, error) {
	prompt := fmt.Sprintf(`你是一位资深A股分析师。请分析以下股票并返回JSON结果。

股票信息：
- 代码：%s
- 名称：%s
- 行业：%s
- 最新收盘价：%.2f
- 市盈率(PE)：%.2f
- 市净率(PB)：%.2f

近期K线走势摘要（近20日）：
%s

量化模型预测摘要：
%s

请从以下维度分析并返回严格JSON格式（不要markdown代码块）：
{
  "riskLevel": "低风险/中低风险/中风险/中高风险/高风险",
  "suggestion": "强烈买入/买入/增持/持有/减持/卖出/强烈卖出",
  "summary": "200字以内的综合分析",
  "signals": ["信号1", "信号2"]
}`, code, name, industry, close, pe, pb, klineSummary, predSummary)

	reply, err := s.ChatCompletion(userID, prompt, nil)
	if err != nil {
		return nil, err
	}

	// Clean markdown code fences
	reply = strings.TrimSpace(reply)
	reply = strings.TrimPrefix(reply, "```json")
	reply = strings.TrimPrefix(reply, "```")
	reply = strings.TrimSuffix(reply, "```")
	reply = strings.TrimSpace(reply)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(reply), &result); err != nil {
		return map[string]interface{}{
			"riskLevel":  "中风险",
			"suggestion": "持有",
			"summary":    reply,
			"signals":    []string{},
		}, nil
	}
	return result, nil
}
