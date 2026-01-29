package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/events"
	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func TestPipeline_FullCycle(t *testing.T) {
	tests := []struct {
		name           string
		baseCPU        float64
		servers        int
		expectedStatus models.ThresholdStatus
		expectedAction models.ScalingAction
	}{
		{
			name:           "high CPU triggers scale up",
			baseCPU:        96.0,
			servers:        4,
			expectedStatus: models.ThresholdCritical,
			expectedAction: models.ActionScaleUp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := collector.NewMockCollector(collector.MockCollectorConfig{BaseCPU: tt.baseCPU, Variance: 1.0})
			mock.SetClusterServers("cluster-1", tt.servers)

			a := analyzer.New(analyzer.Config{CPUHighThreshold: 80.0, CPULowThreshold: 30.0})
			engine := decision.NewEngine(decision.Config{
				MinServers: 2, MaxServers: 10, CPUHighThreshold: 80.0,
			})

			ctx := context.Background()
			metrics, err := mock.Collect(ctx, "cluster-1")
			require.NoError(t, err)

			analyzed := a.Analyze(metrics)
			assert.Equal(t, tt.expectedStatus, analyzed.CPUStatus)

			state := &models.ClusterState{ActiveServers: tt.servers, TotalServers: tt.servers}
			result := engine.Decide(analyzed, nil, state)
			assert.Equal(t, tt.expectedAction, result.Action)
		})
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

	assert.Equal(t, resilience.StateOpen, resilient.CircuitState())

	// Circuit should reject requests
	_, err := resilient.Collect(ctx, "cluster-1")
	assert.ErrorIs(t, err, resilience.ErrCircuitOpen)
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
		assert.Equal(t, models.EventTypeMetricCollected, event.Type)
		assert.Equal(t, "cluster-1", event.ClusterID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}
