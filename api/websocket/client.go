package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Default values (used when config is not provided)
const (
	defaultWriteWait      = 10 * time.Second
	defaultPongWait       = 60 * time.Second
	defaultMaxMessageSize = 512
	defaultBufferSize     = 1024
	defaultClientBuffer   = 256
)

// WebSocketConfig holds runtime WebSocket configuration
type WebSocketSettings struct {
	WriteWait      time.Duration
	PongWait       time.Duration
	PingPeriod     time.Duration
	MaxMessageSize int64
	ReadBuffer     int
	WriteBuffer    int
	ClientBuffer   int
}

// NewWebSocketSettings creates settings from config or uses defaults
func NewWebSocketSettings(cfg *config.WebSocketConfig) *WebSocketSettings {
	settings := &WebSocketSettings{
		WriteWait:      defaultWriteWait,
		PongWait:       defaultPongWait,
		MaxMessageSize: defaultMaxMessageSize,
		ReadBuffer:     defaultBufferSize,
		WriteBuffer:    defaultBufferSize,
		ClientBuffer:   defaultClientBuffer,
	}

	if cfg != nil {
		if cfg.WriteTimeout > 0 {
			settings.WriteWait = cfg.WriteTimeout
		}
		if cfg.PongTimeout > 0 {
			settings.PongWait = cfg.PongTimeout
		}
		if cfg.MaxMessageSize > 0 {
			settings.MaxMessageSize = cfg.MaxMessageSize
		}
		if cfg.ReadBufferSize > 0 {
			settings.ReadBuffer = cfg.ReadBufferSize
		}
		if cfg.WriteBufferSize > 0 {
			settings.WriteBuffer = cfg.WriteBufferSize
		}
		if cfg.ClientBuffer > 0 {
			settings.ClientBuffer = cfg.ClientBuffer
		}
	}

	// Ping period is derived from pong wait
	settings.PingPeriod = (settings.PongWait * 9) / 10
	return settings
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	clusterID string
	settings  *WebSocketSettings
}

type IncomingMessage struct {
	Type      string `json:"type"`
	ClusterID string `json:"cluster_id,omitempty"`
}

func NewClient(hub *Hub, conn *websocket.Conn, clusterID string) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, hub.settings.ClientBuffer),
		clusterID: clusterID,
		settings:  hub.settings,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.settings.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.settings.PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(c.settings.PongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("WebSocket error: %v", err)
			}
			break
		}

		var msg IncomingMessage
		if err := json.Unmarshal(message, &msg); err == nil {
			c.handleMessage(&msg)
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(c.settings.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.settings.WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket frame
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.settings.WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg *IncomingMessage) {
	switch msg.Type {
	case "subscribe":
		if msg.ClusterID != "" {
			c.clusterID = msg.ClusterID
			logger.Infof("Client subscribed to cluster: %s", msg.ClusterID)
			// Send subscription confirmation
			c.sendConfirmation("subscribed", msg.ClusterID)
		}
	case "unsubscribe":
		oldClusterID := c.clusterID
		c.clusterID = ""
		logger.Info("Client unsubscribed from cluster")
		// Send unsubscription confirmation
		c.sendConfirmation("unsubscribed", oldClusterID)
	}
}

func (c *Client) sendConfirmation(action, clusterID string) {
	confirmation := map[string]interface{}{
		"type":       "subscription_update",
		"action":     action,
		"cluster_id": clusterID,
		"timestamp":  time.Now(),
	}
	data, err := json.Marshal(confirmation)
	if err != nil {
		logger.Errorf("Failed to marshal confirmation: %v", err)
		return
	}
	select {
	case c.send <- data:
	default:
		logger.Warn("Client send channel full, dropping confirmation")
	}
}

func ServeWebSocket(hub *Hub) gin.HandlerFunc {
	// Create upgrader with configurable buffer sizes
	upgrader := websocket.Upgrader{
		ReadBufferSize:  hub.settings.ReadBuffer,
		WriteBufferSize: hub.settings.WriteBuffer,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in dev
		},
	}

	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Errorf("WebSocket upgrade failed: %v", err)
			return
		}

		clusterID := c.Query("cluster_id")
		client := NewClient(hub, conn, clusterID)
		hub.Register(client)

		go client.WritePump()
		go client.ReadPump()
	}
}