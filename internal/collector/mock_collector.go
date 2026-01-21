package collector

import (
	"context"
	"math/rand"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type MockCollector struct {
	clusters     map[string]int
	baseCPU      float64
	baseMemory   float64
	variance     float64
	shouldFail   bool
	failureError error
}

type MockCollectorConfig struct {
	BaseCPU    float64
	BaseMemory float64
	Variance   float64
}

func NewMockCollector(cfg MockCollectorConfig) *MockCollector {
	baseCPU := cfg.BaseCPU
	if baseCPU == 0 {
		baseCPU = 50.0
	}

	baseMemory := cfg.BaseMemory
	if baseMemory == 0 {
		baseMemory = 60.0
	}

	variance := cfg.Variance
	if variance == 0 {
		variance = 10.0
	}

	return &MockCollector{
		clusters:   make(map[string]int),
		baseCPU:    baseCPU,
		baseMemory: baseMemory,
		variance:   variance,
	}
}

func (c *MockCollector) SetClusterServers(clusterID string, serverCount int) {
	c.clusters[clusterID] = serverCount
}

func (c *MockCollector) SetBaseCPU(cpu float64) {
	c.baseCPU = cpu
}

func (c *MockCollector) SetShouldFail(shouldFail bool, err error) {
	c.shouldFail = shouldFail
	c.failureError = err
}

func (c *MockCollector) Collect(ctx context.Context, clusterID string) (*models.ClusterMetrics, error) {
	if c.shouldFail {
		if c.failureError != nil {
			return nil, c.failureError
		}
		return nil, ErrCollectionFailed
	}

	serverCount, exists := c.clusters[clusterID]
	if !exists {
		return nil, ErrClusterNotFound
	}

	servers := make([]models.ServerMetric, serverCount)
	for i := 0; i < serverCount; i++ {
		servers[i] = models.ServerMetric{
			ServerID:    models.NewUUID(),
			CPUUsage:    c.randomValue(c.baseCPU, c.variance),
			MemoryUsage: c.randomValue(c.baseMemory, c.variance),
			RequestLoad: int(c.randomValue(100, 50)),
		}
	}

	return &models.ClusterMetrics{
		ClusterID:  clusterID,
		Timestamp: time.Now(),
		Servers:   servers,
	}, nil
}

func (c *MockCollector) randomValue(base, variance float64) float64 {
	value := base + (rand.Float64()*2-1)*variance
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return value
}

func (c *MockCollector) HealthCheck(ctx context.Context) error {
	if c.shouldFail {
		return ErrCollectionFailed
	}
	return nil
}

func (c *MockCollector) Close() error {
	return nil
}