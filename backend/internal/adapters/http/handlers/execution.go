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

type ExecutionHandler struct {
	execService *app.ExecutionService
}

func NewExecutionHandler(execService *app.ExecutionService) *ExecutionHandler {
	return &ExecutionHandler{execService: execService}
}

func (h *ExecutionHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	var input domain.CreateExecutionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	exec, err := h.execService.Create(claims.UserID, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, exec)
}

func (h *ExecutionHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	filter := domain.ExecutionFilter{
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
		s := domain.TestStatus(status)
		filter.Status = &s
	}

	// Non-ROOT users only see their own executions
	if string(claims.Role) != "ROOT" {
		filter.UserID = &claims.UserID
	}

	execs, total, err := h.execService.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(execs, total, filter.Pagination))
}

func (h *ExecutionHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid execution ID")
		return
	}

	exec, err := h.execService.GetByID(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, exec)
}

func (h *ExecutionHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid execution ID")
		return
	}

	if err := h.execService.Cancel(id, claims.UserID, claims.Role == domain.UserRoleRoot); err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]string{"message": "Execution cancelled"})
}

func (h *ExecutionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid execution ID")
		return
	}

	if err := h.execService.Delete(id, claims.UserID, claims.Role == domain.UserRoleRoot); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

func (h *ExecutionHandler) DeleteByTest(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	testID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	deleted, err := h.execService.DeleteByTestID(testID, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]int64{"deleted": deleted})
}

func (h *ExecutionHandler) RecalculateMetrics(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid execution ID")
		return
	}

	exec, err := h.execService.RecalculateMetrics(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, exec)
}

func (h *ExecutionHandler) Logs(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid execution ID")
		return
	}

	exec, err := h.execService.GetByID(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]interface{}{
		"stdout": exec.Stdout,
		"stderr": exec.Stderr,
	})
}
