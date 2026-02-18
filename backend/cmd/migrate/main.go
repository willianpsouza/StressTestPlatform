package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: migrate [up|down]")
	}

	direction := os.Args[1]
	if direction != "up" && direction != "down" {
		log.Fatal("Usage: migrate [up|down]")
	}

	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.Database.URL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create migrations tracking table
	_, err = pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create migrations table: %v", err)
	}

	migrationsDir := "/app/migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "migrations"
	}

	suffix := fmt.Sprintf(".%s.sql", direction)
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*"+suffix))
	if err != nil {
		log.Fatalf("Failed to read migrations: %v", err)
	}

	sort.Strings(files)
	if direction == "down" {
		sort.Sort(sort.Reverse(sort.StringSlice(files)))
	}

	for _, file := range files {
		version := extractVersion(filepath.Base(file))

		if direction == "up" {
			var exists bool
			err := pool.QueryRow(context.Background(),
				"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", version).Scan(&exists)
			if err != nil {
				log.Fatalf("Failed to check migration %s: %v", version, err)
			}
			if exists {
				log.Printf("Skipping %s (already applied)", version)
				continue
			}
		}

		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", file, err)
		}

		log.Printf("Applying %s (%s)...", filepath.Base(file), direction)

		_, err = pool.Exec(context.Background(), string(content))
		if err != nil {
			log.Fatalf("Failed to apply %s: %v", filepath.Base(file), err)
		}

		if direction == "up" {
			_, err = pool.Exec(context.Background(),
				"INSERT INTO schema_migrations (version) VALUES ($1)", version)
		} else {
			_, err = pool.Exec(context.Background(),
				"DELETE FROM schema_migrations WHERE version=$1", version)
		}
		if err != nil {
			log.Fatalf("Failed to track migration %s: %v", version, err)
		}

		log.Printf("Applied %s", filepath.Base(file))
	}

	log.Println("Migrations complete")
}

func extractVersion(filename string) string {
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return filename
}
