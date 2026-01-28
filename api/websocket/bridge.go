package websocket

import (
	"context"
	"encoding/json"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

// EventBridge bridges orchestrator events to WebSocket clients
type EventBridge struct {
	hub        *Hub
	eventsChan <-chan *models.Event
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewEventBridge creates a new bridge between orchestrator events and WebSocket
func NewEventBridge(hub *Hub, eventsChan <-chan *models.Event) *EventBridge {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventBridge{
		hub:        hub,
		eventsChan: eventsChan,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins listening for events and forwarding to WebSocket clients
func (b *EventBridge) Start() {
	go b.run()
	logger.Info("WebSocket event bridge started")
}

// Stop stops the event bridge
func (b *EventBridge) Stop() {
	b.cancel()
	logger.Info("WebSocket event bridge stopped")
}

func (b *EventBridge) run() {
	for {
		select {
		case <-b.ctx.Done():
			return
		case event, ok := <-b.eventsChan:
			if !ok {
				logger.Info("Event channel closed, stopping bridge")
				return
			}
			b.forwardEvent(event)
		}
	}
}

func (b *EventBridge) forwardEvent(event *models.Event) {
	// Convert internal event to WebSocket message format
	wsMessage := b.convertToWSMessage(event)
	if wsMessage == nil {
		return
	}

	data, err := json.Marshal(wsMessage)
	if err != nil {
		logger.Errorf("Failed to marshal WebSocket message: %v", err)
		return
	}

	// Broadcast to all clients subscribed to this cluster
	b.hub.BroadcastToCluster(event.ClusterID, data)
}

// WebSocketEvent is the message format sent to WebSocket clients
type WebSocketEvent struct {
	Type      string      `json:"type"`
	ClusterID string      `json:"cluster_id"`
	Timestamp time.Time   `json:"timestamp"`
	Severity  string      `json:"severity,omitempty"`
	Message   string      `json:"message,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

func (b *EventBridge) convertToWSMessage(event *models.Event) *WebSocketEvent {
	// Map internal event types to WebSocket message types
	wsType := mapEventType(event.Type)
	if wsType == "" {
		return nil // Skip events we don't want to broadcast
	}

	return &WebSocketEvent{
		Type:      wsType,
		ClusterID: event.ClusterID,
		Timestamp: event.Timestamp,
		Severity:  string(event.Severity),
		Message:   event.Message,
		Data:      event.Data,
	}
}

func mapEventType(eventType models.EventType) string {
	switch eventType {
	case models.EventTypeMetricAnalyzed:
		return "metrics"
	case models.EventTypeScalingStarted:
		return "scaling_started"
	case models.EventTypeScalingComplete:
		return "scaling_event"
	case models.EventTypeScalingFailed:
		return "scaling_failed"
	case models.EventTypeAlert:
		return "alert"
	case models.EventTypeServerAdded, models.EventTypeServerRemoved, models.EventTypeServerActivated:
		return "server_update"
	case models.EventTypeDecisionMade:
		return "decision"
	case models.EventTypeError:
		return "error"
	default:
		// Skip metric_collected and other internal events
		return ""
	}
}
