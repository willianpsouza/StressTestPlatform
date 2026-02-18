-- Performance indexes for Grafana dashboard queries
CREATE INDEX IF NOT EXISTS idx_k6_metrics_test_metric_ts ON k6_metrics(test_id, metric_name, timestamp);
CREATE INDEX IF NOT EXISTS idx_k6_metrics_exec_metric ON k6_metrics(execution_id, metric_name);
