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
			JOIN k6_metrics m ON m.test_id = t.id
			ORDER BY d.name`)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		names := make([]string, 0)
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				names = append(names, n)
			}
		}

		data := marshal(names)
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
			JOIN k6_metrics m ON m.test_id = t.id
			WHERE d.name = $1
			ORDER BY t.name`, domain)
		if err != nil {
			writeError(w, 500, err.Error())
			return
		}
		defer rows.Close()

		names := make([]string, 0)
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err == nil {
				names = append(names, n)
			}
		}

		data := marshal(names)
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
WITH params AS (
  SELECT m.* FROM k6_metrics m
  JOIN tests t ON t.id = m.test_id
  JOIN domains d ON d.id = t.domain_id
  WHERE ($1 = '' OR d.name = $1)
    AND ($2 = '' OR t.name = $2)
    AND m.timestamp >= $3 AND m.timestamp <= $4
)
SELECT
  COALESCE((SELECT SUM(metric_value) FROM params WHERE metric_name = 'http_reqs'), 0) AS requests,
  COALESCE((SELECT SUM(metric_value) FROM params WHERE metric_name = 'http_reqs' AND status NOT IN ('200','201')), 0) AS failures,
  COALESCE((SELECT MAX(rps) FROM (
    SELECT SUM(metric_value) / $5 AS rps
    FROM params WHERE metric_name = 'http_reqs'
    GROUP BY floor(extract(epoch FROM timestamp) / $5)
  ) sub), 0) AS peak_rps,
  COALESCE((SELECT SUM(CASE WHEN status NOT IN ('200','201') THEN metric_value ELSE 0 END) * 100.0
    / NULLIF(SUM(metric_value), 0)
    FROM params WHERE metric_name = 'http_reqs'), 0) AS error_rate,
  COALESCE((SELECT AVG(metric_value) FROM params WHERE metric_name = 'http_req_duration'), 0) AS avg_response,
  COALESCE((SELECT PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY metric_value) FROM params WHERE metric_name = 'http_req_duration'), 0) AS p90,
  COALESCE((SELECT PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY metric_value) FROM params WHERE metric_name = 'http_req_duration'), 0) AS p95,
  COALESCE((SELECT MAX(metric_value) FROM params WHERE metric_name = 'http_req_duration'), 0) AS max_response,
  COALESCE((SELECT MAX(metric_value) FROM params WHERE metric_name = 'vus_max'), 0) AS vus_max`

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

// Base query for common filter joins
const tsBaseFrom = `FROM k6_metrics m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.timestamp >= $3 AND m.timestamp <= $4`

func handleTSAll(db *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return tsHandler(db, rdb, "all", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' THEN m.metric_value END), 0) AS requests,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' THEN m.metric_value END) / $5, 0) AS rps,
  COALESCE(SUM(CASE WHEN m.metric_name = 'iterations' THEN m.metric_value END), 0) AS iterations,
  COALESCE(AVG(CASE WHEN m.metric_name = 'http_req_duration' THEN m.metric_value END), 0) AS response_time,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201') THEN m.metric_value END), 0) AS failures
`+tsBaseFrom+`
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "errors", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  COALESCE(SUM(m.metric_value), 0) AS errors
`+tsBaseFrom+`
  AND m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201')
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "response-histogram", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  AVG(m.metric_value) AS avg_response
`+tsBaseFrom+`
  AND m.metric_name = 'http_req_duration'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "requests", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  SUM(m.metric_value) AS requests
`+tsBaseFrom+`
  AND m.metric_name = 'http_reqs'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "vus", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  AVG(m.metric_value) AS vus
`+tsBaseFrom+`
  AND m.metric_name = 'vus'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "percentiles", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  AVG(m.metric_value) AS median,
  PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY m.metric_value) AS p90,
  PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY m.metric_value) AS p95
`+tsBaseFrom+`
  AND m.metric_name = 'http_req_duration'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "rps", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  SUM(m.metric_value) / $5 AS rps
`+tsBaseFrom+`
  AND m.metric_name = 'http_reqs'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "iterations", `
SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
  SUM(m.metric_value) AS iterations
`+tsBaseFrom+`
  AND m.metric_name = 'iterations'
GROUP BY 1 ORDER BY 1`,
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
	return tsHandler(db, rdb, "req-per-vu", `
SELECT r.time, COALESCE(r.reqs / NULLIF(v.vus, 0), 0) AS req_per_vu
FROM (
  SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
    SUM(m.metric_value) AS reqs
  `+tsBaseFrom+`
    AND m.metric_name = 'http_reqs'
  GROUP BY 1
) r
LEFT JOIN (
  SELECT to_timestamp(floor(extract(epoch FROM m.timestamp) / $5) * $5) AS time,
    MAX(m.metric_value) AS vus
  `+tsBaseFrom+`
    AND m.metric_name = 'vus'
  GROUP BY 1
) v ON r.time = v.time
ORDER BY r.time`,
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

func tsHandler(db *pgxpool.Pool, rdb *redis.Client, name, query string, scanner func(pgxRows) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		domain := r.URL.Query().Get("domain")
		test := r.URL.Query().Get("test")
		from, to := parseTimeRange(r)
		interval := intervalSeconds(r)

		key := fmt.Sprintf("m:ts:%s:%s:%s:%d:%d:%d", name, domain, test, from.Unix(), to.Unix(), interval)
		if cached, ok := cacheGet(rdb, key); ok {
			writeJSON(w, cached)
			return
		}

		// The req-per-vu query uses $1-$4 twice (for each subquery), so we need
		// to count how many placeholder sets are needed.
		args := buildTSArgs(query, domain, test, from, to, interval)

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

		data := marshal(result)
		cacheSet(rdb, key, data)
		writeJSON(w, data)
	}
}

// buildTSArgs constructs the query arguments. For the req-per-vu query that
// references $1-$5 twice, the same arguments are reused (PostgreSQL handles
// this natively with numbered params).
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
	// Shouldn't happen with current queries, but handle it
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
  COUNT(*) AS count,
  ROUND(AVG(m.metric_value)::numeric, 2) AS avg_ms,
  ROUND(MIN(m.metric_value)::numeric, 2) AS min_ms,
  ROUND(MAX(m.metric_value)::numeric, 2) AS max_ms,
  ROUND(PERCENTILE_CONT(0.90) WITHIN GROUP (ORDER BY m.metric_value)::numeric, 2) AS p90_ms,
  ROUND(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY m.metric_value)::numeric, 2) AS p95_ms,
  ROUND(PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY m.metric_value)::numeric, 2) AS p99_ms
FROM k6_metrics m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.metric_name = 'http_req_duration'
  AND m.timestamp >= $3 AND m.timestamp <= $4
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
  SUM(m.metric_value)::int AS count
FROM k6_metrics m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
WHERE ($1 = '' OR d.name = $1)
  AND ($2 = '' OR t.name = $2)
  AND m.metric_name = 'http_reqs'
  AND m.status NOT IN ('200','201')
  AND m.timestamp >= $3 AND m.timestamp <= $4
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
  COALESCE(SUM(CASE WHEN metric_name = 'http_reqs' THEN metric_value END), 0) AS total_requests,
  COALESCE(SUM(CASE WHEN metric_name = 'http_reqs' AND status NOT IN ('200','201') THEN metric_value END), 0) AS total_failures,
  COALESCE(AVG(CASE WHEN metric_name = 'http_req_duration' THEN metric_value END), 0) AS avg_response,
  COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY CASE WHEN metric_name = 'http_req_duration' THEN metric_value END), 0) AS p95,
  COUNT(*) AS total_data_points
FROM k6_metrics`).Scan(
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
SELECT
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' THEN m.metric_value END), 0) AS total_requests,
  COALESCE(SUM(CASE WHEN m.metric_name = 'http_reqs' AND m.status NOT IN ('200','201') THEN m.metric_value END), 0) AS total_failures,
  COALESCE(AVG(CASE WHEN m.metric_name = 'http_req_duration' THEN m.metric_value END), 0) AS avg_response,
  COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY CASE WHEN m.metric_name = 'http_req_duration' THEN m.metric_value END), 0) AS p95,
  COUNT(*) AS total_data_points
FROM k6_metrics m
JOIN tests t ON t.id = m.test_id
JOIN domains d ON d.id = t.domain_id
WHERE d.name = $1`, name).Scan(
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
