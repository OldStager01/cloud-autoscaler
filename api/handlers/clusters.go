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
	"github.com/OldStager01/cloud-autoscaler/pkg/validation"
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
	Name       string                `json:"name" binding:"required,min=1,max=100" example:"production-cluster"`
	MinServers int                   `json:"min_servers" binding:"required,min=1" example:"2"`
	MaxServers int                   `json:"max_servers" binding:"required,min=1" example:"10"`
	Config     *models.ClusterConfig `json:"config"`
}

type UpdateClusterRequest struct {
	Name       string                `json:"name" binding:"omitempty,min=1,max=100" example:"updated-cluster"`
	MinServers *int                  `json:"min_servers" binding:"omitempty,min=1" example:"3"`
	MaxServers *int                  `json:"max_servers" binding:"omitempty,min=1" example:"15"`
	Status     string                `json:"status" binding:"omitempty,oneof=active paused" example:"active"`
	Config     *models.ClusterConfig `json:"config"`
}

type ClusterResponse struct {
	ID         string                `json:"id" example:"clstr_abc123"`
	Name       string                `json:"name" example:"production-cluster"`
	MinServers int                   `json:"min_servers" example:"2"`
	MaxServers int                   `json:"max_servers" example:"10"`
	Status     string                `json:"status" example:"active"`
	Config     *models.ClusterConfig `json:"config,omitempty"`
	UserID     *int                  `json:"user_id,omitempty" example:"1"`
	CreatedAt  time.Time             `json:"created_at" example:"2024-01-15T10:30:00Z"`
	UpdatedAt  time.Time             `json:"updated_at" example:"2024-01-15T10:30:00Z"`
}

func toClusterResponse(c *models.Cluster) ClusterResponse {
	return ClusterResponse{
		ID:         c.ID,
		Name:       c.Name,
		MinServers: c.MinServers,
		MaxServers: c.MaxServers,
		Status:      string(c.Status),
		Config:     c.Config,
		UserID:     c.UserID,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

// getUserID extracts the authenticated user's ID from the context
func getUserID(c *gin.Context) (int, bool) {
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(int); ok {
			return id, true
		}
	}
	return 0, false
}

// List godoc
// @Summary List clusters
// @Description Get all clusters owned by the authenticated user
// @Tags Clusters
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "List of clusters"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters [get]
func (h *ClusterHandler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	clusters, err := h.clusterRepo.GetByUserID(ctx, userID)
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

// Get godoc
// @Summary Get cluster
// @Description Get a specific cluster by ID
// @Tags Clusters
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cluster ID"
// @Success 200 {object} ClusterResponse "Cluster details"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 403 {object} map[string]string "Access denied"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters/{id} [get]
func (h *ClusterHandler) Get(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
// Create godoc
// @Summary Create cluster
// @Description Create a new cluster for the authenticated user
// @Tags Clusters
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateClusterRequest true "Cluster details"
// @Success 201 {object} ClusterResponse "Cluster created successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 409 {object} map[string]string "Cluster with this name already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters [post]
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	// Check ownership
	if cluster.UserID == nil || *cluster.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
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

	// Sanitize and validate cluster name
	req.Name = validation.SanitizeString(req.Name)
	if err := validation.ValidateClusterName(req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate server counts
	if err := validation.ValidateServerCount(req.MinServers, req.MaxServers); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	// Get the authenticated user's ID from context
	var userID *int
	if uid, exists := c.Get("user_id"); exists {
		if id, ok := uid.(int); ok {
			userID = &id
		}
	}

	cluster := models.NewCluster(req.Name, req.MinServers, req.MaxServers, userID)
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

// Update godoc
// @Summary Update cluster
// @Description Update an existing cluster
// @Tags Clusters
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cluster ID"
// @Param request body UpdateClusterRequest true "Fields to update"
// @Success 200 {object} ClusterResponse "Cluster updated successfully"
// @Failure 400 {object} map[string]string "Invalid request body"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 403 {object} map[string]string "Access denied"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters/{id} [put]
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

	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	// Check ownership
	if cluster.UserID == nil || *cluster.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Apply updates
// Delete godoc
// @Summary Delete cluster
// @Description Delete a cluster by ID
// @Tags Clusters
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cluster ID"
// @Success 200 {object} map[string]string "Cluster deleted successfully"
// @Failure 401 {object} map[string]string "User not authenticated"
// @Failure 403 {object} map[string]string "Access denied"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters/{id} [delete]
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

	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	// Check ownership before deleting
	cluster, err := h.clusterRepo.GetByID(ctx, id)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cluster"})
		return
	}

	if cluster.UserID == nil || *cluster.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

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
// GetStatus godoc
// @Summary Get cluster status
// @Description Get the current status and server counts for a cluster
// @Tags Clusters
// @Produce json
// @Security BearerAuth
// @Param id path string true "Cluster ID"
// @Success 200 {object} map[string]interface{} "Cluster status with server counts"
// @Failure 404 {object} map[string]string "Cluster not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /clusters/{id}/status [get]

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