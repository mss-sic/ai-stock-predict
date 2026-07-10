package ws

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TestResponse is the result of an agent connectivity test.
type TestResponse struct {
	RequestID string `json:"requestId"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
}

// TestManager handles agent connectivity testing via WS challenge/response.
type TestManager struct {
	hub     *Hub
	mu      sync.Mutex
	pending map[string]chan *TestResponse // requestID → response channel
}

// NewTestManager creates a new test manager.
func NewTestManager(hub *Hub) *TestManager {
	return &TestManager{
		hub:     hub,
		pending: make(map[string]chan *TestResponse),
	}
}

// SendTest sends a test challenge to an account's agent and waits for a response.
// Returns nil error on success, or an error describing the failure.
func (m *TestManager) SendTest(accountID uint, timeoutSec int) error {
	// First verify an agent is connected
	if !m.hub.HasAgent(accountID) {
		return fmt.Errorf("账户 %d 没有已连接的 agent，请先启动本地 agent", accountID)
	}

	requestID := uuid.New().String()
	ch := make(chan *TestResponse, 1)

	m.mu.Lock()
	m.pending[requestID] = ch
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.pending, requestID)
		m.mu.Unlock()
	}()

	// Send test challenge via WebSocket
	m.hub.BroadcastToAccount(accountID, SignalPush{
		Type:      "test_request",
		AccountID: accountID,
		Data: map[string]string{
			"requestId": requestID,
			"timestamp": time.Now().Format(time.RFC3339),
		},
	})

	log.Printf("[test] sent challenge %s to account %d", requestID[:8], accountID)

	// Wait for response with timeout
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	select {
	case resp := <-ch:
		if resp.Success {
			log.Printf("[test] account %d test PASSED (%s)", accountID, resp.Message)
			return nil
		}
		return fmt.Errorf("agent 返回失败: %s", resp.Message)
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		return fmt.Errorf("测试超时（%d 秒内未收到 agent 响应），请确认本地 agent 正在运行", timeoutSec)
	}
}

// ReceiveResponse is called by the HTTP handler when an agent responds to a test challenge.
func (m *TestManager) ReceiveResponse(requestID string, success bool, message string) bool {
	m.mu.Lock()
	ch, ok := m.pending[requestID]
	m.mu.Unlock()

	if !ok {
		log.Printf("[test] no pending request for %s (expired or invalid)", requestID[:8])
		return false
	}

	ch <- &TestResponse{
		RequestID: requestID,
		Success:   success,
		Message:   message,
	}
	log.Printf("[test] received response for %s: success=%v msg=%s", requestID[:8], success, message)
	return true
}

// HasPendingTest checks if there's a pending test for the given request.
func (m *TestManager) HasPendingTest(requestID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.pending[requestID]
	return ok
}
