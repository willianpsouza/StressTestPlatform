package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *domain.User) error {
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, name, role, status, grafana_user_id, grafana_username, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::user_role, $6::user_status, $7, $8, $9, $10)`,
		user.ID, user.Email, user.PasswordHash, user.Name,
		string(user.Role), string(user.Status),
		user.GrafanaUserID, user.GrafanaUsername,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.NewConflictError("Email already exists")
		}
		return err
	}
	return nil
}

func (r *UserRepository) GetByID(id uuid.UUID) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, email, password_hash, name, role::text, status::text,
			grafana_user_id, grafana_username, last_login_at,
			created_at, updated_at, deleted_at
		FROM users WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.Role, &user.Status,
		&user.GrafanaUserID, &user.GrafanaUsername, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, email, password_hash, name, role::text, status::text,
			grafana_user_id, grafana_username, last_login_at,
			created_at, updated_at, deleted_at
		FROM users WHERE email = $1 AND deleted_at IS NULL`, email,
	).Scan(
		&user.ID, &user.Email, &user.PasswordHash, &user.Name,
		&user.Role, &user.Status,
		&user.GrafanaUserID, &user.GrafanaUsername, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt, &user.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) Update(user *domain.User) error {
	user.UpdatedAt = time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE users SET email=$1, password_hash=$2, name=$3, role=$4::user_role, status=$5::user_status,
			grafana_user_id=$6, grafana_username=$7, last_login_at=$8, updated_at=$9
		WHERE id = $10 AND deleted_at IS NULL`,
		user.Email, user.PasswordHash, user.Name,
		string(user.Role), string(user.Status),
		user.GrafanaUserID, user.GrafanaUsername, user.LastLoginAt,
		user.UpdatedAt, user.ID,
	)
	return err
}

func (r *UserRepository) Delete(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE users SET deleted_at = $1, updated_at = $1 WHERE id = $2 AND deleted_at IS NULL`,
		now, id,
	)
	return err
}

func (r *UserRepository) List(filter domain.UserFilter) ([]domain.User, int64, error) {
	where := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter.Role != nil {
		where = append(where, fmt.Sprintf("role::text = $%d", argIdx))
		args = append(args, string(*filter.Role))
		argIdx++
	}
	if filter.Status != nil {
		where = append(where, fmt.Sprintf("status::text = $%d", argIdx))
		args = append(args, string(*filter.Status))
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR email ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	// Count
	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users WHERE %s", whereClause)
	if err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch
	query := fmt.Sprintf(
		`SELECT id, email, password_hash, name, role::text, status::text,
			grafana_user_id, grafana_username, last_login_at,
			created_at, updated_at, deleted_at
		FROM users WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit(), filter.Offset())

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(
			&u.ID, &u.Email, &u.PasswordHash, &u.Name,
			&u.Role, &u.Status,
			&u.GrafanaUserID, &u.GrafanaUsername, &u.LastLoginAt,
			&u.CreatedAt, &u.UpdatedAt, &u.DeletedAt,
		); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}

	if users == nil {
		users = []domain.User{}
	}

	return users, total, nil
}
