package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OldStager01/cloud-autoscaler/pkg/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command line flags
	configPath := flag.String("config", "", "path to config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	fmt.Printf("Starting %s in %s mode\n", cfg.App.Name, cfg.App.Mode)
	fmt.Printf("API server will listen on port %d\n", cfg.API.Port)
	fmt.Printf("Database:  %s:%d/%s\n", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)

	return nil
}