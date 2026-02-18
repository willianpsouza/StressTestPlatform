package app

import (
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type DomainService struct {
	domainRepo domain.DomainRepository
}

func NewDomainService(domainRepo domain.DomainRepository) *DomainService {
	return &DomainService{domainRepo: domainRepo}
}

func (s *DomainService) Create(userID uuid.UUID, input domain.CreateDomainInput) (*domain.Domain, error) {
	if input.Name == "" {
		return nil, domain.NewValidationError(map[string]string{
			"name": "Name is required",
		})
	}

	existing, _ := s.domainRepo.GetByUserAndName(userID, input.Name)
	if existing != nil {
		return nil, domain.NewConflictError("Domain with this name already exists")
	}

	d := &domain.Domain{
		UserID:      userID,
		Name:        input.Name,
		Description: input.Description,
	}

	if err := s.domainRepo.Create(d); err != nil {
		return nil, err
	}

	return d, nil
}

func (s *DomainService) GetByID(id uuid.UUID, userID uuid.UUID, isRoot bool) (*domain.Domain, error) {
	d, err := s.domainRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && d.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}
	return d, nil
}

func (s *DomainService) Update(id uuid.UUID, userID uuid.UUID, isRoot bool, input domain.UpdateDomainInput) (*domain.Domain, error) {
	d, err := s.domainRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if !isRoot && d.UserID != userID {
		return nil, domain.NewForbiddenError("Access denied")
	}

	if input.Name != nil {
		// Check for name uniqueness
		existing, _ := s.domainRepo.GetByUserAndName(d.UserID, *input.Name)
		if existing != nil && existing.ID != d.ID {
			return nil, domain.NewConflictError("Domain with this name already exists")
		}
		d.Name = *input.Name
	}
	if input.Description != nil {
		d.Description = input.Description
	}

	if err := s.domainRepo.Update(d); err != nil {
		return nil, err
	}
	return d, nil
}

func (s *DomainService) Delete(id uuid.UUID, userID uuid.UUID, isRoot bool) error {
	d, err := s.domainRepo.GetByID(id)
	if err != nil {
		return err
	}
	if !isRoot && d.UserID != userID {
		return domain.NewForbiddenError("Access denied")
	}
	return s.domainRepo.Delete(id)
}

func (s *DomainService) List(filter domain.DomainFilter) ([]domain.Domain, int64, error) {
	return s.domainRepo.List(filter)
}
