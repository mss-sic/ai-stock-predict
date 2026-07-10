package ws

import (
	"log"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ai-stock-predict/server/internal/db"
	"github.com/ai-stock-predict/server/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for agent connections
	},
}

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// HandleAgentWS upgrades an HTTP connection to WebSocket after validating the agent token.
// Query param: ?token=tk_xxx
func HandleAgentWS(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing agent token"})
			return
		}

		// Resolve token → account
		var account model.TradingAccount
		if err := db.MySQL.Where("agent_token = ?", token).
			First(&account).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}
		if account.AgentToken != token || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token mismatch"})
			return
		}
		if !model.IsAgentMode(account.BrokerMode) {
			c.JSON(http.StatusForbidden, gin.H{"error": "account not configured for agent execution"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("[ws] upgrade error: %v", err)
			return
		}

		client := &AgentClient{
			hub:         hub,
			accountID:   account.ID,
			send:        make(chan []byte, 64),
			ConnectedAt: time.Now(),
		}

		hub.Register(client)

		go writePump(client, conn)
		go readPump(client, conn)
	}
}

func readPump(client *AgentClient, conn *websocket.Conn) {
	defer func() {
		client.hub.Unregister(client)
		conn.Close()
	}()

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[ws] read error: %v", err)
			}
			break
		}

		// 解析 agent 发来的消息
		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data,omitempty"`
		}
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[ws] invalid message from account=%d: %v", client.accountID, err)
			continue
		}

		switch msg.Type {
		case "agent_hello":
			var hello struct {
				TraderType   string   `json:"traderType"`
				Capabilities []string `json:"capabilities"`
			}
			if err := json.Unmarshal(msg.Data, &hello); err != nil {
				log.Printf("[ws] invalid agent_hello from account=%d: %v", client.accountID, err)
				continue
			}
			client.TraderType = hello.TraderType
			client.Capabilities = hello.Capabilities
			client.ConnectedAt = time.Now()
			log.Printf("[ws] agent_hello account=%d trader=%s caps=%v",
				client.accountID, client.TraderType, client.Capabilities)
		case "heartbeat":
			// agent 心跳，刷新 pong deadline 已足够
		default:
			log.Printf("[ws] unknown message type from account=%d: %s", client.accountID, msg.Type)
		}
	}
}

func writePump(client *AgentClient, conn *websocket.Conn) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[ws] write error: %v", err)
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
