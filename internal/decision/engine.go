package decision

import (
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type Config struct {
	CooldownPeriod        time.Duration
	EmergencyCPUThreshold float64
	MinServers            int
	MaxServers            int
	MaxScaleStep          int
	TargetCPU             float64
	CPUHighThreshold      float64
	CPULowThreshold       float64
	SustainedHighDuration time.Duration
	SustainedLowDuration  time.Duration
}

type Engine struct {
	config         Config
	lastScaleTimes map[string]time.Time
	mu             sync.RWMutex
}

func NewEngine(cfg Config) *Engine {
	if cfg.CooldownPeriod == 0 {
		cfg.CooldownPeriod = 5 * time.Minute
	}
	if cfg.EmergencyCPUThreshold == 0 {
		cfg.EmergencyCPUThreshold = 95.0
	}
	if cfg.MinServers == 0 {
		cfg.MinServers = 2
	}
	if cfg.MaxServers == 0 {
		cfg.MaxServers = 50
	}
	if cfg.MaxScaleStep == 0 {
		cfg.MaxScaleStep = 3
	}
	if cfg.TargetCPU == 0 {
		cfg.TargetCPU = 70.0
	}
	if cfg.CPUHighThreshold == 0 {
		cfg.CPUHighThreshold = 80.0
	}
	if cfg.CPULowThreshold == 0 {
		cfg.CPULowThreshold = 30.0
	}
	if cfg.SustainedHighDuration == 0 {
		cfg.SustainedHighDuration = 2 * time.Minute
	}
	if cfg.SustainedLowDuration == 0 {
		cfg.SustainedLowDuration = 10 * time.Minute
	}

	return &Engine{
		config:         cfg,
		lastScaleTimes: make(map[string]time.Time),
	}
}

func (e *Engine) Decide(
	analyzed *models.AnalyzedMetrics,
	prediction *models.Prediction,
	state *models.ClusterState,
) *models.ScalingDecision {
	decision := &models.ScalingDecision{
		ClusterID:      analyzed.ClusterID,
		Timestamp:      time.Now(),
		CurrentServers: state.ActiveServers,
		TargetServers:  state.ActiveServers,
		Action:         models.ActionMaintain,
	}

	// Emergency override - bypass cooldown for critical CPU
	if analyzed.AvgCPU >= e.config.EmergencyCPUThreshold {
		return e.createScaleUpDecision(decision, state, 3, "emergency_cpu_critical", true)
	}

	// Check cooldown
	if e.isInCooldown(analyzed.ClusterID) {
		decision.CooldownActive = true
		decision.Reason = "in_cooldown"
		logger.WithCluster(analyzed.ClusterID).Debug("Decision:  maintain (cooldown active)")
		return decision
	}

	// Scale up conditions
	if scaleUp, reason := e.shouldScaleUp(analyzed, prediction, state); scaleUp {
		targetDelta := e.calculateScaleUpDelta(analyzed, state)
		predictionUsed := prediction != nil && reason == "predicted_spike_proactive"
		return e.createScaleUpDecision(decision, state, targetDelta, reason, false, predictionUsed)
	}

	// Scale down conditions
	if scaleDown, reason := e.shouldScaleDown(analyzed, prediction, state); scaleDown {
		targetDelta := e.calculateScaleDownDelta(analyzed, state)
		return e.createScaleDownDecision(decision, state, targetDelta, reason)
	}

	decision.Reason = "within_normal_parameters"
	logger.WithCluster(analyzed.ClusterID).Debug("Decision: maintain (normal parameters)")
	return decision
}

func (e *Engine) shouldScaleUp(
	analyzed *models.AnalyzedMetrics,
	prediction *models.Prediction,
	state *models.ClusterState,
) (bool, string) {
	// Check capacity
	if !state.CanScaleUp(e.config.MaxServers) {
		return false, ""
	}

	// Critical status
	if analyzed.CPUStatus == models.ThresholdCritical {
		return true, "cpu_critical"
	}

	// Spike detected
	if analyzed.HasSpike {
		return true, "spike_detected"
	}

	// Sustained high CPU with rising trend
	if analyzed.CPUStatus == models.ThresholdWarning && analyzed.Trend == models.TrendRising {
		if analyzed.SustainedHighAt != nil {
			duration := time.Since(*analyzed.SustainedHighAt)
			if duration >= e.config.SustainedHighDuration {
				return true, "sustained_high_rising"
			}
		}
		return true, "warning_rising_trend"
	}

	// Sustained high CPU
	if analyzed.SustainedHighAt != nil {
		duration := time.Since(*analyzed.SustainedHighAt)
		if duration >= e.config.SustainedHighDuration {
			return true, "sustained_high_cpu"
		}
	}

	// Proactive scaling based on prediction
	if prediction != nil && prediction.IsHighConfidence(0.7) {
		if prediction.PredictedCPU >= e.config.CPUHighThreshold {
			return true, "predicted_spike_proactive"
		}
	}

	return false, ""
}

func (e *Engine) shouldScaleDown(
	analyzed *models.AnalyzedMetrics,
	prediction *models.Prediction,
	state *models.ClusterState,
) (bool, string) {
	// Check capacity
	if !state.CanScaleDown(e.config.MinServers) {
		return false, ""
	}

	// Don't scale down if trend is rising
	if analyzed.Trend == models.TrendRising {
		return false, ""
	}

	// Don't scale down if prediction shows upcoming spike
	if prediction != nil && prediction.IsHighConfidence(0.7) {
		if prediction.PredictedCPU >= e.config.CPUHighThreshold {
			return false, ""
		}
	}

	// Sustained low CPU
	if analyzed.SustainedLowAt != nil {
		duration := time.Since(*analyzed.SustainedLowAt)
		if duration >= e.config.SustainedLowDuration && analyzed.AvgCPU < e.config.CPULowThreshold {
			return true, "sustained_low_cpu"
		}
	}

	// Very low CPU with stable/falling trend
	if analyzed.AvgCPU < e.config.CPULowThreshold && analyzed.Trend == models.TrendFalling {
		return true, "low_cpu_falling_trend"
	}

	return false, ""
}

func (e *Engine) calculateScaleUpDelta(analyzed *models.AnalyzedMetrics, state *models.ClusterState) int {
	if analyzed.AvgCPU >= e.config.EmergencyCPUThreshold {
		return e.config.MaxScaleStep
	}

	// Calculate based on current vs target CPU
	if analyzed.AvgCPU > 0 && state.ActiveServers > 0 {
		ratio := analyzed.AvgCPU / e.config.TargetCPU
		idealServers := int(float64(state.ActiveServers) * ratio)
		delta := idealServers - state.ActiveServers

		if delta < 1 {
			delta = 1
		}
		if delta > e.config.MaxScaleStep {
			delta = e.config.MaxScaleStep
		}
		return delta
	}

	return 1
}

func (e *Engine) calculateScaleDownDelta(analyzed *models.AnalyzedMetrics, state *models.ClusterState) int {
	// Conservative scale down - always 1 at a time
	return 1
}

func (e *Engine) createScaleUpDecision(
	decision *models.ScalingDecision,
	state *models.ClusterState,
	delta int,
	reason string,
	isEmergency bool,
	predictionUsed ...bool,
) *models.ScalingDecision {
	targetServers := state.ActiveServers + delta
	maxAllowed := e.config.MaxServers

	if targetServers > maxAllowed {
		targetServers = maxAllowed
	}

	decision.Action = models.ActionScaleUp
	decision.TargetServers = targetServers
	decision.Reason = reason
	decision.IsEmergency = isEmergency

	if len(predictionUsed) > 0 && predictionUsed[0] {
		decision.PredictionUsed = true
	}

	logger.WithCluster(decision.ClusterID).Infof(
		"Decision: scale_up %d -> %d servers (reason: %s, emergency: %v)",
		decision.CurrentServers, decision.TargetServers, reason, isEmergency,
	)

	return decision
}

func (e *Engine) createScaleDownDecision(
	decision *models.ScalingDecision,
	state *models.ClusterState,
	delta int,
	reason string,
) *models.ScalingDecision {
	targetServers := state.ActiveServers - delta
	minAllowed := e.config.MinServers

	if targetServers < minAllowed {
		targetServers = minAllowed
	}

	decision.Action = models.ActionScaleDown
	decision.TargetServers = targetServers
	decision.Reason = reason

	logger.WithCluster(decision.ClusterID).Infof(
		"Decision: scale_down %d -> %d servers (reason: %s)",
		decision.CurrentServers, decision.TargetServers, reason,
	)

	return decision
}

func (e *Engine) isInCooldown(clusterID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lastScale, exists := e.lastScaleTimes[clusterID]
	if !exists {
		return false
	}

	return time.Since(lastScale) < e.config.CooldownPeriod
}

func (e *Engine) RecordScaling(clusterID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.lastScaleTimes[clusterID] = time.Now()
}

func (e *Engine) ResetCooldown(clusterID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.lastScaleTimes, clusterID)
}

func (e *Engine) GetCooldownRemaining(clusterID string) time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	lastScale, exists := e.lastScaleTimes[clusterID]
	if !exists {
		return 0
	}

	elapsed := time.Since(lastScale)
	if elapsed >= e.config.CooldownPeriod {
		return 0
	}

	return e.config.CooldownPeriod - elapsed
}