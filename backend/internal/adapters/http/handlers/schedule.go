package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/middleware"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type ScheduleHandler struct {
	scheduleService *app.ScheduleService
}

func NewScheduleHandler(scheduleService *app.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{scheduleService: scheduleService}
}

func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	var input domain.CreateScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	schedule, err := h.scheduleService.Create(claims.UserID, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, schedule)
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	filter := domain.ScheduleFilter{
		Pagination: domain.DefaultPagination(),
	}
	filter.Page = queryInt(r.URL.Query(), "page", 1)
	filter.PageSize = queryInt(r.URL.Query(), "page_size", 20)

	if testID := r.URL.Query().Get("test_id"); testID != "" {
		if id, err := uuid.Parse(testID); err == nil {
			filter.TestID = &id
		}
	}
	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.ScheduleStatus(status)
		filter.Status = &s
	}

	if string(claims.Role) != "ROOT" {
		filter.UserID = &claims.UserID
	}

	schedules, total, err := h.scheduleService.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(schedules, total, filter.Pagination))
}

func (h *ScheduleHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid schedule ID")
		return
	}

	schedule, err := h.scheduleService.GetByID(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, schedule)
}

func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid schedule ID")
		return
	}

	var input domain.UpdateScheduleInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	schedule, err := h.scheduleService.Update(id, claims.UserID, claims.Role == domain.UserRoleRoot, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, schedule)
}

func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid schedule ID")
		return
	}

	if err := h.scheduleService.Delete(id, claims.UserID, claims.Role == domain.UserRoleRoot); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

func (h *ScheduleHandler) Pause(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid schedule ID")
		return
	}

	schedule, err := h.scheduleService.Pause(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, schedule)
}

func (h *ScheduleHandler) Resume(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid schedule ID")
		return
	}

	schedule, err := h.scheduleService.Resume(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, schedule)
}
