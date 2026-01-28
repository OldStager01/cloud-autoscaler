package unit

import (
	"strings"
	"testing"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/config"
)

func validConfig() *config.Config {
	return &config.Config{
		App: config.AppConfig{
			Name:     "test-app",
			Mode:     "development",
			LogLevel: "info",
		},
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Name:           "testdb",
			User:           "user",
			Password:       "pass",
			MaxConnections: 10,
		},
		Collector: config.CollectorConfig{
			Interval: 10 * time.Second,
			Timeout:  5 * time.Second,
		},
		Analyzer: config.AnalyzerConfig{
			Thresholds: config.ThresholdConfig{
				CPUHigh: 80.0,
				CPULow:  30.0,
			},
		},
		Decision: config.DecisionConfig{
			MinServers:     2,
			MaxServers:     10,
			MaxScaleStep:   3,
			CooldownPeriod: 30 * time.Second,
		},
		API: config.APIConfig{
			Port: 8080,
		},
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := validConfig()

	err := cfg.Validate()

	if err != nil {
		t.Errorf("expected valid config, got error: %v", err)
	}
}

func TestConfig_Validate_InvalidThresholds(t *testing.T) {
	cfg := validConfig()
	cfg.Analyzer.Thresholds.CPUHigh = 20.0
	cfg.Analyzer.Thresholds.CPULow = 50.0

	err := cfg.Validate()

	if err == nil {
		t.Error("expected error for invalid thresholds")
	}
	if !strings.Contains(err.Error(), "cpu_high must be greater than cpu_low") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfig_Validate_InvalidMinMaxServers(t *testing.T) {
	cfg := validConfig()
	cfg.Decision.MinServers = 10
	cfg.Decision.MaxServers = 5

	err := cfg.Validate()

	if err == nil {
		t.Error("expected error for invalid min/max servers")
	}
	if !strings.Contains(err.Error(), "max_servers must be >= min_servers") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConfig_Validate_InvalidCollectorTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.Collector.Timeout = 15 * time.Second
	cfg.Collector.Interval = 10 * time.Second

	err := cfg.Validate()

	if err == nil {
		t.Error("expected error for invalid collector timeout")
	}
	if !strings.Contains(err.Error(), "timeout must be less than") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	dbCfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "admin",
		Password: "secret",
		SSLMode:  "disable",
	}

	dsn := dbCfg.DSN()

	expected := "host=localhost port=5432 user=admin password=secret dbname=testdb sslmode=disable"
	if dsn != expected {
		t.Errorf("expected DSN %q, got %q", expected, dsn)
	}
}
