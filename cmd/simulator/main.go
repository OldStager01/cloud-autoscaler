package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/simulator"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	port := flag.Int("port", 9000, "simulator server port")
	logLevel := flag.String("log-level", "info", "log level")
	flag.Parse()

	logger.Setup(*logLevel, "development")
	logger.Info("Starting metrics simulator")

	sim := simulator.New(simulator.Config{
		Port: *port,
	})

	if err := sim.Start(); err != nil {
		return fmt.Errorf("failed to start simulator:  %w", err)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down simulator")
	return sim.Stop()
}