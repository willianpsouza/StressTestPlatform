package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type config struct {
	DatabaseURL string
	RedisURL    string
	Port        string
}

func loadConfig() config {
	c := config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
		Port:        os.Getenv("PORT"),
	}
	if c.DatabaseURL == "" {
		c.DatabaseURL = "postgres://stresstest:stresstest_secret@postgres:5432/stresstest?sslmode=disable"
	}
	if c.RedisURL == "" {
		c.RedisURL = "redis://redis:6379/0"
	}
	if c.Port == "" {
		c.Port = "8081"
	}
	return c
}

// ---------------------------------------------------------------------------
// Cache helpers
// ---------------------------------------------------------------------------

const cacheTTL = 30 * time.Second

func cacheGet(rdb *redis.Client, key string) ([]byte, bool) {
	val, err := rdb.Get(context.Background(), key).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

func cacheSet(rdb *redis.Client, key string, data []byte) {
	rdb.Set(context.Background(), key, data, cacheTTL)
}

func writeJSON(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// ---------------------------------------------------------------------------
// Query param helpers
// ---------------------------------------------------------------------------

func parseTimeRange(r *http.Request) (from, to time.Time) {
	now := time.Now()
	from = now.Add(-24 * time.Hour)
	to = now

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := parseFlexibleTime(v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := parseFlexibleTime(v); err == nil {
			to = t
		}
	}
	return
}

func parseFlexibleTime(s string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	// Try epoch millis (Grafana sends ${__from} as millis)
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms), nil
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

func intervalSeconds(r *http.Request) int {
	if v := r.URL.Query().Get("interval"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			return i
		}
	}
	return 5
}

// ---------------------------------------------------------------------------
// Grafana Variable Endpoints
// ---------------------------------------------------------------------------

func handleVariablesDomains(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "m:var:domains"
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		rows, err := db.Query(r.Context(), `
			SELECT DISTINCT d.name
			FROM domains d
			JOIN tests t ON t.domain_id = d.id
			WHERE d.deleted_at IS NULL
			ORDER BY d.name`)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type varItem struct {
			Text  string `json:"__text"`
			Value string `json:"__value"`
		}
		items := make([]varItem, 0)
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				items = append(items, varItem{Text: n, Value: n})
			}
		}

		data := marshal(items)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

func handleVariablesTests(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		if domain == "" {
			writeError(w, 400, "domain query parameter is required")
			return
		}

		key := fmt.Sprintf("m:var:tests:%s", domain)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		rows, err := db.Query(r.Context(), `
			SELECT DISTINCT t.name
			FROM tests t
			JOIN domains d ON d.id = t.domain_id
			WHERE d.name = $1
			  AND t.deleted_at IS NULL
			ORDER BY t.name`, domain)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type varItem struct {
			Text  string `json:"__text"`
			Value string `json:"__value"`
		}
		items := make([]varItem, 0)
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				items = append(items, varItem{Text: n, Value: n})
			}
		}

		data := marshal(items)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// ---------------------------------------------------------------------------
// Grafana Stats (consolidated)
// ---------------------------------------------------------------------------

type statsRow struct {
	Requests    float64 `json:"requests"`
	Failures    float64 `json:"failures"`
	PeakRPS     float64 `json:"peak_rps"`
	ErrorRate   float64 `json:"error_rate"`
	AvgResponse float64 `json:"avg_response"`
	P90         float64 `json:"p90"`
	P95         float64 `json:"p95"`
	MaxResponse float64 `json:"max_response"`
	VusMax      float64 `json:"vus_max"`
	ReqPerVU    float64 `json:"req_per_vu"`
}

func handleGrafanaStats(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		test := r.URL.Query().Get("test")
		from, to := parseTimeRange(r)
		interval := intervalSeconds(r)

		key := fmt.Sprintf("m:stats:%s:%s:%d:%d:%d", domain, test, from.Unix(), to.Unix(), interval)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		query := `
WITH exec_ids AS (
  SELECT e.id
  FROM test_executions e
  JOIN tests t ON t.id = e.test_id
  JOIN domains d ON d.id = t.domain_id
  WHERE ($1 = '' OR d.name = $1)
    AND ($2 = '' OR t.name = $2)
    AND e.started_at >= $3 AND e.started_at <= $4
    AND e.status IN ('COMPLETED', 'FAILED')
),
summaries AS (
  SELECT * FROM k6_metrics_aggregated
  WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE
),
buckets AS (
  SELECT * FROM k6_metrics_aggregated
  WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = FALSE
)
SELECT
  COALESCE((SELECT SUM(sum_value) FROM summaries WHERE metric_name = 'http_reqs' AND url IS NULL), 0) AS requests,
  COALESCE((SELECT SUM(sum_value) FROM summaries WHERE metric_name = 'http_reqs' AND url IS NOT NULL AND status NOT IN ('200','201')), 0) AS failures,
  COALESCE((SELECT MAX(rps) FROM (
    SELECT SUM(sum_value) / $5 AS rps
    FROM buckets WHERE metric_name = 'http_reqs'
    GROUP BY floor(extract(epoch FROM bucket_time) / $5)
  ) sub), 0) AS peak_rps,
  COALESCE((SELECT SUM(CASE WHEN status NOT IN ('200','201') THEN sum_value ELSE 0 END) * 100.0
    / NULLIF((SELECT SUM(sum_value) FROM summaries WHERE metric_name = 'http_reqs' AND url IS NULL), 0)
    FROM summaries WHERE metric_name = 'http_reqs' AND url IS NOT NULL), 0) AS error_rate,
  COALESCE((SELECT SUM(avg_value * count) / NULLIF(SUM(count), 0) FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL), 0) AS avg_response,
  COALESCE((SELECT MAX(p90) FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL), 0) AS p90,
  COALESCE((SELECT MAX(p95) FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL), 0) AS p95,
  COALESCE((SELECT MAX(max_value) FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL), 0) AS max_response,
  COALESCE((SELECT MAX(max_value) FROM summaries WHERE metric_name = 'vus_max' AND url IS NULL), 0) AS vus_max`

		var s statsRow
		err := db.QueryRow(r.Context(), query, domain, test, from, to, float64(interval)).Scan(
			&s.Requests, &s.Failures, &s.PeakRPS, &s.ErrorRate,
			&s.AvgResponse, &s.P90, &s.P95, &s.MaxResponse, &s.VusMax,
		)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if s.VusMax > 0 {
			s.ReqPerVU = s.Requests / s.VusMax
		}

		// Round to 2 decimals
		s.PeakRPS = math.Round(s.PeakRPS*100) / 100
		s.ErrorRate = math.Round(s.ErrorRate*100) / 100
		s.AvgResponse = math.Round(s.AvgResponse*100) / 100
		s.P90 = math.Round(s.P90*100) / 100
		s.P95 = math.Round(s.P95*100) / 100
		s.MaxResponse = math.Round(s.MaxResponse*100) / 100
		s.ReqPerVU = math.Round(s.ReqPerVU*100) / 100

		data := marshal([]statsRow{s})
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// ---------------------------------------------------------------------------
// Grafana Timeseries Endpoints
// ---------------------------------------------------------------------------

// Base FROM for per-second bucket queries (time range <= 12h)
const tsBaseBucket = `FROM k6_metrics_aggregated m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.bucket_time >= $3 AND m.bucket_time <= $4
  AND m.is_summary = FALSE`

const longRangeThreshold = 12 * time.Hour

func handleTSAll(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' THEN m.sum_value END), 0) AS requests,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' THEN m.sum_value END) / $5, 0) AS rps,
  COALESCE(SUM(CASE WHEN m.metric_name = 'iterations' THEN m.sum_value END), 0) AS iterations,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_req_duration' THEN m.avg_value * m.count END)
    / NULLIF(SUM(CASE WHEN m.metric_name = 'http_req_duration' THEN m.count END), 0), 0) AS response_time,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201') THEN m.sum_value END), 0) AS failures
` + tsBaseBucket + `
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(CASE WHEN m.metric_name = 'http_reqs' AND m.url IS NULL THEN m.sum_value END), 0) AS requests,
  COALESCE(MAX(CASE WHEN m.metric_name = 'http_reqs' AND m.url IS NULL THEN m.sum_value END)
    / NULLIF(EXTRACT(EPOCH FROM (e.completed_at - e.started_at)), 0), 0) AS rps,
  COALESCE(MAX(CASE WHEN m.metric_name = 'iterations' AND m.url IS NULL THEN m.sum_value END), 0) AS iterations,
  COALESCE(MAX(CASE WHEN m.metric_name = 'http_req_duration' AND m.url IS NULL THEN m.avg_value END), 0) AS response_time,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' AND m.url IS NOT NULL AND m.status NOT IN ('200','201') THEN m.sum_value END), 0) AS failures
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id AND m.is_summary = TRUE
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at, e.completed_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "all", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time         time.Time `json:"time"`
				Requests     float64   `json:"requests"`
				RPS          float64   `json:"rps"`
				Iterations   float64   `json:"iterations"`
				ResponseTime float64   `json:"response_time"`
				Failures     float64   `json:"failures"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.Requests, &r.RPS, &r.Iterations, &r.ResponseTime, &r.Failures); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSErrors(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  COALESCE(SUM(m.sum_value), 0) AS errors
` + tsBaseBucket + `
  AND m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201')
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(SUM(m.sum_value), 0) AS errors
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NOT NULL
  AND m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201')
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "errors", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time   time.Time `json:"time"`
				Errors float64   `json:"errors"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.Errors); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSResponseHistogram(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  SUM(m.avg_value * m.count) / NULLIF(SUM(m.count), 0) AS avg_response
` + tsBaseBucket + `
  AND m.metric_name = 'http_req_duration'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.avg_value), 0) AS avg_response
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'http_req_duration'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "response-histogram", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time        time.Time `json:"time"`
				AvgResponse float64   `json:"avg_response"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.AvgResponse); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSRequests(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  SUM(m.sum_value) AS requests
` + tsBaseBucket + `
  AND m.metric_name = 'http_reqs'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.sum_value), 0) AS requests
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'http_reqs'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "requests", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time     time.Time `json:"time"`
				Requests float64   `json:"requests"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.Requests); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSVus(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  MAX(m.max_value) AS vus
` + tsBaseBucket + `
  AND m.metric_name = 'vus'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.max_value), 0) AS vus
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'vus'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "vus", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time time.Time `json:"time"`
				VUS  float64   `json:"vus"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.VUS); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSPercentiles(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  SUM(m.p50 * m.count) / NULLIF(SUM(m.count), 0) AS median,
  MAX(m.p90) AS p90,
  MAX(m.p95) AS p95
` + tsBaseBucket + `
  AND m.metric_name = 'http_req_duration'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.p50), 0) AS median,
  COALESCE(MAX(m.p90), 0) AS p90,
  COALESCE(MAX(m.p95), 0) AS p95
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'http_req_duration'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "percentiles", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time   time.Time `json:"time"`
				Median float64   `json:"median"`
				P90    float64   `json:"p90"`
				P95    float64   `json:"p95"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.Median, &r.P90, &r.P95); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSRps(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  SUM(m.sum_value) / $5 AS rps
` + tsBaseBucket + `
  AND m.metric_name = 'http_reqs'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.sum_value)
    / NULLIF(EXTRACT(EPOCH FROM (e.completed_at - e.started_at)), 0), 0) AS rps
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'http_reqs'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at, e.completed_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "rps", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time time.Time `json:"time"`
				RPS  float64   `json:"rps"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.RPS); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSIterations(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
  SUM(m.sum_value) AS iterations
` + tsBaseBucket + `
  AND m.metric_name = 'iterations'
GROUP BY 1 ORDER BY 1`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(MAX(m.sum_value), 0) AS iterations
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
  AND m.metric_name = 'iterations'
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "iterations", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time       time.Time `json:"time"`
				Iterations float64   `json:"iterations"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.Iterations); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

func handleTSReqPerVU(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	bucketQ := `
SELECT r.time, COALESCE(r.reqs / NULLIF(v.vus, 0), 0) AS req_per_vu
FROM (
  SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
    SUM(m.sum_value) AS reqs
  ` + tsBaseBucket + `
    AND m.metric_name = 'http_reqs'
  GROUP BY 1
) r
LEFT JOIN (
  SELECT to_timestamp(floor(extract(epoch FROM m.bucket_time) / $5) * $5) AS time,
    MAX(m.max_value) AS vus
  ` + tsBaseBucket + `
    AND m.metric_name = 'vus'
  GROUP BY 1
) v ON r.time = v.time
ORDER BY r.time`

	summaryQ := `
SELECT e.started_at AS time,
  COALESCE(
    MAX(CASE WHEN m.metric_name = 'http_reqs' THEN m.sum_value END)
    / NULLIF(MAX(CASE WHEN m.metric_name = 'vus' THEN m.max_value END), 0),
    0
  ) AS req_per_vu
FROM test_executions e
JOIN tests t ON t.id = e.test_id
JOIN domains d ON d.id = t.domain_id
LEFT JOIN k6_metrics_aggregated m ON m.execution_id = e.id
  AND m.is_summary = TRUE AND m.url IS NULL
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND e.started_at >= $3 AND e.started_at <= $4
  AND e.status IN ('COMPLETED', 'FAILED')
GROUP BY e.id, e.started_at
ORDER BY e.started_at`

	return tsHandler(db, rdb, "req-per-vu", bucketQ, summaryQ,
		func(rows pgxRows) (any, error) {
			type row struct {
				Time     time.Time `json:"time"`
				ReqPerVU float64   `json:"req_per_vu"`
			}
			var result []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Time, &r.ReqPerVU); err != nil {
					return nil, err
				}
				result = append(result, r)
			}
			if result == nil {
				result = []row{}
			}
			return result, nil
		})
}

// tsHandler is a generic handler builder for timeseries endpoints.
type pgxRows interface {
	Next() bool
	Scan(dest ...any) error
	Close()
}

// fillTimeGaps takes serialized timeseries JSON and inserts zero-value rows
// for gaps larger than 2x the interval. This prevents Grafana from drawing
// continuous lines between separate test executions.
func fillTimeGaps(raw []byte, intervalSec int) []byte {
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) < 2 {
		return raw
	}

	maxGap := time.Duration(intervalSec*2) * time.Second
	step := time.Duration(intervalSec) * time.Second
	var result []map[string]any

	for i, row := range rows {
		result = append(result, row)
		if i >= len(rows)-1 {
			continue
		}

		t1, ok1 := parseJSONTime(row["time"])
		t2, ok2 := parseJSONTime(rows[i+1]["time"])
		if !ok1 || !ok2 {
			continue
		}

		if t2.Sub(t1) > maxGap {
			// Build a zero-value row (copy keys, set numerics to 0)
			zeroRow := map[string]any{"time": t1.Add(step).Format(time.RFC3339Nano)}
			for k := range row {
				if k != "time" {
					zeroRow[k] = 0
				}
			}
			result = append(result, zeroRow)

			// Also insert a zero row just before the next data point
			zeroRow2 := map[string]any{"time": t2.Add(-step).Format(time.RFC3339Nano)}
			for k := range row {
				if k != "time" {
					zeroRow2[k] = 0
				}
			}
			result = append(result, zeroRow2)
		}
	}

	out, err := json.Marshal(result)
	if err != nil {
		return raw
	}
	return out
}

func parseJSONTime(v any) (time.Time, bool) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func tsHandler(db *pgxpool.Pool, rdb *redis.Client, name, bucketQuery, summaryQuery string, scanner func(pgxRows) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		test := r.URL.Query().Get("test")
		from, to := parseTimeRange(r)
		interval := intervalSeconds(r)

		isLongRange := to.Sub(from) > longRangeThreshold
		query := bucketQuery
		if isLongRange {
			query = summaryQuery
		}

		key := fmt.Sprintf("m:ts:%s:%s:%s:%d:%d:%d", name, domain, test, from.Unix(), to.Unix(), interval)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		var args []any
		if isLongRange {
			// Summary queries only use $1-$4 (no interval param)
			args = []any{domain, test, from, to}
		} else {
			args = buildTSArgs(query, domain, test, from, to, interval)
		}

		rows, err := db.Query(r.Context(), query, args...)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		result, err := scanner(rows)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		data := fillTimeGaps(marshal(result), interval)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// buildTSArgs constructs the query arguments. For the req-per-vu bucket query
// that references $1-$5 twice, the same arguments are reused (PostgreSQL
// handles this natively with numbered params).
func buildTSArgs(query string, domain, test string, from, to time.Time, interval int) []any {
	// Count max placeholder index used
	maxIdx := 5
	for i := 6; i <= 10; i++ {
		if strings.Contains(query, fmt.Sprintf("$%d", i)) {
			maxIdx = i
		}
	}
	if maxIdx == 5 {
		return []any{domain, test, from, to, float64(interval)}
	}
	args := []any{domain, test, from, to, float64(interval)}
	for i := 6; i <= maxIdx; i++ {
		args = append(args, args[(i-1)%5])
	}
	return args
}

// ---------------------------------------------------------------------------
// Grafana Table Endpoints
// ---------------------------------------------------------------------------

func handleTableHTTPRequests(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		test := r.URL.Query().Get("test")
		from, to := parseTimeRange(r)

		key := fmt.Sprintf("m:tbl:http:%s:%s:%d:%d", domain, test, from.Unix(), to.Unix())
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		rows, err := db.Query(r.Context(), `
SELECT COALESCE(m.url, 'N/A') AS url,
  COALESCE(m.method, 'N/A') AS method,
  COALESCE(m.status, 'N/A') AS status,
  SUM(m.count)::BIGINT AS count,
  ROUND((SUM(m.avg_value * m.count) / NULLIF(SUM(m.count), 0))::numeric, 2) AS avg_ms,
  ROUND(MIN(m.min_value)::numeric, 2) AS min_ms,
  ROUND(MAX(m.max_value)::numeric, 2) AS max_ms,
  ROUND(MAX(m.p90)::numeric, 2) AS p90_ms,
  ROUND(MAX(m.p95)::numeric, 2) AS p95_ms,
  ROUND(MAX(m.p99)::numeric, 2) AS p99_ms
FROM k6_metrics_aggregated m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
JOIN test_executions e ON e.id = m.execution_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.metric_name = 'http_req_duration'
  AND m.is_summary = TRUE AND m.url IS NOT NULL
  AND e.started_at >= $3 AND e.started_at <= $4
GROUP BY m.url, m.method, m.status
ORDER BY count DESC`, domain, test, from, to)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type tableRow struct {
			URL    string  `json:"url"`
			Method string  `json:"method"`
			Status string  `json:"status"`
			Count  int64   `json:"count"`
			AvgMs  float64 `json:"avg_ms"`
			MinMs  float64 `json:"min_ms"`
			MaxMs  float64 `json:"max_ms"`
			P90Ms  float64 `json:"p90_ms"`
			P95Ms  float64 `json:"p95_ms"`
			P99Ms  float64 `json:"p99_ms"`
		}

		result := make([]tableRow, 0)
		for rows.Next() {
			var tr tableRow
			if err := rows.Scan(&tr.URL, &tr.Method, &tr.Status, &tr.Count,
				&tr.AvgMs, &tr.MinMs, &tr.MaxMs, &tr.P90Ms, &tr.P95Ms, &tr.P99Ms); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			result = append(result, tr)
		}

		data := marshal(result)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

func handleTableErrors(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		test := r.URL.Query().Get("test")
		from, to := parseTimeRange(r)

		key := fmt.Sprintf("m:tbl:err:%s:%s:%d:%d", domain, test, from.Unix(), to.Unix())
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		rows, err := db.Query(r.Context(), `
SELECT COALESCE(m.url, 'N/A') AS url,
  COALESCE(m.method, 'N/A') AS method,
  m.status,
  SUM(m.sum_value)::int AS count
FROM k6_metrics_aggregated m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
JOIN test_executions e ON e.id = m.execution_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.metric_name = 'http_reqs'
  AND m.is_summary = TRUE AND m.url IS NOT NULL
  AND m.status NOT IN ('200','201')
  AND e.started_at >= $3 AND e.started_at <= $4
GROUP BY m.url, m.method, m.status
ORDER BY count DESC`, domain, test, from, to)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		type tableRow struct {
			URL    string `json:"url"`
			Method string `json:"method"`
			Status string `json:"status"`
			Count  int    `json:"count"`
		}

		result := make([]tableRow, 0)
		for rows.Next() {
			var tr tableRow
			if err := rows.Scan(&tr.URL, &tr.Method, &tr.Status, &tr.Count); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			result = append(result, tr)
		}

		data := marshal(result)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// ---------------------------------------------------------------------------
// Frontend Dashboard Endpoints
// ---------------------------------------------------------------------------

type dashboardOverview struct {
	TotalRequests  float64 `json:"total_requests"`
	TotalFailures  float64 `json:"total_failures"`
	ErrorRate      float64 `json:"error_rate"`
	SuccessRate    float64 `json:"success_rate"`
	AvgResponseMs  float64 `json:"avg_response_ms"`
	P95ResponseMs  float64 `json:"p95_response_ms"`
	TotalDataPoints int64  `json:"total_data_points"`
}

func handleDashboardOverview(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "m:dash:overview"
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		var d dashboardOverview
		err := db.QueryRow(r.Context(), `
SELECT
  COALESCE((SELECT SUM(sum_value) FROM k6_metrics_aggregated
    WHERE is_summary = TRUE AND url IS NULL AND metric_name = 'http_reqs'), 0) AS total_requests,
  COALESCE((SELECT SUM(sum_value) FROM k6_metrics_aggregated
    WHERE is_summary = TRUE AND url IS NOT NULL
    AND metric_name = 'http_reqs' AND status NOT IN ('200','201')), 0) AS total_failures,
  COALESCE((SELECT SUM(avg_value * count) / NULLIF(SUM(count), 0) FROM k6_metrics_aggregated
    WHERE is_summary = TRUE AND url IS NULL AND metric_name = 'http_req_duration'), 0) AS avg_response,
  COALESCE((SELECT MAX(p95) FROM k6_metrics_aggregated
    WHERE is_summary = TRUE AND url IS NULL AND metric_name = 'http_req_duration'), 0) AS p95,
  COALESCE((SELECT SUM(count) FROM k6_metrics_aggregated
    WHERE is_summary = TRUE AND url IS NULL), 0) AS total_data_points`).Scan(
			&d.TotalRequests, &d.TotalFailures, &d.AvgResponseMs, &d.P95ResponseMs, &d.TotalDataPoints,
		)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if d.TotalRequests > 0 {
			d.ErrorRate = math.Round(d.TotalFailures/d.TotalRequests*10000) / 100
		}
		d.SuccessRate = math.Round((100-d.ErrorRate)*100) / 100
		d.AvgResponseMs = math.Round(d.AvgResponseMs*100) / 100
		d.P95ResponseMs = math.Round(d.P95ResponseMs*100) / 100

		data := marshal(d)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

func handleDashboardDomain(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			writeError(w, 400, "name query parameter is required")
			return
		}

		key := fmt.Sprintf("m:dash:domain:%s", name)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		var d dashboardOverview
		err := db.QueryRow(r.Context(), `
WITH exec_ids AS (
  SELECT e.id
  FROM test_executions e
  JOIN tests t ON t.id = e.test_id
  JOIN domains d ON d.id = t.domain_id
  WHERE d.name = $1
)
SELECT
  COALESCE((SELECT SUM(sum_value) FROM k6_metrics_aggregated
    WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE AND url IS NULL AND metric_name = 'http_reqs'), 0) AS total_requests,
  COALESCE((SELECT SUM(sum_value) FROM k6_metrics_aggregated
    WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE AND url IS NOT NULL
    AND metric_name = 'http_reqs' AND status NOT IN ('200','201')), 0) AS total_failures,
  COALESCE((SELECT SUM(avg_value * count) / NULLIF(SUM(count), 0) FROM k6_metrics_aggregated
    WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE AND url IS NULL AND metric_name = 'http_req_duration'), 0) AS avg_response,
  COALESCE((SELECT MAX(p95) FROM k6_metrics_aggregated
    WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE AND url IS NULL AND metric_name = 'http_req_duration'), 0) AS p95,
  COALESCE((SELECT SUM(count) FROM k6_metrics_aggregated
    WHERE execution_id IN (SELECT id FROM exec_ids)
    AND is_summary = TRUE AND url IS NULL), 0) AS total_data_points`, name).Scan(
			&d.TotalRequests, &d.TotalFailures, &d.AvgResponseMs, &d.P95ResponseMs, &d.TotalDataPoints,
		)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if d.TotalRequests > 0 {
			d.ErrorRate = math.Round(d.TotalFailures/d.TotalRequests*10000) / 100
		}
		d.SuccessRate = math.Round((100-d.ErrorRate)*100) / 100
		d.AvgResponseMs = math.Round(d.AvgResponseMs*100) / 100
		d.P95ResponseMs = math.Round(d.P95ResponseMs*100) / 100

		data := marshal(d)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// ---------------------------------------------------------------------------
// Execution List & Stats (Analytics)
// ---------------------------------------------------------------------------

type executionListItem struct {
	ID          string     `json:"id"`
	TestName    string     `json:"test_name"`
	DomainName  string     `json:"domain_name"`
	VUs         int        `json:"vus"`
	Duration    string     `json:"duration"`
	Status      string     `json:"status"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func handleExecutionList(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := "m:exec:list"
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		rows, err := db.Query(r.Context(), `
			SELECT e.id, t.name AS test_name, d.name AS domain_name,
			       e.vus, e.duration, e.status, e.started_at, e.completed_at, e.created_at
			FROM test_executions e
			JOIN tests t ON t.id = e.test_id
			JOIN domains d ON d.id = t.domain_id
			WHERE e.status IN ('COMPLETED','FAILED')
			ORDER BY e.created_at DESC
			LIMIT 100`)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		result := make([]executionListItem, 0)
		for rows.Next() {
			var item executionListItem
			if err := rows.Scan(&item.ID, &item.TestName, &item.DomainName,
				&item.VUs, &item.Duration, &item.Status,
				&item.StartedAt, &item.CompletedAt, &item.CreatedAt); err != nil {
				writeError(w, 500, err.Error())
				return
			}
			result = append(result, item)
		}

		data := marshal(result)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

func handleExecutionStats(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			writeError(w, 400, "execution id is required")
			return
		}

		key := fmt.Sprintf("m:exec:stats:%s", id)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		query := `
WITH summaries AS (
  SELECT * FROM k6_metrics_aggregated
  WHERE execution_id = $1 AND is_summary = TRUE
),
buckets AS (
  SELECT * FROM k6_metrics_aggregated
  WHERE execution_id = $1 AND is_summary = FALSE
)
SELECT
  COALESCE((SELECT sum_value FROM summaries WHERE metric_name = 'http_reqs' AND url IS NULL LIMIT 1), 0) AS requests,
  COALESCE((SELECT SUM(sum_value) FROM summaries WHERE metric_name = 'http_reqs' AND url IS NOT NULL AND status NOT IN ('200','201')), 0) AS failures,
  COALESCE((SELECT MAX(rps) FROM (
    SELECT SUM(sum_value) / 5 AS rps
    FROM buckets WHERE metric_name = 'http_reqs'
    GROUP BY floor(extract(epoch FROM bucket_time) / 5)
  ) sub), 0) AS peak_rps,
  COALESCE((SELECT SUM(CASE WHEN status NOT IN ('200','201') THEN sum_value ELSE 0 END) * 100.0
    / NULLIF((SELECT sum_value FROM summaries WHERE metric_name = 'http_reqs' AND url IS NULL LIMIT 1), 0)
    FROM summaries WHERE metric_name = 'http_reqs' AND url IS NOT NULL), 0) AS error_rate,
  COALESCE((SELECT avg_value FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL LIMIT 1), 0) AS avg_response,
  COALESCE((SELECT p90 FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL LIMIT 1), 0) AS p90,
  COALESCE((SELECT p95 FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL LIMIT 1), 0) AS p95,
  COALESCE((SELECT max_value FROM summaries WHERE metric_name = 'http_req_duration' AND url IS NULL LIMIT 1), 0) AS max_response,
  COALESCE((SELECT max_value FROM summaries WHERE metric_name = 'vus_max' AND url IS NULL LIMIT 1), 0) AS vus_max`

		var s statsRow
		err := db.QueryRow(r.Context(), query, id).Scan(
			&s.Requests, &s.Failures, &s.PeakRPS, &s.ErrorRate,
			&s.AvgResponse, &s.P90, &s.P95, &s.MaxResponse, &s.VusMax,
		)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}

		if s.VusMax > 0 {
			s.ReqPerVU = s.Requests / s.VusMax
		}

		s.PeakRPS = math.Round(s.PeakRPS*100) / 100
		s.ErrorRate = math.Round(s.ErrorRate*100) / 100
		s.AvgResponse = math.Round(s.AvgResponse*100) / 100
		s.P90 = math.Round(s.P90*100) / 100
		s.P95 = math.Round(s.P95*100) / 100
		s.MaxResponse = math.Round(s.MaxResponse*100) / 100
		s.ReqPerVU = math.Round(s.ReqPerVU*100) / 100

		data := marshal(s)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	cfg := loadConfig()

	log.Printf("Starting metrics-api on port %s", cfg.Port)

	// PostgreSQL
	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer dbPool.Close()
	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping PostgreSQL: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()
	log.Println("Connected to Redis")

	// Router
	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))

	// Health
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []byte(`{"status":"ok"}`))
	})

	// Grafana variable endpoints
	r.Get("/grafana/variables/domains", handleVariablesDomains(dbPool, rdb))
	r.Get("/grafana/variables/tests", handleVariablesTests(dbPool, rdb))

	// Grafana stats (consolidated)
	r.Get("/grafana/stats", handleGrafanaStats(dbPool, rdb))

	// Grafana timeseries
	r.Get("/grafana/ts/all", handleTSAll(dbPool, rdb))
	r.Get("/grafana/ts/errors", handleTSErrors(dbPool, rdb))
	r.Get("/grafana/ts/response-histogram", handleTSResponseHistogram(dbPool, rdb))
	r.Get("/grafana/ts/requests", handleTSRequests(dbPool, rdb))
	r.Get("/grafana/ts/vus", handleTSVus(dbPool, rdb))
	r.Get("/grafana/ts/percentiles", handleTSPercentiles(dbPool, rdb))
	r.Get("/grafana/ts/rps", handleTSRps(dbPool, rdb))
	r.Get("/grafana/ts/iterations", handleTSIterations(dbPool, rdb))
	r.Get("/grafana/ts/req-per-vu", handleTSReqPerVU(dbPool, rdb))

	// Grafana tables
	r.Get("/grafana/tables/http-requests", handleTableHTTPRequests(dbPool, rdb))
	r.Get("/grafana/tables/errors", handleTableErrors(dbPool, rdb))

	// Frontend dashboard
	r.Get("/dashboard/overview", handleDashboardOverview(dbPool, rdb))
	r.Get("/dashboard/domain", handleDashboardDomain(dbPool, rdb))

	// Execution analytics
	r.Get("/executions/list", handleExecutionList(dbPool, rdb))
	r.Get("/executions/{id}/stats", handleExecutionStats(dbPool, rdb))

	// Server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}
