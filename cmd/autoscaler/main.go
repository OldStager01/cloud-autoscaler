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
		fmt. Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "", "path to config file")
	migrate := flag.Bool("migrate", false, "run database migrations")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt. Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	fmt.Printf("Starting %s in %s mode\n", cfg.App.Name, cfg.App.Mode)

	// Connect to database
	fmt.Printf("Connecting to database at %s:%d/%s...\n",
		cfg.Database.Host, cfg.Database.Port, cfg. Database.Name)

	db, err := database.New(cfg. Database. ToDBConfig())
	if err != nil {
		return fmt.Errorf("failed to connect to database:  %w", err)
	}
	defer db.Close()

	fmt. Println("Database connection established")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Run migrations if flag is set
	if *migrate {
		fmt.Println("Running database migrations...")
		migrator := database.NewMigrator(db)
		if err := migrator.Run(ctx); err != nil {
			return fmt. Errorf("migration failed: %w", err)
		}
		fmt.Println("Migrations completed successfully")
	}

	// Verify tables exist
	tables := []string{"users", "clusters", "servers", "scaling_events", "metrics_history", "predictions"}
	for _, table := range tables {
		exists, err := db.TableExists(ctx, table)
		if err != nil {
			fmt.Printf("Warning: could not check table %s: %v\n", table, err)
			continue
		}
		if exists {
			fmt.Printf("Table %s:  OK\n", table)
		} else {
			fmt.Printf("Table %s: MISSING (run with --migrate)\n", table)
		}
	}

	return nil
}