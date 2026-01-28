package scenarios

import (
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func newScenarioEngine() *decision.Engine {
	return decision.NewEngine(decision.Config{
		CooldownPeriod:          30 * time.Second,
		ScaleDownCooldownPeriod: 30 * time.Second,
		EmergencyCPUThreshold:   95.0,
		MinServers:              2,
		MaxServers:              10,
		MaxScaleStep:            3,
		CPUHighThreshold:        80.0,
		CPULowThreshold:         30.0,
		SustainedHighDuration:   30 * time.Second,
		SustainedLowDuration:    30 * time.Second,
	})
}

func TestScenario_CPUSpike(t *testing.T) {
	engine := newScenarioEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "cluster-1",
		AvgCPU:    75.0,
		CPUStatus: models.ThresholdNormal,
		HasSpike:  true,
	}
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("spike should trigger scale-up, got %s", result.Action)
	}
}

func TestScenario_SustainedHighLoad(t *testing.T) {
	engine := newScenarioEngine()
	sustainedAt := time.Now().Add(-60 * time.Second)

	analyzed := &models.AnalyzedMetrics{
		ClusterID:       "cluster-1",
		AvgCPU:          85.0,
		CPUStatus:       models.ThresholdWarning,
		SustainedHighAt: &sustainedAt,
	}
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("sustained high should scale-up, got %s", result.Action)
	}
}

func TestScenario_SustainedLowLoad(t *testing.T) {
	engine := newScenarioEngine()
	sustainedAt := time.Now().Add(-60 * time.Second)

	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "cluster-1",
		AvgCPU:         20.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendStable,
		SustainedLowAt: &sustainedAt,
	}
	state := &models.ClusterState{ActiveServers: 6, TotalServers: 6}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleDown {
		t.Errorf("sustained low should scale-down, got %s", result.Action)
	}
}

func TestScenario_EmergencyScaling(t *testing.T) {
	engine := newScenarioEngine()
	engine.RecordScaleUp("cluster-1") // Set cooldown

	analyzed := &models.AnalyzedMetrics{
		ClusterID: "cluster-1",
		AvgCPU:    96.0,
		CPUStatus: models.ThresholdCritical,
	}
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("emergency should bypass cooldown, got %s", result.Action)
	}
	if !result.IsEmergency {
		t.Error("should be marked as emergency")
	}
}

func TestScenario_CooldownPreventsFlapping(t *testing.T) {
	engine := newScenarioEngine()
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}

	// First scale-up
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "cluster-1",
		AvgCPU:    85.0,
		CPUStatus: models.ThresholdWarning,
		HasSpike:  true,
	}
	result := engine.Decide(analyzed, nil, state)
	if result.Action != models.ActionScaleUp {
		t.Fatalf("first should scale-up, got %s", result.Action)
	}
	engine.RecordScaleUp("cluster-1")

	// Second attempt should be blocked
	result = engine.Decide(analyzed, nil, state)
	if !result.CooldownActive {
		t.Error("cooldown should block second scale-up")
	}
}

func TestScenario_MaxCapacityReached(t *testing.T) {
	engine := newScenarioEngine()
	analyzed := &models.AnalyzedMetrics{
		ClusterID: "cluster-1",
		AvgCPU:    90.0,
		CPUStatus: models.ThresholdCritical,
	}
	state := &models.ClusterState{ActiveServers: 10, TotalServers: 10}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("at max capacity should maintain, got %s", result.Action)
	}
}

func TestScenario_MinCapacityProtection(t *testing.T) {
	engine := newScenarioEngine()
	sustainedAt := time.Now().Add(-60 * time.Second)

	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "cluster-1",
		AvgCPU:         15.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendStable,
		SustainedLowAt: &sustainedAt,
	}
	state := &models.ClusterState{ActiveServers: 2, TotalServers: 2}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("at min capacity should maintain, got %s", result.Action)
	}
}

func TestScenario_RisingTrendBlocksScaleDown(t *testing.T) {
	engine := newScenarioEngine()
	sustainedAt := time.Now().Add(-60 * time.Second)

	analyzed := &models.AnalyzedMetrics{
		ClusterID:      "cluster-1",
		AvgCPU:         25.0,
		CPUStatus:      models.ThresholdNormal,
		Trend:          models.TrendRising,
		SustainedLowAt: &sustainedAt,
	}
	state := &models.ClusterState{ActiveServers: 6, TotalServers: 6}

	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionMaintain {
		t.Errorf("rising trend should block scale-down, got %s", result.Action)
	}
}

func TestScenario_CollectorFailureAndRecovery(t *testing.T) {
	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	// Simulate high CPU tracking
	analyzed := &models.AnalyzedMetrics{AvgCPU: 85.0}
	tracker.Update("cluster-1", analyzed, cfg)

	if tracker.GetHighDuration("cluster-1") == 0 {
		t.Error("should track high duration")
	}

	// After recovery (CPU drops)
	analyzed = &models.AnalyzedMetrics{AvgCPU: 50.0}
	tracker.Update("cluster-1", analyzed, cfg)

	if tracker.GetHighDuration("cluster-1") != 0 {
		t.Error("high duration should reset after recovery")
	}
}
