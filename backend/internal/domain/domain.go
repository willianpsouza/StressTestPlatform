package domain

import (
	"time"

	"github.com/google/uuid"
)

type Domain struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-"`
}

type CreateDomainInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
}

type UpdateDomainInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type DomainFilter struct {
	UserID *uuid.UUID `json:"user_id,omitempty"`
	Search *string    `json:"search,omitempty"`
	Pagination
}

type DomainRepository interface {
	Create(domain *Domain) error
	GetByID(id uuid.UUID) (*Domain, error)
	GetByUserAndName(userID uuid.UUID, name string) (*Domain, error)
	Update(domain *Domain) error
	Delete(id uuid.UUID) error
	List(filter DomainFilter) ([]Domain, int64, error)
}
