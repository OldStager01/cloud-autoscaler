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
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Setup logger
	logger.Setup(cfg.App.LogLevel, cfg.App.Mode)
	logger.Infof("Starting %s in %s mode", cfg.App.Name, cfg.App.Mode)

	// Connect to database
	logger.Infof("Connecting to database at %s:%d/%s",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := database.New(cfg.Database.ToDBConfig())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	logger.Info("Database connection established")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run migrations if flag is set
	if *migrate {
		logger.Info("Running database migrations")
		migrator := database.NewMigrator(db)
		if err := migrator.Run(ctx); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
		logger.Info("Migrations completed successfully")
		return nil
	}

	// Create and start API server
	server := api.NewServer(cfg.API, db)

	// Graceful shutdown setup
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

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
		logger.Infof("Received signal %v, shutting down", sig)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}

	logger.Info("Server stopped gracefully")
	return nil
}