package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type ExecutionRepository struct {
	db *pgxpool.Pool
}

func NewExecutionRepository(db *pgxpool.Pool) *ExecutionRepository {
	return &ExecutionRepository{db: db}
}

func (r *ExecutionRepository) Create(exec *domain.TestExecution) error {
	exec.ID = uuid.New()
	exec.CreatedAt = time.Now()
	exec.UpdatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO test_executions (id, test_id, user_id, schedule_id, vus, duration, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::test_status, $8, $9)`,
		exec.ID, exec.TestID, exec.UserID, exec.ScheduleID,
		exec.VUs, exec.Duration, string(exec.Status),
		exec.CreatedAt, exec.UpdatedAt,
	)
	return err
}

func (r *ExecutionRepository) GetByID(id uuid.UUID) (*domain.TestExecution, error) {
	exec := &domain.TestExecution{}
	err := r.db.QueryRow(context.Background(),
		`SELECT e.id, e.test_id, e.user_id, e.schedule_id, e.vus, e.duration,
			e.status::text, e.started_at, e.completed_at, e.exit_code,
			e.stdout, e.stderr, e.metrics_summary, e.error_message,
			e.created_at, e.updated_at,
			t.name, d.name, u.name, u.email
		FROM test_executions e
		JOIN tests t ON t.id = e.test_id
		JOIN domains d ON d.id = t.domain_id
		JOIN users u ON u.id = e.user_id
		WHERE e.id = $1`, id,
	).Scan(
		&exec.ID, &exec.TestID, &exec.UserID, &exec.ScheduleID,
		&exec.VUs, &exec.Duration,
		&exec.Status, &exec.StartedAt, &exec.CompletedAt, &exec.ExitCode,
		&exec.Stdout, &exec.Stderr, &exec.MetricsSummary, &exec.ErrorMessage,
		&exec.CreatedAt, &exec.UpdatedAt,
		&exec.TestName, &exec.DomainName, &exec.UserName, &exec.UserEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExecutionNotFound
		}
		return nil, err
	}
	return exec, nil
}

func (r *ExecutionRepository) Update(exec *domain.TestExecution) error {
	exec.UpdatedAt = time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE test_executions SET status=$1::test_status, started_at=$2, completed_at=$3,
			exit_code=$4, stdout=$5, stderr=$6, metrics_summary=$7, error_message=$8, updated_at=$9
		WHERE id=$10`,
		string(exec.Status), exec.StartedAt, exec.CompletedAt,
		exec.ExitCode, exec.Stdout, exec.Stderr, exec.MetricsSummary, exec.ErrorMessage,
		exec.UpdatedAt, exec.ID,
	)
	return err
}

func (r *ExecutionRepository) List(filter domain.ExecutionFilter) ([]domain.TestExecution, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil && !filter.AllUsers {
		where = append(where, fmt.Sprintf("e.user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.TestID != nil {
		where = append(where, fmt.Sprintf("e.test_id = $%d", argIdx))
		args = append(args, *filter.TestID)
		argIdx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("e.status::text = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRow(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM test_executions e WHERE %s", whereClause), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT e.id, e.test_id, e.user_id, e.schedule_id, e.vus, e.duration,
			e.status::text, e.started_at, e.completed_at, e.exit_code,
			e.stdout, e.stderr, e.metrics_summary, e.error_message,
			e.created_at, e.updated_at,
			t.name, d.name, u.name, u.email
		FROM test_executions e
		JOIN tests t ON t.id = e.test_id
		JOIN domains d ON d.id = t.domain_id
		JOIN users u ON u.id = e.user_id
		WHERE %s ORDER BY e.created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit(), filter.Offset())

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var execs []domain.TestExecution
	for rows.Next() {
		var e domain.TestExecution
		if err := rows.Scan(
			&e.ID, &e.TestID, &e.UserID, &e.ScheduleID,
			&e.VUs, &e.Duration,
			&e.Status, &e.StartedAt, &e.CompletedAt, &e.ExitCode,
			&e.Stdout, &e.Stderr, &e.MetricsSummary, &e.ErrorMessage,
			&e.CreatedAt, &e.UpdatedAt,
			&e.TestName, &e.DomainName, &e.UserName, &e.UserEmail,
		); err != nil {
			return nil, 0, err
		}
		execs = append(execs, e)
	}

	if execs == nil {
		execs = []domain.TestExecution{}
	}
	return execs, total, nil
}

func (r *ExecutionRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(context.Background(), `DELETE FROM test_executions WHERE id = $1`, id)
	return err
}

func (r *ExecutionRepository) DeleteByTestID(testID uuid.UUID) (int64, error) {
	tag, err := r.db.Exec(context.Background(),
		`DELETE FROM test_executions WHERE test_id = $1 AND status::text NOT IN ('PENDING', 'RUNNING')`, testID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (r *ExecutionRepository) CountRunningByUser(userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM test_executions WHERE user_id = $1 AND status::text IN ('PENDING', 'RUNNING')`,
		userID,
	).Scan(&count)
	return count, err
}

func (r *ExecutionRepository) MarkOrphansAsFailed() (int, error) {
	now := time.Now()
	tag, err := r.db.Exec(context.Background(),
		`UPDATE test_executions SET status='FAILED'::test_status, error_message='Server restarted', completed_at=$1, updated_at=$1
		WHERE status::text IN ('PENDING', 'RUNNING')`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

func (r *ExecutionRepository) GetStats() (map[string]interface{}, error) {
	stats := map[string]interface{}{}

	var totalTests int64
	r.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM tests WHERE deleted_at IS NULL").Scan(&totalTests)
	stats["total_tests"] = totalTests

	var runningNow int64
	r.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_executions WHERE status::text = 'RUNNING'").Scan(&runningNow)
	stats["running_now"] = runningNow

	var completedToday int64
	r.db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM test_executions WHERE status::text = 'COMPLETED' AND created_at >= CURRENT_DATE",
	).Scan(&completedToday)
	stats["completed_today"] = completedToday

	var failedToday int64
	r.db.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM test_executions WHERE status::text IN ('FAILED', 'TIMEOUT') AND created_at >= CURRENT_DATE",
	).Scan(&failedToday)
	stats["failed_today"] = failedToday

	var totalExecutions int64
	r.db.QueryRow(context.Background(), "SELECT COUNT(*) FROM test_executions").Scan(&totalExecutions)
	stats["total_executions"] = totalExecutions

	return stats, nil
}
