package app

import (
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type ScheduleService struct {
	scheduleRepo domain.ScheduleRepository
	testRepo     domain.TestRepository
}

func NewScheduleService(scheduleRepo domain.ScheduleRepository, testRepo domain.TestRepository) *ScheduleService {
	return &ScheduleService{
		scheduleRepo: scheduleRepo,
		testRepo:     testRepo,
	}
}

func (s *ScheduleService) Create(userID uuid.UUID, input domain.CreateScheduleInput) (*domain.Schedule, error) {
	// Verify test ownership
	test, err := s.testRepo.GetByID(input.TestID)
	if err != nil {
		return nil, err
	}
	if test.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if input.ScheduleType == domain.ScheduleTypeRecurring && (input.CronExpression == nil || *input.CronExpression == "") {
		return nil, domain.NewValidationError(map[string]string{
			"cron_expression": "Cron expression is required for recurring schedules",
		})
	}

	if input.ScheduleType == domain.ScheduleTypeOnce && input.NextRunAt == nil {
		return nil, domain.NewValidationError(map[string]string{
			"next_run_at": "Next run time is required for one-time schedules",
		})
	}

	vus := input.VUs
	if vus <= 0 {
		vus = test.DefaultVUs
	}
	duration := input.Duration
	if duration == "" {
		duration = test.DefaultDuration
	}

	schedule := &domain.Schedule{
		TestID:         input.TestID,
		UserID:         userID,
		ScheduleType:   input.ScheduleType,
		CronExpression: input.CronExpression,
		NextRunAt:      input.NextRunAt,
		VUs:            vus,
		Duration:       duration,
		Status:         domain.ScheduleStatusActive,
	}

	if err := s.scheduleRepo.Create(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *ScheduleService) GetByID(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.Schedule, error) {
	schedule, err := s.scheduleRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && schedule.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}
	return schedule, nil
}

func (s *ScheduleService) Update(id uuid.UUID, userID uuid.UUID, isRoot bool, input domain.UpdateScheduleInput) (*domain.Schedule, error) {
	schedule, err := s.scheduleRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && schedule.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if input.CronExpression != nil {
		schedule.CronExpression = input.CronExpression
	}
	if input.NextRunAt != nil {
		schedule.NextRunAt = input.NextRunAt
	}
	if input.VUs != nil {
		schedule.VUs = *input.VUs
	}
	if input.Duration != nil {
		schedule.Duration = *input.Duration
	}

	if err := s.scheduleRepo.Update(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *ScheduleService) Delete(id uuid.UUID, userID uuid.UUID, isRoot bool) error {
	schedule, err := s.scheduleRepo.GetByID(id)
	if err != nil {
		return err
	}
	if !isRoot && schedule.UserID != userID {
		return domain.NewForbiddenError("Access denied")
	}
	return s.scheduleRepo.Delete(id)
}

func (s *ScheduleService) Pause(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.Schedule, error) {
	schedule, err := s.scheduleRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && schedule.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	schedule.Status = domain.ScheduleStatusPaused
	if err := s.scheduleRepo.Update(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *ScheduleService) Resume(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.Schedule, error) {
	schedule, err := s.scheduleRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && schedule.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	schedule.Status = domain.ScheduleStatusActive
	if err := s.scheduleRepo.Update(schedule); err != nil {
		return nil, err
	}
	return schedule, nil
}

func (s *ScheduleService) List(filter domain.ScheduleFilter) ([]domain.Schedule, int64, error) {
	return s.scheduleRepo.List(filter)
}
