package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/gin-gonic/gin"
)

type HealthHandler struct {
	db *database.DB
}

func NewHealthHandler(db *database.DB) *HealthHandler {
	return &HealthHandler{db:  db}
}

type HealthResponse struct {
	Status    string            `json:"status" example:"healthy"`
	Timestamp string            `json:"timestamp" example:"2024-01-15T10:30:00Z"`
	Checks    map[string]string `json:"checks,omitempty"`
}

// Health godoc
// @Summary Health check
// @Description Get overall health status including database connectivity
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse "Service is healthy"
// @Failure 503 {object} HealthResponse "Service is unhealthy"
// @Router /health [get]
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	status := "healthy"

	// Check database
	if err := h.db.HealthCheck(ctx); err != nil {
		checks["database"] = "unhealthy:  " + err.Error()
		status = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	statusCode := http.StatusOK
	if status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, HealthResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

// Ready godoc
// @Summary Readiness probe
// @Description Check if the service is ready to accept traffic
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse "Service is ready"
// @Failure 503 {object} HealthResponse "Service is not ready"
// @Router /health/ready [get]
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	if err := h.db.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, HealthResponse{
			Status:    "not ready",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ready",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Live godoc
// @Summary Liveness probe
// @Description Check if the service is alive
// @Tags Health
// @Produce json
// @Success 200 {object} HealthResponse "Service is alive"
// @Router /health/live [get]
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "alive",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}