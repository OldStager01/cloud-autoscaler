package models

import "time"

type Trend string

const (
	TrendRising  Trend = "rising"
	TrendFalling Trend = "falling"
	TrendStable  Trend = "stable"
)

type ThresholdStatus string

const (
	ThresholdNormal   ThresholdStatus = "normal"
	ThresholdWarning  ThresholdStatus = "warning"
	ThresholdCritical ThresholdStatus = "critical"
)

// AnalyzedMetrics contains analysis results for collected metrics
type AnalyzedMetrics struct {
	ClusterID       string          `json:"cluster_id"`
	Timestamp       time.Time       `json:"timestamp"`
	AvgCPU          float64         `json:"avg_cpu"`
	AvgMemory       float64         `json:"avg_memory"`
	ServerCount     int             `json:"server_count"`
	CPUStatus       ThresholdStatus `json:"cpu_status"`
	MemoryStatus    ThresholdStatus `json:"memory_status"`
	Trend           Trend           `json:"trend"`
	HasSpike        bool            `json:"has_spike"`
	SpikePercent    float64         `json:"spike_percent,omitempty"`
	Recommendation  string          `json:"recommendation,omitempty"`
	SustainedHighAt *time.Time      `json:"sustained_high_at,omitempty"`
	SustainedLowAt  *time.Time      `json:"sustained_low_at,omitempty"`
}

func (a *AnalyzedMetrics) IsCritical() bool {
	return a.CPUStatus == ThresholdCritical || a.MemoryStatus == ThresholdCritical
}

func (a *AnalyzedMetrics) IsWarning() bool {
	return a.CPUStatus == ThresholdWarning || a.MemoryStatus == ThresholdWarning
}