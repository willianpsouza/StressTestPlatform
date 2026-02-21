package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type K6Runner struct {
	mu         sync.Mutex
	running    map[uuid.UUID]map[uuid.UUID]context.CancelFunc // userID -> execID -> cancel
	execRepo   domain.ExecutionRepository
	testRepo   domain.TestRepository
	metricRepo domain.MetricRepository
	k6Config   config.K6Config
}

func NewK6Runner(
	execRepo domain.ExecutionRepository,
	testRepo domain.TestRepository,
	metricRepo domain.MetricRepository,
	k6Config config.K6Config,
) *K6Runner {
	return &K6Runner{
		running:    make(map[uuid.UUID]map[uuid.UUID]context.CancelFunc),
		execRepo:   execRepo,
		testRepo:   testRepo,
		metricRepo: metricRepo,
		k6Config:   k6Config,
	}
}

func (r *K6Runner) CountRunning(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.running[userID])
}

func (r *K6Runner) Run(execution *domain.TestExecution) error {
	// Check concurrency limit (short lock, map read only)
	r.mu.Lock()
	if len(r.running[execution.UserID]) >= r.k6Config.MaxConcurrent {
		r.mu.Unlock()
		return domain.NewTooManyRequestsError(
			fmt.Sprintf("Maximum %d concurrent tests per user", r.k6Config.MaxConcurrent),
		)
	}
	r.mu.Unlock()

	// I/O and parsing outside the lock
	test, err := r.testRepo.GetByID(execution.TestID)
	if err != nil {
		return err
	}

	vus := execution.VUs
	if vus > r.k6Config.MaxVUs {
		vus = r.k6Config.MaxVUs
	}

	dur, err := time.ParseDuration(execution.Duration)
	if err != nil {
		dur = 30 * time.Second
	}
	if dur > r.k6Config.MaxDuration {
		dur = r.k6Config.MaxDuration
	}

	ctx, cancel := context.WithTimeout(context.Background(), dur+30*time.Second)

	// Re-check and register under lock (prevents race between check and register)
	r.mu.Lock()
	if len(r.running[execution.UserID]) >= r.k6Config.MaxConcurrent {
		r.mu.Unlock()
		cancel()
		return domain.NewTooManyRequestsError(
			fmt.Sprintf("Maximum %d concurrent tests per user", r.k6Config.MaxConcurrent),
		)
	}
	if r.running[execution.UserID] == nil {
		r.running[execution.UserID] = make(map[uuid.UUID]context.CancelFunc)
	}
	r.running[execution.UserID][execution.ID] = cancel
	r.mu.Unlock()

	go r.execute(ctx, cancel, execution, test, vus, dur)

	return nil
}

func (r *K6Runner) Cancel(userID, execID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if userExecs, ok := r.running[userID]; ok {
		if cancel, ok := userExecs[execID]; ok {
			cancel()
			return true
		}
	}
	return false
}

func (r *K6Runner) execute(ctx context.Context, cancel context.CancelFunc, execution *domain.TestExecution, test *domain.Test, vus int, dur time.Duration) {
	defer cancel()
	defer r.cleanup(execution.UserID, execution.ID)

	// Mark as RUNNING
	now := time.Now()
	execution.Status = domain.TestStatusRunning
	execution.StartedAt = &now
	r.execRepo.Update(execution)

	// CSV output file
	csvPath := filepath.Join(os.TempDir(), fmt.Sprintf("k6-%s.csv", execution.ID))
	defer os.Remove(csvPath)

	// Build K6 command — output to CSV
	cmd := exec.CommandContext(ctx, "k6", "run",
		"--vus", strconv.Itoa(vus),
		"--duration", dur.String(),
		"--out", "csv="+csvPath,
		"--summary-trend-stats", "avg,min,med,max,p(90),p(95),p(99)",
		test.ScriptPath,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.Printf("[K6] Starting execution %s for test %s (vus=%d, duration=%s)",
		execution.ID, test.Name, vus, dur)

	err := cmd.Run()

	completedAt := time.Now()
	execution.CompletedAt = &completedAt

	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	execution.Stdout = &stdoutStr
	execution.Stderr = &stderrStr

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			execution.Status = domain.TestStatusTimeout
			errMsg := "Test exceeded maximum duration"
			execution.ErrorMessage = &errMsg
		} else if ctx.Err() == context.Canceled {
			execution.Status = domain.TestStatusCancelled
			errMsg := "Test was cancelled"
			execution.ErrorMessage = &errMsg
		} else {
			execution.Status = domain.TestStatusFailed
			errMsg := err.Error()
			execution.ErrorMessage = &errMsg
		}

		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			execution.ExitCode = &code
		}
	} else {
		execution.Status = domain.TestStatusCompleted
		code := 0
		execution.ExitCode = &code
	}

	// Import CSV metrics into PostgreSQL (even if test failed, partial data may exist)
	if _, statErr := os.Stat(csvPath); statErr == nil {
		imported, importErr := r.importCSVMetrics(csvPath, execution.ID, test.ID)
		if importErr != nil {
			log.Printf("[K6] Failed to import CSV metrics for execution %s: %v", execution.ID, importErr)
		} else {
			log.Printf("[K6] Imported %d metric rows for execution %s", imported, execution.ID)
		}

		// Compute and persist metrics summary (must run before aggregation since it reads raw data)
		if summary, sumErr := r.metricRepo.ComputeExecutionSummary(execution.ID); sumErr != nil {
			log.Printf("[K6] Failed to compute metrics summary for execution %s: %v", execution.ID, sumErr)
		} else {
			execution.MetricsSummary = summary
		}

		// Aggregate metrics into k6_metrics_aggregated and clean up raw data
		if aggErr := r.metricRepo.AggregateAndCleanup(execution.ID); aggErr != nil {
			log.Printf("[K6] Failed to aggregate metrics for execution %s: %v", execution.ID, aggErr)
		} else {
			log.Printf("[K6] Aggregated and cleaned up raw metrics for execution %s", execution.ID)
		}
	}

	if err := r.execRepo.Update(execution); err != nil {
		log.Printf("[K6] Failed to update execution %s: %v", execution.ID, err)
	}

	log.Printf("[K6] Execution %s finished with status %s", execution.ID, execution.Status)
}

// importCSVMetrics parses the K6 CSV output and bulk inserts into PostgreSQL.
// K6 CSV columns: metric_name,timestamp,metric_value,check,error,error_code,
// expected_response,group,method,name,proto,scenario,service,status,subproto,tls_version,url,extra_tags
func (r *K6Runner) importCSVMetrics(csvPath string, executionID, testID uuid.UUID) (int, error) {
	f, err := os.Open(csvPath)
	if err != nil {
		return 0, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1 // variable fields

	// Read header
	header, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("read csv header: %w", err)
	}

	// Map column names to indices
	colIdx := make(map[string]int)
	for i, name := range header {
		colIdx[strings.TrimSpace(name)] = i
	}

	// Validate required columns
	for _, col := range []string{"metric_name", "timestamp", "metric_value"} {
		if _, ok := colIdx[col]; !ok {
			return 0, fmt.Errorf("missing required column: %s", col)
		}
	}

	var metrics []domain.K6Metric
	total := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // skip malformed rows
		}

		metricName := getCol(record, colIdx, "metric_name")
		if metricName == "" {
			continue
		}

		// Parse timestamp (K6 outputs Unix epoch in microseconds)
		tsStr := getCol(record, colIdx, "timestamp")
		ts, err := parseK6Timestamp(tsStr)
		if err != nil {
			continue
		}

		// Parse metric value
		valStr := getCol(record, colIdx, "metric_value")
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		m := domain.K6Metric{
			ExecutionID: executionID,
			TestID:      testID,
			MetricName:  metricName,
			Timestamp:   ts,
			MetricValue: val,
		}

		if v := getCol(record, colIdx, "method"); v != "" {
			m.Method = &v
		}
		if v := getCol(record, colIdx, "status"); v != "" {
			m.Status = &v
		}
		if v := getCol(record, colIdx, "url"); v != "" {
			m.URL = &v
		}
		if v := getCol(record, colIdx, "scenario"); v != "" {
			m.Scenario = &v
		}

		metrics = append(metrics, m)

		// Flush in batches of 1000 to avoid memory buildup
		if len(metrics) >= 1000 {
			if err := r.metricRepo.BulkInsert(metrics); err != nil {
				return total, fmt.Errorf("bulk insert batch: %w", err)
			}
			total += len(metrics)
			metrics = metrics[:0]
		}
	}

	// Flush remaining
	if len(metrics) > 0 {
		if err := r.metricRepo.BulkInsert(metrics); err != nil {
			return total, fmt.Errorf("bulk insert final batch: %w", err)
		}
		total += len(metrics)
	}

	return total, nil
}

func getCol(record []string, colIdx map[string]int, name string) string {
	idx, ok := colIdx[name]
	if !ok || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func parseK6Timestamp(s string) (time.Time, error) {
	// K6 CSV outputs timestamp as Unix epoch in microseconds (integer)
	us, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		// If > 1e15, it's microseconds; if > 1e12, milliseconds; otherwise seconds
		if us > 1e15 {
			return time.UnixMicro(us), nil
		}
		if us > 1e12 {
			return time.UnixMilli(us), nil
		}
		return time.Unix(us, 0), nil
	}
	// Fallback: try RFC3339
	return time.Parse(time.RFC3339, s)
}

func (r *K6Runner) cleanup(userID, execID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if userExecs, ok := r.running[userID]; ok {
		delete(userExecs, execID)
		if len(userExecs) == 0 {
			delete(r.running, userID)
		}
	}
}

func (r *K6Runner) RecoverOrphans() {
	count, err := r.execRepo.MarkOrphansAsFailed()
	if err != nil {
		log.Printf("[K6] Failed to recover orphan executions: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[K6] Recovered %d orphan executions (marked as FAILED)", count)
	}
}
