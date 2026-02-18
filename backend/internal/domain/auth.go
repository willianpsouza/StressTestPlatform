package domain

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	UserAgent *string    `json:"user_agent,omitempty"`
	IPAddress *string    `json:"ip_address,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

func (s *Session) IsValid() bool {
	return !s.IsExpired() && !s.IsRevoked()
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	User         User   `json:"user"`
}

type TokenClaims struct {
	UserID uuid.UUID `json:"user_id"`
	Email  string    `json:"email"`
	Role   UserRole  `json:"role"`
}

type SessionRepository interface {
	Create(session *Session) error
	GetByTokenHash(hash string) (*Session, error)
	Revoke(id uuid.UUID) error
	RevokeAllForUser(userID uuid.UUID) error
	CleanExpired() error
}
