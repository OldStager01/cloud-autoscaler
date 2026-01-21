package models

import "time"

type ServerState string

const (
	ServerStateProvisioning ServerState = "PROVISIONING"
	ServerStateActive       ServerState = "ACTIVE"
	ServerStateDraining     ServerState = "DRAINING"
	ServerStateTerminated   ServerState = "TERMINATED"
)

type Server struct {
	ID           string      `json:"id"`
	ClusterID    string      `json:"cluster_id"`
	State        ServerState `json:"state"`
	CreatedAt    time.Time   `json:"created_at"`
	ActivatedAt  *time.Time  `json:"activated_at,omitempty"`
	TerminatedAt *time.Time  `json:"terminated_at,omitempty"`
}

func NewServer(clusterID string) *Server {
	return &Server{
		ID:        NewUUID(),
		ClusterID: clusterID,
		State:     ServerStateProvisioning,
		CreatedAt: time.Now(),
	}
}

func (s *Server) Activate() {
	now := time.Now()
	s.State = ServerStateActive
	s.ActivatedAt = &now
}

func (s *Server) Drain() {
	s.State = ServerStateDraining
}

func (s *Server) Terminate() {
	now := time.Now()
	s.State = ServerStateTerminated
	s.TerminatedAt = &now
}

func (s *Server) IsActive() bool {
	return s.State == ServerStateActive
}

func (s *Server) IsRunning() bool {
	return s.State == ServerStateProvisioning || s.State == ServerStateActive
}