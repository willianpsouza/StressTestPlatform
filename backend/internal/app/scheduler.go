package app

import (
	"log"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type Scheduler struct {
	scheduleRepo domain.ScheduleRepository
	execRepo     domain.ExecutionRepository
	runner       *K6Runner
	ticker       *time.Ticker
	done         chan bool
}

func NewScheduler(
	scheduleRepo domain.ScheduleRepository,
	execRepo domain.ExecutionRepository,
	runner *K6Runner,
) *Scheduler {
	return &Scheduler{
		scheduleRepo: scheduleRepo,
		execRepo:     execRepo,
		runner:       runner,
		done:         make(chan bool),
	}
}

func (s *Scheduler) Start() {
	s.ticker = time.NewTicker(10 * time.Second)
	log.Println("[Scheduler] Started (polling every 10s)")

	go func() {
		for {
			select {
			case <-s.ticker.C:
				s.poll()
			case <-s.done:
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	if s.ticker != nil {
		s.ticker.Stop()
	}
	s.done <- true
	log.Println("[Scheduler] Stopped")
}

func (s *Scheduler) poll() {
	schedules, err := s.scheduleRepo.GetDueSchedules()
	if err != nil {
		log.Printf("[Scheduler] Failed to get due schedules: %v", err)
		return
	}

	for _, schedule := range schedules {
		s.executeSchedule(&schedule)
	}
}

func (s *Scheduler) executeSchedule(schedule *domain.Schedule) {
	log.Printf("[Scheduler] Executing schedule %s for test %s", schedule.ID, schedule.TestID)

	// Create execution
	exec := &domain.TestExecution{
		TestID:     schedule.TestID,
		UserID:     schedule.UserID,
		ScheduleID: &schedule.ID,
		VUs:        schedule.VUs,
		Duration:   schedule.Duration,
		Status:     domain.TestStatusPending,
	}

	if err := s.execRepo.Create(exec); err != nil {
		log.Printf("[Scheduler] Failed to create execution for schedule %s: %v", schedule.ID, err)
		return
	}

	// Start execution
	if err := s.runner.Run(exec); err != nil {
		log.Printf("[Scheduler] Failed to start execution for schedule %s: %v", schedule.ID, err)
		exec.Status = domain.TestStatusFailed
		errMsg := err.Error()
		exec.ErrorMessage = &errMsg
		s.execRepo.Update(exec)
	}

	// Update schedule
	now := time.Now()
	schedule.LastRunAt = &now
	schedule.RunCount++

	if schedule.ScheduleType == domain.ScheduleTypeOnce {
		schedule.Status = domain.ScheduleStatusCompleted
		schedule.NextRunAt = nil
	} else if schedule.ScheduleType == domain.ScheduleTypeRecurring {
		if schedule.CronExpression != nil {
			nextRun, err := getNextCronRun(*schedule.CronExpression)
			if err != nil {
				log.Printf("[Scheduler] Failed to parse cron for schedule %s: %v", schedule.ID, err)
				schedule.Status = domain.ScheduleStatusPaused
			} else {
				schedule.NextRunAt = &nextRun
			}
		}
	}

	if err := s.scheduleRepo.Update(schedule); err != nil {
		log.Printf("[Scheduler] Failed to update schedule %s: %v", schedule.ID, err)
	}
}

func getNextCronRun(expression string) (time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expression)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(time.Now()), nil
}
