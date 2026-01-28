package unit

import (
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func TestClusterMetrics_CalculateAggregates(t *testing.T) {
	metrics := &models.ClusterMetrics{
		ClusterID: "test-cluster",
		Timestamp: time.Now(),
		Servers: []models.ServerMetric{
			{ServerID: "s1", CPUUsage: 40.0, MemoryUsage: 50.0, RequestLoad: 100},
			{ServerID: "s2", CPUUsage: 60.0, MemoryUsage: 70.0, RequestLoad: 200},
		},
	}

	agg := metrics.CalculateAggregates()

	if agg.AvgCPU != 50.0 {
		t.Errorf("expected AvgCPU 50.0, got %f", agg.AvgCPU)
	}
	if agg.AvgMemory != 60.0 {
		t.Errorf("expected AvgMemory 60.0, got %f", agg.AvgMemory)
	}
	if agg.MaxCPU != 60.0 {
		t.Errorf("expected MaxCPU 60.0, got %f", agg.MaxCPU)
	}
	if agg.MinCPU != 40.0 {
		t.Errorf("expected MinCPU 40.0, got %f", agg.MinCPU)
	}
	if agg.ServerCount != 2 {
		t.Errorf("expected ServerCount 2, got %d", agg.ServerCount)
	}
}

func TestClusterState_CanScaleUp(t *testing.T) {
	state := &models.ClusterState{TotalServers: 5}

	if !state.CanScaleUp(10) {
		t.Error("expected CanScaleUp to be true when below max")
	}
	if state.CanScaleUp(5) {
		t.Error("expected CanScaleUp to be false when at max")
	}
}

func TestClusterState_CanScaleDown(t *testing.T) {
	state := &models.ClusterState{ActiveServers: 5}

	if !state.CanScaleDown(2) {
		t.Error("expected CanScaleDown to be true when above min")
	}
	if state.CanScaleDown(5) {
		t.Error("expected CanScaleDown to be false when at min")
	}
}

func TestScalingDecision_ShouldExecute(t *testing.T) {
	tests := []struct {
		name     string
		decision models.ScalingDecision
		want     bool
	}{
		{
			name:     "ScaleUp without cooldown",
			decision: models.ScalingDecision{Action: models.ActionScaleUp, CooldownActive: false},
			want:     true,
		},
		{
			name:     "Maintain action",
			decision: models.ScalingDecision{Action: models.ActionMaintain, CooldownActive: false},
			want:     false,
		},
		{
			name:     "ScaleUp with cooldown",
			decision: models.ScalingDecision{Action: models.ActionScaleUp, CooldownActive: true},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.decision.ShouldExecute(); got != tt.want {
				t.Errorf("ShouldExecute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzedMetrics_IsCritical(t *testing.T) {
	tests := []struct {
		name     string
		analyzed models.AnalyzedMetrics
		want     bool
	}{
		{
			name:     "CPU critical",
			analyzed: models.AnalyzedMetrics{CPUStatus: models.ThresholdCritical, MemoryStatus: models.ThresholdNormal},
			want:     true,
		},
		{
			name:     "Memory critical",
			analyzed: models.AnalyzedMetrics{CPUStatus: models.ThresholdNormal, MemoryStatus: models.ThresholdCritical},
			want:     true,
		},
		{
			name:     "Both normal",
			analyzed: models.AnalyzedMetrics{CPUStatus: models.ThresholdNormal, MemoryStatus: models.ThresholdNormal},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.analyzed.IsCritical(); got != tt.want {
				t.Errorf("IsCritical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAnalyzedMetrics_IsWarning(t *testing.T) {
	analyzed := models.AnalyzedMetrics{
		CPUStatus:    models.ThresholdWarning,
		MemoryStatus: models.ThresholdNormal,
	}

	if !analyzed.IsWarning() {
		t.Error("expected IsWarning to be true")
	}
}
