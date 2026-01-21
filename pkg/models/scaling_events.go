package models

import "time"

type ScalingEventStatus string

const (
	ScalingEventSuccess ScalingEventStatus = "success"
	ScalingEventFailed  ScalingEventStatus = "failed"
	ScalingEventPartial ScalingEventStatus = "partial"
)

// ScalingEvent represents a recorded scaling action
type ScalingEvent struct {
	ID             int                `json:"id"`
	ClusterID      string             `json:"cluster_id"`
	Timestamp      time.Time          `json:"timestamp"`
	Action         ScalingAction      `json:"action"`
	ServersBefore  int                `json:"servers_before"`
	ServersAfter   int                `json:"servers_after"`
	TriggerReason  string             `json:"trigger_reason"`
	PredictionUsed bool               `json:"prediction_used"`
	Confidence     *float64           `json:"confidence,omitempty"`
	Status         ScalingEventStatus `json:"status"`
}

func NewScalingEvent(decision ScalingDecision, status ScalingEventStatus) *ScalingEvent {
	event := &ScalingEvent{
		ClusterID:      decision.ClusterID,
		Timestamp:      decision.Timestamp,
		Action:         decision.Action,
		ServersBefore:  decision.CurrentServers,
		ServersAfter:   decision.TargetServers,
		TriggerReason:  decision.Reason,
		PredictionUsed:  decision.PredictionUsed,
		Status:         status,
	}
	if decision.Confidence > 0 {
		event.Confidence = &decision.Confidence
	}
	return event
}