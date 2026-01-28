package integration

import (
	"context"
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/events"
	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func TestPipeline_FullCycle(t *testing.T) {
	// Setup with high CPU to trigger critical status
	mock := collector.NewMockCollector(collector.MockCollectorConfig{BaseCPU: 96.0, Variance: 1.0})
	mock.SetClusterServers("cluster-1", 4)

	a := analyzer.New(analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0})
	engine := decision.NewEngine(decision.Config{
		MinServers: 2, MaxServers: 10, CPUHighThreshold: 80.0,
	})

	// Collect
	ctx := context.Background()
	metrics, err := mock.Collect(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("collect failed: %v", err)
	}

	// Analyze
	analyzed := a.Analyze(metrics)
	if analyzed.CPUStatus != models.ThresholdCritical {
		t.Errorf("expected Critical status, got %s", analyzed.CPUStatus)
	}

	// Decide
	state := &models.ClusterState{ActiveServers: 4, TotalServers: 4}
	result := engine.Decide(analyzed, nil, state)

	if result.Action != models.ActionScaleUp {
		t.Errorf("expected ScaleUp, got %s", result.Action)
	}
}

func TestResilientCollector_WithCircuitBreaker(t *testing.T) {
	mock := collector.NewMockCollector(collector.MockCollectorConfig{})
	mock.SetClusterServers("cluster-1", 4)
	mock.SetShouldFail(true, nil)

	resilient := collector.NewResilientCollector(collector.ResilientCollectorConfig{
		Collector:     mock,
		MaxFailures:   3,
		Timeout:       100 * time.Millisecond,
		RetryAttempts: 1,
	})

	ctx := context.Background()

	// Fail enough times to open circuit
	for i := 0; i < 3; i++ {
		resilient.Collect(ctx, "cluster-1")
	}

	if resilient.CircuitState() != resilience.StateOpen {
		t.Errorf("expected circuit to be open, got %v", resilient.CircuitState())
	}

	// Circuit should reject requests
	_, err := resilient.Collect(ctx, "cluster-1")
	if err != resilience.ErrCircuitOpen {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestEventBus_EventFlow(t *testing.T) {
	bus := events.NewEventBus(10)
	defer bus.Close()

	ch := bus.Subscribe(models.EventTypeMetricCollected)
	publisher := events.NewPublisher(bus)

	metrics := &models.ClusterMetrics{ClusterID: "cluster-1", Timestamp: time.Now()}
	publisher.MetricCollected("cluster-1", metrics)

	select {
	case event := <-ch:
		if event.Type != models.EventTypeMetricCollected {
			t.Errorf("expected MetricCollected, got %s", event.Type)
		}
		if event.ClusterID != "cluster-1" {
			t.Errorf("expected cluster-1, got %s", event.ClusterID)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}
