package unit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*config.Config)
		expectErr   bool
		errContains string
	}{
		{
			name:       "valid config",
			modifyFunc: func(c *config.Config) {},
			expectErr:  false,
		},
		{
			name: "invalid thresholds - CPUHigh less than CPULow",
			modifyFunc: func(c *config.Config) {
				c.Analyzer.Thresholds.CPUHigh = 20.0
				c.Analyzer.Thresholds.CPULow = 50.0
			},
			expectErr:   true,
			errContains: "cpu_high must be greater than cpu_low",
		},
		{
			name: "invalid min/max servers",
			modifyFunc: func(c *config.Config) {
				c.Decision.MinServers = 10
				c.Decision.MaxServers = 5
			},
			expectErr:   true,
			errContains: "max_servers must be >= min_servers",
		},
		{
			name: "invalid collector timeout",
			modifyFunc: func(c *config.Config) {
				c.Collector.Timeout = 15 * time.Second
				c.Collector.Interval = 10 * time.Second
			},
			expectErr:   true,
			errContains: "timeout must be less than",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modifyFunc(cfg)

			err := cfg.Validate()

			if tt.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				assert.NoError(t, err)
			}
		})
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
	assert.Equal(t, expected, dsn)
}
