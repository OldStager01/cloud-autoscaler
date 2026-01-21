-- 002_timescale_hypertables.sql
-- TimescaleDB hypertables for time-series data

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Metrics history table
CREATE TABLE IF NOT EXISTS metrics_history (
    time         TIMESTAMPTZ NOT NULL,
    cluster_id   UUID NOT NULL,
    server_id    UUID,
    cpu_usage    FLOAT NOT NULL,
    memory_usage FLOAT NOT NULL,
    request_load INT NOT NULL,
    
    CONSTRAINT metrics_history_cpu_check CHECK (cpu_usage >= 0 AND cpu_usage <= 100),
    CONSTRAINT metrics_history_memory_check CHECK (memory_usage >= 0 AND memory_usage <= 100)
);

-- Convert to hypertable
SELECT create_hypertable('metrics_history', 'time', if_not_exists => TRUE);

-- Indexes for metrics queries
CREATE INDEX idx_metrics_cluster_time ON metrics_history(cluster_id, time DESC);
CREATE INDEX idx_metrics_time ON metrics_history(time DESC);

-- Predictions table
CREATE TABLE IF NOT EXISTS predictions (
    id            SERIAL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cluster_id    UUID NOT NULL,
    forecast_time TIMESTAMPTZ NOT NULL,
    predicted_cpu FLOAT NOT NULL,
    actual_cpu    FLOAT,
    confidence    FLOAT,
    model_version VARCHAR(20),
    
    CONSTRAINT predictions_cpu_check CHECK (predicted_cpu >= 0 AND predicted_cpu <= 100)
);

-- Convert to hypertable
SELECT create_hypertable('predictions', 'created_at', if_not_exists => TRUE);

-- Indexes for predictions
CREATE INDEX idx_predictions_cluster_time ON predictions(cluster_id, created_at DESC);
CREATE INDEX idx_predictions_forecast ON predictions(cluster_id, forecast_time);

-- Pattern history for ML (aggregated data)
CREATE TABLE IF NOT EXISTS pattern_history (
    cluster_id   UUID NOT NULL,
    day_of_week  INT NOT NULL,
    hour_of_day  INT NOT NULL,
    avg_cpu      FLOAT NOT NULL,
    avg_memory   FLOAT NOT NULL,
    avg_load     FLOAT NOT NULL,
    sample_count INT DEFAULT 0,
    last_updated TIMESTAMPTZ DEFAULT NOW(),
    
    PRIMARY KEY (cluster_id, day_of_week, hour_of_day),
    CONSTRAINT pattern_day_check CHECK (day_of_week >= 0 AND day_of_week <= 6),
    CONSTRAINT pattern_hour_check CHECK (hour_of_day >= 0 AND hour_of_day <= 23)
);