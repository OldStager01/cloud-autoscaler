package models

import "time"

type ScalingAction string

const (
	ActionScaleUp   ScalingAction = "SCALE_UP"
	ActionScaleDown ScalingAction = "SCALE_DOWN"
	ActionMaintain  ScalingAction = "MAINTAIN"
)

// ScalingDecision represents a scaling decision made by the decision engine
type ScalingDecision struct {
	ClusterID      string        `json:"cluster_id"`
	Timestamp      time.Time     `json:"timestamp"`
	Action         ScalingAction `json:"action"`
	CurrentServers int           `json:"current_servers"`
	TargetServers  int           `json:"target_servers"`
	Reason         string        `json:"reason"`
	PredictionUsed bool          `json:"prediction_used"`
	Confidence     float64       `json:"confidence,omitempty"`
	IsEmergency    bool          `json:"is_emergency"`
	CooldownActive bool          `json:"cooldown_active"`
}

func (d *ScalingDecision) ServerDelta() int {
	return d.TargetServers - d.CurrentServers
}

func (d *ScalingDecision) ShouldExecute() bool {
	return d.Action != ActionMaintain && !d.CooldownActive
}