package unit

import (
	"testing"
	"time"

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

func TestAnalyzer_Analyze_NormalCPU(t *testing.T) {
	a := newTestAnalyzer()
	metrics := &models.ClusterMetrics{
		ClusterID: "test-cluster",
		Timestamp: time.Now(),
		Servers: []models.ServerMetric{
			{ServerID: "s1", CPUUsage: 50.0, MemoryUsage: 60.0},
			{ServerID: "s2", CPUUsage: 55.0, MemoryUsage: 65.0},
		},
	}

	result := a.Analyze(metrics)

	if result.CPUStatus != models.ThresholdNormal {
		t.Errorf("expected Normal, got %s", result.CPUStatus)
	}
}

func TestAnalyzer_Analyze_CriticalCPU(t *testing.T) {
	a := newTestAnalyzer()
	metrics := &models.ClusterMetrics{
		ClusterID: "test-cluster",
		Timestamp: time.Now(),
		Servers: []models.ServerMetric{
			{ServerID: "s1", CPUUsage: 96.0, MemoryUsage: 60.0},
			{ServerID: "s2", CPUUsage: 97.0, MemoryUsage: 65.0},
		},
	}

	result := a.Analyze(metrics)

	if result.CPUStatus != models.ThresholdCritical {
		t.Errorf("expected Critical, got %s", result.CPUStatus)
	}
}

func TestAnalyzer_CalculateTrend_Rising(t *testing.T) {
	a := newTestAnalyzer()
	clusterID := "test-cluster"

	for i := 0; i < 5; i++ {
		metrics := &models.ClusterMetrics{
			ClusterID: clusterID,
			Timestamp: time.Now(),
			Servers: []models.ServerMetric{
				{ServerID: "s1", CPUUsage: 40.0 + float64(i*10)},
			},
		}
		a.Analyze(metrics)
	}

	metrics := &models.ClusterMetrics{
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 90.0}},
	}
	result := a.Analyze(metrics)

	if result.Trend != models.TrendRising {
		t.Errorf("expected Rising, got %s", result.Trend)
	}
}

func TestAnalyzer_CalculateTrend_Falling(t *testing.T) {
	a := newTestAnalyzer()
	clusterID := "test-cluster"

	for i := 0; i < 5; i++ {
		metrics := &models.ClusterMetrics{
			ClusterID: clusterID,
			Timestamp: time.Now(),
			Servers: []models.ServerMetric{
				{ServerID: "s1", CPUUsage: 90.0 - float64(i*10)},
			},
		}
		a.Analyze(metrics)
	}

	metrics := &models.ClusterMetrics{
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 30.0}},
	}
	result := a.Analyze(metrics)

	if result.Trend != models.TrendFalling {
		t.Errorf("expected Falling, got %s", result.Trend)
	}
}

func TestAnalyzer_DetectSpike(t *testing.T) {
	a := newTestAnalyzer()
	clusterID := "test-cluster"

	for i := 0; i < 3; i++ {
		metrics := &models.ClusterMetrics{
			ClusterID: clusterID,
			Timestamp: time.Now(),
			Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 40.0}},
		}
		a.Analyze(metrics)
	}

	metrics := &models.ClusterMetrics{
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Servers:   []models.ServerMetric{{ServerID: "s1", CPUUsage: 95.0}},
	}
	result := a.Analyze(metrics)

	if !result.HasSpike {
		t.Error("expected spike to be detected")
	}
}

func TestSustainedTracker_Update_HighCPU(t *testing.T) {
	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	analyzed := &models.AnalyzedMetrics{AvgCPU: 85.0}
	tracker.Update("test-cluster", analyzed, cfg)

	if analyzed.SustainedHighAt == nil {
		t.Error("expected SustainedHighAt to be set")
	}
}

func TestSustainedTracker_Update_LowCPU(t *testing.T) {
	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	analyzed := &models.AnalyzedMetrics{AvgCPU: 20.0}
	tracker.Update("test-cluster", analyzed, cfg)

	if analyzed.SustainedLowAt == nil {
		t.Error("expected SustainedLowAt to be set")
	}
}

func TestSustainedTracker_Reset(t *testing.T) {
	tracker := analyzer.NewSustainedTracker()
	cfg := analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0}

	analyzed := &models.AnalyzedMetrics{AvgCPU: 85.0}
	tracker.Update("test-cluster", analyzed, cfg)
	tracker.Reset("test-cluster")

	duration := tracker.GetHighDuration("test-cluster")
	if duration != 0 {
		t.Error("expected duration to be 0 after reset")
	}
}
