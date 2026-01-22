package simulator

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type ClusterSimConfig struct {
	InitialServers int
	BaseCPU        float64
	BaseMemory     float64
	Variance       float64
}

type ClusterSim struct {
	id         string
	servers    []*ServerSim
	baseCPU    float64
	baseMemory float64
	variance   float64
	pattern    Pattern
	spike      *Spike
	mu         sync.RWMutex
}

type ServerSim struct {
	ID        string
	State     models.ServerState
	CreatedAt time.Time
}

type Spike struct {
	TargetCPU  float64
	StartTime  time.Time
	Duration   time.Duration
	RampUp     time.Duration
	OriginalCPU float64
}

func NewClusterSim(id string, cfg ClusterSimConfig) *ClusterSim {
	cluster := &ClusterSim{
		id:         id,
		baseCPU:    cfg.BaseCPU,
		baseMemory: cfg.BaseMemory,
		variance:   cfg.Variance,
		pattern:    PatternSteady,
		servers:    make([]*ServerSim, 0, cfg.InitialServers),
	}

	for i := 0; i < cfg.InitialServers; i++ {
		cluster.servers = append(cluster.servers, &ServerSim{
			ID:         models.NewUUID(),
			State:     models.ServerStateActive,
			CreatedAt: time.Now(),
		})
	}

	return cluster
}

func (c *ClusterSim) CollectMetrics() *MetricsResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	currentCPU := c.calculateCurrentCPU()
	servers := make([]ServerMetrics, 0, len(c.servers))

	for _, srv := range c.servers {
		if srv.State != models.ServerStateActive {
			continue
		}

		servers = append(servers, ServerMetrics{
			ServerID:    srv.ID,
			CPUUsage:    c.randomValue(currentCPU, c.variance),
			MemoryUsage: c.randomValue(c.baseMemory, c.variance/2),
			RequestLoad: int(c.randomValue(100, 30)),
		})
	}

	return &MetricsResponse{
		ClusterID: c.id,
		Timestamp: time.Now().Format(time.RFC3339),
		Servers:   servers,
	}
}

func (c *ClusterSim) calculateCurrentCPU() float64 {
	baseCPU := c.baseCPU

	// Apply pattern modification
	baseCPU = c.pattern.Apply(baseCPU)

	// Apply spike if active
	if c.spike != nil {
		elapsed := time.Since(c.spike.StartTime)
		
		if elapsed > c.spike.Duration {
			// Spike ended
			c.spike = nil
		} else if elapsed < c.spike.RampUp {
			// Ramping up
			progress := float64(elapsed) / float64(c.spike.RampUp)
			baseCPU = c.spike.OriginalCPU + (c.spike.TargetCPU-c.spike.OriginalCPU)*progress
		} else {
			// At peak
			baseCPU = c.spike.TargetCPU
		}
	}

	return baseCPU
}

func (c *ClusterSim) randomValue(base, variance float64) float64 {
	value := base + (rand.Float64()*2-1)*variance
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}
	return math.Round(value*100) / 100
}

func (c *ClusterSim) SetBaseCPU(cpu float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseCPU = cpu
}

func (c *ClusterSim) SetBaseMemory(memory float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseMemory = memory
}

func (c *ClusterSim) SetVariance(variance float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.variance = variance
}

func (c *ClusterSim) SetPattern(pattern Pattern) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pattern = pattern
}

func (c *ClusterSim) GetPattern() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pattern.Name()
}

func (c *ClusterSim) InjectSpike(targetCPU float64, duration, rampUp time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.spike = &Spike{
		TargetCPU:    targetCPU,
		StartTime:   time.Now(),
		Duration:    duration,
		RampUp:       rampUp,
		OriginalCPU: c.baseCPU,
	}
}

func (c *ClusterSim) AddServers(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < count; i++ {
		c.servers = append(c.servers, &ServerSim{
			ID:        models.NewUUID(),
			State:     models.ServerStateActive,
			CreatedAt: time.Now(),
		})
	}
}

func (c *ClusterSim) RemoveServers(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	removed := 0
	for i := len(c.servers) - 1; i >= 0 && removed < count; i-- {
		if c.servers[i].State == models.ServerStateActive {
			c.servers = append(c.servers[:i], c.servers[i+1:]...)
			removed++
		}
	}
}

func (c *ClusterSim) ServerCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, srv := range c.servers {
		if srv.State == models.ServerStateActive {
			count++
		}
	}
	return count
}

func (c *ClusterSim) Status() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	spikeInfo := map[string]interface{}{"active": false}
	if c.spike != nil {
		elapsed := time.Since(c.spike.StartTime)
		remaining := c.spike.Duration - elapsed
		if remaining < 0 {
			remaining = 0
		}
		spikeInfo = map[string]interface{}{
			"active":     true,
			"target_cpu": c.spike.TargetCPU,
			"remaining":  remaining.String(),
		}
	}

	return map[string]interface{}{
		"id":           c.id,
		"server_count": c.ServerCount(),
		"base_cpu":     c.baseCPU,
		"base_memory":  c.baseMemory,
		"variance":     c.variance,
		"pattern":       c.pattern.Name(),
		"spike":        spikeInfo,
	}
}

type MetricsResponse struct {
	ClusterID string          `json:"cluster_id"`
	Timestamp string          `json:"timestamp"`
	Servers   []ServerMetrics `json:"servers"`
}

type ServerMetrics struct {
	ServerID    string  `json:"server_id"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	RequestLoad int     `json:"request_load"`
}