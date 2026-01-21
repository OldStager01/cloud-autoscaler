package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Config file settings
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/autoscaler")
	}

	// Environment variable settings
	v.SetEnvPrefix("AUTOSCALER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found, use defaults and env vars
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "cloud-autoscaler")
	v.SetDefault("app.mode", "development")
	v.SetDefault("app.log_level", "info")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "autoscaler")
	v.SetDefault("database.user", "admin")
	v.SetDefault("database.password", "password")
	v.SetDefault("database.max_connections", 25)
	v.SetDefault("database.ssl_mode", "disable")

	// Collector defaults
	v.SetDefault("collector.type", "http")
	v.SetDefault("collector.endpoint", "http://localhost:9000/metrics")
	v.SetDefault("collector.interval", "10s")
	v.SetDefault("collector.timeout", "5s")
	v.SetDefault("collector.retry_attempts", 3)
	v.SetDefault("collector.circuit_breaker.max_failures", 5)
	v.SetDefault("collector.circuit_breaker.timeout", "30s")

	// Analyzer defaults
	v.SetDefault("analyzer.thresholds.cpu_high", 80.0)
	v.SetDefault("analyzer.thresholds.cpu_low", 30.0)
	v.SetDefault("analyzer.thresholds.memory_high", 85.0)
	v.SetDefault("analyzer.trend_window", "5m")
	v.SetDefault("analyzer.spike_threshold", 50.0)

	// Decision defaults
	v.SetDefault("decision.cooldown_period", "5m")
	v.SetDefault("decision.emergency_cpu_threshold", 95.0)
	v.SetDefault("decision.min_servers", 2)
	v.SetDefault("decision.max_servers", 50)
	v.SetDefault("decision.max_scale_step", 3)

	// Predictor defaults
	v.SetDefault("predictor.enabled", false)
	v.SetDefault("predictor.type", "pattern_matching")
	v.SetDefault("predictor.forecast_window", "15m")
	v.SetDefault("predictor.min_confidence", 0.7)

	// Scaler defaults
	v.SetDefault("scaler.type", "simulator")
	v.SetDefault("scaler.provision_time", "10s")
	v.SetDefault("scaler.drain_timeout", "30s")

	// API defaults
	v.SetDefault("api.port", 8080)
	v.SetDefault("api.read_timeout", "15s")
	v.SetDefault("api.write_timeout", "15s")
	v.SetDefault("api.rate_limit", 100)
	v.SetDefault("api.jwt_secret", "change-me-in-production")

	// WebSocket defaults
	v.SetDefault("websocket.max_connections", 1000)
	v.SetDefault("websocket.ping_interval", "30s")

	// Prometheus defaults
	v.SetDefault("prometheus.enabled", true)
	v.SetDefault("prometheus.port", 9090)
}