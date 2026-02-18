package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type MetricRepository struct {
	pool *pgxpool.Pool
}

func NewMetricRepository(pool *pgxpool.Pool) *MetricRepository {
	return &MetricRepository{pool: pool}
}

func (r *MetricRepository) BulkInsert(metrics []domain.K6Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	// Batch insert with VALUES list
	const batchSize = 500
	for i := 0; i < len(metrics); i += batchSize {
		end := i + batchSize
		if end > len(metrics) {
			end = len(metrics)
		}
		batch := metrics[i:end]

		values := make([]string, 0, len(batch))
		args := make([]interface{}, 0, len(batch)*8)
		argIdx := 1

		for _, m := range batch {
			values = append(values, fmt.Sprintf(
				"($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)",
				argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4,
				argIdx+5, argIdx+6, argIdx+7, argIdx+8,
			))
			args = append(args, m.ExecutionID, m.TestID, m.MetricName,
				m.Timestamp, m.MetricValue, m.Method, m.Status, m.URL, m.Scenario)
			argIdx += 9
		}

		query := fmt.Sprintf(
			`INSERT INTO k6_metrics (execution_id, test_id, metric_name, timestamp, metric_value, method, status, url, scenario)
			VALUES %s`, strings.Join(values, ","),
		)

		if _, err := r.pool.Exec(context.Background(), query, args...); err != nil {
			return fmt.Errorf("bulk insert metrics: %w", err)
		}
	}
	return nil
}

func (r *MetricRepository) GetTimeseries(executionID uuid.UUID, metricName string) ([]domain.MetricDatapoint, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT timestamp, metric_value FROM k6_metrics
		WHERE execution_id = $1 AND metric_name = $2
		ORDER BY timestamp`, executionID, metricName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []domain.MetricDatapoint
	for rows.Next() {
		var p domain.MetricDatapoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if points == nil {
		points = []domain.MetricDatapoint{}
	}
	return points, nil
}

func (r *MetricRepository) GetMetricNames(executionID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT DISTINCT metric_name FROM k6_metrics WHERE execution_id = $1 ORDER BY metric_name`,
		executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (r *MetricRepository) GetSummary(executionID uuid.UUID) ([]domain.MetricSummary, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT metric_name, COUNT(*), AVG(metric_value), MIN(metric_value), MAX(metric_value),
			PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY metric_value),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY metric_value)
		FROM k6_metrics WHERE execution_id = $1
		GROUP BY metric_name ORDER BY metric_name`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.MetricSummary
	for rows.Next() {
		var s domain.MetricSummary
		if err := rows.Scan(&s.MetricName, &s.Count, &s.Avg, &s.Min, &s.Max, &s.P90, &s.P95); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if summaries == nil {
		summaries = []domain.MetricSummary{}
	}
	return summaries, nil
}

func (r *MetricRepository) ComputeExecutionSummary(executionID uuid.UUID) (domain.JSONMap, error) {
	var totalRequests, totalFailures, avgResponse, errorRate float64
	err := r.pool.QueryRow(context.Background(), `
		SELECT
			COALESCE(SUM(CASE WHEN metric_name = 'http_reqs' THEN metric_value END), 0),
			COALESCE(SUM(CASE WHEN metric_name = 'http_reqs' AND status NOT IN ('200','201') THEN metric_value ELSE 0 END), 0),
			COALESCE(AVG(CASE WHEN metric_name = 'http_req_duration' THEN metric_value END), 0)
		FROM k6_metrics WHERE execution_id = $1`, executionID,
	).Scan(&totalRequests, &totalFailures, &avgResponse)
	if err != nil {
		return nil, err
	}

	if totalRequests > 0 {
		errorRate = math.Round(totalFailures/totalRequests*10000) / 100
	}
	avgResponse = math.Round(avgResponse*100) / 100

	return domain.JSONMap{
		"total_requests":  totalRequests,
		"avg_response_ms": avgResponse,
		"error_rate":      errorRate,
	}, nil
}

func (r *MetricRepository) DeleteByExecution(executionID uuid.UUID) error {
	_, err := r.pool.Exec(context.Background(),
		`DELETE FROM k6_metrics WHERE execution_id = $1`, executionID)
	return err
}

// Grafana queries — join with tests and domains to filter by name

func (r *MetricRepository) GetTimeseriesByFilter(filter domain.MetricFilter) ([]domain.MetricDatapoint, error) {
	query := `SELECT m.timestamp, m.metric_value FROM k6_metrics m
		JOIN tests t ON t.id = m.test_id
		JOIN domains d ON d.id = t.domain_id
		WHERE d.name = $1 AND t.name = $2 AND m.metric_name = $3
			AND m.timestamp >= $4 AND m.timestamp <= $5
		ORDER BY m.timestamp`

	rows, err := r.pool.Query(context.Background(), query,
		filter.DomainName, filter.TestName, filter.MetricName, filter.From, filter.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []domain.MetricDatapoint
	for rows.Next() {
		var p domain.MetricDatapoint
		if err := rows.Scan(&p.Timestamp, &p.Value); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if points == nil {
		points = []domain.MetricDatapoint{}
	}
	return points, nil
}

func (r *MetricRepository) GetMetricNamesByFilter(domainName, testName string) ([]string, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT DISTINCT m.metric_name FROM k6_metrics m
		JOIN tests t ON t.id = m.test_id
		JOIN domains d ON d.id = t.domain_id
		WHERE d.name = $1 AND t.name = $2
		ORDER BY m.metric_name`, domainName, testName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

func (r *MetricRepository) GetSummaryByFilter(domainName, testName string) ([]domain.MetricSummary, error) {
	rows, err := r.pool.Query(context.Background(),
		`SELECT m.metric_name, COUNT(*), AVG(m.metric_value), MIN(m.metric_value), MAX(m.metric_value),
			PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY m.metric_value),
			PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY m.metric_value)
		FROM k6_metrics m
		JOIN tests t ON t.id = m.test_id
		JOIN domains d ON d.id = t.domain_id
		WHERE d.name = $1 AND t.name = $2
		GROUP BY m.metric_name ORDER BY m.metric_name`, domainName, testName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []domain.MetricSummary
	for rows.Next() {
		var s domain.MetricSummary
		if err := rows.Scan(&s.MetricName, &s.Count, &s.Avg, &s.Min, &s.Max, &s.P90, &s.P95); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	if summaries == nil {
		summaries = []domain.MetricSummary{}
	}
	return summaries, nil
}
