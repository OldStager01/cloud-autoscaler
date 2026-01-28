package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/api/handlers"
	"github.com/OldStager01/cloud-autoscaler/api/middleware"
	"github.com/OldStager01/cloud-autoscaler/api/websocket"
	"github.com/OldStager01/cloud-autoscaler/internal/auth"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router         *gin.Engine
	httpServer     *http.Server
	config         config.APIConfig
	db             *database.DB
	authService    *auth.Service
	wsHub          *websocket.Hub
	wsBridge       *websocket.EventBridge
	clusterManager handlers.ClusterManager
}

func NewServer(cfg config.APIConfig, db *database.DB, clusterManager handlers.ClusterManager) *Server {
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	authService := auth.NewService(cfg.JWTSecret, 24*time.Hour)
	wsHub := websocket.NewHub()

	s := &Server{
		router:         router,
		config:         cfg,
		db:             db,
		authService:    authService,
		wsHub:          wsHub,
		clusterManager: clusterManager,
	}

	s.setupMiddleware()
	s.setupRoutes()

	// Start WebSocket hub
	go wsHub.Run()

	// Start event bridge to forward orchestrator events to WebSocket clients
	if clusterManager != nil {
		eventsChan := clusterManager.SubscribeAllEvents()
		s.wsBridge = websocket.NewEventBridge(wsHub, eventsChan)
		s.wsBridge.Start()
	}

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Recovery())
	s.router.Use(middleware.CORS(middleware.DefaultCORSConfig()))
	s.router.Use(middleware.RequestLogger())
	s.router.Use(middleware.TraceID())

	rateLimiter := middleware.NewRateLimiter(s.config.RateLimit, time.Minute)
	s.router.Use(middleware.RateLimit(rateLimiter))
}

func (s *Server) setupRoutes() {
	// Repositories
	userRepo := queries.NewUserRepository(s.db.DB)
	clusterRepo := queries.NewClusterRepository(s.db.DB)
	metricsRepo := queries.NewMetricsRepository(s.db.DB)
	eventsRepo := queries.NewScalingEventRepository(s.db.DB)

	// Handlers
	healthHandler := handlers.NewHealthHandler(s.db)
	authHandler := handlers.NewAuthHandler(userRepo, s.authService)
	clusterHandler := handlers.NewClusterHandler(clusterRepo, s.clusterManager)
	metricsHandler := handlers.NewMetricsHandler(metricsRepo, eventsRepo)

	// Public routes
	s.router.GET("/health", healthHandler.Health)
	s.router.GET("/health/ready", healthHandler.Ready)
	s.router.GET("/health/live", healthHandler.Live)

	// Auth routes
	s.router.POST("/auth/login", authHandler.Login)

	// WebSocket route
	s.router.GET("/ws", websocket.ServeWebSocket(s.wsHub))

	// Protected routes
	protected := s.router.Group("/")
	protected.Use(middleware.JWTAuth(s.authService))
	{
		// Clusters
		protected.GET("/clusters", clusterHandler.List)
		protected.POST("/clusters", clusterHandler.Create)
		protected.GET("/clusters/:id", clusterHandler.Get)
		protected.PUT("/clusters/:id", clusterHandler.Update)
		protected.DELETE("/clusters/:id", clusterHandler.Delete)
		protected.GET("/clusters/:id/status", clusterHandler.GetStatus)

		// Metrics
		protected.GET("/clusters/:id/metrics", metricsHandler.GetMetrics)
		protected.GET("/clusters/:id/metrics/latest", metricsHandler.GetLatestMetrics)
		protected.GET("/clusters/:id/metrics/hourly", metricsHandler.GetHourlyMetrics)

		// Scaling Events
		protected.GET("/clusters/:id/events", metricsHandler.GetScalingEvents)
		protected.GET("/clusters/:id/events/stats", metricsHandler.GetScalingStats)
		protected.GET("/events/recent", metricsHandler.GetRecentEvents)
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  60 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the event bridge first
	if s.wsBridge != nil {
		s.wsBridge.Stop()
	}

	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Router() *gin.Engine {
	return s.router
}

func (s *Server) WebSocketHub() *websocket.Hub {
	return s.wsHub
}