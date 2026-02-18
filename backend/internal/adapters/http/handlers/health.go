package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type HealthHandler struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	config *config.Config
}

func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client, cfg *config.Config) *HealthHandler {
	return &HealthHandler{db: db, redis: redis, config: cfg}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.OK(w, map[string]interface{}{
		"status":  "ok",
		"service": h.config.App.Name,
		"project": h.config.App.ProjectName,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["postgres"] = "ok"
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}

	response.JSON(w, status, map[string]interface{}{
		"status": map[bool]string{true: "ok", false: "degraded"}[healthy],
		"checks": checks,
	})
}
