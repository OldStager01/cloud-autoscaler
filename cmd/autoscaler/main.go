package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

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
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	fmt.Printf("Starting %s in %s mode\n", cfg.App.Name, cfg.App.Mode)

	// Connect to database
	fmt.Printf("Connecting to database at %s:%d/%s...\n",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	db, err := database.New(cfg.Database.ToDBConfig())
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	fmt.Println("Database connection established")
	// Get database info
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	version, err := db.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get database version: %w", err)
	}
	fmt.Printf("Database version: %s\n", version)

	// Check TimescaleDB extension
	tsEnabled, err := db.IsTimescaleDBEnabled(ctx)
	if err != nil {
		fmt.Printf("Warning: could not check TimescaleDB status: %v\n", err)
	} else if tsEnabled {
		fmt.Println("TimescaleDB extension is enabled")
	} else {
		fmt.Println("TimescaleDB extension is not enabled")
	}

	// Print connection pool stats
	stats := db.GetConnectionStats()
	fmt.Printf("Connection pool - Open: %d, Idle: %d, InUse: %d\n",
		stats.OpenConnections, stats.Idle, stats.InUse)

	return nil
}