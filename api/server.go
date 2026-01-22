package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/api/handlers"
	"github.com/OldStager01/cloud-autoscaler/api/middleware"
	"github.com/OldStager01/cloud-autoscaler/internal/auth"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/OldStager01/cloud-autoscaler/pkg/database/queries"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router      *gin.Engine
	httpServer  *http.Server
	config      config.APIConfig
	db          *database.DB
	authService *auth.Service
}

func NewServer(cfg config.APIConfig, db *database.DB) *Server {
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	authService := auth.NewService(cfg.JWTSecret, 24*time.Hour)

	s := &Server{
		router:       router,
		config:      cfg,
		db:          db,
		authService: authService,
	}

	s.setupMiddleware()
	s.setupRoutes()

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

	// Handlers
	healthHandler := handlers.NewHealthHandler(s.db)
	authHandler := handlers.NewAuthHandler(userRepo, s.authService)
	clusterHandler := handlers.NewClusterHandler(clusterRepo)

	// Public routes
	s.router.GET("/health", healthHandler.Health)
	s.router.GET("/health/ready", healthHandler.Ready)
	s.router.GET("/health/live", healthHandler.Live)

	// Auth routes
	s.router.POST("/auth/login", authHandler.Login)

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
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Router() *gin.Engine {
	return s.router
}