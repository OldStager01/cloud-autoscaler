package orchestrator

import (
	"context"
	"sync"
	"time"

	"github.com/OldStager01/cloud-autoscaler/internal/analyzer"
	"github.com/OldStager01/cloud-autoscaler/internal/collector"
	"github.com/OldStager01/cloud-autoscaler/internal/decision"
	"github.com/OldStager01/cloud-autoscaler/internal/events"
	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/internal/scaler"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type PipelineConfig struct {
	ClusterID        string
	CollectInterval  time.Duration
	Collector        collector.Collector
	Analyzer         *analyzer.Analyzer
	SustainedTracker *analyzer.SustainedTracker
	DecisionEngine   *decision.Engine
	Scaler           scaler.Scaler
	EventPublisher   *events.Publisher
	AnalyzerConfig   analyzer.Config
}

type Pipeline struct {
	config   PipelineConfig
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
	mu       sync.Mutex
}

func NewPipeline(cfg PipelineConfig) *Pipeline {
	if cfg.CollectInterval == 0 {
		cfg.CollectInterval = 10 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Pipeline{
		config:  cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (p *Pipeline) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return nil
	}

	p.running = true
	p.wg.Add(1)
	go p.run()

	logger.WithCluster(p.config.ClusterID).Info("Pipeline started")
	return nil
}

func (p *Pipeline) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	p.cancel()
	p.wg.Wait()

	logger.WithCluster(p.config.ClusterID).Info("Pipeline stopped")
}

func (p *Pipeline) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Pipeline) run() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.config.CollectInterval)
	defer ticker.Stop()

	// Run immediately on start
	p.runCycle()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C: 
			p.runCycle()
		}
	}
}

func (p *Pipeline) runCycle() {
	ctx, cancel := context.WithTimeout(p.ctx, p.config.CollectInterval-time.Second)
	defer cancel()

	clusterID := p.config.ClusterID

	// Step 1: Collect metrics
	metrics, err := p.collect(ctx)
	if err != nil {
		logger.WithCluster(clusterID).Errorf("Collection failed: %v", err)
		p.config.EventPublisher.Error(clusterID, "Metric collection failed", err)
		return
	}

	// Step 2: Analyze metrics
	analyzed := p.analyze(metrics)

	// Step 3: Get current cluster state
	state, err := p.config.Scaler.GetClusterState(ctx, clusterID)
	if err != nil {
		logger.WithCluster(clusterID).Errorf("Failed to get cluster state: %v", err)
		p.config.EventPublisher.Error(clusterID, "Failed to get cluster state", err)
		return
	}

	// Step 4: Make scaling decision
	scalingDecision := p.decide(analyzed, state)

	// Step 5: Execute scaling if needed
	if scalingDecision.ShouldExecute() {
		p.execute(ctx, scalingDecision)
	}
}

func (p *Pipeline) collect(ctx context.Context) (*models.ClusterMetrics, error) {
	metrics, err := p.config.Collector.Collect(ctx, p.config.ClusterID)
	if err != nil {
		return nil, err
	}

	p.config.EventPublisher.MetricCollected(p.config.ClusterID, metrics)
	return metrics, nil
}

func (p *Pipeline) analyze(metrics *models.ClusterMetrics) *models.AnalyzedMetrics {
	analyzed := p.config.Analyzer.Analyze(metrics)
	p.config.SustainedTracker.Update(p.config.ClusterID, analyzed, p.config.AnalyzerConfig)
	p.config.EventPublisher.MetricAnalyzed(p.config.ClusterID, analyzed)

	// Check for alert conditions
	if analyzed.IsCritical() {
		p.config.EventPublisher.Alert(
			p.config.ClusterID,
			models.SeverityCritical,
			"CPU or Memory critical",
			analyzed,
		)
	}

	return analyzed
}

func (p *Pipeline) decide(analyzed *models.AnalyzedMetrics, state *models.ClusterState) *models.ScalingDecision {
	// TODO: Add prediction support later
	scalingDecision := p.config.DecisionEngine.Decide(analyzed, nil, state)
	p.config.EventPublisher.DecisionMade(p.config.ClusterID, scalingDecision)
	return scalingDecision
}

func (p *Pipeline) execute(ctx context.Context, scalingDecision *models.ScalingDecision) {
	clusterID := p.config.ClusterID
	p.config.EventPublisher.ScalingStarted(clusterID, scalingDecision)

	var result *scaler.ScaleResult
	var err error

	switch scalingDecision.Action {
	case models.ActionScaleUp:
		delta := scalingDecision.TargetServers - scalingDecision.CurrentServers
		result, err = p.config.Scaler.ScaleUp(ctx, clusterID, delta)

	case models.ActionScaleDown:
		delta := scalingDecision.CurrentServers - scalingDecision.TargetServers
		result, err = p.config.Scaler.ScaleDown(ctx, clusterID, delta)
	}

	if err != nil {
		p.config.EventPublisher.ScalingFailed(clusterID, scalingDecision.Reason, err)
		return
	}

	// Record scaling for cooldown
	p.config.DecisionEngine.RecordScaling(clusterID)

	// Create scaling event record
	status := models.ScalingEventSuccess
	if result.PartialSuccess {
		status = models.ScalingEventPartial
	}

	scalingEvent := models.NewScalingEvent(*scalingDecision, status)
	p.config.EventPublisher.ScalingComplete(clusterID, scalingEvent)

	logger.WithCluster(clusterID).Infof(
		"Scaling complete: %s %d -> %d servers",
		scalingDecision.Action,
		scalingDecision.CurrentServers,
		scalingDecision.TargetServers,
	)
}