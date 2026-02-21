-- Aggregated metrics table: pre-computed from raw k6_metrics at end of each test execution.
-- Row types:
--   is_summary=FALSE, bucket_time NOT NULL  -> per-second time buckets (for timeseries)
--   is_summary=TRUE,  url IS NULL           -> global summary per metric per execution (for stat cards)
--   is_summary=TRUE,  url IS NOT NULL       -> per-endpoint summary (for HTTP tables)

CREATE TABLE k6_metrics_aggregated (
    id            BIGSERIAL PRIMARY KEY,
    execution_id  UUID NOT NULL REFERENCES test_executions(id) ON DELETE CASCADE,
    test_id       UUID NOT NULL REFERENCES tests(id) ON DELETE CASCADE,
    bucket_time   TIMESTAMPTZ,
    metric_name   VARCHAR(100) NOT NULL,
    url           VARCHAR(500),
    method        VARCHAR(20),
    status        VARCHAR(10),
    scenario      VARCHAR(100),
    count         BIGINT NOT NULL DEFAULT 0,
    sum_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    avg_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    min_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_value     DOUBLE PRECISION NOT NULL DEFAULT 0,
    p50           DOUBLE PRECISION,
    p90           DOUBLE PRECISION,
    p95           DOUBLE PRECISION,
    p99           DOUBLE PRECISION,
    is_summary    BOOLEAN NOT NULL DEFAULT FALSE
);

-- Timeseries queries: WHERE test_id AND metric_name AND bucket_time range
CREATE INDEX idx_k6ma_ts ON k6_metrics_aggregated(test_id, metric_name, bucket_time)
  WHERE is_summary = FALSE;

-- Summary per execution (execution stats, stat cards)
CREATE INDEX idx_k6ma_exec_summary ON k6_metrics_aggregated(execution_id, metric_name)
  WHERE is_summary = TRUE;

-- Table endpoints: breakdown by endpoint
CREATE INDEX idx_k6ma_endpoint ON k6_metrics_aggregated(test_id, metric_name, url, method, status)
  WHERE is_summary = TRUE AND url IS NOT NULL;

-- Dashboard overview: global summary
CREATE INDEX idx_k6ma_global_summary ON k6_metrics_aggregated(metric_name)
  WHERE is_summary = TRUE AND url IS NULL;

-- FK lookups
CREATE INDEX idx_k6ma_exec_id ON k6_metrics_aggregated(execution_id);
CREATE INDEX idx_k6ma_test_id ON k6_metrics_aggregated(test_id);

-- ---------------------------------------------------------------------------
-- SP 2: cleanup raw metrics (called by sp_aggregate)
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sp_cleanup_raw_metrics(p_execution_id UUID)
RETURNS VOID AS $$
BEGIN
    DELETE FROM k6_metrics WHERE execution_id = p_execution_id;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------------
-- SP 1: aggregate raw metrics into k6_metrics_aggregated, then cleanup
-- ---------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION sp_aggregate_execution_metrics(p_execution_id UUID)
RETURNS VOID AS $$
DECLARE
    v_test_id UUID;
BEGIN
    -- 1. Get test_id from raw data
    SELECT test_id INTO v_test_id
    FROM k6_metrics
    WHERE execution_id = p_execution_id
    LIMIT 1;

    IF v_test_id IS NULL THEN
        RETURN; -- no raw data to aggregate
    END IF;

    -- 2. Delete existing aggregated data for idempotency
    DELETE FROM k6_metrics_aggregated WHERE execution_id = p_execution_id;

    -- 3. Insert per-second bucket rows (for timeseries)
    INSERT INTO k6_metrics_aggregated (
        execution_id, test_id, bucket_time, metric_name,
        url, method, status, scenario,
        count, sum_value, avg_value, min_value, max_value,
        p50, p90, p95, p99, is_summary
    )
    SELECT
        p_execution_id,
        v_test_id,
        date_trunc('second', timestamp) AS bucket,
        metric_name,
        url, method, status, scenario,
        COUNT(*)::BIGINT,
        SUM(metric_value),
        AVG(metric_value),
        MIN(metric_value),
        MAX(metric_value),
        PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY metric_value),
        FALSE
    FROM k6_metrics
    WHERE execution_id = p_execution_id
    GROUP BY date_trunc('second', timestamp), metric_name, url, method, status, scenario;

    -- 4. Insert global summary rows (one per metric_name, no endpoint dimensions)
    INSERT INTO k6_metrics_aggregated (
        execution_id, test_id, bucket_time, metric_name,
        url, method, status, scenario,
        count, sum_value, avg_value, min_value, max_value,
        p50, p90, p95, p99, is_summary
    )
    SELECT
        p_execution_id,
        v_test_id,
        NULL,
        metric_name,
        NULL, NULL, NULL, NULL,
        COUNT(*)::BIGINT,
        SUM(metric_value),
        AVG(metric_value),
        MIN(metric_value),
        MAX(metric_value),
        PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY metric_value),
        TRUE
    FROM k6_metrics
    WHERE execution_id = p_execution_id
    GROUP BY metric_name;

    -- 5. Insert per-endpoint summary rows (for HTTP tables)
    INSERT INTO k6_metrics_aggregated (
        execution_id, test_id, bucket_time, metric_name,
        url, method, status, scenario,
        count, sum_value, avg_value, min_value, max_value,
        p50, p90, p95, p99, is_summary
    )
    SELECT
        p_execution_id,
        v_test_id,
        NULL,
        metric_name,
        url, method, status, NULL,
        COUNT(*)::BIGINT,
        SUM(metric_value),
        AVG(metric_value),
        MIN(metric_value),
        MAX(metric_value),
        PERCENTILE_CONT(0.50) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY metric_value),
        PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY metric_value),
        TRUE
    FROM k6_metrics
    WHERE execution_id = p_execution_id
      AND url IS NOT NULL
    GROUP BY metric_name, url, method, status;

    -- 6. Cleanup raw metrics
    PERFORM sp_cleanup_raw_metrics(p_execution_id);
END;
$$ LANGUAGE plpgsql;
