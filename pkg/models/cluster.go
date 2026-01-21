package models

import (
	"encoding/json"
	"time"
)

type ClusterStatus string

const (
	ClusterStatusActive ClusterStatus = "active"
	ClusterStatusPaused ClusterStatus = "paused"
	ClusterStatusError  ClusterStatus = "error"
)

type ClusterConfig struct {
	CollectorEndpoint string  `json:"collector_endpoint,omitempty"`
	TargetCPU         float64 `json:"target_cpu,omitempty"`
}

type Cluster struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	MinServers    int            `json:"min_servers"`
	MaxServers    int            `json:"max_servers"`
	Status        ClusterStatus  `json:"status"`
	Config        *ClusterConfig `json:"config,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	LastScaleTime *time.Time     `json:"last_scale_time,omitempty"`
}

func NewCluster(name string, minServers, maxServers int) *Cluster {
	now := time.Now()
	return &Cluster{
		ID:          NewUUID(),
		Name:       name,
		MinServers: minServers,
		MaxServers: maxServers,
		Status:     ClusterStatusActive,
		CreatedAt:  now,
		UpdatedAt:   now,
	}
}

func (c *Cluster) IsActive() bool {
	return c.Status == ClusterStatusActive
}

func (c *Cluster) ConfigJSON() ([]byte, error) {
	if c.Config == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(c.Config)
}

func (c *Cluster) ParseConfig(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	c.Config = &ClusterConfig{}
	return json.Unmarshal(data, c.Config)
}