package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/OldStager01/cloud-autoscaler/api/handlers"
	"github.com/OldStager01/cloud-autoscaler/api/middleware"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/gin-gonic/gin"
)

type Server struct {
	router     *gin.Engine
	httpServer *http.Server
	config     config.APIConfig
	db         *database.DB
}

func NewServer(cfg config.APIConfig, db *database.DB) *Server {
	if cfg.JWTSecret == "" || cfg.JWTSecret == "change-me-in-production" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	s := &Server{
		router:  router,
		config: cfg,
		db:     db,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

func (s *Server) setupMiddleware() {
	s.router.Use(gin.Recovery())
	s.router.Use(middleware.RequestLogger())
	s.router.Use(middleware.TraceID())
}

func (s *Server) setupRoutes() {
	healthHandler := handlers.NewHealthHandler(s.db)

	s.router.GET("/health", healthHandler.Health)
	s.router.GET("/health/ready", healthHandler.Ready)
	s.router.GET("/health/live", healthHandler.Live)
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:   60 * time.Second,
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