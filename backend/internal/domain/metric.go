package domain

import (
	"time"

	"github.com/google/uuid"
)

type K6Metric struct {
	ID          int64     `json:"id"`
	ExecutionID uuid.UUID `json:"execution_id"`
	TestID      uuid.UUID `json:"test_id"`
	MetricName  string    `json:"metric_name"`
	Timestamp   time.Time `json:"timestamp"`
	MetricValue float64   `json:"metric_value"`
	Method      *string   `json:"method,omitempty"`
	Status      *string   `json:"status,omitempty"`
	URL         *string   `json:"url,omitempty"`
	Scenario    *string   `json:"scenario,omitempty"`
}

type MetricDatapoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricFilter is used by Grafana queries — filter by domain name, test name, and date range.
type MetricFilter struct {
	DomainName string
	TestName   string
	MetricName string
	From       time.Time
	To         time.Time
}

type MetricRepository interface {
	BulkInsert(metrics []K6Metric) error
	GetTimeseries(executionID uuid.UUID, metricName string) ([]MetricDatapoint, error)
	GetMetricNames(executionID uuid.UUID) ([]string, error)
	GetSummary(executionID uuid.UUID) ([]MetricSummary, error)
	ComputeExecutionSummary(executionID uuid.UUID) (JSONMap, error)
	AggregateAndCleanup(executionID uuid.UUID) error
	DeleteByExecution(executionID uuid.UUID) error

	// Grafana queries — filter by domain/test/date
	GetTimeseriesByFilter(filter MetricFilter) ([]MetricDatapoint, error)
	GetMetricNamesByFilter(domainName, testName string) ([]string, error)
	GetSummaryByFilter(domainName, testName string) ([]MetricSummary, error)
}

type MetricSummary struct {
	MetricName string  `json:"metric_name"`
	Count      int64   `json:"count"`
	Avg        float64 `json:"avg"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
	P90        float64 `json:"p90"`
	P95        float64 `json:"p95"`
}
