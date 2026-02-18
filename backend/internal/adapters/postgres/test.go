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

type TestRepository struct {
	db *pgxpool.Pool
}

func NewTestRepository(db *pgxpool.Pool) *TestRepository {
	return &TestRepository{db: db}
}

func (r *TestRepository) Create(t *domain.Test) error {
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()

	_, err := r.db.Exec(context.Background(),
		`INSERT INTO tests (id, domain_id, user_id, name, description, script_filename, script_path,
			script_size_bytes, default_vus, default_duration, influxdb_bucket, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		t.ID, t.DomainID, t.UserID, t.Name, t.Description, t.ScriptFilename, t.ScriptPath,
		t.ScriptSizeBytes, t.DefaultVUs, t.DefaultDuration, t.InfluxDBBucket,
		t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return domain.NewConflictError("Test with this name already exists in this domain")
		}
		return err
	}
	return nil
}

func (r *TestRepository) GetByID(id uuid.UUID) (*domain.Test, error) {
	t := &domain.Test{}
	err := r.db.QueryRow(context.Background(),
		`SELECT t.id, t.domain_id, t.user_id, t.name, t.description,
			t.script_filename, t.script_path, t.script_size_bytes,
			t.default_vus, t.default_duration, t.influxdb_bucket,
			t.created_at, t.updated_at, t.deleted_at,
			d.name, u.name, u.email
		FROM tests t
		JOIN domains d ON d.id = t.domain_id
		JOIN users u ON u.id = t.user_id
		WHERE t.id = $1 AND t.deleted_at IS NULL`, id,
	).Scan(
		&t.ID, &t.DomainID, &t.UserID, &t.Name, &t.Description,
		&t.ScriptFilename, &t.ScriptPath, &t.ScriptSizeBytes,
		&t.DefaultVUs, &t.DefaultDuration, &t.InfluxDBBucket,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
		&t.DomainName, &t.UserName, &t.UserEmail,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTestNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *TestRepository) GetByDomainAndName(domainID uuid.UUID, name string) (*domain.Test, error) {
	t := &domain.Test{}
	err := r.db.QueryRow(context.Background(),
		`SELECT id, domain_id, user_id, name, description,
			script_filename, script_path, script_size_bytes,
			default_vus, default_duration, influxdb_bucket,
			created_at, updated_at, deleted_at
		FROM tests WHERE domain_id = $1 AND name = $2 AND deleted_at IS NULL`, domainID, name,
	).Scan(
		&t.ID, &t.DomainID, &t.UserID, &t.Name, &t.Description,
		&t.ScriptFilename, &t.ScriptPath, &t.ScriptSizeBytes,
		&t.DefaultVUs, &t.DefaultDuration, &t.InfluxDBBucket,
		&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTestNotFound
		}
		return nil, err
	}
	return t, nil
}

func (r *TestRepository) Update(t *domain.Test) error {
	t.UpdatedAt = time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE tests SET name=$1, description=$2, script_filename=$3, script_path=$4,
			script_size_bytes=$5, default_vus=$6, default_duration=$7, updated_at=$8
		WHERE id=$9 AND deleted_at IS NULL`,
		t.Name, t.Description, t.ScriptFilename, t.ScriptPath,
		t.ScriptSizeBytes, t.DefaultVUs, t.DefaultDuration, t.UpdatedAt, t.ID,
	)
	return err
}

func (r *TestRepository) Delete(id uuid.UUID) error {
	now := time.Now()
	_, err := r.db.Exec(context.Background(),
		`UPDATE tests SET deleted_at=$1, updated_at=$1 WHERE id=$2 AND deleted_at IS NULL`,
		now, id,
	)
	return err
}

func (r *TestRepository) List(filter domain.TestFilter) ([]domain.Test, int64, error) {
	where := []string{"t.deleted_at IS NULL"}
	args := []interface{}{}
	argIdx := 1

	if filter.UserID != nil {
		where = append(where, fmt.Sprintf("t.user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}
	if filter.DomainID != nil {
		where = append(where, fmt.Sprintf("t.domain_id = $%d", argIdx))
		args = append(args, *filter.DomainID)
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		where = append(where, fmt.Sprintf("(t.name ILIKE $%d OR t.description ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+*filter.Search+"%")
		argIdx++
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	err := r.db.QueryRow(context.Background(),
		fmt.Sprintf("SELECT COUNT(*) FROM tests t WHERE %s", whereClause), args...,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT t.id, t.domain_id, t.user_id, t.name, t.description,
			t.script_filename, t.script_path, t.script_size_bytes,
			t.default_vus, t.default_duration, t.influxdb_bucket,
			t.created_at, t.updated_at, t.deleted_at,
			d.name, u.name, u.email
		FROM tests t
		JOIN domains d ON d.id = t.domain_id
		JOIN users u ON u.id = t.user_id
		WHERE %s ORDER BY t.created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1,
	)
	args = append(args, filter.Limit(), filter.Offset())

	rows, err := r.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tests []domain.Test
	for rows.Next() {
		var t domain.Test
		if err := rows.Scan(
			&t.ID, &t.DomainID, &t.UserID, &t.Name, &t.Description,
			&t.ScriptFilename, &t.ScriptPath, &t.ScriptSizeBytes,
			&t.DefaultVUs, &t.DefaultDuration, &t.InfluxDBBucket,
			&t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
			&t.DomainName, &t.UserName, &t.UserEmail,
		); err != nil {
			return nil, 0, err
		}
		tests = append(tests, t)
	}

	if tests == nil {
		tests = []domain.Test{}
	}
	return tests, total, nil
}
