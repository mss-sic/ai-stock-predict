package ws

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// PendingRequest represents a command dispatched and awaiting response.
type PendingRequest struct {
	RequestID  string
	AccountID  uint
	Action     string
	CreatedAt  time.Time
	ResultChan chan CommandResponse
}

// Commander tracks in-flight command requests with timeout.
type Commander struct {
	mu       sync.Mutex
	pending  map[string]*PendingRequest // requestID → pending
}

// NewCommander creates a new command tracker.
func NewCommander() *Commander {
	return &Commander{
		pending: make(map[string]*PendingRequest),
	}
}

// GenerateRequestID creates a unique request ID: cmd_ + 8 hex chars.
func GenerateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "cmd_" + hex.EncodeToString(b)
}

// Dispatch sends a command via the hub and registers a pending request.
// Returns a channel that will receive the response or timeout.
func (c *Commander) Dispatch(hub *Hub, accountID uint, action string, payload interface{}, timeout time.Duration) (*PendingRequest, error) {
	if !hub.HasAgent(accountID) {
		return nil, fmt.Errorf("agent not connected for account %d", accountID)
	}

	requestID := GenerateRequestID()
	req := &PendingRequest{
		RequestID:  requestID,
		AccountID:  accountID,
		Action:     action,
		CreatedAt:  time.Now(),
		ResultChan: make(chan CommandResponse, 1),
	}

	c.mu.Lock()
	c.pending[requestID] = req
	c.mu.Unlock()

	cmd := CommandPush{
		Type:      "command",
		RequestID: requestID,
		AccountID: accountID,
		Action:    action,
		Payload:   payload,
	}
	hub.BroadcastCommand(accountID, cmd)
	log.Printf("[commander] dispatched %s requestID=%s account=%d", action, requestID, accountID)

	// Start timeout goroutine
	go func() {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-timer.C:
			c.mu.Lock()
			if p, ok := c.pending[requestID]; ok {
				delete(c.pending, requestID)
				select {
				case p.ResultChan <- CommandResponse{Status: "failed", Error: "timeout waiting for agent response"}:
				default:
				}
			}
			c.mu.Unlock()
		case <-req.ResultChan:
			// Response received, nothing to do
		}
	}()

	return req, nil
}

// ReceiveResponse handles an agent's response and routes it to the waiting request.
func (c *Commander) ReceiveResponse(requestID string, resp CommandResponse) bool {
	c.mu.Lock()
	req, ok := c.pending[requestID]
	if ok {
		delete(c.pending, requestID)
	}
	c.mu.Unlock()

	if !ok {
		log.Printf("[commander] response for unknown requestID=%s", requestID)
		return false
	}

	select {
	case req.ResultChan <- resp:
		log.Printf("[commander] response received requestID=%s status=%s", requestID, resp.Status)
		return true
	default:
		return false
	}
}
