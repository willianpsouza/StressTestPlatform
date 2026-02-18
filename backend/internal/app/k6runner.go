package app

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type K6Runner struct {
	mu           sync.Mutex
	running      map[uuid.UUID]map[uuid.UUID]context.CancelFunc // userID -> execID -> cancel
	execRepo     domain.ExecutionRepository
	testRepo     domain.TestRepository
	influxURL    string
	influxToken  string
	influxOrg    string
	k6Config     config.K6Config
}

func NewK6Runner(
	execRepo domain.ExecutionRepository,
	testRepo domain.TestRepository,
	influxURL, influxToken, influxOrg string,
	k6Config config.K6Config,
) *K6Runner {
	return &K6Runner{
		running:     make(map[uuid.UUID]map[uuid.UUID]context.CancelFunc),
		execRepo:    execRepo,
		testRepo:    testRepo,
		influxURL:   influxURL,
		influxToken: influxToken,
		influxOrg:   influxOrg,
		k6Config:    k6Config,
	}
}

func (r *K6Runner) CountRunning(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.running[userID])
}

func (r *K6Runner) Run(execution *domain.TestExecution) error {
	r.mu.Lock()
	count := len(r.running[execution.UserID])
	if count >= r.k6Config.MaxConcurrent {
		r.mu.Unlock()
		return domain.NewTooManyRequestsError(
			fmt.Sprintf("Maximum %d concurrent tests per user", r.k6Config.MaxConcurrent),
		)
	}

	// Get test info
	test, err := r.testRepo.GetByID(execution.TestID)
	if err != nil {
		r.mu.Unlock()
		return err
	}

	// Cap VUs and duration
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

	// Context with timeout (duration + 30s buffer)
	ctx, cancel := context.WithTimeout(context.Background(), dur+30*time.Second)

	if r.running[execution.UserID] == nil {
		r.running[execution.UserID] = make(map[uuid.UUID]context.CancelFunc)
	}
	r.running[execution.UserID][execution.ID] = cancel
	r.mu.Unlock()

	// Run in goroutine
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

	// Build K6 command
	influxOutput := fmt.Sprintf("experimental-influxdb=%s?org=%s&bucket=%s&token=%s",
		r.influxURL, r.influxOrg, test.InfluxDBBucket, r.influxToken,
	)

	cmd := exec.CommandContext(ctx, "k6", "run",
		"--vus", strconv.Itoa(vus),
		"--duration", dur.String(),
		"--out", influxOutput,
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

	if err := r.execRepo.Update(execution); err != nil {
		log.Printf("[K6] Failed to update execution %s: %v", execution.ID, err)
	}

	log.Printf("[K6] Execution %s finished with status %s", execution.ID, execution.Status)
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
