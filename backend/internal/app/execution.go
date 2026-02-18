package app

import (
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type ExecutionService struct {
	execRepo domain.ExecutionRepository
	testRepo domain.TestRepository
	runner   *K6Runner
}

func NewExecutionService(
	execRepo domain.ExecutionRepository,
	testRepo domain.TestRepository,
	runner *K6Runner,
) *ExecutionService {
	return &ExecutionService{
		execRepo: execRepo,
		testRepo: testRepo,
		runner:   runner,
	}
}

func (s *ExecutionService) Create(userID uuid.UUID, input domain.CreateExecutionInput) (*domain.TestExecution, error) {
	// Verify test exists and user owns it
	test, err := s.testRepo.GetByID(input.TestID)
	if err != nil {
		return nil, err
	}
	if test.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	vus := input.VUs
	if vus <= 0 {
		vus = test.DefaultVUs
	}
	duration := input.Duration
	if duration == "" {
		duration = test.DefaultDuration
	}

	exec := &domain.TestExecution{
		TestID:   input.TestID,
		UserID:   userID,
		VUs:      vus,
		Duration: duration,
		Status:   domain.TestStatusPending,
	}

	if err := s.execRepo.Create(exec); err != nil {
		return nil, err
	}

	// Start execution
	if err := s.runner.Run(exec); err != nil {
		// Mark as failed if we can't start
		exec.Status = domain.TestStatusFailed
		errMsg := err.Error()
		exec.ErrorMessage = &errMsg
		s.execRepo.Update(exec)
		return exec, nil
	}

	return exec, nil
}

func (s *ExecutionService) GetByID(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.TestExecution, error) {
	exec, err := s.execRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && exec.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}
	return exec, nil
}

func (s *ExecutionService) Cancel(id uuid.UUID, userID uuid.UUID, isRoot bool) error {
	exec, err := s.execRepo.GetByID(id)
	if err != nil {
		return err
	}
	if !isRoot && exec.UserID != userID {
		return domain.NewForbiddenError("Access denied")
	}

	if exec.Status != domain.TestStatusRunning && exec.Status != domain.TestStatusPending {
		return domain.NewValidationError(map[string]string{
			"status": "Can only cancel running or pending executions",
		})
	}

	s.runner.Cancel(exec.UserID, exec.ID)
	return nil
}

func (s *ExecutionService) List(filter domain.ExecutionFilter) ([]domain.TestExecution, int64, error) {
	return s.execRepo.List(filter)
}

func (s *ExecutionService) GetStats() (map[string]interface{}, error) {
	return s.execRepo.GetStats()
}
