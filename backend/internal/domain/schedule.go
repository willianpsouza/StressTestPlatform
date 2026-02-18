package domain

import (
	"time"

	"github.com/google/uuid"
)

type ScheduleType string

const (
	ScheduleTypeOnce      ScheduleType = "ONCE"
	ScheduleTypeRecurring ScheduleType = "RECURRING"
)

type ScheduleStatus string

const (
	ScheduleStatusActive    ScheduleStatus = "ACTIVE"
	ScheduleStatusPaused    ScheduleStatus = "PAUSED"
	ScheduleStatusCompleted ScheduleStatus = "COMPLETED"
	ScheduleStatusCancelled ScheduleStatus = "CANCELLED"
)

type Schedule struct {
	ID             uuid.UUID      `json:"id"`
	TestID         uuid.UUID      `json:"test_id"`
	UserID         uuid.UUID      `json:"user_id"`
	ScheduleType   ScheduleType   `json:"schedule_type"`
	CronExpression *string        `json:"cron_expression,omitempty"`
	NextRunAt      *time.Time     `json:"next_run_at,omitempty"`
	VUs            int            `json:"vus"`
	Duration       string         `json:"duration"`
	Status         ScheduleStatus `json:"status"`
	LastRunAt      *time.Time     `json:"last_run_at,omitempty"`
	RunCount       int            `json:"run_count"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`

	// Joined fields
	TestName   *string `json:"test_name,omitempty"`
	DomainName *string `json:"domain_name,omitempty"`
}

type CreateScheduleInput struct {
	TestID         uuid.UUID    `json:"test_id"`
	ScheduleType   ScheduleType `json:"schedule_type"`
	CronExpression *string      `json:"cron_expression,omitempty"`
	NextRunAt      *time.Time   `json:"next_run_at,omitempty"`
	VUs            int          `json:"vus"`
	Duration       string       `json:"duration"`
}

type UpdateScheduleInput struct {
	CronExpression *string    `json:"cron_expression,omitempty"`
	NextRunAt      *time.Time `json:"next_run_at,omitempty"`
	VUs            *int       `json:"vus,omitempty"`
	Duration       *string    `json:"duration,omitempty"`
}

type ScheduleFilter struct {
	UserID *uuid.UUID      `json:"user_id,omitempty"`
	TestID *uuid.UUID      `json:"test_id,omitempty"`
	Status *ScheduleStatus `json:"status,omitempty"`
	Pagination
}

type ScheduleRepository interface {
	Create(schedule *Schedule) error
	GetByID(id uuid.UUID) (*Schedule, error)
	Update(schedule *Schedule) error
	Delete(id uuid.UUID) error
	List(filter ScheduleFilter) ([]Schedule, int64, error)
	GetDueSchedules() ([]Schedule, error)
}
