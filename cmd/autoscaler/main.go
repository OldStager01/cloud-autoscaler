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
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
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

	// Load clusters from database and start pipelines
	if err := startClusterPipelines(cfg, db, orch); err != nil {
		logger.Errorf("Failed to start cluster pipelines: %v", err)
	}

	// Create API server with orchestrator for dynamic cluster management
	server := api.NewServer(cfg.API, db, orch)

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
		logger.Errorf("API server shutdown error:  %v", err)
	}

	logger.Info("Shutdown complete")
	return nil
}

func startClusterPipelines(cfg *config.Config, db *database.DB, orch *orchestrator.Orchestrator) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get active clusters from database
	clusterRepo := queries.NewClusterRepository(db.DB)
	clusters, err := clusterRepo.GetAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch clusters: %w", err)
	}

	logger.Infof("Found %d clusters in database", len(clusters))

	for _, cluster := range clusters {
		if cluster.Status != models.ClusterStatusActive {
			logger.Infof("Skipping cluster %s (status: %s)", cluster.Name, cluster.Status)
			continue
		}

		// Create HTTP collector pointing to simulator
		simulatorURL := fmt.Sprintf("http://localhost:9000/metrics/%s", cluster.ID)
		coll := collector.NewHTTPCollector(collector.HTTPCollectorConfig{
			Endpoint:  simulatorURL,
			Timeout: 5 * time.Second,
		})

		// Create simulator scaler
		scal := scaler.NewSimulatorScaler(scaler.SimulatorConfig{
			ProvisionTime: 3 * time.Second,
			DrainTimeout:  2 * time.Second,
		})
		scal.InitializeCluster(cluster.ID, cluster.MinServers)

		// Start cluster pipeline
		if err := orch.StartCluster(cluster, coll, scal); err != nil {
			logger.Errorf("Failed to start cluster %s: %v", cluster.Name, err)
			continue
		}

		logger.Infof("Started pipeline for cluster:  %s", cluster.Name)
	}

	logger.Infof("Started %d cluster pipelines", orch.ClusterCount())
	return nil
}