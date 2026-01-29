package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

func TestEngine_Decide(t *testing.T) {
	sustainedPast := time.Now().Add(-60 * time.Second)

	tests := []struct {
		name             string
		analyzed         *models.AnalyzedMetrics
		state            *models.ClusterState
		expectedAction   models.ScalingAction
		expectedEmerg    bool
		checkCooldown    bool
		expectedCooldown bool
	}{
		{
			name: "emergency scale up on very high CPU",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "test-cluster",
				AvgCPU:    96.0,
				CPUStatus: models.ThresholdCritical,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionScaleUp,
			expectedEmerg:  true,
		},
		{
			name: "scale up on critical CPU",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "test-cluster",
				AvgCPU:    92.0,
				CPUStatus: models.ThresholdCritical,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionScaleUp,
			expectedEmerg:  false,
		},
		{
			name: "scale up on sustained high CPU",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:       "test-cluster",
				AvgCPU:          85.0,
				CPUStatus:       models.ThresholdWarning,
				SustainedHighAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionScaleUp,
		},
		{
			name: "scale down on sustained low CPU",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "test-cluster",
				AvgCPU:         20.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendStable,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionScaleDown,
		},
		{
			name: "maintain on normal params",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "test-cluster",
				AvgCPU:    50.0,
				CPUStatus: models.ThresholdNormal,
				Trend:     models.TrendStable,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionMaintain,
		},
		{
			name: "maintain at max servers even with spike",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "test-cluster",
				AvgCPU:    85.0,
				CPUStatus: models.ThresholdWarning,
				HasSpike:  true,
			},
			state:          &models.ClusterState{ActiveServers: 10, TotalServers: 10},
			expectedAction: models.ActionMaintain,
		},
		{
			name: "maintain at min servers even with low CPU",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "test-cluster",
				AvgCPU:         20.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendStable,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 2, TotalServers: 2},
			expectedAction: models.ActionMaintain,
		},
		{
			name: "scale up on spike detected",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "test-cluster",
				AvgCPU:    75.0,
				CPUStatus: models.ThresholdNormal,
				HasSpike:  true,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionScaleUp,
		},
		{
			name: "maintain due to rising trend blocking scale down",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "test-cluster",
				AvgCPU:         25.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendRising,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 5, TotalServers: 5},
			expectedAction: models.ActionMaintain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newTestEngine()

			result := engine.Decide(tt.analyzed, nil, tt.state)

			assert.Equal(t, tt.expectedAction, result.Action)
			if tt.expectedEmerg {
				assert.True(t, result.IsEmergency, "expected IsEmergency to be true")
			}
			if tt.checkCooldown {
				assert.Equal(t, tt.expectedCooldown, result.CooldownActive)
			}
		})
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

	assert.True(t, result.CooldownActive, "expected CooldownActive to be true")
}
