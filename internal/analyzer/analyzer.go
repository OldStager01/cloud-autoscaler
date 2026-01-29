package analyzer

import (
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type Config struct {
	CPUHighThreshold    float64
	CPULowThreshold     float64
	MemoryHighThreshold float64
	TrendWindow         time.Duration
	SpikeThreshold      float64
	MaxHistoryLength    int
}

type Analyzer struct {
	config        Config
	history       map[string][]metricsSnapshot
	historyMu     sync.RWMutex
	maxHistoryLen int
}

type metricsSnapshot struct {
	timestamp time.Time
	avgCPU    float64
	avgMemory float64
}

func New(cfg Config) *Analyzer {
	if cfg.CPUHighThreshold == 0 {
		cfg.CPUHighThreshold = 80.0
	}
	if cfg.CPULowThreshold == 0 {
		cfg.CPULowThreshold = 30.0
	}
	if cfg.MemoryHighThreshold == 0 {
		cfg.MemoryHighThreshold = 85.0
	}
	if cfg.TrendWindow == 0 {
		cfg.TrendWindow = 5 * time.Minute
	}
	if cfg.SpikeThreshold == 0 {
		cfg.SpikeThreshold = 50.0
	}
	
	maxHistoryLen := cfg.MaxHistoryLength
	if maxHistoryLen == 0 {
		maxHistoryLen = 30
	}

	return &Analyzer{
		config:        cfg,
		history:       make(map[string][]metricsSnapshot),
		maxHistoryLen: maxHistoryLen,
	}
}

func (a *Analyzer) Analyze(metrics *models.ClusterMetrics) *models.AnalyzedMetrics {
	if len(metrics.Servers) == 0 {
		return &models.AnalyzedMetrics{
			ClusterID:    metrics.ClusterID,
			Timestamp:   metrics.Timestamp,
			ServerCount: 0,
			CPUStatus:   models.ThresholdNormal,
			Trend:       models.TrendStable,
		}
	}

	aggregated := metrics.CalculateAggregates()

	a.recordSnapshot(metrics.ClusterID, aggregated.AvgCPU, aggregated.AvgMemory, metrics.Timestamp)

	cpuStatus := a.evaluateCPUThreshold(aggregated.AvgCPU)
	memoryStatus := a.evaluateMemoryThreshold(aggregated.AvgMemory)
	trend := a.calculateTrend(metrics.ClusterID)
	hasSpike, spikePercent := a.detectSpike(metrics.ClusterID, aggregated.AvgCPU)

	analyzed := &models.AnalyzedMetrics{
		ClusterID:      metrics.ClusterID,
		Timestamp:      metrics.Timestamp,
		AvgCPU:         aggregated.AvgCPU,
		AvgMemory:      aggregated.AvgMemory,
		ServerCount:    aggregated.ServerCount,
		CPUStatus:      cpuStatus,
		MemoryStatus:   memoryStatus,
		Trend:          trend,
		HasSpike:       hasSpike,
		SpikePercent:   spikePercent,
		Recommendation: a.generateRecommendation(cpuStatus, trend, hasSpike),
	}

	logger.WithCluster(metrics.ClusterID).Debugf(
		"Analyzed:  cpu=%.1f%% (%s), trend=%s, spike=%v",
		aggregated.AvgCPU, cpuStatus, trend, hasSpike,
	)

	return analyzed
}

func (a *Analyzer) recordSnapshot(clusterID string, avgCPU, avgMemory float64, timestamp time.Time) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	snapshot := metricsSnapshot{
		timestamp: timestamp,
		avgCPU:    avgCPU,
		avgMemory: avgMemory,
	}

	history := a.history[clusterID]
	history = append(history, snapshot)

	if len(history) > a.maxHistoryLen {
		history = history[len(history)-a.maxHistoryLen:]
	}

	a.history[clusterID] = history
}

func (a *Analyzer) evaluateCPUThreshold(cpu float64) models.ThresholdStatus {
	switch {
	case cpu >= 95:
		return models.ThresholdCritical
	case cpu >= a.config.CPUHighThreshold:
		return models.ThresholdWarning
	default:
		return models.ThresholdNormal
	}
}

func (a *Analyzer) evaluateMemoryThreshold(memory float64) models.ThresholdStatus {
	switch {
	case memory >= 95:
		return models.ThresholdCritical
	case memory >= a.config.MemoryHighThreshold:
		return models.ThresholdWarning
	default:
		return models.ThresholdNormal
	}
}

func (a *Analyzer) calculateTrend(clusterID string) models.Trend {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	history := a.history[clusterID]
	if len(history) < 3 {
		return models.TrendStable
	}

	cutoff := time.Now().Add(-a.config.TrendWindow)
	var recentSnapshots []metricsSnapshot
	for _, s := range history {
		if s.timestamp.After(cutoff) {
			recentSnapshots = append(recentSnapshots, s)
		}
	}

	if len(recentSnapshots) < 3 {
		return models.TrendStable
	}

	firstHalf := recentSnapshots[:len(recentSnapshots)/2]
	secondHalf := recentSnapshots[len(recentSnapshots)/2:]

	firstAvg := a.averageCPU(firstHalf)
	secondAvg := a.averageCPU(secondHalf)

	diff := secondAvg - firstAvg
	threshold := 3.0 // TODO: Make configurable

	switch {
	case diff > threshold: 
		return models.TrendRising
	case diff < -threshold:
		return models.TrendFalling
	default:
		return models.TrendStable
	}
}

func (a *Analyzer) averageCPU(snapshots []metricsSnapshot) float64 {
	if len(snapshots) == 0 {
		return 0
	}
	var total float64
	for _, s := range snapshots {
		total += s.avgCPU
	}
	return total / float64(len(snapshots))
}

func (a *Analyzer) detectSpike(clusterID string, currentCPU float64) (bool, float64) {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	history := a.history[clusterID]
	if len(history) < 2 {
		return false, 0
	}

	oneMinuteAgo := time.Now().Add(-1 * time.Minute)
	var previousCPU float64
	found := false

	for i := len(history) - 2; i >= 0; i-- {
		if history[i].timestamp.Before(oneMinuteAgo) {
			previousCPU = history[i].avgCPU
			found = true
			break
		}
	}

	if !found {
		if len(history) >= 2 {
			previousCPU = history[len(history)-2].avgCPU
		} else {
			return false, 0
		}
	}

	if previousCPU == 0 {
		return false, 0
	}

	changePercent := ((currentCPU - previousCPU) / previousCPU) * 100

	if changePercent >= a.config.SpikeThreshold {
		return true, changePercent
	}

	return false, changePercent
}

func (a *Analyzer) generateRecommendation(cpuStatus models.ThresholdStatus, trend models.Trend, hasSpike bool) string {
	switch {
	case cpuStatus == models.ThresholdCritical: 
		return "immediate_scale_up"
	case hasSpike:
		return "scale_up_spike_detected"
	case cpuStatus == models.ThresholdWarning && trend == models.TrendRising:
		return "scale_up_rising_trend"
	case cpuStatus == models.ThresholdWarning: 
		return "monitor_closely"
	case cpuStatus == models.ThresholdNormal && trend == models.TrendFalling:
		return "consider_scale_down"
	default: 
		return "maintain"
	}
}

func (a *Analyzer) GetHistory(clusterID string) []metricsSnapshot {
	a.historyMu.RLock()
	defer a.historyMu.RUnlock()

	history := a.history[clusterID]
	result := make([]metricsSnapshot, len(history))
	copy(result, history)
	return result
}

func (a *Analyzer) ClearHistory(clusterID string) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	delete(a.history, clusterID)
}