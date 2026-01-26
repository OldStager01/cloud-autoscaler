package config

import (
	"fmt"
	"time"
)

type Config struct {
	App        AppConfig        `mapstructure:"app"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Collector  CollectorConfig  `mapstructure:"collector"`
	Analyzer   AnalyzerConfig   `mapstructure:"analyzer"`
	Decision   DecisionConfig   `mapstructure:"decision"`
	Predictor  PredictorConfig  `mapstructure:"predictor"`
	Scaler     ScalerConfig     `mapstructure:"scaler"`
	API        APIConfig        `mapstructure:"api"`
	WebSocket  WebSocketConfig  `mapstructure:"websocket"`
	Prometheus PrometheusConfig `mapstructure:"prometheus"`
}

type AppConfig struct {
	Name     string `mapstructure:"name"`
	Mode     string `mapstructure:"mode"`
	LogLevel string `mapstructure:"log_level"`
}

type DatabaseConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Name           string `mapstructure:"name"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
	MaxConnections int    `mapstructure:"max_connections"`
	SSLMode        string `mapstructure:"ssl_mode"`
}

func (d DatabaseConfig) DSN() string {
	sslMode := d.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, sslMode,
	)
}

type CollectorConfig struct {
	Type           string               `mapstructure:"type"`
	Endpoint       string               `mapstructure:"endpoint"`
	Interval       time.Duration        `mapstructure:"interval"`
	Timeout        time.Duration        `mapstructure:"timeout"`
	RetryAttempts  int                  `mapstructure:"retry_attempts"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	MaxFailures int           `mapstructure:"max_failures"`
	Timeout     time.Duration `mapstructure:"timeout"`
}

type AnalyzerConfig struct {
	Thresholds     ThresholdConfig `mapstructure:"thresholds"`
	TrendWindow    time.Duration   `mapstructure:"trend_window"`
	SpikeThreshold float64         `mapstructure:"spike_threshold"`
}

type ThresholdConfig struct {
	CPUHigh    float64 `mapstructure:"cpu_high"`
	CPULow     float64 `mapstructure:"cpu_low"`
	MemoryHigh float64 `mapstructure:"memory_high"`
}

type DecisionConfig struct {
	CooldownPeriod          time.Duration `mapstructure:"cooldown_period"`
	ScaleDownCooldownPeriod time.Duration `mapstructure:"scale_down_cooldown_period"`
	SustainedHighDuration   time.Duration `mapstructure:"sustained_high_duration"`
	SustainedLowDuration    time.Duration `mapstructure:"sustained_low_duration"`
	EmergencyCPUThreshold   float64       `mapstructure:"emergency_cpu_threshold"`
	MinServers              int           `mapstructure:"min_servers"`
	MaxServers              int           `mapstructure:"max_servers"`
	MaxScaleStep            int           `mapstructure:"max_scale_step"`
}

type PredictorConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	Type           string        `mapstructure:"type"`
	ForecastWindow time.Duration `mapstructure:"forecast_window"`
	MinConfidence  float64       `mapstructure:"min_confidence"`
}

type ScalerConfig struct {
	Type          string        `mapstructure:"type"`
	ProvisionTime time.Duration `mapstructure:"provision_time"`
	DrainTimeout  time.Duration `mapstructure:"drain_timeout"`
}

type APIConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	RateLimit    int           `mapstructure:"rate_limit"`
	JWTSecret    string        `mapstructure:"jwt_secret"`
}

type WebSocketConfig struct {
	MaxConnections int           `mapstructure:"max_connections"`
	PingInterval   time.Duration `mapstructure:"ping_interval"`
}

type PrometheusConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}