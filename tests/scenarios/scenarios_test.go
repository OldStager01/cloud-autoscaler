package scenarios

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

func TestScenario_ScalingDecisions(t *testing.T) {
	sustainedPast := time.Now().Add(-60 * time.Second)

	tests := []struct {
		name           string
		analyzed       *models.AnalyzedMetrics
		state          *models.ClusterState
		setupCooldown  bool
		expectedAction models.ScalingAction
		expectedEmerg  bool
	}{
		{
			name: "CPU spike triggers scale-up",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "cluster-1",
				AvgCPU:    75.0,
				CPUStatus: models.ThresholdNormal,
				HasSpike:  true,
			},
			state:          &models.ClusterState{ActiveServers: 4, TotalServers: 4},
			expectedAction: models.ActionScaleUp,
		},
		{
			name: "sustained high load triggers scale-up",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:       "cluster-1",
				AvgCPU:          85.0,
				CPUStatus:       models.ThresholdWarning,
				SustainedHighAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 4, TotalServers: 4},
			expectedAction: models.ActionScaleUp,
		},
		{
			name: "sustained low load triggers scale-down",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "cluster-1",
				AvgCPU:         20.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendStable,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 6, TotalServers: 6},
			expectedAction: models.ActionScaleDown,
		},
		{
			name: "emergency bypasses cooldown",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "cluster-1",
				AvgCPU:    96.0,
				CPUStatus: models.ThresholdCritical,
			},
			state:          &models.ClusterState{ActiveServers: 4, TotalServers: 4},
			setupCooldown:  true,
			expectedAction: models.ActionScaleUp,
			expectedEmerg:  true,
		},
		{
			name: "max capacity reached maintains",
			analyzed: &models.AnalyzedMetrics{
				ClusterID: "cluster-1",
				AvgCPU:    90.0,
				CPUStatus: models.ThresholdCritical,
			},
			state:          &models.ClusterState{ActiveServers: 10, TotalServers: 10},
			expectedAction: models.ActionMaintain,
		},
		{
			name: "min capacity protection maintains",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "cluster-1",
				AvgCPU:         15.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendStable,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 2, TotalServers: 2},
			expectedAction: models.ActionMaintain,
		},
		{
			name: "rising trend blocks scale-down",
			analyzed: &models.AnalyzedMetrics{
				ClusterID:      "cluster-1",
				AvgCPU:         25.0,
				CPUStatus:      models.ThresholdNormal,
				Trend:          models.TrendRising,
				SustainedLowAt: &sustainedPast,
			},
			state:          &models.ClusterState{ActiveServers: 6, TotalServers: 6},
			expectedAction: models.ActionMaintain,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newScenarioEngine()
			if tt.setupCooldown {
				engine.RecordScaleUp("cluster-1")
			}

			result := engine.Decide(tt.analyzed, nil, tt.state)

			assert.Equal(t, tt.expectedAction, result.Action)
			if tt.expectedEmerg {
				assert.True(t, result.IsEmergency, "expected emergency flag")
			}
		})
	}
}

func TestScenario_CooldownPreventsFlapping(t *testing.T) {
	engine := newScenarioEngine()
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}

	analyzed := &models.AnalyzedMetrics{
		ClusterID: "cluster-1",
		AvgCPU:    85.0,
		CPUStatus: models.ThresholdWarning,
		HasSpike:  true,
	}

	// First scale-up should succeed
	result := engine.Decide(analyzed, nil, state)
	assert.Equal(t, models.ActionScaleUp, result.Action, "first should scale-up")

	engine.RecordScaleUp("cluster-1")

	// Second attempt should be blocked by cooldown
	result = engine.Decide(analyzed, nil, state)
	assert.True(t, result.CooldownActive, "cooldown should block second scale-up")
}

func TestScenario_CollectorFailureAndRecovery(t *testing.T) {
	tests := []struct {
		name             string
		avgCPU           float64
		expectHighReset  bool
	}{
		{
			name:            "high CPU is tracked",
			avgCPU:          85.0,
			expectHighReset: false,
		},
		{
			name:            "CPU drop resets tracking",
			avgCPU:          50.0,
			expectHighReset: true,
		},
	}

	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzed := &models.AnalyzedMetrics{AvgCPU: tt.avgCPU}
			tracker.Update("cluster-1", analyzed, cfg)

			if tt.expectHighReset {
				assert.Zero(t, tracker.GetHighDuration("cluster-1"), "high duration should reset")
			} else {
				assert.NotZero(t, tracker.GetHighDuration("cluster-1"), "should track high duration")
			}
		})
	}
}
