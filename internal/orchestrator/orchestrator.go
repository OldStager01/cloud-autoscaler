package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/events"
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/resilience"
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
	wg             sync.WaitGroup
	analyzerConfig analyzer.Config
	decisionConfig decision.Config
	started        bool
}

func New(cfg *config.Config, db *database.DB) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	eventBus := events.NewEventBus(100)

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
		CooldownPeriod:          cfg.Decision.CooldownPeriod,
		ScaleDownCooldownPeriod: cfg.Decision.ScaleDownCooldownPeriod,
		SustainedHighDuration:   cfg.Decision.SustainedHighDuration,
		SustainedLowDuration:    cfg.Decision.SustainedLowDuration,
		EmergencyCPUThreshold:   cfg.Decision.EmergencyCPUThreshold,
		MinServers:              cfg.Decision.MinServers,
		MaxServers:              cfg.Decision.MaxServers,
		MaxScaleStep:            cfg.Decision.MaxScaleStep,
		CPUHighThreshold:        cfg.Analyzer.Thresholds.CPUHigh,
		CPULowThreshold:         cfg.Analyzer.Thresholds.CPULow,
	}

	return &Orchestrator{
		config:         cfg,
		db:              db,
		eventBus:       eventBus,
		eventLogger:    eventLogger,
		pipelines:      make(map[string]*Pipeline),
		ctx:            ctx,
		cancel:          cancel,
		analyzerConfig: analyzerCfg,
		decisionConfig: decisionCfg,
	}
}

func (o *Orchestrator) Start() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.started {
		return nil
	}

	logger.Info("Orchestrator starting")
	o.eventLogger.Start()
	o.started = true

	return nil
}

func (o *Orchestrator) Stop(ctx context.Context) error {
	o.mu.Lock()
	if !o.started {
		o.mu.Unlock()
		return nil
	}
	o.started = false
	o.mu.Unlock()

	logger.Info("Orchestrator stopping")

	// Stop all pipelines concurrently
	var wg sync.WaitGroup
	o.mu.RLock()
	for clusterID, pipeline := range o.pipelines {
		wg.Add(1)
		go func(id string, p *Pipeline) {
			defer wg.Done()
			logger.Infof("Stopping pipeline for cluster %s", id)
			p.Stop()
		}(clusterID, pipeline)
	}
	o.mu.RUnlock()

	// Wait for pipelines or context timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("All pipelines stopped")
	case <-ctx.Done():
		logger.Warn("Timeout waiting for pipelines to stop")
	}

	// Cancel main context
	o.cancel()

	// Wait for orchestrator goroutines
	o.wg.Wait()

	// Stop event logger
	o.eventLogger.Stop()

	// Close event bus
	o.eventBus.Close()

	logger.Info("Orchestrator stopped")
	return nil
}

func (o *Orchestrator) StartCluster(cluster *models.Cluster, coll collector.Collector, scal scaler.Scaler) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if _, exists := o.pipelines[cluster.ID]; exists {
		return fmt.Errorf("pipeline already exists for cluster %s", cluster.ID)
	}

	// Wrap collector with resilience
	resilientColl := collector.NewResilientCollector(collector.ResilientCollectorConfig{
		Collector:     coll,
		MaxFailures:   o.config.Collector.CircuitBreaker.MaxFailures,
		Timeout:       o.config.Collector.CircuitBreaker.Timeout,
		RetryAttempts: o.config.Collector.RetryAttempts,
		OnStateChange: func(name string, from, to resilience.State) {
			logger.WithCluster(cluster.ID).Warnf(
				"Circuit breaker %s:  %s -> %s", name, from, to,
			)
			if to == resilience.StateOpen {
				events.NewPublisher(o.eventBus).Alert(
					cluster.ID,
					models.SeverityWarning,
					"Circuit breaker opened for collector",
					map[string]interface{}{"from": from.String(), "to": to.String()},
				)
			}
		},
	})

	pipeline := NewPipeline(PipelineConfig{
		ClusterID:         cluster.ID,
		CollectInterval:  o.config.Collector.Interval,
		Collector:        resilientColl,
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

func (o *Orchestrator) StartAllClusters(clusters []*models.Cluster, collectorFactory func(string) collector.Collector, scalerFactory func(string) scaler.Scaler) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(clusters))

	for _, cluster := range clusters {
		if cluster.Status != models.ClusterStatusActive {
			continue
		}

		wg.Add(1)
		go func(c *models.Cluster) {
			defer wg.Done()
			coll := collectorFactory(c.ID)
			scal := scalerFactory(c.ID)
			if err := o.StartCluster(c, coll, scal); err != nil {
				errChan <- fmt.Errorf("cluster %s:  %w", c.ID, err)
			}
		}(cluster)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to start %d clusters: %v", len(errs), errs)
	}

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

func (o *Orchestrator) ClusterCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.pipelines)
}

func (o *Orchestrator) SubscribeEvents(eventType models.EventType) <-chan *models.Event {
	return o.eventBus.Subscribe(eventType)
}

func (o *Orchestrator) SubscribeAllEvents() <-chan *models.Event {
	return o.eventBus.SubscribeAll()
}

func (o *Orchestrator) WaitForShutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return o.Stop(ctx)
}