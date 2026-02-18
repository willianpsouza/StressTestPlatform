package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SettingsRepository struct {
	pool *pgxpool.Pool
}

func NewSettingsRepository(pool *pgxpool.Pool) *SettingsRepository {
	return &SettingsRepository{pool: pool}
}

func (r *SettingsRepository) Get(key string) (string, error) {
	var value string
	err := r.pool.QueryRow(context.Background(),
		"SELECT value FROM system_settings WHERE key = $1", key).Scan(&value)
	if err != nil {
		return "", nil // Key not found, return empty
	}
	return value, nil
}

func (r *SettingsRepository) Set(key, value string) error {
	_, err := r.pool.Exec(context.Background(), `
		INSERT INTO system_settings (key, value, updated_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $3`,
		key, value, time.Now().UTC())
	return err
}

func (r *SettingsRepository) GetAll() (map[string]string, error) {
	rows, err := r.pool.Query(context.Background(),
		"SELECT key, value FROM system_settings ORDER BY key")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, nil
}
