package unit

import (
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func newTestEngine() *decision.Engine {
	return decision.NewEngine(decision.Config{
		CooldownPeriod:          30 * time.Second,
		ScaleDownCooldownPeriod: 30 * time.Second,
		EmergencyCPUThreshold:   95.0,
		MinServers:              2,
		MaxServers:              10,
		MaxScaleStep:            3,
		TargetCPU:               70.0,
		CPUHighThreshold:        80.0,
		CPULowThreshold:         30.0,
		SustainedHighDuration:   30 * time.Second,
		SustainedLowDuration:    30 * time.Second,
	})
}

func TestEngine_Decide_EmergencyScaleUp(t *testing.T) {
	engine := newTestEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    96.0,
		CPUStatus: models.ThresholdCritical,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("expected ScaleUp, got %s", result.Action)
	}
	if result.IsEmergency != true {
		t.Error("expected IsEmergency to be true")
	}
}

func TestEngine_Decide_ScaleUp_CriticalCPU(t *testing.T) {
	engine := newTestEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    92.0,
		CPUStatus: models.ThresholdCritical,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("expected ScaleUp, got %s", result.Action)
	}
}

func TestEngine_Decide_ScaleUp_SustainedHighCPU(t *testing.T) {
	engine := newTestEngine()
	sustainedTime := time.Now().Add(-60 * time.Second)
	analyzed := &models.AnalyzedMetrics{
		ClusterID:       "test-cluster",
		AvgCPU:          85.0,
		CPUStatus:       models.ThresholdWarning,
		SustainedHighAt: &sustainedTime,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("expected ScaleUp, got %s", result.Action)
	}
}

func TestEngine_Decide_ScaleDown_SustainedLowCPU(t *testing.T) {
	engine := newTestEngine()
	sustainedTime := time.Now().Add(-60 * time.Second)
	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "test-cluster",
		AvgCPU:         20.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendStable,
		SustainedLowAt: &sustainedTime,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleDown {
		t.Errorf("expected ScaleDown, got %s", result.Action)
	}
}

func TestEngine_Decide_Maintain_NormalParams(t *testing.T) {
	engine := newTestEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    50.0,
		CPUStatus: models.ThresholdNormal,
		Trend:     models.TrendStable,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("expected Maintain, got %s", result.Action)
	}
}

func TestEngine_Decide_CooldownActive(t *testing.T) {
	engine := newTestEngine()
	engine.RecordScaleUp("test-cluster")

	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    85.0,
		CPUStatus: models.ThresholdWarning,
		Trend:     models.TrendRising,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.CooldownActive != true {
		t.Error("expected CooldownActive to be true")
	}
}

func TestEngine_Decide_MaxServersReached(t *testing.T) {
	engine := newTestEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    85.0,
		CPUStatus: models.ThresholdWarning,
		HasSpike:  true,
	}
	state := &models.ClusterState{ActiveServers: 10, TotalServers: 10}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("expected Maintain at max servers, got %s", result.Action)
	}
}

func TestEngine_Decide_MinServersReached(t *testing.T) {
	engine := newTestEngine()
	sustainedTime := time.Now().Add(-60 * time.Second)
	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "test-cluster",
		AvgCPU:         20.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendStable,
		SustainedLowAt: &sustainedTime,
	}
	state := &models.ClusterState{ActiveServers: 2, TotalServers: 2}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("expected Maintain at min servers, got %s", result.Action)
	}
}

func TestEngine_Decide_ScaleUp_SpikeDetected(t *testing.T) {
	engine := newTestEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "test-cluster",
		AvgCPU:    75.0,
		CPUStatus: models.ThresholdNormal,
		HasSpike:  true,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("expected ScaleUp on spike, got %s", result.Action)
	}
}

func TestEngine_Decide_ScaleDown_BlockedByRisingTrend(t *testing.T) {
	engine := newTestEngine()
	sustainedTime := time.Now().Add(-60 * time.Second)
	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "test-cluster",
		AvgCPU:         25.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendRising,
		SustainedLowAt: &sustainedTime,
	}
	state := &models.ClusterState{ActiveServers: 5, TotalServers: 5}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("expected Maintain due to rising trend, got %s", result.Action)
	}
}
