package orchestrator

import (
	"context"
	"fmt"
	"sync"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/events"
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/scaler"
	"github.com/OldStager01/cloud-autoscaler/pkg/config"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type Orchestrator struct {
	config         *config.Config
	db             *database.DB
	eventBus       *events.EventBus
	eventLogger    *events.EventLogger
	pipelines      map[string]*Pipeline
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	analyzerConfig analyzer.Config
	decisionConfig decision.Config
}

func New(cfg *config.Config, db *database.DB) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	eventBus := events.NewEventBus(100)

	// Subscribe event logger to all events
	allEvents := eventBus.SubscribeAll()
	eventLogger := events.NewEventLogger(db, allEvents)

	analyzerCfg := analyzer.Config{
		CPUHighThreshold:    cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:     cfg.Analyzer.Thresholds.CPULow,
		MemoryHighThreshold: cfg.Analyzer.Thresholds.MemoryHigh,
		TrendWindow:         cfg.Analyzer.TrendWindow,
		SpikeThreshold:      cfg.Analyzer.SpikeThreshold,
	}

	decisionCfg := decision.Config{
		CooldownPeriod:         cfg.Decision.CooldownPeriod,
		EmergencyCPUThreshold: cfg.Decision.EmergencyCPUThreshold,
		MinServers:            cfg.Decision.MinServers,
		MaxServers:             cfg.Decision.MaxServers,
		MaxScaleStep:          cfg.Decision.MaxScaleStep,
		CPUHighThreshold:      cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:       cfg.Analyzer.Thresholds.CPULow,
	}

	return &Orchestrator{
		config:         cfg,
		db:             db,
		eventBus:       eventBus,
		eventLogger:    eventLogger,
		pipelines:       make(map[string]*Pipeline),
		ctx:            ctx,
		cancel:         cancel,
		analyzerConfig: analyzerCfg,
		decisionConfig: decisionCfg,
	}
}

func (o *Orchestrator) Start() error {
	logger.Info("Orchestrator starting")
	o.eventLogger.Start()
	return nil
}

func (o *Orchestrator) Stop() {
	logger.Info("Orchestrator stopping")

	// Stop all pipelines
	o.mu.Lock()
	for clusterID, pipeline := range o.pipelines {
		logger.Infof("Stopping pipeline for cluster %s", clusterID)
		pipeline.Stop()
	}
	o.mu.Unlock()

	// Cancel context
	o.cancel()

	// Stop event logger
	o.eventLogger.Stop()

	// Close event bus
	o.eventBus.Close()

	logger.Info("Orchestrator stopped")
}

func (o *Orchestrator) StartCluster(cluster *models.Cluster, coll collector.Collector, scal scaler.Scaler) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.pipelines[cluster.ID]; exists {
		return fmt.Errorf("pipeline already exists for cluster %s", cluster.ID)
	}

	pipeline := NewPipeline(PipelineConfig{
		ClusterID:        cluster.ID,
		CollectInterval:  o.config.Collector.Interval,
		Collector:        coll,
		Analyzer:         analyzer.New(o.analyzerConfig),
		SustainedTracker: analyzer.NewSustainedTracker(),
		DecisionEngine:   decision.NewEngine(o.decisionConfig),
		Scaler:           scal,
		EventPublisher:   events.NewPublisher(o.eventBus),
		AnalyzerConfig:   o.analyzerConfig,
	})

	if err := pipeline.Start(); err != nil {
		return fmt.Errorf("failed to start pipeline:  %w", err)
	}

	o.pipelines[cluster.ID] = pipeline
	logger.WithCluster(cluster.ID).Info("Cluster pipeline started")

	return nil
}

func (o *Orchestrator) StopCluster(clusterID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	pipeline, exists := o.pipelines[clusterID]
	if ! exists {
		return fmt.Errorf("no pipeline found for cluster %s", clusterID)
	}

	pipeline.Stop()
	delete(o.pipelines, clusterID)
	logger.WithCluster(clusterID).Info("Cluster pipeline stopped")

	return nil
}

func (o *Orchestrator) GetClusterStatus(clusterID string) (bool, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	pipeline, exists := o.pipelines[clusterID]
	if !exists {
		return false, fmt.Errorf("no pipeline found for cluster %s", clusterID)
	}

	return pipeline.IsRunning(), nil
}

func (o *Orchestrator) ListRunningClusters() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	clusters := make([]string, 0, len(o.pipelines))
	for clusterID, pipeline := range o.pipelines {
		if pipeline.IsRunning() {
			clusters = append(clusters, clusterID)
		}
	}
	return clusters
}

func (o *Orchestrator) SubscribeEvents(eventType models.EventType) <-chan *models.Event {
	return o.eventBus.Subscribe(eventType)
}

func (o *Orchestrator) SubscribeAllEvents() <-chan *models.Event {
	return o.eventBus.SubscribeAll()
}