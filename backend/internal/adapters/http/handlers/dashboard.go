package handlers

import (
	"net/http"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type DashboardHandler struct {
	execService *app.ExecutionService
}

func NewDashboardHandler(execService *app.ExecutionService) *DashboardHandler {
	return &DashboardHandler{execService: execService}
}

// All executions across all users (visible to everyone)
func (h *DashboardHandler) ListExecutions(w http.ResponseWriter, r *http.Request) {
	filter := domain.ExecutionFilter{
		AllUsers:   true,
		Pagination: domain.DefaultPagination(),
	}
	filter.Page = queryInt(r.URL.Query(), "page", 1)
	filter.PageSize = queryInt(r.URL.Query(), "page_size", 20)

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.TestStatus(status)
		filter.Status = &s
	}

	execs, total, err := h.execService.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(execs, total, filter.Pagination))
}

// Global stats
func (h *DashboardHandler) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.execService.GetStats()
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, stats)
}
