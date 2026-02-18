package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

const metricsCacheTTL = 30 * time.Second

type MetricsHandler struct {
	metricRepo domain.MetricRepository
	domainRepo domain.DomainRepository
	testRepo   domain.TestRepository
	redis      *redis.Client
}

func NewMetricsHandler(
	metricRepo domain.MetricRepository,
	domainRepo domain.DomainRepository,
	testRepo domain.TestRepository,
	redis *redis.Client,
) *MetricsHandler {
	return &MetricsHandler{
		metricRepo: metricRepo,
		domainRepo: domainRepo,
		testRepo:   testRepo,
		redis:      redis,
	}
}

// ListDomains returns domain names that have test data.
// GET /api/v1/grafana/domains
func (h *MetricsHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	cacheKey := "grafana:domains"
	if cached := h.getCache(cacheKey); cached != nil {
		response.OK(w, cached)
		return
	}

	filter := domain.DomainFilter{
		Pagination: domain.Pagination{Page: 1, PageSize: 500},
	}
	domains, _, err := h.domainRepo.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	names := make([]string, 0, len(domains))
	for _, d := range domains {
		names = append(names, d.Name)
	}

	h.setCache(cacheKey, names)
	response.OK(w, names)
}

// ListTests returns test names for a domain.
// GET /api/v1/grafana/tests?domain=MyDomain
func (h *MetricsHandler) ListTests(w http.ResponseWriter, r *http.Request) {
	domainName := r.URL.Query().Get("domain")
	if domainName == "" {
		response.BadRequest(w, "domain query parameter is required")
		return
	}

	cacheKey := fmt.Sprintf("grafana:tests:%s", domainName)
	if cached := h.getCache(cacheKey); cached != nil {
		response.OK(w, cached)
		return
	}

	// Get all tests, filter by domain name from joined data
	filter := domain.TestFilter{
		Pagination: domain.Pagination{Page: 1, PageSize: 500},
	}
	tests, _, err := h.testRepo.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	names := make([]string, 0)
	for _, t := range tests {
		if t.DomainName != nil && *t.DomainName == domainName {
			names = append(names, t.Name)
		}
	}

	h.setCache(cacheKey, names)
	response.OK(w, names)
}

// ListMetricNames returns available metric names for a domain+test.
// GET /api/v1/grafana/metrics?domain=MyDomain&test=MyTest
func (h *MetricsHandler) ListMetricNames(w http.ResponseWriter, r *http.Request) {
	domainName := r.URL.Query().Get("domain")
	testName := r.URL.Query().Get("test")
	if domainName == "" || testName == "" {
		response.BadRequest(w, "domain and test query parameters are required")
		return
	}

	cacheKey := fmt.Sprintf("grafana:metric_names:%s:%s", domainName, testName)
	if cached := h.getCache(cacheKey); cached != nil {
		response.OK(w, cached)
		return
	}

	names, err := h.metricRepo.GetMetricNamesByFilter(domainName, testName)
	if err != nil {
		response.Error(w, err)
		return
	}

	h.setCache(cacheKey, names)
	response.OK(w, names)
}

// GetTimeseries returns timeseries data for Grafana panels.
// GET /api/v1/grafana/timeseries?domain=MyDomain&test=MyTest&metric=http_req_duration&from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z
func (h *MetricsHandler) GetTimeseries(w http.ResponseWriter, r *http.Request) {
	domainName := r.URL.Query().Get("domain")
	testName := r.URL.Query().Get("test")
	metricName := r.URL.Query().Get("metric")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if domainName == "" || testName == "" || metricName == "" {
		response.BadRequest(w, "domain, test, and metric query parameters are required")
		return
	}

	// Parse date range (default: last 24h)
	now := time.Now()
	from := now.Add(-24 * time.Hour)
	to := now

	if fromStr != "" {
		if parsed, err := parseFlexibleTime(fromStr); err == nil {
			from = parsed
		}
	}
	if toStr != "" {
		if parsed, err := parseFlexibleTime(toStr); err == nil {
			to = parsed
		}
	}

	cacheKey := fmt.Sprintf("grafana:ts:%s:%s:%s:%d:%d", domainName, testName, metricName, from.Unix(), to.Unix())
	if cached := h.getCache(cacheKey); cached != nil {
		response.OK(w, cached)
		return
	}

	points, err := h.metricRepo.GetTimeseriesByFilter(domain.MetricFilter{
		DomainName: domainName,
		TestName:   testName,
		MetricName: metricName,
		From:       from,
		To:         to,
	})
	if err != nil {
		response.Error(w, err)
		return
	}

	h.setCache(cacheKey, points)
	response.OK(w, points)
}

// GetSummary returns aggregated metric summary for a domain+test.
// GET /api/v1/grafana/summary?domain=MyDomain&test=MyTest
func (h *MetricsHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	domainName := r.URL.Query().Get("domain")
	testName := r.URL.Query().Get("test")
	if domainName == "" || testName == "" {
		response.BadRequest(w, "domain and test query parameters are required")
		return
	}

	cacheKey := fmt.Sprintf("grafana:summary:%s:%s", domainName, testName)
	if cached := h.getCache(cacheKey); cached != nil {
		response.OK(w, cached)
		return
	}

	summaries, err := h.metricRepo.GetSummaryByFilter(domainName, testName)
	if err != nil {
		response.Error(w, err)
		return
	}

	h.setCache(cacheKey, summaries)
	response.OK(w, summaries)
}

// Redis cache helpers

func (h *MetricsHandler) getCache(key string) interface{} {
	ctx := context.Background()
	val, err := h.redis.Get(ctx, key).Result()
	if err != nil {
		return nil
	}
	var result interface{}
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil
	}
	return result
}

func (h *MetricsHandler) setCache(key string, value interface{}) {
	ctx := context.Background()
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	h.redis.Set(ctx, key, string(data), metricsCacheTTL)
}

func parseFlexibleTime(s string) (time.Time, error) {
	// Try ISO 8601 with millis
	if t, err := time.Parse("2006-01-02T15:04:05.000Z", s); err == nil {
		return t, nil
	}
	// Try RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try RFC3339 with nanos
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	// Try date only
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}
