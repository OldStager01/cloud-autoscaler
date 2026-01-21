package models

import "time"

type EventType string

const (
	EventTypeMetricCollected EventType = "metric_collected"
	EventTypeMetricAnalyzed  EventType = "metric_analyzed"
	EventTypeDecisionMade    EventType = "decision_made"
	EventTypeScalingStarted  EventType = "scaling_started"
	EventTypeScalingComplete EventType = "scaling_complete"
	EventTypeScalingFailed   EventType = "scaling_failed"
	EventTypeServerAdded     EventType = "server_added"
	EventTypeServerRemoved   EventType = "server_removed"
	EventTypeServerActivated EventType = "server_activated"
	EventTypeAlert           EventType = "alert"
	EventTypeError           EventType = "error"
)

type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityCritical EventSeverity = "critical"
)

// Event represents an internal system event
type Event struct {
	ID        string        `json:"id"`
	Type      EventType     `json:"type"`
	Severity  EventSeverity `json:"severity"`
	ClusterID string        `json:"cluster_id,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Message   string        `json:"message"`
	Data      interface{}   `json:"data,omitempty"`
	TraceID   string        `json:"trace_id,omitempty"`
}

func NewEvent(eventType EventType, clusterID, message string) *Event {
	return &Event{
		ID:         NewUUID(),
		Type:      eventType,
		Severity:  SeverityInfo,
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Message:   message,
	}
}

func (e *Event) WithSeverity(severity EventSeverity) *Event {
	e.Severity = severity
	return e
}

func (e *Event) WithData(data interface{}) *Event {
	e.Data = data
	return e
}

func (e *Event) WithTraceID(traceID string) *Event {
	e.TraceID = traceID
	return e
}