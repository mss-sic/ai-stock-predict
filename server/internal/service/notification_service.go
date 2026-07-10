package service

import (
	"bytes"
	"io"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// NotificationService handles multi-channel notification delivery.
// Supports: 钉钉 Bot (webhook + keyword/signature), 飞书 Bot (webhook + keyword/signature),
// 企业微信 Bot (webhook), Email (SMTP).
//
// Security settings:
//   - keyword: custom keyword verification (recommended). All messages prepend the keyword.
//   - secret: HMAC-SHA256 signature verification (optional, more secure).
//     DingTalk: appends &timestamp=...&sign=... to URL.
//     Feishu: adds timestamp + sign to request body.
type NotificationService struct{}

func NewNotificationService() *NotificationService {
	return &NotificationService{}
}

func (s *NotificationService) SendToUser(uid uint, title, body string) error {
	var configs []model.NotificationConfig
	db.MySQL.Where("user_id = ? AND enabled = true", uid).Find(&configs)
	if len(configs) == 0 {
		log.Printf("[notify] no enabled channels for user %d, skipping", uid)
	return nil
	}
	var lastErr error
	for _, cfg := range configs {
		err := s.sendVia(&cfg, uid, title, body)
		if err != nil {
			log.Printf("[notify] channel %s failed for user %d: %v", cfg.Channel, uid, err)
			lastErr = err
			s.recordNotification(uid, cfg.Channel, title, body, "failed", err.Error())
			continue
		}
		s.recordNotification(uid, cfg.Channel, title, body, "sent", "")
	}
	return lastErr
}

// SendToChannels sends to specific notification configs (by ID).
func (s *NotificationService) SendToChannels(channelIDs []uint, title, body string) error {
	var configs []model.NotificationConfig
	db.MySQL.Where("id IN ? AND enabled = true", channelIDs).Find(&configs)
	var lastErr error
	for _, cfg := range configs {
		err := s.sendVia(&cfg, cfg.UserID, title, body)
		if err != nil {
			log.Printf("[notify] channel %s failed: %v", cfg.Channel, err)
			lastErr = err
		}
	}
	return lastErr
}

// getKeyword extracts the custom keyword from config.
func getKeyword(cfg *model.NotificationConfig) string {
	if kw, ok := cfg.Config["keyword"].(string); ok {
		return kw
	}
	return ""
}

// getSecret extracts the signing secret from config.
func getSecret(cfg *model.NotificationConfig) string {
	if s, ok := cfg.Config["secret"].(string); ok {
		return s
	}
	return ""
}

// applyKeyword prepends keyword to content if configured.
func applyKeyword(cfg *model.NotificationConfig, content string) string {
	kw := getKeyword(cfg)
	if kw != "" {
		return kw + "\n" + content
	}
	return content
}

// dingTalkSign computes DingTalk webhook signature.
// Algorithm: timestamp + "\n" + secret → HMAC-SHA256 → base64 → URL encode
func dingTalkSign(secret string) (timestamp, sign string) {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(ts + "\n" + secret))
	sign = base64.StdEncoding.EncodeToString(h.Sum(nil))
	return ts, url.QueryEscape(sign)
}

// feishuSign computes Feishu webhook signature.
func feishuSign(secret string) (timestamp, sign string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := hmac.New(sha256.New, []byte(ts+"\n"+secret))
	h.Write([]byte(""))
	sign = base64.StdEncoding.EncodeToString(h.Sum(nil))
	return ts, sign
}

func (s *NotificationService) sendVia(cfg *model.NotificationConfig, uid uint, title, body string) error {
	switch cfg.Channel {
	case "wecom_bot":
		return s.sendWeComBot(cfg, title, body)
	case "dingtalk_bot":
		return s.sendDingTalkBot(cfg, title, body)
	case "feishu_bot":
		return s.sendFeishuBot(cfg, title, body)
	case "email":
		return s.sendEmail(cfg, title, body)
	default:
		return fmt.Errorf("unsupported channel: %s", cfg.Channel)
	}
}

func (s *NotificationService) sendWeComBot(cfg *model.NotificationConfig, title, body string) error {
	webhookURL := ""
	if url, ok := cfg.Config["webhook_url"].(string); ok {
		webhookURL = url
	}
	if webhookURL == "" {
		return fmt.Errorf("wecom_bot: missing webhook_url")
	}

	content := applyKeyword(cfg, fmt.Sprintf("## %s\n%s", title, body))
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": content,
		},
	}
	jsonBody, _ := json.Marshal(msg)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("wecom_bot post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("wecom_bot returned %d", resp.StatusCode)
	}
	return nil
}

func (s *NotificationService) sendDingTalkBot(cfg *model.NotificationConfig, title, body string) error {
	webhookURL := ""
	if u, ok := cfg.Config["webhook_url"].(string); ok {
		webhookURL = u
	}
	if webhookURL == "" {
		return fmt.Errorf("dingtalk_bot: missing webhook_url")
	}

	// Signature verification
	secret := getSecret(cfg)
	if secret != "" {
		ts, sign := dingTalkSign(secret)
		webhookURL += "&timestamp=" + ts + "&sign=" + sign
	}

	// Keyword verification — prepend to message
	text := applyKeyword(cfg, fmt.Sprintf("## %s\n%s", title, body))

	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	}
	jsonBody, _ := json.Marshal(msg)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("dingtalk_bot post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("dingtalk_bot returned %d", resp.StatusCode)
	}
	return nil
}

func (s *NotificationService) sendFeishuBot(cfg *model.NotificationConfig, title, body string) error {
	webhookURL := ""
	if u, ok := cfg.Config["webhook_url"].(string); ok {
		webhookURL = u
	}
	if webhookURL == "" {
		return fmt.Errorf("feishu_bot: missing webhook_url")
	}

	// Signature verification
	secret := getSecret(cfg)
	if secret != "" {
		ts, sign := feishuSign(secret)
		webhookURL += "&timestamp=" + ts + "&sign=" + sign
	}

	// Check if body is a JSON envelope with "card" field (from live handler)
	var cardPayload map[string]string
	if err := json.Unmarshal([]byte(body), &cardPayload); err == nil {
		if cardJSON, ok := cardPayload["card"]; ok && cardJSON != "" {
			msg := map[string]interface{}{
				"msg_type": "interactive",
			}
			var card map[string]interface{}
			if json.Unmarshal([]byte(cardJSON), &card) == nil {
				msg["card"] = card
			}
			jsonBody, _ := json.Marshal(msg)
			log.Printf("[notify] feishu sending card to %s: %d bytes", webhookURL[:60], len(jsonBody))
			resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
			if err != nil {
				return fmt.Errorf("feishu_bot post: %w", err)
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				log.Printf("[notify] feishu_bot returned %d: %s", resp.StatusCode, string(respBody))
				return fmt.Errorf("feishu_bot returned %d: %s", resp.StatusCode, string(respBody))
			}
			log.Printf("[notify] feishu_bot card success: %s", string(respBody))
			return nil
		}
		// Fallback: use text part from envelope
		body = cardPayload["text"]
	}

	// Fallback: text format for non-card callers
	content := applyKeyword(cfg, fmt.Sprintf("%s\n%s", title, body))

	msg := map[string]interface{}{
		"msg_type": "text",
		"content": map[string]string{
			"text": content,
		},
	}
	jsonBody, _ := json.Marshal(msg)
	log.Printf("[notify] feishu sending text to %s: %d bytes", webhookURL[:60], len(jsonBody))
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("feishu_bot post: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("[notify] feishu_bot returned %d: %s", resp.StatusCode, string(respBody))
		return fmt.Errorf("feishu_bot returned %d: %s", resp.StatusCode, string(respBody))
	}
	log.Printf("[notify] feishu_bot success: %s", string(respBody))
	return nil
}

func (s *NotificationService) sendEmail(cfg *model.NotificationConfig, title, body string) error {
	host, _ := cfg.Config["smtp_host"].(string)
	port, _ := cfg.Config["smtp_port"].(string)
	user, _ := cfg.Config["smtp_user"].(string)
	pass, _ := cfg.Config["smtp_pass"].(string)
	to, _ := cfg.Config["to"].(string)
	if host == "" || to == "" {
		return fmt.Errorf("email: missing smtp_host or to")
	}
	if port == "" {
		port = "587"
	}
	auth := smtp.PlainAuth("", user, pass, host)
	msg := []byte(fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		user, to, title, body))
	return smtp.SendMail(host+":"+port, auth, user, []string{to}, msg)
}

func (s *NotificationService) recordNotification(uid uint, channel, title, body, status, errMsg string) {
	notif := model.Notification{
		UserID: uid, Channel: channel, Title: title, Body: body, Status: status, ErrorMsg: errMsg,
	}
	db.MySQL.Create(&notif)
}
