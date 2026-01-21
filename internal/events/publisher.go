package events

import (
	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type Publisher struct {
	bus     *EventBus
	traceID string
}

func NewPublisher(bus *EventBus) *Publisher {
	return &Publisher{bus: bus}
}

func (p *Publisher) WithTraceID(traceID string) *Publisher {
	return &Publisher{
		bus:     p.bus,
		traceID: traceID,
	}
}

func (p *Publisher) publish(event *models.Event) {
	if p.traceID != "" {
		event.TraceID = p.traceID
	}
	p.bus.Publish(event)
}

func (p *Publisher) MetricCollected(clusterID string, metrics *models.ClusterMetrics) {
	event := models.NewEvent(models.EventTypeMetricCollected, clusterID, "Metrics collected").
		WithData(metrics)
	p.publish(event)
}

func (p *Publisher) MetricAnalyzed(clusterID string, analyzed *models.AnalyzedMetrics) {
	event := models.NewEvent(models.EventTypeMetricAnalyzed, clusterID, "Metrics analyzed").
		WithData(analyzed)
	
	if analyzed.IsCritical() {
		event.WithSeverity(models.SeverityCritical)
	} else if analyzed.IsWarning() {
		event.WithSeverity(models.SeverityWarning)
	}
	
	p.publish(event)
}

func (p *Publisher) DecisionMade(clusterID string, decision *models.ScalingDecision) {
	msg := "Scaling decision:  " + string(decision.Action)
	event := models.NewEvent(models.EventTypeDecisionMade, clusterID, msg).
		WithData(decision)
	
	if decision.IsEmergency {
		event.WithSeverity(models.SeverityCritical)
	}
	
	p.publish(event)
}

func (p *Publisher) ScalingStarted(clusterID string, decision *models.ScalingDecision) {
	msg := "Scaling started: " + string(decision.Action)
	event := models.NewEvent(models.EventTypeScalingStarted, clusterID, msg).
		WithData(decision)
	p.publish(event)
}

func (p *Publisher) ScalingComplete(clusterID string, scalingEvent *models.ScalingEvent) {
	msg := "Scaling complete: " + string(scalingEvent.Action)
	event := models.NewEvent(models.EventTypeScalingComplete, clusterID, msg).
		WithData(scalingEvent)
	p.publish(event)
}

func (p *Publisher) ScalingFailed(clusterID string, reason string, err error) {
	msg := "Scaling failed: " + reason
	event := models.NewEvent(models.EventTypeScalingFailed, clusterID, msg).
		WithSeverity(models.SeverityCritical).
		WithData(map[string]interface{}{
			"reason": reason,
			"error":   err.Error(),
		})
	p.publish(event)
}

func (p *Publisher) ServerAdded(server *models.Server) {
	event := models.NewEvent(models.EventTypeServerAdded, server.ClusterID, "Server added").
		WithData(server)
	p.publish(event)
}

func (p *Publisher) ServerRemoved(server *models.Server) {
	event := models.NewEvent(models.EventTypeServerRemoved, server.ClusterID, "Server removed").
		WithData(server)
	p.publish(event)
}

func (p *Publisher) ServerActivated(server *models.Server) {
	event := models.NewEvent(models.EventTypeServerActivated, server.ClusterID, "Server activated").
		WithData(server)
	p.publish(event)
}

func (p *Publisher) Alert(clusterID string, severity models.EventSeverity, message string, data interface{}) {
	event := models.NewEvent(models.EventTypeAlert, clusterID, message).
		WithSeverity(severity).
		WithData(data)
	p.publish(event)
}

func (p *Publisher) Error(clusterID string, message string, err error) {
	event := models.NewEvent(models.EventTypeError, clusterID, message).
		WithSeverity(models.SeverityCritical).
		WithData(map[string]interface{}{
			"error": err.Error(),
		})
	p.publish(event)
}