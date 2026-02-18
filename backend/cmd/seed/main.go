package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

func main() {
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	email := getEnv("SEED_ROOT_EMAIL", "admin@stresstest.local")
	password := getEnv("SEED_ROOT_PASSWORD", "admin123")
	name := getEnv("SEED_ROOT_NAME", "Admin")

	// Check if ROOT user already exists
	var exists bool
	err = pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)", email,
	).Scan(&exists)
	if err != nil {
		log.Fatalf("Failed to check existing user: %v", err)
	}

	if exists {
		log.Printf("ROOT user %s already exists, skipping", email)
		return
	}

	passwordHash, err := app.HashPassword(password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	id := uuid.New()
	now := time.Now()

	_, err = pool.Exec(context.Background(),
		`INSERT INTO users (id, email, password_hash, name, role, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'ROOT'::user_role, 'ACTIVE'::user_status, $5, $5)`,
		id, email, passwordHash, name, now,
	)
	if err != nil {
		log.Fatalf("Failed to create ROOT user: %v", err)
	}

	log.Printf("Created ROOT user: %s (%s)", name, email)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
