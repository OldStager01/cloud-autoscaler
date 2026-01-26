package analyzer

import (
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type SustainedTracker struct {
	highStartTimes map[string]time.Time
	lowStartTimes  map[string]time.Time
	mu             sync.RWMutex
}

func NewSustainedTracker() *SustainedTracker {
	return &SustainedTracker{
		highStartTimes: make(map[string]time.Time),
		lowStartTimes:  make(map[string]time.Time),
	}
}

func (t *SustainedTracker) Update(clusterID string, analyzed *models.AnalyzedMetrics, cfg Config) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	// Track high CPU: start when above threshold, clear when below
	if analyzed.AvgCPU >= cfg.CPUHighThreshold {
		if _, exists := t.highStartTimes[clusterID]; !exists {
			t.highStartTimes[clusterID] = now
		}
	} else {
		delete(t.highStartTimes, clusterID)
	}

	// Track low CPU: start when below threshold, clear when above
	if analyzed.AvgCPU <= cfg.CPULowThreshold {
		if _, exists := t.lowStartTimes[clusterID]; !exists {
			t.lowStartTimes[clusterID] = now
		}
	} else {
		delete(t.lowStartTimes, clusterID)
	}

	if startTime, exists := t.highStartTimes[clusterID]; exists {
		analyzed.SustainedHighAt = &startTime
	}
	if startTime, exists := t.lowStartTimes[clusterID]; exists {
		analyzed.SustainedLowAt = &startTime
	}
}

func (t *SustainedTracker) GetHighDuration(clusterID string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if startTime, exists := t.highStartTimes[clusterID]; exists {
		return time.Since(startTime)
	}
	return 0
}

func (t *SustainedTracker) GetLowDuration(clusterID string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if startTime, exists := t.lowStartTimes[clusterID]; exists {
		return time.Since(startTime)
	}
	return 0
}

func (t *SustainedTracker) Reset(clusterID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.highStartTimes, clusterID)
	delete(t.lowStartTimes, clusterID)
}