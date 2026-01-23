package scaler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type SimulatorScaler struct {
	stateTracker   *StateTracker
	provisionTime  time.Duration
	drainTimeout   time.Duration
	simulatorURL   string
	httpClient     *http.Client
	mu             sync.Mutex
}

type SimulatorConfig struct {
	ProvisionTime time.Duration
	DrainTimeout  time.Duration
	SimulatorURL  string
	Callbacks     StateCallbacks
}

func NewSimulatorScaler(cfg SimulatorConfig) *SimulatorScaler {
	if cfg.ProvisionTime == 0 {
		cfg.ProvisionTime = 10 * time.Second
	}
	if cfg.DrainTimeout == 0 {
		cfg.DrainTimeout = 30 * time.Second
	}
	if cfg.SimulatorURL == "" {
		cfg.SimulatorURL = "http://localhost:9000"
	}

	return &SimulatorScaler{
		stateTracker:   NewStateTracker(cfg.Callbacks),
		provisionTime:  cfg.ProvisionTime,
		drainTimeout:   cfg.DrainTimeout,
		simulatorURL:   cfg.SimulatorURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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

	logger.WithCluster(clusterID).Infof("Scaling up: adding %d servers", count)

	// Notify external simulator to add servers
	if err := s.notifySimulator(clusterID, count, 0); err != nil {
		logger.WithCluster(clusterID).Warnf("Failed to notify simulator: %v", err)
	}

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

	// Notify external simulator to remove servers
	if err := s.notifySimulator(clusterID, 0, toRemove); err != nil {
		logger.WithCluster(clusterID).Warnf("Failed to notify simulator: %v", err)
	}

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

// notifySimulator calls the external simulator API to add/remove servers
func (s *SimulatorScaler) notifySimulator(clusterID string, addServers, removeServers int) error {
	payload := map[string]interface{}{}
	if addServers > 0 {
		payload["add_servers"] = addServers
	}
	if removeServers > 0 {
		payload["remove_servers"] = removeServers
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/clusters/%s", s.simulatorURL, clusterID)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call simulator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("simulator returned status %d", resp.StatusCode)
	}

	logger.Infof("Notified simulator: cluster=%s add=%d remove=%d", clusterID, addServers, removeServers)
	return nil
}