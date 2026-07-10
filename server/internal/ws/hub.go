// Package ws provides WebSocket real-time communication for the local trading agent.
package ws

import (
	"encoding/json"
	"time"
	"log"
	"sync"
)

// Hub maintains the set of active agent clients grouped by account channel.
type Hub struct {
	mu       sync.RWMutex
	channels map[uint]map[*AgentClient]bool // accountID → clients
}

// AgentClient represents a single WebSocket connection from a local agent.
type AgentClient struct {
	TraderType   string   `json:"traderType"`   // agent_hello 上报的本地客户端类型
	Capabilities []string `json:"capabilities"` // agent 支持的 capability 列表
	ConnectedAt  time.Time `json:"connectedAt"`
	hub       *Hub
	accountID uint
	send      chan []byte
}

// ── Message Types ──

// SignalPush is the message pushed to agents when a new signal is ready.
type SignalPush struct {
	Type      string      `json:"type"`      // "new_signal" | "kicked" | "heartbeat" | "command"
	AccountID uint        `json:"accountId"`
	Data      interface{} `json:"data,omitempty"`
}

// CommandPush is the message pushed to agents for broker operations.
// Extended from SignalPush — Type="command", RequestID for correlation.
type CommandPush struct {
	Type      string      `json:"type"`      // "command"
	RequestID string      `json:"requestId"` // unique UID for response tracking
	AccountID uint        `json:"accountId"`
	Action    string      `json:"action"`    // sync_positions | get_balance | place_order | cancel_order | query_orders | test
	Payload   interface{} `json:"payload,omitempty"`
}

// CommandResponse is what the agent POSTs back via REST.
type CommandResponse struct {
	RequestID string      `json:"requestId"`
	Status    string      `json:"status"`  // "ok" | "failed"
	Result    interface{} `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// ── Hub Construction ──

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		channels: make(map[uint]map[*AgentClient]bool),
	}
}

// ── Client Management ──

// Register adds a client to its account's channel. If another client already
// exists for this account, it is kicked first (single agent per account).
func (h *Hub) Register(client *AgentClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if existing, ok := h.channels[client.accountID]; ok {
		for c := range existing {
			kickMsg, _ := json.Marshal(SignalPush{
				Type:      "kicked",
				AccountID: client.accountID,
				Data:      map[string]string{"reason": "new_connection"},
			})
			select {
			case c.send <- kickMsg:
			default:
			}
			close(c.send)
			delete(existing, c)
		}
	}

	if h.channels[client.accountID] == nil {
		h.channels[client.accountID] = make(map[*AgentClient]bool)
	}
	h.channels[client.accountID][client] = true
	log.Printf("[ws] agent connected: account=%d (total channels=%d)", client.accountID, len(h.channels))
}

// Unregister removes a client from its channel.
func (h *Hub) Unregister(client *AgentClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.channels[client.accountID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)
			if len(clients) == 0 {
				delete(h.channels, client.accountID)
			}
		}
	}
	log.Printf("[ws] agent disconnected: account=%d", client.accountID)
}

// KickAccount forcefully disconnects all agents for an account.
func (h *Hub) KickAccount(accountID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.channels[accountID]; ok {
		for c := range clients {
			kickMsg, _ := json.Marshal(SignalPush{
				Type:      "kicked",
				AccountID: accountID,
				Data:      map[string]string{"reason": "token_revoked"},
			})
			select {
			case c.send <- kickMsg:
			default:
			}
			close(c.send)
			delete(clients, c)
		}
		delete(h.channels, accountID)
		log.Printf("[ws] kicked all agents for account=%d", accountID)
	}
}

// ── Broadcasting ──

// BroadcastToAccount sends a SignalPush to all agents for an account.
func (h *Hub) BroadcastToAccount(accountID uint, msg SignalPush) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients, ok := h.channels[accountID]
	if !ok {
		return
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] marshal error: %v", err)
		return
	}
	for c := range clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

// BroadcastSignal pushes a new pending_auto signal to the account's agents.
func (h *Hub) BroadcastSignal(accountID uint, signalData interface{}) {
	h.BroadcastToAccount(accountID, SignalPush{
		Type:      "new_signal",
		AccountID: accountID,
		Data:      signalData,
	})
}

// BroadcastCommand sends a command to the account's connected agents.
// The command is wrapped as a SignalPush with Type="command" for backward-compat.
func (h *Hub) BroadcastCommand(accountID uint, cmd CommandPush) {
	h.BroadcastToAccount(accountID, SignalPush{
		Type:      "command",
		AccountID: accountID,
		Data:      cmd,
	})
}

// HasAgent checks if an account has any connected agents.
func (h *Hub) HasAgent(accountID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients, ok := h.channels[accountID]
	return ok && len(clients) > 0
}
