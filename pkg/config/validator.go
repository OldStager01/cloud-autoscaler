package config

import (
	"errors"
	"fmt"
)

func (c *Config) Validate() error {
	var errs []error

	// App validation
	if c.App.Name == "" {
		errs = append(errs, errors.New("app.name is required"))
	}

	validModes := map[string]bool{"development": true, "production": true, "test": true}
	if !validModes[c.App.Mode] {
		errs = append(errs, fmt.Errorf("app.mode must be one of: development, production, test"))
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[c.App.LogLevel] {
		errs = append(errs, fmt.Errorf("app.log_level must be one of: debug, info, warn, error"))
	}

	// Database validation
	if c.Database.Host == "" {
		errs = append(errs, errors.New("database.host is required"))
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		errs = append(errs, errors.New("database.port must be between 1 and 65535"))
	}
	if c.Database.Name == "" {
		errs = append(errs, errors.New("database.name is required"))
	}
	if c.Database.MaxConnections <= 0 {
		errs = append(errs, errors.New("database.max_connections must be positive"))
	}

	// Collector validation
	if c.Collector.Interval <= 0 {
		errs = append(errs, errors.New("collector.interval must be positive"))
	}
	if c.Collector.Timeout <= 0 {
		errs = append(errs, errors.New("collector.timeout must be positive"))
	}
	if c.Collector.Timeout >= c.Collector.Interval {
		errs = append(errs, errors.New("collector.timeout must be less than collector.interval"))
	}

	// Analyzer validation
	if c.Analyzer.Thresholds.CPUHigh <= c.Analyzer.Thresholds.CPULow {
		errs = append(errs, errors.New("analyzer.thresholds.cpu_high must be greater than cpu_low"))
	}
	if c.Analyzer.Thresholds.CPUHigh <= 0 || c.Analyzer.Thresholds.CPUHigh > 100 {
		errs = append(errs, errors.New("analyzer.thresholds.cpu_high must be between 0 and 100"))
	}
	if c.Analyzer.Thresholds.CPULow < 0 || c.Analyzer.Thresholds.CPULow >= 100 {
		errs = append(errs, errors.New("analyzer.thresholds.cpu_low must be between 0 and 100"))
	}

	// Decision validation
	if c.Decision.MinServers <= 0 {
		errs = append(errs, errors.New("decision.min_servers must be positive"))
	}
	if c.Decision.MaxServers < c.Decision.MinServers {
		errs = append(errs, errors.New("decision.max_servers must be >= min_servers"))
	}
	if c.Decision.MaxScaleStep <= 0 {
		errs = append(errs, errors.New("decision.max_scale_step must be positive"))
	}
	if c.Decision.CooldownPeriod <= 0 {
		errs = append(errs, errors.New("decision.cooldown_period must be positive"))
	}

	// API validation
	if c.API.Port <= 0 || c.API.Port > 65535 {
		errs = append(errs, errors.New("api.port must be between 1 and 65535"))
	}
	if c.App.Mode == "production" && c.API.JWTSecret == "change-me-in-production" {
		errs = append(errs, errors.New("api.jwt_secret must be changed in production"))
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed: %v", errs)
	}

	return nil
}