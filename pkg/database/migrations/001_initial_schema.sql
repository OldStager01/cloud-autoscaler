-- 001_initial_schema.sql
-- Core tables for users, clusters, and servers

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Users table for authentication
CREATE TABLE IF NOT EXISTS users (
    id            SERIAL PRIMARY KEY,
    username      VARCHAR(50) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- Clusters configuration
CREATE TABLE IF NOT EXISTS clusters (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(100) NOT NULL,
    min_servers  INT NOT NULL DEFAULT 2,
    max_servers  INT NOT NULL DEFAULT 50,
    status       VARCHAR(20) NOT NULL DEFAULT 'active',
    config       JSONB,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT clusters_name_unique UNIQUE (name),
    CONSTRAINT clusters_status_check CHECK (status IN ('active', 'paused', 'error')),
    CONSTRAINT clusters_servers_check CHECK (min_servers > 0 AND max_servers >= min_servers)
);

-- Servers in each cluster
CREATE TABLE IF NOT EXISTS servers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cluster_id    UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    state         VARCHAR(20) NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    activated_at  TIMESTAMPTZ,
    terminated_at TIMESTAMPTZ,
    
    CONSTRAINT servers_state_check CHECK (state IN ('PROVISIONING', 'ACTIVE', 'DRAINING', 'TERMINATED'))
);

CREATE INDEX idx_servers_cluster_id ON servers(cluster_id);
CREATE INDEX idx_servers_cluster_state ON servers(cluster_id, state);
CREATE INDEX idx_servers_state ON servers(state);

-- Scaling events log
CREATE TABLE IF NOT EXISTS scaling_events (
    id              SERIAL PRIMARY KEY,
    cluster_id      UUID NOT NULL REFERENCES clusters(id) ON DELETE CASCADE,
    timestamp       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action          VARCHAR(20) NOT NULL,
    servers_before  INT NOT NULL,
    servers_after   INT NOT NULL,
    trigger_reason  TEXT,
    prediction_used BOOLEAN DEFAULT FALSE,
    confidence      FLOAT,
    status          VARCHAR(20) DEFAULT 'success',
    
    CONSTRAINT scaling_events_action_check CHECK (action IN ('SCALE_UP', 'SCALE_DOWN', 'MAINTAIN')),
    CONSTRAINT scaling_events_status_check CHECK (status IN ('success', 'failed', 'partial'))
);

CREATE INDEX idx_scaling_events_cluster_id ON scaling_events(cluster_id);
CREATE INDEX idx_scaling_events_timestamp ON scaling_events(timestamp DESC);
CREATE INDEX idx_scaling_events_cluster_time ON scaling_events(cluster_id, timestamp DESC);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Trigger for clusters updated_at
CREATE TRIGGER update_clusters_updated_at
    BEFORE UPDATE ON clusters
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();