package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/postgres"
)

type SettingsHandler struct {
	settingsRepo *postgres.SettingsRepository
}

func NewSettingsHandler(settingsRepo *postgres.SettingsRepository) *SettingsHandler {
	return &SettingsHandler{settingsRepo: settingsRepo}
}

func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	settings, err := h.settingsRepo.GetAll()
	if err != nil {
		response.InternalError(w)
		return
	}
	// Mask sensitive values
	masked := make(map[string]string)
	for k, v := range settings {
		if k == "grafana_token" && len(v) > 8 {
			masked[k] = v[:4] + "..." + v[len(v)-4:]
		} else {
			masked[k] = v
		}
	}
	response.OK(w, masked)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var input map[string]string
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	allowed := map[string]bool{
		"grafana_token": true,
	}

	for key, value := range input {
		if !allowed[key] {
			response.BadRequest(w, "Setting '"+key+"' is not allowed")
			return
		}
		if err := h.settingsRepo.Set(key, value); err != nil {
			response.InternalError(w)
			return
		}
	}

	response.OK(w, map[string]string{"message": "Settings updated"})
}
