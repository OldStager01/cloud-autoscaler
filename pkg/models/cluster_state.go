package models

import "time"

// ClusterState represents the runtime state of a cluster
type ClusterState struct {
	ClusterID       string     `json:"cluster_id"`
	TotalServers    int        `json:"total_servers"`
	ActiveServers   int        `json:"active_servers"`
	ProvisioningCnt int        `json:"provisioning_count"`
	DrainingCount   int        `json:"draining_count"`
	LastScaleTime   *time.Time `json:"last_scale_time,omitempty"`
	LastScaleAction string     `json:"last_scale_action,omitempty"`
}

func (cs *ClusterState) CanScaleUp(maxServers int) bool {
	return cs.TotalServers < maxServers
}

func (cs *ClusterState) CanScaleDown(minServers int) bool {
	return cs.ActiveServers > minServers
}

func (cs *ClusterState) AvailableCapacity(maxServers int) int {
	return maxServers - cs.TotalServers
}