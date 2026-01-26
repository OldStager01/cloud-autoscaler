package scaler

import (
	"context"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type StateTracker struct {
	servers   map[string]*models.Server
	clusters  map[string][]string // clusterID -> []serverID
	mu        sync.RWMutex
	callbacks StateCallbacks
}

type StateCallbacks struct {
	OnServerActivated  func(server *models.Server)
	OnServerTerminated func(server *models.Server)
	OnStateChanged     func(server *models.Server, oldState, newState models.ServerState)
}

func NewStateTracker(callbacks StateCallbacks) *StateTracker {
	return &StateTracker{
		servers:   make(map[string]*models.Server),
		clusters:  make(map[string][]string),
		callbacks: callbacks,
	}
}

func (t *StateTracker) AddServer(server *models.Server) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.servers[server.ID] = server
	t.clusters[server.ClusterID] = append(t.clusters[server.ClusterID], server.ID)

	logger.WithCluster(server.ClusterID).Infof(
		"Server %s added with state %s", server.ID[: 8], server.State,
	)
}

func (t *StateTracker) UpdateState(serverID string, newState models.ServerState) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	server, exists := t.servers[serverID]
	if !exists {
		return ErrClusterNotFound
	}

	oldState := server.State
	server.State = newState

	switch newState {
	case models.ServerStateActive:
		now := time.Now()
		server.ActivatedAt = &now
		if t.callbacks.OnServerActivated != nil {
			go t.callbacks.OnServerActivated(server)
		}
	case models.ServerStateTerminated:
		now := time.Now()
		server.TerminatedAt = &now
		if t.callbacks.OnServerTerminated != nil {
			go t.callbacks.OnServerTerminated(server)
		}
	}

	if t.callbacks.OnStateChanged != nil {
		go t.callbacks.OnStateChanged(server, oldState, newState)
	}

	logger.WithCluster(server.ClusterID).Infof(
		"Server %s state changed:  %s -> %s", serverID[:8], oldState, newState,
	)

	return nil
}

func (t *StateTracker) GetServer(serverID string) (*models.Server, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	server, exists := t.servers[serverID]
	if !exists {
		return nil, false
	}

	// Return a copy
	serverCopy := *server
	return &serverCopy, true
}

func (t *StateTracker) GetClusterServers(clusterID string) []*models.Server {
	t.mu.RLock()
	defer t.mu.RUnlock()

	serverIDs := t.clusters[clusterID]
	servers := make([]*models.Server, 0, len(serverIDs))

	for _, id := range serverIDs {
		if server, exists := t.servers[id]; exists {
			serverCopy := *server
			servers = append(servers, &serverCopy)
		}
	}

	return servers
}

func (t *StateTracker) GetClusterState(clusterID string) *models.ClusterState {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state := &models.ClusterState{
		ClusterID: clusterID,
	}

	serverIDs := t.clusters[clusterID]
	for _, id := range serverIDs {
		server, exists := t.servers[id]
		if !exists {
			continue
		}

		switch server.State {
		case models.ServerStateProvisioning:
			state.ProvisioningCnt++
			state.TotalServers++
		case models.ServerStateActive:
			state.ActiveServers++
			state.TotalServers++
		case models.ServerStateDraining:
			state.DrainingCount++
			state.TotalServers++
		case models.ServerStateTerminated:
			// Don't count terminated servers
		}
	}

	return state
}

func (t *StateTracker) GetActiveServers(clusterID string) []*models.Server {
	t.mu.RLock()
	defer t.mu.RUnlock()

	serverIDs := t.clusters[clusterID]
	servers := make([]*models.Server, 0)

	for _, id := range serverIDs {
		server, exists := t.servers[id]
		if exists && server.State == models.ServerStateActive {
			serverCopy := *server
			servers = append(servers, &serverCopy)
		}
	}

	return servers
}

func (t *StateTracker) RemoveServer(serverID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	server, exists := t.servers[serverID]
	if !exists {
		return
	}

	clusterID := server.ClusterID
	delete(t.servers, serverID)

	// Remove from cluster list
	serverIDs := t.clusters[clusterID]
	for i, id := range serverIDs {
		if id == serverID {
			t.clusters[clusterID] = append(serverIDs[:i], serverIDs[i+1:]...)
			break
		}
	}
}

func (t *StateTracker) CleanupTerminated(clusterID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	var removed int
	serverIDs := t.clusters[clusterID]
	activeIDs := make([]string, 0, len(serverIDs))

	for _, id := range serverIDs {
		server, exists := t.servers[id]
		if !exists {
			continue
		}

		if server.State == models.ServerStateTerminated {
			delete(t.servers, id)
			removed++
		} else {
			activeIDs = append(activeIDs, id)
		}
	}

	t.clusters[clusterID] = activeIDs
	return removed
}

// WaitForActivation waits for a server to become active
func (t *StateTracker) WaitForActivation(ctx context.Context, serverID string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			server, exists := t.GetServer(serverID)
			if !exists {
				return ErrClusterNotFound
			}
			if server.State == models.ServerStateActive {
				return nil
			}
			if server.State == models.ServerStateTerminated {
				return ErrProvisionFailed
			}
		}
	}
}