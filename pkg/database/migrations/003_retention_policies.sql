-- 003_retention_policies.sql
-- Data retention and compression policies

-- Retention policy for metrics_history (30 days)
SELECT add_retention_policy('metrics_history', INTERVAL '30 days', if_not_exists => TRUE);

-- Retention policy for predictions (90 days)
SELECT add_retention_policy('predictions', INTERVAL '90 days', if_not_exists => TRUE);

-- Enable compression on metrics_history
ALTER TABLE metrics_history SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'cluster_id'
);

-- Compression policy (compress data older than 7 days)
SELECT add_compression_policy('metrics_history', INTERVAL '7 days', if_not_exists => TRUE);

-- Continuous aggregate for hourly metrics
CREATE MATERIALIZED VIEW IF NOT EXISTS metrics_hourly
WITH (timescaledb.continuous) AS
SELECT 
    time_bucket('1 hour', time) AS hour,
    cluster_id,
    AVG(cpu_usage) AS avg_cpu,
    AVG(memory_usage) AS avg_memory,
    AVG(request_load) AS avg_load,
    MAX(cpu_usage) AS max_cpu,
    MIN(cpu_usage) AS min_cpu,
    COUNT(*) AS sample_count
FROM metrics_history
GROUP BY hour, cluster_id
WITH NO DATA;

-- Refresh policy for continuous aggregate
SELECT add_continuous_aggregate_policy('metrics_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '30 minutes',
    if_not_exists => TRUE);