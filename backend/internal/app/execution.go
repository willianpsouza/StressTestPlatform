package app

import (
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type ExecutionService struct {
	execRepo   domain.ExecutionRepository
	testRepo   domain.TestRepository
	metricRepo domain.MetricRepository
	runner     *K6Runner
}

func NewExecutionService(
	execRepo domain.ExecutionRepository,
	testRepo domain.TestRepository,
	metricRepo domain.MetricRepository,
	runner *K6Runner,
) *ExecutionService {
	return &ExecutionService{
		execRepo:   execRepo,
		testRepo:   testRepo,
		metricRepo: metricRepo,
		runner:     runner,
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

func (s *ExecutionService) Delete(id uuid.UUID, userID uuid.UUID, isRoot bool) error {
	exec, err := s.execRepo.GetByID(id)
	if err != nil {
		return err
	}
	if !isRoot && exec.UserID != userID {
		return domain.NewForbiddenError("Access denied")
	}
	if exec.Status == domain.TestStatusRunning || exec.Status == domain.TestStatusPending {
		return domain.NewValidationError(map[string]string{
			"status": "Cannot delete running or pending executions",
		})
	}

	// Delete associated metrics first
	s.metricRepo.DeleteByExecution(exec.ID)

	return s.execRepo.Delete(id)
}

func (s *ExecutionService) DeleteByTestID(testID uuid.UUID, userID uuid.UUID, isRoot bool) (int64, error) {
	test, err := s.testRepo.GetByID(testID)
	if err != nil {
		return 0, err
	}
	if !isRoot && test.UserID != userID {
		return 0, domain.NewForbiddenError("Access denied")
	}

	// Get all finished executions for this test to delete their metrics
	filter := domain.ExecutionFilter{
		TestID:     &testID,
		Pagination: domain.Pagination{Page: 1, PageSize: 10000},
	}
	if !isRoot {
		filter.UserID = &userID
	}
	execs, _, err := s.execRepo.List(filter)
	if err != nil {
		return 0, err
	}
	for _, e := range execs {
		if e.Status != domain.TestStatusRunning && e.Status != domain.TestStatusPending {
			s.metricRepo.DeleteByExecution(e.ID)
		}
	}

	return s.execRepo.DeleteByTestID(testID)
}

func (s *ExecutionService) List(filter domain.ExecutionFilter) ([]domain.TestExecution, int64, error) {
	return s.execRepo.List(filter)
}

func (s *ExecutionService) GetStats() (map[string]interface{}, error) {
	return s.execRepo.GetStats()
}
