package simulator

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
)

type Config struct {
	Port int
}

type Simulator struct {
	config     Config
	clusters   map[string]*ClusterSim
	mu         sync. RWMutex
	httpServer *http.Server
}

func New(cfg Config) *Simulator {
	if cfg.Port == 0 {
		cfg.Port = 9000
	}

	return &Simulator{
		config:   cfg,
		clusters: make(map[string]*ClusterSim),
	}
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func (s *Simulator) Start() error {
	mux := http.NewServeMux()

	// Routes with CORS
	mux.HandleFunc("/health", cors(s.healthHandler))
	mux.HandleFunc("/metrics/", cors(s.metricsHandler))
	mux.HandleFunc("/clusters", cors(s.listClustersHandler))
	mux.HandleFunc("/clusters/", cors(s.clusterHandler))
	mux.HandleFunc("/spike", cors(s.spikeHandler))
	mux.HandleFunc("/pattern", cors(s.patternHandler))

	addr := fmt.Sprintf(":%d", s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:       mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	logger.Infof("Simulator listening on %s", addr)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Errorf("Simulator server error: %v", err)
		}
	}()

	return nil
}


func (s *Simulator) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Simulator) GetOrCreateCluster(clusterID string) *ClusterSim {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cluster, exists := s.clusters[clusterID]; exists {
		return cluster
	}

	cluster := NewClusterSim(clusterID, ClusterSimConfig{
		InitialServers: 3,
		BaseCPU:        50.0,
		BaseMemory:      60.0,
		Variance:       10.0,
	})
	s.clusters[clusterID] = cluster

	logger.Infof("Created new simulated cluster:  %s", clusterID)
	return cluster
}

func (s *Simulator) GetCluster(clusterID string) (*ClusterSim, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, exists := s.clusters[clusterID]
	return cluster, exists
}

// HTTP Handlers

func (s *Simulator) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "metrics-simulator",
	})
}

func (s *Simulator) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract cluster ID from path:  /metrics/{clusterID}
	clusterID := r.URL.Path[len("/metrics/"):]
	if clusterID == "" {
		http.Error(w, "cluster ID required", http.StatusBadRequest)
		return
	}

	cluster := s.GetOrCreateCluster(clusterID)
	metrics := cluster.CollectMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

func (s *Simulator) listClustersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	clusters := make([]map[string]interface{}, 0, len(s.clusters))
	for id, cluster := range s.clusters {
		clusters = append(clusters, map[string]interface{}{
			"id":           id,
			"server_count": cluster.ServerCount(),
			"pattern":       cluster.GetPattern(),
		})
	}
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"clusters": clusters,
		"count":    len(clusters),
	})
}

func (s *Simulator) clusterHandler(w http.ResponseWriter, r *http.Request) {
	// Extract cluster ID from path: /clusters/{clusterID}
	clusterID := r.URL.Path[len("/clusters/"):]
	if clusterID == "" {
		http.Error(w, "cluster ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getClusterHandler(w, r, clusterID)
	case http.MethodPost:
		s.createClusterHandler(w, r, clusterID)
	case http.MethodPut:
		s.updateClusterHandler(w, r, clusterID)
	case http.MethodDelete:
		s.deleteClusterHandler(w, r, clusterID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Simulator) getClusterHandler(w http.ResponseWriter, r *http.Request, clusterID string) {
	cluster, exists := s.GetCluster(clusterID)
	if !exists {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster.Status())
}

type CreateClusterRequest struct {
	Servers    int     `json:"servers"`
	BaseCPU    float64 `json:"base_cpu"`
	BaseMemory float64 `json:"base_memory"`
	Variance   float64 `json:"variance"`
}

func (s *Simulator) createClusterHandler(w http.ResponseWriter, r *http.Request, clusterID string) {
	var req CreateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Servers <= 0 {
		req.Servers = 3
	}
	if req.BaseCPU <= 0 {
		req.BaseCPU = 50.0
	}
	if req.BaseMemory <= 0 {
		req.BaseMemory = 60.0
	}
	if req.Variance <= 0 {
		req.Variance = 10.0
	}

	s.mu.Lock()
	cluster := NewClusterSim(clusterID, ClusterSimConfig{
		InitialServers: req.Servers,
		BaseCPU:        req.BaseCPU,
		BaseMemory:     req.BaseMemory,
		Variance:       req.Variance,
	})
	s.clusters[clusterID] = cluster
	s.mu.Unlock()

	logger.Infof("Created cluster %s with %d servers", clusterID, req.Servers)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(cluster.Status())
}

type UpdateClusterRequest struct {
	BaseCPU    *float64 `json:"base_cpu"`
	BaseMemory *float64 `json:"base_memory"`
	Variance   *float64 `json:"variance"`
	AddServers *int     `json:"add_servers"`
	RemoveServers *int  `json:"remove_servers"`
}

func (s *Simulator) updateClusterHandler(w http.ResponseWriter, r *http.Request, clusterID string) {
	cluster, exists := s.GetCluster(clusterID)
	if !exists {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}

	var req UpdateClusterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.BaseCPU != nil {
		cluster.SetBaseCPU(*req.BaseCPU)
	}
	if req.BaseMemory != nil {
		cluster.SetBaseMemory(*req.BaseMemory)
	}
	if req.Variance != nil {
		cluster.SetVariance(*req.Variance)
	}
	if req.AddServers != nil && *req.AddServers > 0 {
		cluster.AddServers(*req.AddServers)
	}
	if req.RemoveServers != nil && *req.RemoveServers > 0 {
		cluster.RemoveServers(*req.RemoveServers)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cluster.Status())
}

func (s *Simulator) deleteClusterHandler(w http.ResponseWriter, r *http.Request, clusterID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.clusters[clusterID]; !exists {
		http.Error(w, "cluster not found", http.StatusNotFound)
		return
	}

	delete(s.clusters, clusterID)
	logger.Infof("Deleted cluster %s", clusterID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "cluster deleted"})
}

type SpikeRequest struct {
	ClusterID  string  `json:"cluster_id"`
	CPUTarget  float64 `json:"cpu_target"`
	Duration   string  `json:"duration"`
	RampUp     string  `json:"ramp_up"`
}

func (s *Simulator) spikeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpikeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cluster, exists := s.GetCluster(req.ClusterID)
	if !exists {
		cluster = s.GetOrCreateCluster(req.ClusterID)
	}

	duration, err := time.ParseDuration(req.Duration)
	if err != nil {
		duration = 5 * time.Minute
	}

	rampUp, err := time.ParseDuration(req.RampUp)
	if err != nil {
		rampUp = 30 * time.Second
	}

	cluster.InjectSpike(req.CPUTarget, duration, rampUp)

	logger.Infof("Injected spike on cluster %s:  target=%.1f%%, duration=%s", 
		req.ClusterID, req.CPUTarget, duration)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":     "spike injected",
		"cluster_id":  req.ClusterID,
		"cpu_target": req.CPUTarget,
		"duration":   duration.String(),
		"ramp_up":    rampUp.String(),
	})
}

type PatternRequest struct {
	ClusterID string `json:"cluster_id"`
	Pattern   string `json:"pattern"` // "steady", "daily", "weekly", "random", "gradual_rise"
}

func (s *Simulator) patternHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PatternRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	cluster, exists := s.GetCluster(req.ClusterID)
	if !exists {
		cluster = s.GetOrCreateCluster(req.ClusterID)
	}

	pattern := ParsePattern(req.Pattern)
	cluster.SetPattern(pattern)

	logger.Infof("Set pattern %s on cluster %s", req.Pattern, req.ClusterID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "pattern set",
		"cluster_id":  req.ClusterID,
		"pattern":    req.Pattern,
	})
}