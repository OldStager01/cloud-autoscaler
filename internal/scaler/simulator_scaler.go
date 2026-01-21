package scaler

import (
	"context"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type SimulatorScaler struct {
	stateTracker  *StateTracker
	provisionTime time.Duration
	drainTimeout  time.Duration
	mu            sync.Mutex
}

type SimulatorConfig struct {
	ProvisionTime time.Duration
	DrainTimeout  time.Duration
	Callbacks     StateCallbacks
}

func NewSimulatorScaler(cfg SimulatorConfig) *SimulatorScaler {
	if cfg.ProvisionTime == 0 {
		cfg.ProvisionTime = 10 * time.Second
	}
	if cfg.DrainTimeout == 0 {
		cfg.DrainTimeout = 30 * time.Second
	}

	return &SimulatorScaler{
		stateTracker:   NewStateTracker(cfg.Callbacks),
		provisionTime: cfg.ProvisionTime,
		drainTimeout:  cfg.DrainTimeout,
	}
}

func (s *SimulatorScaler) ScaleUp(ctx context.Context, clusterID string, count int) (*ScaleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count <= 0 {
		return nil, ErrInvalidTarget
	}

	result := &ScaleResult{
		ClusterID:    clusterID,
		ServersAdded: make([]string, 0, count),
	}

	logger.WithCluster(clusterID).Infof("Scaling up:  adding %d servers", count)

	for i := 0; i < count; i++ {
		server := models.NewServer(clusterID)
		s.stateTracker.AddServer(server)
		result.ServersAdded = append(result.ServersAdded, server.ID)

		// Simulate async provisioning
		go s.simulateProvisioning(server.ID)
	}

	result.Success = true
	return result, nil
}

func (s *SimulatorScaler) simulateProvisioning(serverID string) {
	time.Sleep(s.provisionTime)

	if err := s.stateTracker.UpdateState(serverID, models.ServerStateActive); err != nil {
		logger.Errorf("Failed to activate server %s: %v", serverID[: 8], err)
	}
}

func (s *SimulatorScaler) ScaleDown(ctx context.Context, clusterID string, count int) (*ScaleResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if count <= 0 {
		return nil, ErrInvalidTarget
	}

	result := &ScaleResult{
		ClusterID:      clusterID,
		ServersRemoved: make([]string, 0, count),
	}

	activeServers := s.stateTracker.GetActiveServers(clusterID)
	if len(activeServers) == 0 {
		return nil, ErrClusterNotFound
	}

	toRemove := count
	if toRemove > len(activeServers) {
		toRemove = len(activeServers)
		result.PartialSuccess = true
	}

	logger.WithCluster(clusterID).Infof("Scaling down: removing %d servers", toRemove)

	for i := 0; i < toRemove; i++ {
		server := activeServers[i]
		result.ServersRemoved = append(result.ServersRemoved, server.ID)

		// Start draining
		s.stateTracker.UpdateState(server.ID, models.ServerStateDraining)

		// Simulate async termination
		go s.simulateTermination(server.ID)
	}

	result.Success = true
	return result, nil
}

func (s *SimulatorScaler) simulateTermination(serverID string) {
	// Simulate drain period
	time.Sleep(s.drainTimeout / 3)

	if err := s.stateTracker.UpdateState(serverID, models.ServerStateTerminated); err != nil {
		logger.Errorf("Failed to terminate server %s: %v", serverID[:8], err)
	}
}

func (s *SimulatorScaler) GetClusterState(ctx context.Context, clusterID string) (*models.ClusterState, error) {
	return s.stateTracker.GetClusterState(clusterID), nil
}

func (s *SimulatorScaler) GetServer(ctx context.Context, serverID string) (*models.Server, error) {
	server, exists := s.stateTracker.GetServer(serverID)
	if !exists {
		return nil, ErrClusterNotFound
	}
	return server, nil
}

func (s *SimulatorScaler) Close() error {
	return nil
}

// InitializeCluster sets up initial servers for a cluster
func (s *SimulatorScaler) InitializeCluster(clusterID string, serverCount int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := 0; i < serverCount; i++ {
		server := models.NewServer(clusterID)
		server.Activate()
		s.stateTracker.AddServer(server)
	}

	logger.WithCluster(clusterID).Infof("Initialized cluster with %d active servers", serverCount)
}

// GetStateTracker returns the internal state tracker for testing
func (s *SimulatorScaler) GetStateTracker() *StateTracker {
	return s.stateTracker
}