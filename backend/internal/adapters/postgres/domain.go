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

type DomainRepository struct {
	db *pgxpool.Pool
}

func NewDomainRepository(db *pgxpool.Pool) *DomainRepository {
	return &DomainRepository{db: db}
}

func (r *DomainRepository) Create(d *domain.Domain) error {
	d.ID = uuid.New()
	d.CreatedAt = time.Now()
	d.UpdatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO domains (id, user_id, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.UserID, d.Name, d.Description, d.CreatedAt, d.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.NewConflictError("Domain with this name already exists")
		}
		return err
	}
	return nil
}

func (r *DomainRepository) GetByID(id uuid.UUID) (*domain.Domain, error) {
	d := &domain.Domain{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, user_id, name, description, created_at, updated_at, deleted_at
		FROM domains WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDomainNotFound
		}
		return nil, err
	}
	return d, nil
}

func (r *DomainRepository) GetByUserAndName(userID uuid.UUID, name string) (*domain.Domain, error) {
	d := &domain.Domain{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, user_id, name, description, created_at, updated_at, deleted_at
		FROM domains WHERE user_id = $1 AND name = $2 AND deleted_at IS NULL`, userID, name,
	).Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDomainNotFound
		}
		return nil, err
	}
	return d, nil
}

func (r *DomainRepository) Update(d *domain.Domain) error {
	d.UpdatedAt = time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE domains SET name=$1, description=$2, updated_at=$3 WHERE id=$4 AND deleted_at IS NULL`,
		d.Name, d.Description, d.UpdatedAt, d.ID,
	)
	return err
}

func (r *DomainRepository) Delete(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE domains SET deleted_at=$1, updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		now, id,
	)
	return err
}

func (r *DomainRepository) List(filter domain.DomainFilter) ([]domain.Domain, int64, error) {
	where := []string{"deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRow(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM domains WHERE %s", whereClause), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, user_id, name, description, created_at, updated_at, deleted_at
		FROM domains WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit(), filter.Offset())

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var domains []domain.Domain
	for rows.Next() {
		var d domain.Domain
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.Description, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt); err != nil {
			return nil, 0, err
		}
		domains = append(domains, d)
	}

	if domains == nil {
		domains = []domain.Domain{}
	}
	return domains, total, nil
}
