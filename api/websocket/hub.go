package websocket

import (
	"sync"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
)

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Infof("WebSocket client connected (total: %d)", h.ClientCount())

		case client := <-h. unregister:
			h. mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h. clients, client)
				close(client.send)
			}
			h.mu. Unlock()
			logger.Infof("WebSocket client disconnected (total: %d)", h.ClientCount())

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		logger.Warn("Broadcast channel full, dropping message")
	}
}

func (h *Hub) BroadcastToCluster(clusterID string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.clusterID == "" || client.clusterID == clusterID {
			select {
			case client.send <- message:
			default:
			}
		}
	}
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}