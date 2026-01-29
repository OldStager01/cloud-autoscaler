package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/gin-gonic/gin"
)

type MetricsHandler struct {
	metricsRepo *queries.MetricsRepository
	eventsRepo  *queries.ScalingEventRepository
	clusterRepo *queries.ClusterRepository
	config      *config.APIConfig
}

func NewMetricsHandler(metricsRepo *queries.MetricsRepository, eventsRepo *queries.ScalingEventRepository, clusterRepo *queries.ClusterRepository, cfg *config.APIConfig) *MetricsHandler {
	return &MetricsHandler{
		metricsRepo: metricsRepo,
		eventsRepo:  eventsRepo,
		clusterRepo: clusterRepo,
		config:      cfg,
	}
}

// verifyClusterOwnership checks if the authenticated user owns the cluster
func (h *MetricsHandler) verifyClusterOwnership(c *gin.Context, clusterID string) bool {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return false
	}

	cluster, err := h.clusterRepo.GetByID(c.Request.Context(), clusterID)
	if err != nil {
		if err == queries.ErrClusterNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
			return false
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify cluster ownership"})
		return false
	}

	if cluster.UserID == nil || *cluster.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return false
	}

	return true
}

func (h *MetricsHandler) getDefaultLimit() int {
	if h.config != nil && h.config.DefaultLimit > 0 {
		return h.config.DefaultLimit
	}
	return 100
}

func (h *MetricsHandler) getMaxLimit() int {
	if h.config != nil && h.config.MaxLimit > 0 {
		return h.config.MaxLimit
	}
	return 1000
}

func (h *MetricsHandler) GetMetrics(c *gin.Context) {
	clusterID := c.Param("id")

	if !h.verifyClusterOwnership(c, clusterID) {
		return
	}

	from, to := h.parseTimeRange(c)
	limit := h.parseLimit(c, h.getDefaultLimit())
	aggregated := c.Query("aggregated") == "true"
	bucketMinutes := h.parseInt(c.Query("bucket"), 5)

	ctx := c.Request.Context()

	if aggregated {
		metrics, err := h.metricsRepo.GetAggregated(ctx, clusterID, from, to, bucketMinutes)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"cluster_id":  clusterID,
			"from":       from,
			"to":         to,
			"aggregated": true,
			"bucket_minutes": bucketMinutes,
			"data":       metrics,
			"count":      len(metrics),
		})
		return
	}

	metrics, err := h.metricsRepo.GetRaw(ctx, clusterID, from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cluster_id": clusterID,
		"from":       from,
		"to":         to,
		"aggregated":  false,
		"data":       metrics,
		"count":      len(metrics),
	})
}

func (h *MetricsHandler) GetLatestMetrics(c *gin.Context) {
	clusterID := c.Param("id")

	if !h.verifyClusterOwnership(c, clusterID) {
		return
	}

	ctx := c.Request.Context()

	metrics, err := h.metricsRepo.GetLatest(ctx, clusterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metrics"})
		return
	}

	if metrics == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no recent metrics found"})
		return
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *MetricsHandler) GetHourlyMetrics(c *gin.Context) {
	clusterID := c.Param("id")

	if !h.verifyClusterOwnership(c, clusterID) {
		return
	}

	from, to := h.parseTimeRange(c)
	ctx := c.Request.Context()

	metrics, err := h.metricsRepo.GetHourly(ctx, clusterID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch hourly metrics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cluster_id": clusterID,
		"from":       from,
		"to":         to,
		"data":       metrics,
		"count":      len(metrics),
	})
}

func (h *MetricsHandler) GetScalingEvents(c *gin.Context) {
	clusterID := c.Param("id")

	if !h.verifyClusterOwnership(c, clusterID) {
		return
	}

	from, to := h.parseTimeRange(c)
	limit := h.parseLimit(c, 50)
	ctx := c.Request.Context()

	events, err := h.eventsRepo.GetByCluster(ctx, clusterID, from, to, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch scaling events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cluster_id": clusterID,
		"from":        from,
		"to":          to,
		"data":       events,
		"count":      len(events),
	})
}

func (h *MetricsHandler) GetScalingStats(c *gin.Context) {
	clusterID := c.Param("id")

	if !h.verifyClusterOwnership(c, clusterID) {
		return
	}

	from, to := h.parseTimeRange(c)
	ctx := c.Request.Context()

	stats, err := h.eventsRepo.GetStats(ctx, clusterID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch scaling stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *MetricsHandler) GetRecentEvents(c *gin.Context) {
	userID, ok := getUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not authenticated"})
		return
	}

	limit := h.parseLimit(c, 20)
	ctx := c.Request.Context()

	events, err := h.eventsRepo.GetRecentByUserID(ctx, userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch recent events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  events,
		"count": len(events),
	})
}

func (h *MetricsHandler) parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	to := time.Now()
	from := to.Add(-1 * time.Hour) // Default:  last hour

	if fromStr := c.Query("from"); fromStr != "" {
		if parsed, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = parsed
		}
	}

	if toStr := c.Query("to"); toStr != "" {
		if parsed, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = parsed
		}
	}

	// Handle relative time (e.g., "1h", "24h", "7d")
	if rangeStr := c.Query("range"); rangeStr != "" {
		duration := h.parseDuration(rangeStr)
		from = to.Add(-duration)
	}

	return from, to
}

func (h *MetricsHandler) parseLimit(c *gin.Context, defaultLimit int) int {
	maxLimit := h.getMaxLimit()
	limit := defaultLimit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > maxLimit {
				limit = maxLimit
			}
		}
	}
	return limit
}

func (h *MetricsHandler) parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	if parsed, err := strconv.Atoi(s); err == nil {
		return parsed
	}
	return defaultVal
}

func (h *MetricsHandler) parseDuration(s string) time.Duration {
	if len(s) < 2 {
		return time.Hour
	}

	unit := s[len(s)-1]
	valueStr := s[:len(s)-1]
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return time.Hour
	}

	switch unit {
	case 'm':
		return time.Duration(value) * time.Minute
	case 'h': 
		return time.Duration(value) * time.Hour
	case 'd':
		return time.Duration(value) * 24 * time.Hour
	default:
		return time.Hour
	}
}