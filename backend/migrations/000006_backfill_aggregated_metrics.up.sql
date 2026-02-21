-- Backfill: aggregate all existing raw metrics that haven't been aggregated yet.
DO $$
DECLARE
    r RECORD;
    total INT := 0;
BEGIN
    FOR r IN
        SELECT DISTINCT execution_id
        FROM k6_metrics
        WHERE execution_id NOT IN (
            SELECT DISTINCT execution_id FROM k6_metrics_aggregated
        )
    LOOP
        RAISE NOTICE 'Aggregating execution %', r.execution_id;
        PERFORM sp_aggregate_execution_metrics(r.execution_id);
        total := total + 1;
    END LOOP;
    RAISE NOTICE 'Done. Aggregated % executions.', total;
END $$;
