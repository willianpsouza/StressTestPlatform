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

type DomainHandler struct {
	domainService *app.DomainService
}

func NewDomainHandler(domainService *app.DomainService) *DomainHandler {
	return &DomainHandler{domainService: domainService}
}

func (h *DomainHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	filter := domain.DomainFilter{
		Pagination: domain.DefaultPagination(),
	}
	filter.Page = queryInt(r.URL.Query(), "page", 1)
	filter.PageSize = queryInt(r.URL.Query(), "page_size", 20)

	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = &search
	}

	// Non-ROOT users only see their own domains
	if string(claims.Role) != "ROOT" {
		filter.UserID = &claims.UserID
	}

	domains, total, err := h.domainService.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(domains, total, filter.Pagination))
}

func (h *DomainHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	var input domain.CreateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	d, err := h.domainService.Create(claims.UserID, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, d)
}

func (h *DomainHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid domain ID")
		return
	}

	d, err := h.domainService.GetByID(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, d)
}

func (h *DomainHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid domain ID")
		return
	}

	var input domain.UpdateDomainInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	d, err := h.domainService.Update(id, claims.UserID, claims.Role == domain.UserRoleRoot, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, d)
}

func (h *DomainHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid domain ID")
		return
	}

	if err := h.domainService.Delete(id, claims.UserID, claims.Role == domain.UserRoleRoot); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}
