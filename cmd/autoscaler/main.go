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
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
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
	testPipeline := flag.Bool("test-pipeline", false, "test collector and analyzer")
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
		return runPipelineTest(cfg)
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

func runPipelineTest(cfg *config.Config) error {
	logger.Info("Running pipeline test with mock collector")

	mockCollector := collector.NewMockCollector(collector.MockCollectorConfig{
		BaseCPU:    70.0,
		BaseMemory: 60.0,
		Variance:   15.0,
	})

	mockCollector.SetClusterServers("test-cluster-1", 5)

	analyzerInstance := analyzer.New(analyzer.Config{
		CPUHighThreshold:    cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:     cfg.Analyzer.Thresholds.CPULow,
		MemoryHighThreshold: cfg.Analyzer.Thresholds.MemoryHigh,
		TrendWindow:         cfg.Analyzer.TrendWindow,
		SpikeThreshold:       cfg.Analyzer.SpikeThreshold,
	})

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		metrics, err := mockCollector.Collect(ctx, "test-cluster-1")
		if err != nil {
			return fmt.Errorf("collection failed: %w", err)
		}

		logger.Infof("Collected metrics: %d servers, timestamp=%s",
			len(metrics.Servers), metrics.Timestamp.Format(time.RFC3339))

		for _, s := range metrics.Servers {
			logger.Debugf("  Server %s: cpu=%.1f%%, memory=%.1f%%, load=%d",
				s.ServerID[: 8], s.CPUUsage, s.MemoryUsage, s.RequestLoad)
		}

		analyzed := analyzerInstance.Analyze(metrics)

		logger.Infof("Analysis result: cpu=%.1f%% (%s), memory=%.1f%% (%s), trend=%s, spike=%v, recommendation=%s",
			analyzed.AvgCPU, analyzed.CPUStatus,
			analyzed.AvgMemory, analyzed.MemoryStatus,
			analyzed.Trend, analyzed.HasSpike,
			analyzed.Recommendation)

		time.Sleep(500 * time.Millisecond)
	}

	logger.Info("Simulating CPU spike...")
	mockCollector.SetBaseCPU(92.0)

	metrics, _ := mockCollector.Collect(ctx, "test-cluster-1")
	analyzed := analyzerInstance.Analyze(metrics)

	logger.Infof("After spike: cpu=%.1f%% (%s), spike=%v (%.1f%%), recommendation=%s",
		analyzed.AvgCPU, analyzed.CPUStatus,
		analyzed.HasSpike, analyzed.SpikePercent,
		analyzed.Recommendation)

	logger.Info("Pipeline test completed successfully")
	return nil
}