package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	UserRoleRoot UserRole = "ROOT"
	UserRoleUser UserRole = "USER"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "ACTIVE"
	UserStatusInactive  UserStatus = "INACTIVE"
	UserStatusSuspended UserStatus = "SUSPENDED"
)

type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	Name            string     `json:"name"`
	Role            UserRole   `json:"role"`
	Status          UserStatus `json:"status"`
	GrafanaUserID   *int       `json:"grafana_user_id,omitempty"`
	GrafanaUsername *string    `json:"grafana_username,omitempty"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`
}

func (u *User) IsRoot() bool {
	return u.Role == UserRoleRoot
}

type RegisterInput struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	Name            string `json:"name"`
}

type UpdateUserInput struct {
	Name   *string     `json:"name,omitempty"`
	Role   *UserRole   `json:"role,omitempty"`
	Status *UserStatus `json:"status,omitempty"`
}

type UpdateProfileInput struct {
	Name *string `json:"name,omitempty"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type UserFilter struct {
	Role   *UserRole   `json:"role,omitempty"`
	Status *UserStatus `json:"status,omitempty"`
	Search *string     `json:"search,omitempty"`
	Pagination
}

type UserRepository interface {
	Create(user *User) error
	GetByID(id uuid.UUID) (*User, error)
	GetByEmail(email string) (*User, error)
	Update(user *User) error
	Delete(id uuid.UUID) error
	List(filter UserFilter) ([]User, int64, error)
}
