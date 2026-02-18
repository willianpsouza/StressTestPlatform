package domain

import (
	"time"

	"github.com/google/uuid"
)

type TestStatus string

const (
	TestStatusPending   TestStatus = "PENDING"
	TestStatusRunning   TestStatus = "RUNNING"
	TestStatusCompleted TestStatus = "COMPLETED"
	TestStatusFailed    TestStatus = "FAILED"
	TestStatusCancelled TestStatus = "CANCELLED"
	TestStatusTimeout   TestStatus = "TIMEOUT"
)

type TestExecution struct {
	ID             uuid.UUID  `json:"id"`
	TestID         uuid.UUID  `json:"test_id"`
	UserID         uuid.UUID  `json:"user_id"`
	ScheduleID     *uuid.UUID `json:"schedule_id,omitempty"`
	VUs            int        `json:"vus"`
	Duration       string     `json:"duration"`
	Status         TestStatus `json:"status"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	ExitCode       *int       `json:"exit_code,omitempty"`
	Stdout         *string    `json:"stdout,omitempty"`
	Stderr         *string    `json:"stderr,omitempty"`
	MetricsSummary JSONMap    `json:"metrics_summary,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Joined fields
	TestName   *string `json:"test_name,omitempty"`
	DomainName *string `json:"domain_name,omitempty"`
	UserName   *string `json:"user_name,omitempty"`
	UserEmail  *string `json:"user_email,omitempty"`
}

type CreateExecutionInput struct {
	TestID   uuid.UUID `json:"test_id"`
	VUs      int       `json:"vus"`
	Duration string    `json:"duration"`
}

type ExecutionFilter struct {
	UserID     *uuid.UUID  `json:"user_id,omitempty"`
	TestID     *uuid.UUID  `json:"test_id,omitempty"`
	Status     *TestStatus `json:"status,omitempty"`
	AllUsers   bool        `json:"all_users,omitempty"`
	Pagination
}

type ExecutionRepository interface {
	Create(exec *TestExecution) error
	GetByID(id uuid.UUID) (*TestExecution, error)
	Update(exec *TestExecution) error
	List(filter ExecutionFilter) ([]TestExecution, int64, error)
	CountRunningByUser(userID uuid.UUID) (int, error)
	MarkOrphansAsFailed() (int, error)
	GetStats() (map[string]interface{}, error)
}
