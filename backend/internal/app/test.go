package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type TestService struct {
	testRepo   domain.TestRepository
	domainRepo domain.DomainRepository
	k6Config   config.K6Config
}

func NewTestService(
	testRepo domain.TestRepository,
	domainRepo domain.DomainRepository,
	k6Config config.K6Config,
) *TestService {
	return &TestService{
		testRepo:   testRepo,
		domainRepo: domainRepo,
		k6Config:   k6Config,
	}
}

func (s *TestService) Create(userID uuid.UUID, input domain.CreateTestInput, filename string, scriptReader io.Reader, scriptSize int64) (*domain.Test, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError(map[string]string{
			"name": "Name is required",
		})
	}

	if !strings.HasSuffix(strings.ToLower(filename), ".js") {
		return nil, domain.NewValidationError(map[string]string{
			"script": "Script must be a .js file",
		})
	}

	if scriptSize > 1024*1024 {
		return nil, domain.NewValidationError(map[string]string{
			"script": "Script must be less than 1MB",
		})
	}

	// Verify domain ownership
	d, err := s.domainRepo.GetByID(input.DomainID)
	if err != nil {
		return nil, err
	}
	if d.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	// Check name uniqueness within domain
	existing, _ := s.testRepo.GetByDomainAndName(input.DomainID, input.Name)
	if existing != nil {
		return nil, domain.NewConflictError("Test with this name already exists in this domain")
	}

	// Generate test ID
	testID := uuid.New()

	// Save script to disk
	scriptDir := filepath.Join(s.k6Config.ScriptsPath, userID.String(), d.ID.String())
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create script directory: %w", err)
	}

	scriptPath := filepath.Join(scriptDir, testID.String()+".js")
	f, err := os.Create(scriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create script file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, scriptReader)
	if err != nil {
		os.Remove(scriptPath)
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	// Set defaults
	vus := input.DefaultVUs
	if vus <= 0 {
		vus = 1
	}
	duration := input.DefaultDuration
	if duration == "" {
		duration = "30s"
	}

	test := &domain.Test{
		ID:              testID,
		DomainID:        input.DomainID,
		UserID:          userID,
		Name:            input.Name,
		Description:     input.Description,
		ScriptFilename:  filename,
		ScriptPath:      scriptPath,
		ScriptSizeBytes: written,
		DefaultVUs:      vus,
		DefaultDuration: duration,
	}

	if err := s.testRepo.Create(test); err != nil {
		os.Remove(scriptPath)
		return nil, err
	}

	return test, nil
}

func (s *TestService) GetByID(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.Test, error) {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && t.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}
	return t, nil
}

func (s *TestService) Update(id uuid.UUID, userID uuid.UUID, isRoot bool, input domain.UpdateTestInput) (*domain.Test, error) {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && t.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if input.Name != nil {
		existing, _ := s.testRepo.GetByDomainAndName(t.DomainID, *input.Name)
		if existing != nil && existing.ID != t.ID {
			return nil, domain.NewConflictError("Test with this name already exists in this domain")
		}
		t.Name = *input.Name
	}
	if input.Description != nil {
		t.Description = input.Description
	}
	if input.DefaultVUs != nil {
		t.DefaultVUs = *input.DefaultVUs
	}
	if input.DefaultDuration != nil {
		t.DefaultDuration = *input.DefaultDuration
	}

	if err := s.testRepo.Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TestService) UpdateScript(id uuid.UUID, userID uuid.UUID, isRoot bool, filename string, reader io.Reader, size int64) (*domain.Test, error) {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && t.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if !strings.HasSuffix(strings.ToLower(filename), ".js") {
		return nil, domain.NewValidationError(map[string]string{
			"script": "Script must be a .js file",
		})
	}
	if size > 1024*1024 {
		return nil, domain.NewValidationError(map[string]string{
			"script": "Script must be less than 1MB",
		})
	}

	f, err := os.Create(t.ScriptPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create script file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}

	t.ScriptFilename = filename
	t.ScriptSizeBytes = written

	if err := s.testRepo.Update(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TestService) Delete(id uuid.UUID, userID uuid.UUID, isRoot bool) error {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return err
	}
	if !isRoot && t.UserID != userID {
		return domain.NewForbiddenError("Access denied")
	}

	// Remove script file
	os.Remove(t.ScriptPath)

	return s.testRepo.Delete(id)
}

func (s *TestService) GetScriptContent(id uuid.UUID, userID uuid.UUID, isRoot bool) (string, error) {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return "", err
	}
	if !isRoot && t.UserID != userID {
		return "", domain.NewForbiddenError("Access denied")
	}

	content, err := os.ReadFile(t.ScriptPath)
	if err != nil {
		return "", fmt.Errorf("failed to read script: %w", err)
	}
	return string(content), nil
}

func (s *TestService) SaveScriptContent(id uuid.UUID, userID uuid.UUID, isRoot bool, content string) (*domain.Test, error) {
	t, err := s.testRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && t.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if len(content) > 1024*1024 {
		return nil, domain.NewValidationError(map[string]string{
			"content": "Script must be less than 1MB",
		})
	}

	if err := os.WriteFile(t.ScriptPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script: %w", err)
	}

	t.ScriptSizeBytes = int64(len(content))
	if err := s.testRepo.Update(t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *TestService) List(filter domain.TestFilter) ([]domain.Test, int64, error) {
	return s.testRepo.List(filter)
}
