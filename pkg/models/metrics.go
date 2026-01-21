package models

import "time"

// ServerMetric represents metrics for a single server
type ServerMetric struct {
	ServerID    string  `json:"server_id"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	RequestLoad int     `json:"request_load"`
}

// ClusterMetrics represents collected metrics for a cluster
type ClusterMetrics struct {
	ClusterID string         `json:"cluster_id"`
	Timestamp time.Time      `json:"timestamp"`
	Servers   []ServerMetric `json:"servers"`
}

// AggregatedMetrics represents computed metrics for a cluster
type AggregatedMetrics struct {
	ClusterID     string    `json:"cluster_id"`
	Timestamp     time.Time `json:"timestamp"`
	AvgCPU        float64   `json:"avg_cpu"`
	AvgMemory     float64   `json:"avg_memory"`
	AvgLoad       float64   `json:"avg_load"`
	MaxCPU        float64   `json:"max_cpu"`
	MinCPU        float64   `json:"min_cpu"`
	ServerCount   int       `json:"server_count"`
	ActiveServers int       `json:"active_servers"`
}

// MetricRecord represents a single metric entry for database storage
type MetricRecord struct {
	Time        time.Time `json:"time"`
	ClusterID   string    `json:"cluster_id"`
	ServerID    string    `json:"server_id,omitempty"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	RequestLoad int       `json:"request_load"`
}

// CalculateAggregates computes aggregate metrics from server metrics
func (cm *ClusterMetrics) CalculateAggregates() AggregatedMetrics {
	if len(cm.Servers) == 0 {
		return AggregatedMetrics{
			ClusterID:  cm.ClusterID,
			Timestamp: cm.Timestamp,
		}
	}

	var totalCPU, totalMemory, totalLoad float64
	maxCPU := cm.Servers[0].CPUUsage
	minCPU := cm.Servers[0].CPUUsage

	for _, s := range cm.Servers {
		totalCPU += s.CPUUsage
		totalMemory += s.MemoryUsage
		totalLoad += float64(s.RequestLoad)

		if s.CPUUsage > maxCPU {
			maxCPU = s.CPUUsage
		}
		if s.CPUUsage < minCPU {
			minCPU = s.CPUUsage
		}
	}

	count := len(cm.Servers)
	return AggregatedMetrics{
		ClusterID:     cm.ClusterID,
		Timestamp:     cm.Timestamp,
		AvgCPU:         totalCPU / float64(count),
		AvgMemory:     totalMemory / float64(count),
		AvgLoad:       totalLoad / float64(count),
		MaxCPU:        maxCPU,
		MinCPU:        minCPU,
		ServerCount:   count,
		ActiveServers: count,
	}
}