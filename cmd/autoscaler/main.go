package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/OldStager01/cloud-autoscaler/api"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/metrics"
	"github.com/OldStager01/cloud-autoscaler/internal/orchestrator"
	"github.com/OldStager01/cloud-autoscaler/internal/scaler"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config file")
	migrate := flag.Bool("migrate", false, "run database migrations")
	testMultiCluster := flag.Bool("test-multi-cluster", false, "test multi-cluster orchestrator")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger.Setup(cfg.App.LogLevel, cfg.App.Mode)
	logger.Infof("Starting %s in %s mode", cfg.App.Name, cfg.App.Mode)

	db, err := database.New(cfg.Database.ToDBConfig())
	if err != nil {
		return fmt.Errorf("failed to connect to database:  %w", err)
	}
	defer db.Close()

	logger.Info("Database connection established")

	if *migrate {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		logger.Info("Running database migrations")
		migrator := database.NewMigrator(db)
		if err := migrator.Run(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("Migrations completed successfully")
		return nil
	}

	if *testMultiCluster {
		return runMultiClusterTest(cfg, db)
	}

	return runServer(cfg, db)
}

func runServer(cfg *config.Config, db *database.DB) error {
	// Start Prometheus metrics server
	if cfg.Prometheus.Enabled {
		metrics.StartServer(cfg.Prometheus.Port)
	}

	// Create orchestrator
	orch := orchestrator.New(cfg, db)
	if err := orch.Start(); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	// Create API server
	server := api.NewServer(cfg.API, db)

	// Setup graceful shutdown
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	errChan := make(chan error, 1)
	go func() {
		logger.Infof("API server listening on port %d", cfg.API.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for shutdown signal or error
	select {
	case err := <-errChan: 
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdownChan: 
		logger.Infof("Received signal %v, initiating graceful shutdown", sig)
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown orchestrator first (stops all pipelines)
	logger.Info("Stopping orchestrator...")
	if err := orch.Stop(shutdownCtx); err != nil {
		logger.Errorf("Orchestrator shutdown error: %v", err)
	}

	// Then shutdown API server
	logger.Info("Stopping API server...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("API server shutdown error: %v", err)
	}

	logger.Info("Shutdown complete")
	return nil
}

func runMultiClusterTest(cfg *config.Config, db *database.DB) error {
	logger.Info("Running multi-cluster test")

	// Start Prometheus metrics server
	if cfg.Prometheus.Enabled {
		metrics.StartServer(cfg.Prometheus.Port)
	}

	// Create orchestrator
	orch := orchestrator.New(cfg, db)
	if err := orch.Start(); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	// Subscribe to events
	eventChan := orch.SubscribeAllEvents()
	go func() {
		for event := range eventChan {
			logger.Infof("[EVENT] %s:  %s (cluster: %s)",
				event.Type, event.Message, event.ClusterID[: 8])
		}
	}()

	// Define test clusters (using seeded IDs)
	clusters := []*models.Cluster{
		{
			ID:         "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			Name:       "web-production",
			MinServers: 2,
			MaxServers: 10,
			Status:     models.ClusterStatusActive,
		},
		{
			ID:          "b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22",
			Name:       "api-production",
			MinServers: 2,
			MaxServers: 10,
			Status:     models.ClusterStatusActive,
		},
	}

	// Create mock collectors with different behaviors
	mockCollectors := map[string]*collector.MockCollector{
		clusters[0].ID: collector.NewMockCollector(collector.MockCollectorConfig{
			BaseCPU:    55.0,
			BaseMemory: 60.0,
			Variance:   10.0,
		}),
		clusters[1].ID: collector.NewMockCollector(collector.MockCollectorConfig{
			BaseCPU:     40.0,
			BaseMemory: 50.0,
			Variance:   5.0,
		}),
	}

	// Initialize clusters
	for _, cluster := range clusters {
		mockCollectors[cluster.ID].SetClusterServers(cluster.ID, 3)
	}

	// Create scalers
	scalers := make(map[string]*scaler.SimulatorScaler)
	for _, cluster := range clusters {
		scalers[cluster.ID] = scaler.NewSimulatorScaler(scaler.SimulatorConfig{
			ProvisionTime: 3 * time.Second,
			DrainTimeout:  2 * time.Second,
		})
		scalers[cluster.ID].InitializeCluster(cluster.ID, 3)
	}

	// Start all clusters
	for _, cluster := range clusters {
		if err := orch.StartCluster(cluster, mockCollectors[cluster.ID], scalers[cluster.ID]); err != nil {
			logger.Errorf("Failed to start cluster %s: %v", cluster.Name, err)
		}
	}

	logger.Infof("Started %d clusters", orch.ClusterCount())

	// Phase 1: Normal operation
	logger.Info("=== Phase 1: Normal operation (15 seconds) ===")
	time.Sleep(15 * time.Second)

	// Phase 2: Simulate high load on web cluster only
	logger.Info("=== Phase 2: High load on web-production (15 seconds) ===")
	mockCollectors[clusters[0].ID].SetBaseCPU(88.0)
	time.Sleep(15 * time.Second)

	// Phase 3: Both clusters high load
	logger.Info("=== Phase 3: High load on both clusters (15 seconds) ===")
	mockCollectors[clusters[1].ID].SetBaseCPU(85.0)
	time.Sleep(15 * time.Second)

	// Phase 4: Back to normal
	logger.Info("=== Phase 4: Back to normal (10 seconds) ===")
	mockCollectors[clusters[0].ID].SetBaseCPU(50.0)
	mockCollectors[clusters[1].ID].SetBaseCPU(45.0)
	time.Sleep(10 * time.Second)

	// Print final states
	logger.Info("=== Final Cluster States ===")
	for _, cluster := range clusters {
		state, _ := scalers[cluster.ID].GetClusterState(context.Background(), cluster.ID)
		logger.Infof("Cluster %s: active=%d, provisioning=%d, draining=%d",
			cluster.Name, state.ActiveServers, state.ProvisioningCnt, state.DrainingCount)
	}

	// Check Prometheus metrics
	logger.Infof("Prometheus metrics available at http://localhost:%d/metrics", cfg.Prometheus.Port)

	// Graceful shutdown
	logger.Info("Initiating graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := orch.Stop(ctx); err != nil {
		logger.Errorf("Shutdown error: %v", err)
	}

	logger.Info("Multi-cluster test completed")
	return nil
}