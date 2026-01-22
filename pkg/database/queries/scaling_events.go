package queries

import (
	"context"
	"database/sql"
	"time"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

type ScalingEventRepository struct {
	db *sql.DB
}

func NewScalingEventRepository(db *sql.DB) *ScalingEventRepository {
	return &ScalingEventRepository{db: db}
}

type ScalingEventRecord struct {
	ID             int        `json:"id"`
	ClusterID      string     `json:"cluster_id"`
	Timestamp      time.Time  `json:"timestamp"`
	Action         string     `json:"action"`
	ServersBefore  int        `json:"servers_before"`
	ServersAfter   int        `json:"servers_after"`
	TriggerReason  string     `json:"trigger_reason"`
	PredictionUsed bool       `json:"prediction_used"`
	Confidence     *float64   `json:"confidence,omitempty"`
	Status         string     `json:"status"`
}

func (r *ScalingEventRepository) GetByCluster(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]ScalingEventRecord, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, cluster_id, timestamp, action, servers_before, servers_after, 
			   trigger_reason, prediction_used, confidence, status
		FROM scaling_events
		WHERE cluster_id = $1 AND timestamp >= $2 AND timestamp <= $3
		ORDER BY timestamp DESC
		LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, clusterID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ScalingEventRecord
	for rows.Next() {
		var e ScalingEventRecord
		err := rows.Scan(
			&e.ID, &e.ClusterID, &e.Timestamp, &e.Action,
			&e.ServersBefore, &e.ServersAfter, &e.TriggerReason,
			&e.PredictionUsed, &e.Confidence, &e.Status,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (r *ScalingEventRepository) GetRecent(ctx context.Context, limit int) ([]ScalingEventRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, cluster_id, timestamp, action, servers_before, servers_after, 
			   trigger_reason, prediction_used, confidence, status
		FROM scaling_events
		ORDER BY timestamp DESC
		LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ScalingEventRecord
	for rows.Next() {
		var e ScalingEventRecord
		err := rows.Scan(
			&e.ID, &e.ClusterID, &e.Timestamp, &e.Action,
			&e.ServersBefore, &e.ServersAfter, &e.TriggerReason,
			&e.PredictionUsed, &e.Confidence, &e.Status,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}

	return events, rows.Err()
}

func (r *ScalingEventRepository) GetStats(ctx context.Context, clusterID string, from, to time.Time) (*ScalingStats, error) {
	query := `
		SELECT 
			COUNT(*) FILTER (WHERE action = 'SCALE_UP') AS scale_up_count,
			COUNT(*) FILTER (WHERE action = 'SCALE_DOWN') AS scale_down_count,
			COUNT(*) FILTER (WHERE status = 'success') AS success_count,
			COUNT(*) FILTER (WHERE status = 'failed') AS failed_count,
			COUNT(*) FILTER (WHERE prediction_used = true) AS prediction_count
		FROM scaling_events
		WHERE cluster_id = $1 AND timestamp >= $2 AND timestamp <= $3`

	var stats ScalingStats
	err := r.db.QueryRowContext(ctx, query, clusterID, from, to).Scan(
		&stats.ScaleUpCount, &stats.ScaleDownCount,
		&stats.SuccessCount, &stats.FailedCount, &stats.PredictionCount,
	)

	if err != nil {
		return nil, err
	}

	stats.ClusterID = clusterID
	stats.From = from
	stats.To = to

	return &stats, nil
}

func (r *ScalingEventRepository) Insert(ctx context.Context, event *models.ScalingEvent) error {
	query := `
		INSERT INTO scaling_events 
			(cluster_id, timestamp, action, servers_before, servers_after, 
			 trigger_reason, prediction_used, confidence, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	return r.db.QueryRowContext(ctx, query,
		event.ClusterID,
		event.Timestamp,
		event.Action,
		event.ServersBefore,
		event.ServersAfter,
		event.TriggerReason,
		event.PredictionUsed,
		event.Confidence,
		event.Status,
	).Scan(&event.ID)
}

type ScalingStats struct {
	ClusterID       string    `json:"cluster_id"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	ScaleUpCount    int       `json:"scale_up_count"`
	ScaleDownCount  int       `json:"scale_down_count"`
	SuccessCount    int       `json:"success_count"`
	FailedCount     int       `json:"failed_count"`
	PredictionCount int       `json:"prediction_count"`
}