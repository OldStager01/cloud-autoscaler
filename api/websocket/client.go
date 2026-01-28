package websocket

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in dev
	},
}

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	clusterID string
}

type IncomingMessage struct {
	Type      string `json:"type"`
	ClusterID string `json:"cluster_id,omitempty"`
}

func NewClient(hub *Hub, conn *websocket.Conn, clusterID string) *Client {
	return &Client{
		hub:       hub,
		conn:       conn,
		send:      make(chan []byte, 256),
		clusterID: clusterID,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
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
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
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
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
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