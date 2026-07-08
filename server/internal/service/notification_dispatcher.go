package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
)

// NotificationDispatcher sends risk alerts to configured channels.
type NotificationDispatcher struct {
	mu       sync.Mutex
	windows  map[string]*rateWindow // "userID:channel" → window
}

type rateWindow struct {
	count     int
	startTime time.Time
}

var defaultDispatcher = &NotificationDispatcher{
	windows: make(map[string]*rateWindow),
}

// Dispatch sends a risk alert to all subscribed channels for a user.
func (d *NotificationDispatcher) Dispatch(userID uint, alert model.RiskAlert) {
	var configs []model.NotificationConfig
	db.MySQL.Where("user_id = ? AND enabled = true", userID).Find(&configs)

	for _, cfg := range configs {
		if !d.rateLimit(userID, cfg.Channel, alert.Level) {
			continue
		}
		if !d.shouldSendNow(alert.Level) {
			// Queue for morning delivery
			log.Printf("[Notif] queued %s alert for user %d (night silence)", alert.Level, userID)
			continue
		}
		go d.sendWebhook(cfg, alert)
	}
}

func (d *NotificationDispatcher) rateLimit(userID uint, channel, level string) bool {
	if level == "high" {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	key := fmt.Sprintf("%d:%s", userID, channel)
	w := d.windows[key]
	if w == nil || time.Since(w.startTime) > 5*time.Minute {
		d.windows[key] = &rateWindow{count: 1, startTime: time.Now()}
		return true
	}
	w.count++
	return w.count <= 3
}

func (d *NotificationDispatcher) shouldSendNow(level string) bool {
	if level == "high" {
		return true
	}
	h := time.Now().Hour()
	return h >= 7 && h < 22
}

func (d *NotificationDispatcher) sendWebhook(cfg model.NotificationConfig, alert model.RiskAlert) {
	title := fmt.Sprintf("[%s] %s", alert.Level, alert.Type)
	body := alert.Description
	if alert.StockName != "" {
		title = fmt.Sprintf("[%s] %s %s", alert.Level, alert.StockName, alert.Type)
	}

	var payload []byte
	var targetURL string

	switch cfg.Channel {
	case "dingtalk_bot":
		payload, targetURL = buildDingTalk(cfg, title, body)
	case "feishu_bot":
		payload, targetURL = buildFeishu(cfg, title, body)
	case "wecom_bot":
		payload, targetURL = buildWecom(cfg, title, body)
	default:
		return
	}

	// Send with retry
	for attempt := 0; attempt < 3; attempt++ {
		resp, err := http.Post(targetURL, "application/json", bytes.NewReader(payload))
		if err == nil && resp.StatusCode < 400 {
			resp.Body.Close()
			log.Printf("[Notif] sent %s via %s", alert.RuleKey, cfg.Channel)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("[Notif] retry %d for %s: %v", attempt+1, cfg.Channel, err)
		time.Sleep(time.Duration(attempt+1) * 10 * time.Second)
	}
	log.Printf("[Notif] failed %s after 3 retries", cfg.Channel)
}

func buildDingTalk(cfg model.NotificationConfig, title, body string) ([]byte, string) {
	webhookURL := ""
	if cfg.Config != nil {
		if u, ok := cfg.Config["webhook_url"].(string); ok {
			webhookURL = u
		}
	}
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("### %s\n\n%s\n\n> %s", title, body, time.Now().Format("2006-01-02 15:04:05")),
		},
	}
	b, _ := json.Marshal(msg)
	return b, webhookURL
}

func buildFeishu(cfg model.NotificationConfig, title, body string) ([]byte, string) {
	webhookURL := ""
	if cfg.Config != nil {
		if u, ok := cfg.Config["webhook_url"].(string); ok {
			webhookURL = u
		}
	}
	msg := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title":    map[string]string{"content": title, "tag": "plain_text"},
				"template": "red",
			},
			"elements": []map[string]interface{}{
				{"tag": "div", "text": map[string]string{"content": body, "tag": "lark_md"}},
				{"tag": "hr"},
				{"tag": "note", "elements": []map[string]string{{"content": time.Now().Format("2006-01-02 15:04:05"), "tag": "plain_text"}}},
			},
		},
	}
	b, _ := json.Marshal(msg)
	return b, webhookURL
}

func buildWecom(cfg model.NotificationConfig, title, body string) ([]byte, string) {
	webhookURL := ""
	if cfg.Config != nil {
		if u, ok := cfg.Config["webhook_url"].(string); ok {
			webhookURL = u
		}
	}
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("### %s\n%s\n<font color=\"comment\">%s</font>", title, body, time.Now().Format("2006-01-02 15:04")),
		},
	}
	b, _ := json.Marshal(msg)
	return b, webhookURL
}

// GetDispatcher returns the global notification dispatcher.
func GetDispatcher() *NotificationDispatcher { return defaultDispatcher }

// DispatchTest sends a test message to verify webhook configuration.
func (d *NotificationDispatcher) DispatchTest(cfg model.NotificationConfig) {
	testAlert := model.RiskAlert{
		Level:       "low",
		Type:        "测试消息",
		Description: "这是一条测试消息，您的 Webhook 配置工作正常！",
		StockName:   "测试",
		RuleKey:     "test",
	}
	d.sendWebhook(cfg, testAlert)
}
