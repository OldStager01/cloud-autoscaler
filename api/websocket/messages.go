package websocket

import (
	"encoding/json"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type MessageType string

const (
	MessageTypeMetrics      MessageType = "metrics"
	MessageTypeScalingEvent MessageType = "scaling_event"
	MessageTypeAlert        MessageType = "alert"
	MessageTypeServerUpdate MessageType = "server_update"
	MessageTypeClusterState MessageType = "cluster_state"
)

type OutgoingMessage struct {
	Type      MessageType `json:"type"`
	ClusterID string      `json:"cluster_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

func NewMessage(msgType MessageType, clusterID string, data interface{}) *OutgoingMessage {
	return &OutgoingMessage{
		Type:       msgType,
		ClusterID: clusterID,
		Timestamp: time.Now(),
		Data:      data,
	}
}

func (m *OutgoingMessage) JSON() []byte {
	data, _ := json.Marshal(m)
	return data
}

type MetricsData struct {
	AvgCPU      float64 `json:"avg_cpu"`
	AvgMemory   float64 `json:"avg_memory"`
	ServerCount int     `json:"server_count"`
}

type ScalingEventData struct {
	Action        string  `json:"action"`
	ServersBefore int     `json:"servers_before"`
	ServersAfter  int     `json:"servers_after"`
	Reason        string  `json:"reason"`
	Status        string  `json:"status"`
}

type AlertData struct {
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

type ClusterStateData struct {
	TotalServers    int    `json:"total_servers"`
	ActiveServers   int    `json:"active_servers"`
	Provisioning    int    `json:"provisioning"`
	Draining        int    `json:"draining"`
	Status          string `json:"status"`
}

func BroadcastMetrics(hub *Hub, clusterID string, analyzed *models. AnalyzedMetrics) {
	data := MetricsData{
		AvgCPU:      analyzed.AvgCPU,
		AvgMemory:   analyzed.AvgMemory,
		ServerCount: analyzed.ServerCount,
	}
	msg := NewMessage(MessageTypeMetrics, clusterID, data)
	hub.BroadcastToCluster(clusterID, msg.JSON())
}

func BroadcastScalingEvent(hub *Hub, event *models.ScalingEvent) {
	data := ScalingEventData{
		Action:        string(event.Action),
		ServersBefore: event.ServersBefore,
		ServersAfter:   event.ServersAfter,
		Reason:        event.TriggerReason,
		Status:        string(event.Status),
	}
	msg := NewMessage(MessageTypeScalingEvent, event.ClusterID, data)
	hub.BroadcastToCluster(event.ClusterID, msg.JSON())
}

func BroadcastAlert(hub *Hub, clusterID string, severity, message string) {
	data := AlertData{
		Severity:  severity,
		Message:  message,
	}
	msg := NewMessage(MessageTypeAlert, clusterID, data)
	hub.BroadcastToCluster(clusterID, msg.JSON())
}

func BroadcastClusterState(hub *Hub, clusterID string, state *models. ClusterState) {
	data := ClusterStateData{
		TotalServers:  state.TotalServers,
		ActiveServers: state.ActiveServers,
		Provisioning:  state. ProvisioningCnt,
		Draining:      state.DrainingCount,
	}
	msg := NewMessage(MessageTypeClusterState, clusterID, data)
	hub.BroadcastToCluster(clusterID, msg.JSON())
}