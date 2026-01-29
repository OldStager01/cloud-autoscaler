package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func TestClusterMetrics_CalculateAggregates(t *testing.T) {
	tests := []struct {
		name        string
		servers     []models.ServerMetric
		expectedAvg float64
		expectedMax float64
		expectedMin float64
		expectedCnt int
	}{
		{
			name: "two servers",
			servers: []models.ServerMetric{
				{ServerID: "s1", CPUUsage: 40.0, MemoryUsage: 50.0, RequestLoad: 100},
				{ServerID: "s2", CPUUsage: 60.0, MemoryUsage: 70.0, RequestLoad: 200},
			},
			expectedAvg: 50.0,
			expectedMax: 60.0,
			expectedMin: 40.0,
			expectedCnt: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := &models.ClusterMetrics{
				ClusterID: "test-cluster",
				Timestamp: time.Now(),
				Servers:   tt.servers,
			}

			agg := metrics.CalculateAggregates()

			assert.Equal(t, tt.expectedAvg, agg.AvgCPU)
			assert.Equal(t, tt.expectedMax, agg.MaxCPU)
			assert.Equal(t, tt.expectedMin, agg.MinCPU)
			assert.Equal(t, tt.expectedCnt, agg.ServerCount)
		})
	}
}

func TestClusterState_CanScaleUp(t *testing.T) {
	tests := []struct {
		name         string
		totalServers int
		maxServers   int
		expected     bool
	}{
		{
			name:         "can scale up when below max",
			totalServers: 5,
			maxServers:   10,
			expected:     true,
		},
		{
			name:         "cannot scale up when at max",
			totalServers: 5,
			maxServers:   5,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.ClusterState{TotalServers: tt.totalServers}

			result := state.CanScaleUp(tt.maxServers)

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClusterState_CanScaleDown(t *testing.T) {
	tests := []struct {
		name          string
		activeServers int
		minServers    int
		expected      bool
	}{
		{
			name:          "can scale down when above min",
			activeServers: 5,
			minServers:    2,
			expected:      true,
		},
		{
			name:          "cannot scale down when at min",
			activeServers: 5,
			minServers:    5,
			expected:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &models.ClusterState{ActiveServers: tt.activeServers}

			result := state.CanScaleDown(tt.minServers)

			assert.Equal(t, tt.expected, result)
		})
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
			result := tt.decision.ShouldExecute()

			assert.Equal(t, tt.want, result)
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
			result := tt.analyzed.IsCritical()

			assert.Equal(t, tt.want, result)
		})
	}
}

func TestAnalyzedMetrics_IsWarning(t *testing.T) {
	analyzed := models.AnalyzedMetrics{
		CPUStatus:    models.ThresholdWarning,
		MemoryStatus: models.ThresholdNormal,
	}

	assert.True(t, analyzed.IsWarning(), "expected IsWarning to be true")
}
