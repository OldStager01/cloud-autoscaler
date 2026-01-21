package collector

import (
	"context"
	"errors"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

var (
	ErrCollectionFailed  = errors.New("metric collection failed")
	ErrTimeout           = errors.New("collection timeout")
	ErrClusterNotFound   = errors.New("cluster not found")
	ErrInvalidResponse   = errors.New("invalid response from data source")
)

// Collector defines the interface for metric collection
type Collector interface {
	// Collect fetches metrics for a specific cluster
	Collect(ctx context.Context, clusterID string) (*models.ClusterMetrics, error)

	// HealthCheck verifies the collector can reach its data source
	HealthCheck(ctx context.Context) error

	// Close releases any resources held by the collector
	Close() error
}