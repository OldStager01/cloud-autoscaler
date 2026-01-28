package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/scaler"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
	"github.com/gin-gonic/gin"
)

// ClusterManager interface for orchestrator operations
type ClusterManager interface {
	StartCluster(cluster *models.Cluster, coll collector.Collector, scal scaler.Scaler) error
	StopCluster(clusterID string) error
	SubscribeAllEvents() <-chan *models.Event
}

type ClusterHandler struct {
	clusterRepo    *queries.ClusterRepository
	clusterManager ClusterManager
	simulatorURL   string
	httpClient     *http.Client
}

func NewClusterHandler(clusterRepo *queries.ClusterRepository, clusterManager ClusterManager) *ClusterHandler {
	return &ClusterHandler{
		clusterRepo:    clusterRepo,
		clusterManager: clusterManager,
		simulatorURL:   "http://localhost:9000",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type CreateClusterRequest struct {
	Name       string                `json:"name" binding:"required,min=1,max=100"`
	MinServers int                   `json:"min_servers" binding:"required,min=1"`
	MaxServers int                   `json:"max_servers" binding:"required,min=1"`
	Config     *models.ClusterConfig `json:"config"`
}

type UpdateClusterRequest struct {
	Name       string                `json:"name" binding:"omitempty,min=1,max=100"`
	MinServers *int                  `json:"min_servers" binding:"omitempty,min=1"`
	MaxServers *int                  `json:"max_servers" binding:"omitempty,min=1"`
	Status     string                `json:"status" binding:"omitempty,oneof=active paused"`
	Config     *models.ClusterConfig `json:"config"`
}

type ClusterResponse struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	MinServers int                   `json:"min_servers"`
	MaxServers int                   `json:"max_servers"`
	Status     string                `json:"status"`
	Config     *models.ClusterConfig `json:"config,omitempty"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

func toClusterResponse(c *models.Cluster) ClusterResponse {
	return ClusterResponse{
		ID:         c.ID,
		Name:       c.Name,
		MinServers: c.MinServers,
		MaxServers: c.MaxServers,
		Status:      string(c.Status),
		Config:     c.Config,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func (h *ClusterHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	clusters, err := h.clusterRepo.GetAll(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch clusters"})
		return
	}

	response := make([]ClusterResponse, len(clusters))
	for i, cluster := range clusters {
		response[i] = toClusterResponse(cluster)
	}

	c.JSON(http.StatusOK, gin.H{
		"clusters":  response,
		"count":    len(response),
	})
}

func (h *ClusterHandler) Get(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	c.JSON(http.StatusOK, toClusterResponse(cluster))
}

func (h *ClusterHandler) Create(c *gin.Context) {
	var req CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.MaxServers < req.MinServers {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_servers must be >= min_servers"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Check for duplicate name
	existing, err := h.clusterRepo.GetByName(ctx, req.Name)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "cluster with this name already exists"})
		return
	}

	cluster := models.NewCluster(req.Name, req.MinServers, req.MaxServers)
	cluster.Config = req.Config

	if err := h.clusterRepo.Create(ctx, cluster); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cluster"})
		return
	}

	// Start monitoring pipeline for the new cluster
	if h.clusterManager != nil {
		// Create cluster in simulator with correct server count
		h.createInSimulator(cluster.ID, cluster.MinServers)

		simulatorURL := "http://localhost:9000/metrics/" + cluster.ID
		coll := collector.NewHTTPCollector(collector.HTTPCollectorConfig{
			Endpoint: simulatorURL,
			Timeout:  5 * time.Second,
		})

		scal := scaler.NewSimulatorScaler(scaler.SimulatorConfig{
			ProvisionTime: 3 * time.Second,
			DrainTimeout:  2 * time.Second,
		})
		scal.InitializeCluster(cluster.ID, cluster.MinServers)

		if err := h.clusterManager.StartCluster(cluster, coll, scal); err != nil {
			// Log error but don't fail the request - cluster is created
			c.JSON(http.StatusCreated, gin.H{
				"cluster": toClusterResponse(cluster),
				"warning": "cluster created but monitoring failed to start: " + err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusCreated, toClusterResponse(cluster))
}

func (h *ClusterHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req UpdateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	// Apply updates
	if req.Name != "" {
		cluster.Name = req.Name
	}
	if req.MinServers != nil {
		cluster.MinServers = *req.MinServers
	}
	if req.MaxServers != nil {
		cluster.MaxServers = *req.MaxServers
	}
	if req.Status != "" {
		cluster.Status = models.ClusterStatus(req.Status)
	}
	if req.Config != nil {
		cluster.Config = req.Config
	}

	if cluster.MaxServers < cluster.MinServers {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max_servers must be >= min_servers"})
		return
	}

	if err := h.clusterRepo.Update(ctx, cluster); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update cluster"})
		return
	}

	c.JSON(http.StatusOK, toClusterResponse(cluster))
}

func (h *ClusterHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Stop monitoring pipeline first
	if h.clusterManager != nil {
		_ = h.clusterManager.StopCluster(id) // Ignore error if not running
	}

	// Delete from simulator
	h.deleteFromSimulator(id)

	if err := h.clusterRepo.Delete(ctx, id); err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete cluster"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cluster deleted"})
}

// deleteFromSimulator notifies the simulator to delete a cluster
func (h *ClusterHandler) deleteFromSimulator(clusterID string) {
	url := h.simulatorURL + "/clusters/" + clusterID
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// createInSimulator creates a cluster in the simulator with the specified server count
func (h *ClusterHandler) createInSimulator(clusterID string, serverCount int) {
	payload := map[string]interface{}{
		"servers":     serverCount,
		"base_cpu":    50.0,
		"base_memory": 60.0,
		"variance":    10.0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := h.simulatorURL + "/clusters/" + clusterID
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (h *ClusterHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	serverCounts, err := h.clusterRepo.GetServerCounts(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch server counts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cluster_id":   cluster.ID,
		"name":         cluster.Name,
		"status":       cluster.Status,
		"servers":  gin.H{
			"total":        serverCounts.Total,
			"active":       serverCounts.Active,
			"provisioning": serverCounts.Provisioning,
			"draining":      serverCounts.Draining,
		},
	})
}