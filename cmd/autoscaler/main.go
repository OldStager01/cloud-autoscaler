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
	testOrchestrator := flag.Bool("test-orchestrator", false, "test orchestrator")
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
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	logger.Info("Database connection established")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if *migrate {
		logger.Info("Running database migrations")
		migrator := database.NewMigrator(db)
		if err := migrator.Run(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("Migrations completed successfully")
		return nil
	}

	if *testOrchestrator {
		return runOrchestratorTest(cfg, db)
	}

	server := api.NewServer(cfg.API, db)

	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		logger.Infof("API server listening on port %d", cfg.API.Port)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return fmt.Errorf("server error: %w", err)
	case sig := <-shutdownChan: 
		logger.Infof("Received signal %v, shutting down", sig)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	logger.Info("Server stopped gracefully")
	return nil
}

func runOrchestratorTest(cfg *config.Config, db *database.DB) error {
	logger.Info("Running orchestrator test")

	// Create orchestrator
	orch := orchestrator.New(cfg, db)
	if err := orch.Start(); err != nil {
		return fmt.Errorf("failed to start orchestrator: %w", err)
	}

	// Subscribe to events for logging
	eventChan := orch.SubscribeAllEvents()
	go func() {
		for event := range eventChan {
			logger.Infof("[EVENT] %s:  %s (cluster: %s, severity: %s)",
				event.Type, event.Message, event.ClusterID, event.Severity)
		}
	}()

	// Create test cluster
	cluster := &models.Cluster{
		ID:         "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
		Name:       "Test Cluster",
		MinServers: 2,
		MaxServers: 10,
		Status:     models.ClusterStatusActive,
	}

	// Create mock collector
	mockColl := collector.NewMockCollector(collector.MockCollectorConfig{
		BaseCPU:     55.0,
		BaseMemory: 60.0,
		Variance:   10.0,
	})
	mockColl.SetClusterServers(cluster.ID, 3)

	// Create simulator scaler
	simScaler := scaler.NewSimulatorScaler(scaler.SimulatorConfig{
		ProvisionTime: 3 * time.Second,
		DrainTimeout:  2 * time.Second,
	})
	simScaler.InitializeCluster(cluster.ID, 3)

	// Start cluster pipeline
	if err := orch.StartCluster(cluster, mockColl, simScaler); err != nil {
		return fmt.Errorf("failed to start cluster:  %w", err)
	}

	// Let it run for a few cycles at normal CPU
	logger.Info("=== Phase 1: Normal operation (15 seconds) ===")
	time.Sleep(15 * time.Second)

	// Simulate high CPU
	logger.Info("=== Phase 2: High CPU (20 seconds) ===")
	mockColl.SetBaseCPU(85.0)
	time.Sleep(20 * time.Second)

	// Simulate emergency
	logger.Info("=== Phase 3: Emergency CPU (10 seconds) ===")
	mockColl.SetBaseCPU(96.0)
	time.Sleep(10 * time.Second)

	// Back to normal
	logger.Info("=== Phase 4: Back to normal (10 seconds) ===")
	mockColl.SetBaseCPU(50.0)
	time.Sleep(10 * time.Second)

	// Check running clusters
	running := orch.ListRunningClusters()
	logger.Infof("Running clusters: %v", running)

	// Get final state
	state, _ := simScaler.GetClusterState(context.Background(), cluster.ID)
	logger.Infof("Final state: active=%d, provisioning=%d, draining=%d",
		state.ActiveServers, state.ProvisioningCnt, state.DrainingCount)

	// Stop cluster
	if err := orch.StopCluster(cluster.ID); err != nil {
		logger.Errorf("Failed to stop cluster: %v", err)
	}

	// Stop orchestrator
	orch.Stop()

	logger.Info("Orchestrator test completed")
	return nil
}