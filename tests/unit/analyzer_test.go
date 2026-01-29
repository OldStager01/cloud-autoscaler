package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func newTestAnalyzer() *analyzer.Analyzer {
	return analyzer.New(analyzer.Config{
		CPUHighThreshold:    80.0,
		CPULowThreshold:     30.0,
		MemoryHighThreshold: 85.0,
		TrendWindow:         5 * time.Minute,
		SpikeThreshold:      50.0,
	})
}

func TestAnalyzer_Analyze_CPUStatus(t *testing.T) {
	tests := []struct {
		name           string
		servers        []models.ServerMetric
		expectedStatus models.ThresholdStatus
	}{
		{
			name: "normal CPU usage",
			servers: []models.ServerMetric{
				{ServerID: "s1", CPUUsage: 50.0, MemoryUsage: 60.0},
				{ServerID: "s2", CPUUsage: 55.0, MemoryUsage: 65.0},
			},
			expectedStatus: models.ThresholdNormal,
		},
		{
			name: "critical CPU usage",
			servers: []models.ServerMetric{
				{ServerID: "s1", CPUUsage: 96.0, MemoryUsage: 60.0},
				{ServerID: "s2", CPUUsage: 97.0, MemoryUsage: 65.0},
			},
			expectedStatus: models.ThresholdCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAnalyzer()
			metrics := &models.ClusterMetrics{
				ClusterID: "test-cluster",
				Timestamp: time.Now(),
				Servers:   tt.servers,
			}

			result := a.Analyze(metrics)

			assert.Equal(t, tt.expectedStatus, result.CPUStatus)
		})
	}
}

func TestAnalyzer_CalculateTrend(t *testing.T) {
	tests := []struct {
		name          string
		initialValues []float64
		finalValue    float64
		expectedTrend models.Trend
	}{
		{
			name:          "rising trend",
			initialValues: []float64{40.0, 50.0, 60.0, 70.0, 80.0},
			finalValue:    90.0,
			expectedTrend: models.TrendRising,
		},
		{
			name:          "falling trend",
			initialValues: []float64{90.0, 80.0, 70.0, 60.0, 50.0},
			finalValue:    30.0,
			expectedTrend: models.TrendFalling,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestAnalyzer()
			clusterID := "test-cluster"

			for _, cpuValue := range tt.initialValues {
				metrics := &models.ClusterMetrics{
					ClusterID: clusterID,
					Timestamp: time.Now(),
					Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: cpuValue}},
				}
				a.Analyze(metrics)
			}

			metrics := &models.ClusterMetrics{
				ClusterID: clusterID,
				Timestamp: time.Now(),
				Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: tt.finalValue}},
			}
			result := a.Analyze(metrics)

			assert.Equal(t, tt.expectedTrend, result.Trend)
		})
	}
}

func TestAnalyzer_DetectSpike(t *testing.T) {
	a := newTestAnalyzer()
	clusterID := "test-cluster"

	// Build baseline with stable low CPU
	for i := 0; i < 3; i++ {
		metrics := &models.ClusterMetrics{
			ClusterID: clusterID,
			Timestamp: time.Now(),
			Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 40.0}},
		}
		a.Analyze(metrics)
	}

	// Spike to high CPU
	metrics := &models.ClusterMetrics{
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 95.0}},
	}
	result := a.Analyze(metrics)

	assert.True(t, result.HasSpike, "expected spike to be detected")
}

func TestSustainedTracker(t *testing.T) {
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	tests := []struct {
		name              string
		avgCPU            float64
		expectHighAt      bool
		expectLowAt       bool
	}{
		{
			name:         "high CPU sets SustainedHighAt",
			avgCPU:       85.0,
			expectHighAt: true,
			expectLowAt:  false,
		},
		{
			name:         "low CPU sets SustainedLowAt",
			avgCPU:       20.0,
			expectHighAt: false,
			expectLowAt:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := analyzer.NewSustainedTracker()
			analyzed := &models.AnalyzedMetrics{AvgCPU: tt.avgCPU}

			tracker.Update("test-cluster", analyzed, cfg)

			if tt.expectHighAt {
				require.NotNil(t, analyzed.SustainedHighAt, "expected SustainedHighAt to be set")
			}
			if tt.expectLowAt {
				require.NotNil(t, analyzed.SustainedLowAt, "expected SustainedLowAt to be set")
			}
		})
	}
}

func TestSustainedTracker_Reset(t *testing.T) {
	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	analyzed := &models.AnalyzedMetrics{AvgCPU: 85.0}
	tracker.Update("test-cluster", analyzed, cfg)
	tracker.Reset("test-cluster")

	duration := tracker.GetHighDuration("test-cluster")
	assert.Zero(t, duration, "expected duration to be 0 after reset")
}
