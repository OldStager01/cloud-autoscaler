package scaler

import (
	"context"
	"errors"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

var (
	ErrScalingFailed    = errors.New("scaling operation failed")
	ErrInvalidTarget    = errors.New("invalid target server count")
	ErrClusterNotFound  = errors.New("cluster not found")
	ErrTimeout          = errors.New("scaling operation timeout")
	ErrProvisionFailed  = errors.New("server provisioning failed")
	ErrTerminateFailed  = errors.New("server termination failed")
)

// ScaleResult contains the result of a scaling operation
type ScaleResult struct {
	ClusterID       string
	Success         bool
	ServersAdded    []string
	ServersRemoved  []string
	Error           error
	PartialSuccess  bool
}

// Scaler defines the interface for executing scaling operations
type Scaler interface {
	// ScaleUp adds servers to a cluster
	ScaleUp(ctx context.Context, clusterID string, count int) (*ScaleResult, error)

	// ScaleDown removes servers from a cluster
	ScaleDown(ctx context.Context, clusterID string, count int) (*ScaleResult, error)

	// GetClusterState returns current state of servers in a cluster
	GetClusterState(ctx context.Context, clusterID string) (*models.ClusterState, error)

	// GetServer returns details of a specific server
	GetServer(ctx context.Context, serverID string) (*models.Server, error)

	// Close releases resources
	Close() error
}