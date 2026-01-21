package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type HTTPCollector struct {
	client   *http.Client
	endpoint string
	timeout  time.Duration
}

type HTTPCollectorConfig struct {
	Endpoint      string
	Timeout       time.Duration
	RetryAttempts int
}

func NewHTTPCollector(cfg HTTPCollectorConfig) *HTTPCollector {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	return &HTTPCollector{
		client:  &http.Client{
			Timeout: timeout,
		},
		endpoint: cfg.Endpoint,
		timeout:  timeout,
	}
}

// simulatorResponse matches the expected response from the simulator service
type simulatorResponse struct {
	ClusterID string                    `json:"cluster_id"`
	Timestamp string                    `json:"timestamp"`
	Servers   []simulatorServerResponse `json:"servers"`
}

type simulatorServerResponse struct {
	ServerID    string  `json:"server_id"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	RequestLoad int     `json:"request_load"`
}

func (c *HTTPCollector) Collect(ctx context.Context, clusterID string) (*models.ClusterMetrics, error) {
	url := fmt.Sprintf("%s/%s", c.endpoint, clusterID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to create request:  %v", ErrCollectionFailed, err)
	}

	req.Header.Set("Accept", "application/json")

	logger.WithCluster(clusterID).Debugf("Collecting metrics from %s", url)

	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, ErrTimeout
		}
		return nil, fmt.Errorf("%w: %v", ErrCollectionFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrClusterNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: unexpected status code %d", ErrCollectionFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to read response body: %v", ErrCollectionFailed, err)
	}

	var simResp simulatorResponse
	if err := json.Unmarshal(body, &simResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}

	metrics := c.convertResponse(clusterID, &simResp)

	logger.WithCluster(clusterID).Debugf("Collected metrics for %d servers", len(metrics.Servers))

	return metrics, nil
}

func (c *HTTPCollector) convertResponse(clusterID string, resp *simulatorResponse) *models.ClusterMetrics {
	servers := make([]models.ServerMetric, len(resp.Servers))
	for i, s := range resp.Servers {
		servers[i] = models.ServerMetric{
			ServerID:    s.ServerID,
			CPUUsage:    s.CPUUsage,
			MemoryUsage: s.MemoryUsage,
			RequestLoad: s.RequestLoad,
		}
	}

	timestamp := time.Now()
	if resp.Timestamp != "" {
		if parsed, err := time.Parse(time.RFC3339, resp.Timestamp); err == nil {
			timestamp = parsed
		}
	}

	return &models.ClusterMetrics{
		ClusterID:  clusterID,
		Timestamp: timestamp,
		Servers:   servers,
	}
}

func (c *HTTPCollector) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *HTTPCollector) Close() error {
	c.client.CloseIdleConnections()
	return nil
}