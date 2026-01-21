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
	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
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
	testPipeline := flag.Bool("test-pipeline", false, "test full pipeline")
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

	if *testPipeline {
		return runFullPipelineTest(cfg)
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

func runFullPipelineTest(cfg *config.Config) error {
	logger.Info("Running full pipeline test")
	clusterID := "test-cluster-1"
	ctx := context.Background()

	// Initialize components
	mockCollector := collector.NewMockCollector(collector.MockCollectorConfig{
		BaseCPU:    50.0,
		BaseMemory:  60.0,
		Variance:   10.0,
	})
	mockCollector.SetClusterServers(clusterID, 3)

	analyzerInstance := analyzer.New(analyzer.Config{
		CPUHighThreshold:     cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:     cfg.Analyzer.Thresholds.CPULow,
		MemoryHighThreshold: cfg.Analyzer.Thresholds.MemoryHigh,
		TrendWindow:         cfg.Analyzer.TrendWindow,
		SpikeThreshold:      cfg.Analyzer.SpikeThreshold,
	})

	sustainedTracker := analyzer.NewSustainedTracker()

	decisionEngine := decision.NewEngine(decision.Config{
		CooldownPeriod:        5 * time.Second, // Short for testing
		EmergencyCPUThreshold: cfg.Decision.EmergencyCPUThreshold,
		MinServers:            cfg.Decision.MinServers,
		MaxServers:            cfg.Decision.MaxServers,
		MaxScaleStep:          cfg.Decision.MaxScaleStep,
		CPUHighThreshold:      cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:       cfg.Analyzer.Thresholds.CPULow,
		SustainedHighDuration: 2 * time.Second, // Short for testing
		SustainedLowDuration:  5 * time.Second, // Short for testing
	})

	simulatorScaler := scaler.NewSimulatorScaler(scaler.SimulatorConfig{
		ProvisionTime: 2 * time.Second, // Short for testing
		DrainTimeout:  3 * time.Second, // Short for testing
		Callbacks: scaler.StateCallbacks{
			OnServerActivated: func(server *models.Server) {
				logger.WithCluster(server.ClusterID).Infof("Callback: server %s activated", server.ID[: 8])
			},
			OnServerTerminated: func(server *models.Server) {
				logger.WithCluster(server.ClusterID).Infof("Callback: server %s terminated", server.ID[:8])
			},
		},
	})

	// Initialize cluster with 3 active servers
	simulatorScaler.InitializeCluster(clusterID, 3)

	logger.Info("=== Phase 1: Normal operation ===")
	for i := 0; i < 3; i++ {
		runPipelineCycle(ctx, clusterID, mockCollector, analyzerInstance, sustainedTracker, decisionEngine, simulatorScaler, cfg)
		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("=== Phase 2: High CPU - should trigger scale up ===")
	mockCollector.SetBaseCPU(85.0)
	for i := 0; i < 4; i++ {
		runPipelineCycle(ctx, clusterID, mockCollector, analyzerInstance, sustainedTracker, decisionEngine, simulatorScaler, cfg)
		time.Sleep(1 * time.Second)
	}

	// Wait for servers to provision
	logger.Info("Waiting for servers to activate...")
	time.Sleep(3 * time.Second)

	logger.Info("=== Phase 3: Emergency CPU ===")
	mockCollector.SetBaseCPU(96.0)
	runPipelineCycle(ctx, clusterID, mockCollector, analyzerInstance, sustainedTracker, decisionEngine, simulatorScaler, cfg)

	// Wait for emergency servers
	time.Sleep(3 * time.Second)

	logger.Info("=== Phase 4: Low CPU - should trigger scale down ===")
	mockCollector.SetBaseCPU(20.0)
	for i := 0; i < 6; i++ {
		runPipelineCycle(ctx, clusterID, mockCollector, analyzerInstance, sustainedTracker, decisionEngine, simulatorScaler, cfg)
		time.Sleep(1 * time.Second)
	}

	// Final state
	time.Sleep(2 * time.Second)
	finalState, _ := simulatorScaler.GetClusterState(ctx, clusterID)
	logger.Infof("=== Final cluster state: total=%d, active=%d, provisioning=%d, draining=%d ===",
		finalState.TotalServers, finalState.ActiveServers, finalState.ProvisioningCnt, finalState.DrainingCount)

	logger.Info("Full pipeline test completed")
	return nil
}

func runPipelineCycle(
	ctx context.Context,
	clusterID string,
	coll *collector.MockCollector,
	anal *analyzer.Analyzer,
	sustained *analyzer.SustainedTracker,
	engine *decision.Engine,
	scal *scaler.SimulatorScaler,
	cfg *config.Config,
) {
	// Collect
	metrics, err := coll.Collect(ctx, clusterID)
	if err != nil {
		logger.Errorf("Collection failed: %v", err)
		return
	}

	// Analyze
	analyzed := anal.Analyze(metrics)
	sustained.Update(clusterID, analyzed, analyzer.Config{
		CPUHighThreshold:  cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:   cfg.Analyzer.Thresholds.CPULow,
	})

	// Get current state
	state, _ := scal.GetClusterState(ctx, clusterID)

	logger.Infof("Metrics: cpu=%.1f%% (%s), servers=%d active, trend=%s",
		analyzed.AvgCPU, analyzed.CPUStatus, state.ActiveServers, analyzed.Trend)

	// Decide
	decisionResult := engine.Decide(analyzed, nil, state)

	// Execute
	if decisionResult.ShouldExecute() {
		var result *scaler.ScaleResult
		var err error

		switch decisionResult.Action {
		case models.ActionScaleUp:
			delta := decisionResult.TargetServers - decisionResult.CurrentServers
			result, err = scal.ScaleUp(ctx, clusterID, delta)
		case models.ActionScaleDown:
			delta := decisionResult.CurrentServers - decisionResult.TargetServers
			result, err = scal.ScaleDown(ctx, clusterID, delta)
		}

		if err != nil {
			logger.Errorf("Scaling failed: %v", err)
		} else if result.Success {
			engine.RecordScaling(clusterID)
			logger.Infof("Scaling executed:  added=%d, removed=%d",
				len(result.ServersAdded), len(result.ServersRemoved))
		}
	}
}