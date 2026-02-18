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

type ScheduleRepository struct {
	db *pgxpool.Pool
}

func NewScheduleRepository(db *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{db: db}
}

func (r *ScheduleRepository) Create(s *domain.Schedule) error {
	s.ID = uuid.New()
	s.CreatedAt = time.Now()
	s.UpdatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO schedules (id, test_id, user_id, schedule_type, cron_expression, next_run_at,
			vus, duration, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4::schedule_type, $5, $6, $7, $8, $9::schedule_status, $10, $11)`,
		s.ID, s.TestID, s.UserID, string(s.ScheduleType), s.CronExpression, s.NextRunAt,
		s.VUs, s.Duration, string(s.Status), s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (r *ScheduleRepository) GetByID(id uuid.UUID) (*domain.Schedule, error) {
	s := &domain.Schedule{}
	err := r.db.QueryRow(context.Background(),
		`SELECT s.id, s.test_id, s.user_id, s.schedule_type::text, s.cron_expression, s.next_run_at,
			s.vus, s.duration, s.status::text, s.last_run_at, s.run_count,
			s.created_at, s.updated_at,
			t.name, d.name
		FROM schedules s
		JOIN tests t ON t.id = s.test_id
		JOIN domains d ON d.id = t.domain_id
		WHERE s.id = $1`, id,
	).Scan(
		&s.ID, &s.TestID, &s.UserID, &s.ScheduleType, &s.CronExpression, &s.NextRunAt,
		&s.VUs, &s.Duration, &s.Status, &s.LastRunAt, &s.RunCount,
		&s.CreatedAt, &s.UpdatedAt,
		&s.TestName, &s.DomainName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrScheduleNotFound
		}
		return nil, err
	}
	return s, nil
}

func (r *ScheduleRepository) Update(s *domain.Schedule) error {
	s.UpdatedAt = time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE schedules SET cron_expression=$1, next_run_at=$2, vus=$3, duration=$4,
			status=$5::schedule_status, last_run_at=$6, run_count=$7, updated_at=$8
		WHERE id=$9`,
		s.CronExpression, s.NextRunAt, s.VUs, s.Duration,
		string(s.Status), s.LastRunAt, s.RunCount, s.UpdatedAt, s.ID,
	)
	return err
}

func (r *ScheduleRepository) Delete(id uuid.UUID) error {
	_, err := r.db.Exec(context.Background(),
		`UPDATE schedules SET status='CANCELLED'::schedule_status, updated_at=NOW() WHERE id=$1`, id,
	)
	return err
}

func (r *ScheduleRepository) List(filter domain.ScheduleFilter) ([]domain.Schedule, int64, error) {
	where := []string{"s.status::text != 'CANCELLED'"}
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		where = append(where, fmt.Sprintf("s.user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.TestID != nil {
		where = append(where, fmt.Sprintf("s.test_id = $%d", argIdx))
		args = append(args, *filter.TestID)
		argIdx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("s.status::text = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRow(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM schedules s WHERE %s", whereClause), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT s.id, s.test_id, s.user_id, s.schedule_type::text, s.cron_expression, s.next_run_at,
			s.vus, s.duration, s.status::text, s.last_run_at, s.run_count,
			s.created_at, s.updated_at,
			t.name, d.name
		FROM schedules s
		JOIN tests t ON t.id = s.test_id
		JOIN domains d ON d.id = t.domain_id
		WHERE %s ORDER BY s.created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit(), filter.Offset())

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var schedules []domain.Schedule
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(
			&s.ID, &s.TestID, &s.UserID, &s.ScheduleType, &s.CronExpression, &s.NextRunAt,
			&s.VUs, &s.Duration, &s.Status, &s.LastRunAt, &s.RunCount,
			&s.CreatedAt, &s.UpdatedAt,
			&s.TestName, &s.DomainName,
		); err != nil {
			return nil, 0, err
		}
		schedules = append(schedules, s)
	}

	if schedules == nil {
		schedules = []domain.Schedule{}
	}
	return schedules, total, nil
}

func (r *ScheduleRepository) GetDueSchedules() ([]domain.Schedule, error) {
	rows, err := r.db.Query(context.Background(),
		`SELECT s.id, s.test_id, s.user_id, s.schedule_type::text, s.cron_expression, s.next_run_at,
			s.vus, s.duration, s.status::text, s.last_run_at, s.run_count,
			s.created_at, s.updated_at
		FROM schedules s
		WHERE s.status::text = 'ACTIVE' AND s.next_run_at <= NOW()`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []domain.Schedule
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(
			&s.ID, &s.TestID, &s.UserID, &s.ScheduleType, &s.CronExpression, &s.NextRunAt,
			&s.VUs, &s.Duration, &s.Status, &s.LastRunAt, &s.RunCount,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}
