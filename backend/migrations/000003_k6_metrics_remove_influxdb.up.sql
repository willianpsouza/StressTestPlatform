-- K6 metrics table (replaces InfluxDB)
CREATE TABLE IF NOT EXISTS k6_metrics (
    id BIGSERIAL PRIMARY KEY,
    execution_id UUID NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    test_id UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    metric_name VARCHAR(100) NOT NULL,
    timestamp TIMESTAMPTZ NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    method VARCHAR(20),
    status VARCHAR(10),
    url VARCHAR(500),
    scenario VARCHAR(100)
);

CREATE INDEX idx_k6_metrics_exec ON k6_metrics(execution_id);
CREATE INDEX idx_k6_metrics_test ON k6_metrics(test_id);
CREATE INDEX idx_k6_metrics_ts ON k6_metrics(execution_id, metric_name, timestamp);

-- Drop influxdb_bucket column (no longer needed)
ALTER TABLE tests DROP COLUMN IF EXISTS influxdb_bucket;
