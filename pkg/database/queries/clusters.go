package queries

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/OldStager01/cloud-autoscaler/pkg/models"
)

var ErrClusterNotFound = errors.New("cluster not found")

type ClusterRepository struct {
	db *sql.DB
}

func NewClusterRepository(db *sql.DB) *ClusterRepository {
	return &ClusterRepository{db: db}
}

func (r *ClusterRepository) GetAll(ctx context.Context) ([]*models.Cluster, error) {
	query := `
		SELECT id, name, min_servers, max_servers, status, config, created_at, updated_at 
		FROM clusters 
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clusters []*models.Cluster
	for rows.Next() {
		cluster, err := r.scanCluster(rows)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, cluster)
	}

	return clusters, rows.Err()
}

func (r *ClusterRepository) GetByID(ctx context.Context, id string) (*models.Cluster, error) {
	query := `
		SELECT id, name, min_servers, max_servers, status, config, created_at, updated_at 
		FROM clusters 
		WHERE id = $1`

	row := r.db.QueryRowContext(ctx, query, id)
	cluster, err := r.scanClusterRow(row)
	if err == sql.ErrNoRows {
		return nil, ErrClusterNotFound
	}
	return cluster, err
}

func (r *ClusterRepository) GetByName(ctx context.Context, name string) (*models.Cluster, error) {
	query := `
		SELECT id, name, min_servers, max_servers, status, config, created_at, updated_at 
		FROM clusters 
		WHERE name = $1`

	row := r.db.QueryRowContext(ctx, query, name)
	cluster, err := r.scanClusterRow(row)
	if err == sql.ErrNoRows {
		return nil, ErrClusterNotFound
	}
	return cluster, err
}

func (r *ClusterRepository) Create(ctx context.Context, cluster *models.Cluster) error {
	configJSON, err := json.Marshal(cluster.Config)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO clusters (id, name, min_servers, max_servers, status, config) 
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at`

	return r.db.QueryRowContext(ctx, query,
		cluster.ID,
		cluster.Name,
		cluster.MinServers,
		cluster.MaxServers,
		cluster.Status,
		configJSON,
	).Scan(&cluster.CreatedAt, &cluster.UpdatedAt)
}

func (r *ClusterRepository) Update(ctx context.Context, cluster *models.Cluster) error {
	configJSON, err := json.Marshal(cluster.Config)
	if err != nil {
		return err
	}

	query := `
		UPDATE clusters 
		SET name = $2, min_servers = $3, max_servers = $4, status = $5, config = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`

	result := r.db.QueryRowContext(ctx, query,
		cluster.ID,
		cluster.Name,
		cluster.MinServers,
		cluster.MaxServers,
		cluster.Status,
		configJSON,
	)

	return result.Scan(&cluster.UpdatedAt)
}

func (r *ClusterRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM clusters WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrClusterNotFound
	}

	return nil
}

func (r *ClusterRepository) GetActiveCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM clusters WHERE status = 'active'`
	var count int
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}

func (r *ClusterRepository) scanCluster(rows *sql.Rows) (*models.Cluster, error) {
	var cluster models.Cluster
	var configJSON []byte
	var status string

	err := rows.Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.MinServers,
		&cluster.MaxServers,
		&status,
		&configJSON,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	cluster.Status = models.ClusterStatus(status)
	if len(configJSON) > 0 {
		cluster.Config = &models.ClusterConfig{}
		json.Unmarshal(configJSON, cluster.Config)
	}

	return &cluster, nil
}

func (r *ClusterRepository) scanClusterRow(row *sql.Row) (*models.Cluster, error) {
	var cluster models.Cluster
	var configJSON []byte
	var status string

	err := row.Scan(
		&cluster.ID,
		&cluster.Name,
		&cluster.MinServers,
		&cluster.MaxServers,
		&status,
		&configJSON,
		&cluster.CreatedAt,
		&cluster.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	cluster.Status = models.ClusterStatus(status)
	if len(configJSON) > 0 {
		cluster.Config = &models.ClusterConfig{}
		json.Unmarshal(configJSON, cluster.Config)
	}

	return &cluster, nil
}

type ServerCount struct {
	ClusterID   string
	Total       int
	Active      int
	Provisioning int
	Draining    int
}

func (r *ClusterRepository) GetServerCounts(ctx context.Context, clusterID string) (*ServerCount, error) {
	query := `
		SELECT 
			cluster_id,
			COUNT(*) FILTER (WHERE state != 'TERMINATED') as total,
			COUNT(*) FILTER (WHERE state = 'ACTIVE') as active,
			COUNT(*) FILTER (WHERE state = 'PROVISIONING') as provisioning,
			COUNT(*) FILTER (WHERE state = 'DRAINING') as draining
		FROM servers 
		WHERE cluster_id = $1
		GROUP BY cluster_id`

	var sc ServerCount
	err := r.db.QueryRowContext(ctx, query, clusterID).Scan(
		&sc.ClusterID,
		&sc.Total,
		&sc.Active,
		&sc.Provisioning,
		&sc.Draining,
	)

	if err == sql.ErrNoRows {
		return &ServerCount{ClusterID: clusterID}, nil
	}

	return &sc, err
}