package queries

import (
	"context"
	"database/sql"
	"time"
)

type MetricsRepository struct {
	db *sql.DB
}

func NewMetricsRepository(db *sql.DB) *MetricsRepository {
	return &MetricsRepository{db: db}
}

type MetricPoint struct {
	Time        time.Time `json:"time"`
	ClusterID   string    `json:"cluster_id"`
	ServerID    *string   `json:"server_id,omitempty"`
	CPUUsage    float64   `json:"cpu_usage"`
	MemoryUsage float64   `json:"memory_usage"`
	RequestLoad int       `json:"request_load"`
}

type AggregatedMetricPoint struct {
	Time        time.Time `json:"time"`
	ClusterID   string    `json:"cluster_id"`
	AvgCPU      float64   `json:"avg_cpu"`
	AvgMemory   float64   `json:"avg_memory"`
	AvgLoad     float64   `json:"avg_load"`
	MaxCPU      float64   `json:"max_cpu"`
	MinCPU      float64   `json:"min_cpu"`
	SampleCount int       `json:"sample_count"`
}

func (r *MetricsRepository) GetRaw(ctx context.Context, clusterID string, from, to time.Time, limit int) ([]MetricPoint, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `
		SELECT time, cluster_id, server_id, cpu_usage, memory_usage, request_load
		FROM metrics_history
		WHERE cluster_id = $1 AND time >= $2 AND time <= $3
		ORDER BY time DESC
		LIMIT $4`

	rows, err := r.db.QueryContext(ctx, query, clusterID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []MetricPoint
	for rows.Next() {
		var m MetricPoint
		err := rows.Scan(&m.Time, &m.ClusterID, &m.ServerID, &m.CPUUsage, &m.MemoryUsage, &m.RequestLoad)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) GetAggregated(ctx context.Context, clusterID string, from, to time.Time, bucketMinutes int) ([]AggregatedMetricPoint, error) {
	if bucketMinutes <= 0 {
		bucketMinutes = 5
	}

	query := `
		SELECT 
			time_bucket($4, time) AS bucket,
			cluster_id,
			AVG(cpu_usage) AS avg_cpu,
			AVG(memory_usage) AS avg_memory,
			AVG(request_load) AS avg_load,
			MAX(cpu_usage) AS max_cpu,
			MIN(cpu_usage) AS min_cpu,
			COUNT(*) AS sample_count
		FROM metrics_history
		WHERE cluster_id = $1 AND time >= $2 AND time <= $3
		GROUP BY bucket, cluster_id
		ORDER BY bucket DESC`

	interval := time.Duration(bucketMinutes) * time.Minute
	rows, err := r.db.QueryContext(ctx, query, clusterID, from, to, interval)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []AggregatedMetricPoint
	for rows.Next() {
		var m AggregatedMetricPoint
		err := rows.Scan(&m.Time, &m.ClusterID, &m.AvgCPU, &m.AvgMemory, &m.AvgLoad, &m.MaxCPU, &m.MinCPU, &m.SampleCount)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) GetHourly(ctx context.Context, clusterID string, from, to time.Time) ([]AggregatedMetricPoint, error) {
	query := `
		SELECT 
			hour AS time,
			cluster_id,
			avg_cpu,
			avg_memory,
			avg_load,
			max_cpu,
			min_cpu,
			sample_count
		FROM metrics_hourly
		WHERE cluster_id = $1 AND hour >= $2 AND hour <= $3
		ORDER BY hour DESC`

	rows, err := r.db.QueryContext(ctx, query, clusterID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []AggregatedMetricPoint
	for rows.Next() {
		var m AggregatedMetricPoint
		err := rows.Scan(&m.Time, &m.ClusterID, &m.AvgCPU, &m.AvgMemory, &m.AvgLoad, &m.MaxCPU, &m.MinCPU, &m.SampleCount)
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

func (r *MetricsRepository) GetLatest(ctx context.Context, clusterID string) (*AggregatedMetricPoint, error) {
	query := `
		SELECT 
			MAX(time) AS time,
			cluster_id,
			AVG(cpu_usage) AS avg_cpu,
			AVG(memory_usage) AS avg_memory,
			AVG(request_load) AS avg_load,
			MAX(cpu_usage) AS max_cpu,
			MIN(cpu_usage) AS min_cpu,
			COUNT(*) AS sample_count
		FROM metrics_history
		WHERE cluster_id = $1 AND time > NOW() - INTERVAL '1 minute'
		GROUP BY cluster_id`

	var m AggregatedMetricPoint
	err := r.db.QueryRowContext(ctx, query, clusterID).Scan(
		&m.Time, &m.ClusterID, &m.AvgCPU, &m.AvgMemory, &m.AvgLoad, &m.MaxCPU, &m.MinCPU, &m.SampleCount,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (r *MetricsRepository) Insert(ctx context.Context, m *MetricPoint) error {
	query := `
		INSERT INTO metrics_history (time, cluster_id, server_id, cpu_usage, memory_usage, request_load)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := r.db.ExecContext(ctx, query, m.Time, m.ClusterID, m.ServerID, m.CPUUsage, m.MemoryUsage, m.RequestLoad)
	return err
}

func (r *MetricsRepository) InsertBatch(ctx context.Context, metrics []MetricPoint) error {
	if len(metrics) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics_history (time, cluster_id, server_id, cpu_usage, memory_usage, request_load)
		VALUES ($1, $2, $3, $4, $5, $6)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, m := range metrics {
		_, err := stmt.ExecContext(ctx, m.Time, m.ClusterID, m.ServerID, m.CPUUsage, m.MemoryUsage, m.RequestLoad)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}