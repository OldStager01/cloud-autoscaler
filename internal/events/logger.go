package events

import (
	"context"
	"encoding/json"

	"github.com/OldStager01/cloud-autoscaler/internal/logger"
	"github.com/OldStager01/cloud-autoscaler/pkg/database"
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type EventLogger struct {
	db        *database.DB
	eventChan <-chan *models.Event
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewEventLogger(db *database.DB, eventChan <-chan *models.Event) *EventLogger {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventLogger{
		db:         db,
		eventChan: eventChan,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (l *EventLogger) Start() {
	go l.run()
}

func (l *EventLogger) Stop() {
	l.cancel()
}

func (l *EventLogger) run() {
	for {
		select {
		case <-l.ctx.Done():
			return
		case event, ok := <-l.eventChan:
			if !ok {
				return
			}
			l.processEvent(event)
		}
	}
}

func (l *EventLogger) processEvent(event *models.Event) {
	// Log to structured logger
	entry := logger.WithFields(map[string]interface{}{
		"event_type":  event.Type,
		"cluster_id":  event.ClusterID,
		"severity":   event.Severity,
		"trace_id":   event.TraceID,
	})

	switch event.Severity {
	case models.SeverityCritical:
		entry.Error(event.Message)
	case models.SeverityWarning:
		entry.Warn(event.Message)
	default:
		entry.Info(event.Message)
	}

	// Persist specific events to database
	switch event.Type {
	case models.EventTypeScalingComplete:
		l.persistScalingEvent(event)
	case models.EventTypeMetricCollected:
		l.persistMetrics(event)
	}
}

func (l *EventLogger) persistScalingEvent(event *models.Event) {
	scalingEvent, ok := event.Data.(*models.ScalingEvent)
	if !ok {
		return
	}

	query := `
		INSERT INTO scaling_events 
			(cluster_id, timestamp, action, servers_before, servers_after, trigger_reason, prediction_used, confidence, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	var confidence *float64
	if scalingEvent.Confidence != nil {
		confidence = scalingEvent.Confidence
	}

	_, err := l.db.ExecContext(l.ctx, query,
		scalingEvent.ClusterID,
		scalingEvent.Timestamp,
		scalingEvent.Action,
		scalingEvent.ServersBefore,
		scalingEvent.ServersAfter,
		scalingEvent.TriggerReason,
		scalingEvent.PredictionUsed,
		confidence,
		scalingEvent.Status,
	)

	if err != nil {
		logger.Errorf("Failed to persist scaling event: %v", err)
	}
}

func (l *EventLogger) persistMetrics(event *models.Event) {
	metrics, ok := event.Data.(*models.ClusterMetrics)
	if !ok {
		return
	}

	query := `
		INSERT INTO metrics_history (time, cluster_id, server_id, cpu_usage, memory_usage, request_load)
		VALUES ($1, $2, $3, $4, $5, $6)`

	for _, server := range metrics.Servers {
		_, err := l.db.ExecContext(l.ctx, query,
			metrics.Timestamp,
			metrics.ClusterID,
			server.ServerID,
			server.CPUUsage,
			server.MemoryUsage,
			server.RequestLoad,
		)

		if err != nil {
			logger.Errorf("Failed to persist metrics:  %v", err)
		}
	}
}

func (l *EventLogger) LogToJSON(event *models.Event) string {
	data, _ := json.Marshal(event)
	return string(data)
}