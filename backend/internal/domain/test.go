package domain

import (
	"time"

	"github.com/google/uuid"
)

type Test struct {
	ID              uuid.UUID  `json:"id"`
	DomainID        uuid.UUID  `json:"domain_id"`
	UserID          uuid.UUID  `json:"user_id"`
	Name            string     `json:"name"`
	Description     *string    `json:"description,omitempty"`
	ScriptFilename  string     `json:"script_filename"`
	ScriptPath      string     `json:"-"`
	ScriptSizeBytes int64      `json:"script_size_bytes"`
	DefaultVUs      int        `json:"default_vus"`
	DefaultDuration string     `json:"default_duration"`
	InfluxDBBucket  string     `json:"influxdb_bucket"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`

	// Joined fields
	DomainName *string `json:"domain_name,omitempty"`
	UserName   *string `json:"user_name,omitempty"`
	UserEmail  *string `json:"user_email,omitempty"`
}

type CreateTestInput struct {
	DomainID        uuid.UUID `json:"domain_id"`
	Name            string    `json:"name"`
	Description     *string   `json:"description,omitempty"`
	DefaultVUs      int       `json:"default_vus"`
	DefaultDuration string    `json:"default_duration"`
}

type UpdateTestInput struct {
	Name            *string `json:"name,omitempty"`
	Description     *string `json:"description,omitempty"`
	DefaultVUs      *int    `json:"default_vus,omitempty"`
	DefaultDuration *string `json:"default_duration,omitempty"`
}

type TestFilter struct {
	UserID   *uuid.UUID `json:"user_id,omitempty"`
	DomainID *uuid.UUID `json:"domain_id,omitempty"`
	Search   *string    `json:"search,omitempty"`
	Pagination
}

type TestRepository interface {
	Create(test *Test) error
	GetByID(id uuid.UUID) (*Test, error)
	GetByDomainAndName(domainID uuid.UUID, name string) (*Test, error)
	Update(test *Test) error
	Delete(id uuid.UUID) error
	List(filter TestFilter) ([]Test, int64, error)
}
