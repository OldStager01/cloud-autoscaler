package collector

import (
	"context"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type ResilientCollector struct {
	collector      Collector
	circuitBreaker *resilience.CircuitBreaker
	retryAttempts  int
	retryDelay     time.Duration
}

type ResilientCollectorConfig struct {
	Collector      Collector
	MaxFailures    int
	Timeout        time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
	OnStateChange  func(name string, from, to resilience.State)
}

func NewResilientCollector(cfg ResilientCollectorConfig) *ResilientCollector {
	if cfg.RetryAttempts <= 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 1 * time.Second
	}

	cb := resilience.NewCircuitBreaker(resilience.CircuitBreakerConfig{
		Name:          "collector",
		MaxFailures:   cfg.MaxFailures,
		Timeout:       cfg.Timeout,
		OnStateChange: cfg.OnStateChange,
	})

	return &ResilientCollector{
		collector:      cfg.Collector,
		circuitBreaker: cb,
		retryAttempts:  cfg.RetryAttempts,
		retryDelay:     cfg.RetryDelay,
	}
}

func (c *ResilientCollector) Collect(ctx context.Context, clusterID string) (*models.ClusterMetrics, error) {
	var metrics *models.ClusterMetrics
	var lastErr error

	err := c.circuitBreaker.Execute(func() error {
		for attempt := 1; attempt <= c.retryAttempts; attempt++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			var err error
			metrics, err = c.collector.Collect(ctx, clusterID)
			if err == nil {
				return nil
			}

			lastErr = err
			logger.WithCluster(clusterID).Warnf(
				"Collection attempt %d/%d failed:  %v",
				attempt, c.retryAttempts, err,
			)

			if attempt < c.retryAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(c.retryDelay):
				}
			}
		}
		return lastErr
	})

	if err != nil {
		return nil, err
	}

	return metrics, nil
}

func (c *ResilientCollector) HealthCheck(ctx context.Context) error {
	return c.collector.HealthCheck(ctx)
}

func (c *ResilientCollector) Close() error {
	return c.collector.Close()
}

func (c *ResilientCollector) CircuitState() resilience.State {
	return c.circuitBreaker.State()
}

func (c *ResilientCollector) ResetCircuit() {
	c.circuitBreaker.Reset()
}