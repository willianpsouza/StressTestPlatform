package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/grafana"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/postgres"
)

type ServicesHandler struct {
	db           *pgxpool.Pool
	redis        *redis.Client
	grafClient   *grafana.Client
	settingsRepo *postgres.SettingsRepository
}

func NewServicesHandler(
	db *pgxpool.Pool,
	redis *redis.Client,
	grafClient *grafana.Client,
	settingsRepo *postgres.SettingsRepository,
) *ServicesHandler {
	return &ServicesHandler{
		db:           db,
		redis:        redis,
		grafClient:   grafClient,
		settingsRepo: settingsRepo,
	}
}

type serviceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func (h *ServicesHandler) CheckServices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	services := []serviceStatus{}

	// PostgreSQL
	if err := h.db.Ping(ctx); err != nil {
		services = append(services, serviceStatus{Name: "postgres", Status: "error", Message: err.Error()})
	} else {
		services = append(services, serviceStatus{Name: "postgres", Status: "ok"})
	}

	// Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		services = append(services, serviceStatus{Name: "redis", Status: "error", Message: err.Error()})
	} else {
		services = append(services, serviceStatus{Name: "redis", Status: "ok"})
	}

	// Grafana — first try token from settings, then basic ping
	grafStatus := serviceStatus{Name: "grafana"}
	grafanaToken, _ := h.settingsRepo.Get("grafana_token")
	if grafanaToken != "" {
		if err := h.grafClient.PingWithToken(grafanaToken); err != nil {
			grafStatus.Status = "warning"
			grafStatus.Message = "Token configurado mas invalido: " + err.Error()
		} else {
			grafStatus.Status = "ok"
			grafStatus.Message = "Conectado via API token"
		}
	} else {
		if err := h.grafClient.Ping(); err != nil {
			grafStatus.Status = "error"
			grafStatus.Message = err.Error()
		} else {
			grafStatus.Status = "warning"
			grafStatus.Message = "Acessivel mas sem API token configurado"
		}
	}
	services = append(services, grafStatus)

	// Metrics API
	metricsStatus := serviceStatus{Name: "metrics-api"}
	metricsClient := &http.Client{Timeout: 3 * time.Second}
	metricsResp, err := metricsClient.Get("http://metrics-api:8081/health")
	if err != nil {
		metricsStatus.Status = "error"
		metricsStatus.Message = err.Error()
	} else {
		defer metricsResp.Body.Close()
		var health struct{ Status string `json:"status"` }
		if err := json.NewDecoder(metricsResp.Body).Decode(&health); err != nil || health.Status != "ok" {
			metricsStatus.Status = "warning"
			metricsStatus.Message = fmt.Sprintf("HTTP %d", metricsResp.StatusCode)
		} else {
			metricsStatus.Status = "ok"
		}
	}
	services = append(services, metricsStatus)

	// K6 Engine
	k6Path, err := exec.LookPath("k6")
	if err != nil {
		services = append(services, serviceStatus{Name: "k6", Status: "error", Message: "k6 binary not found"})
	} else {
		out, err := exec.Command(k6Path, "version").Output()
		if err != nil {
			services = append(services, serviceStatus{Name: "k6", Status: "error", Message: err.Error()})
		} else {
			version := strings.TrimSpace(string(out))
			services = append(services, serviceStatus{Name: "k6", Status: "ok", Message: version})
		}
	}

	response.OK(w, services)
}
