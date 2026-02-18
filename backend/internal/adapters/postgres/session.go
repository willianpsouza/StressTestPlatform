package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type SessionRepository struct {
	db *pgxpool.Pool
}

func NewSessionRepository(db *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *domain.Session) error {
	session.ID = uuid.New()
	session.CreatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO sessions (id, user_id, token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		session.ID, session.UserID, session.TokenHash,
		session.UserAgent, session.IPAddress,
		session.ExpiresAt, session.CreatedAt,
	)
	return err
}

func (r *SessionRepository) GetByTokenHash(hash string) (*domain.Session, error) {
	session := &domain.Session{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, user_id, token_hash, user_agent, ip_address, expires_at, created_at, revoked_at
		FROM sessions WHERE token_hash = $1`, hash,
	).Scan(
		&session.ID, &session.UserID, &session.TokenHash,
		&session.UserAgent, &session.IPAddress,
		&session.ExpiresAt, &session.CreatedAt, &session.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return session, nil
}

func (r *SessionRepository) Revoke(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE sessions SET revoked_at = $1 WHERE id = $2`, now, id,
	)
	return err
}

func (r *SessionRepository) RevokeAllForUser(userID uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE sessions SET revoked_at = $1 WHERE user_id = $2 AND revoked_at IS NULL`, now, userID,
	)
	return err
}

func (r *SessionRepository) CleanExpired() error {
	_, err := r.db.Exec(context.Background(),
		`DELETE FROM sessions WHERE expires_at < NOW() OR revoked_at IS NOT NULL`,
	)
	return err
}
