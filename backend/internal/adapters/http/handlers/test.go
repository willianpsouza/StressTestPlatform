package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/middleware"
	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type TestHandler struct {
	testService *app.TestService
}

func NewTestHandler(testService *app.TestService) *TestHandler {
	return &TestHandler{testService: testService}
}

func (h *TestHandler) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	filter := domain.TestFilter{
		Pagination: domain.DefaultPagination(),
	}
	filter.Page = queryInt(r.URL.Query(), "page", 1)
	filter.PageSize = queryInt(r.URL.Query(), "page_size", 20)

	if domainID := r.URL.Query().Get("domain_id"); domainID != "" {
		if id, err := uuid.Parse(domainID); err == nil {
			filter.DomainID = &id
		}
	}
	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = &search
	}

	// Non-ROOT users only see their own tests
	if string(claims.Role) != "ROOT" {
		filter.UserID = &claims.UserID
	}

	tests, total, err := h.testService.List(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(tests, total, filter.Pagination))
}

func (h *TestHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		response.BadRequest(w, "Invalid multipart form")
		return
	}

	// Parse metadata
	var input domain.CreateTestInput

	domainIDStr := r.FormValue("domain_id")
	domainID, err := uuid.Parse(domainIDStr)
	if err != nil {
		response.BadRequest(w, "Invalid domain_id")
		return
	}
	input.DomainID = domainID
	input.Name = r.FormValue("name")
	input.DefaultDuration = r.FormValue("default_duration")
	if input.DefaultDuration == "" {
		input.DefaultDuration = "30s"
	}
	input.DefaultVUs = 1
	if vus := r.FormValue("default_vus"); vus != "" {
		if v, err := strconv.Atoi(vus); err == nil {
			input.DefaultVUs = v
		}
	}
	if desc := r.FormValue("description"); desc != "" {
		input.Description = &desc
	}

	// Get script file
	file, header, err := r.FormFile("script")
	if err != nil {
		response.BadRequest(w, "Script file is required")
		return
	}
	defer file.Close()

	test, err := h.testService.Create(claims.UserID, claims.Role == domain.UserRoleRoot, input, header.Filename, file, header.Size)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, test)
}

func (h *TestHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	test, err := h.testService.GetByID(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, test)
}

func (h *TestHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	var input domain.UpdateTestInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	test, err := h.testService.Update(id, claims.UserID, claims.Role == domain.UserRoleRoot, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, test)
}

func (h *TestHandler) UpdateScript(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	if err := r.ParseMultipartForm(2 << 20); err != nil {
		response.BadRequest(w, "Invalid multipart form")
		return
	}

	file, header, err := r.FormFile("script")
	if err != nil {
		response.BadRequest(w, "Script file is required")
		return
	}
	defer file.Close()

	test, err := h.testService.UpdateScript(id, claims.UserID, claims.Role == domain.UserRoleRoot, header.Filename, file, header.Size)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, test)
}

func (h *TestHandler) GetScriptContent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	content, err := h.testService.GetScriptContent(id, claims.UserID, claims.Role == domain.UserRoleRoot)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]string{"content": content})
}

func (h *TestHandler) SaveScriptContent(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	test, err := h.testService.SaveScriptContent(id, claims.UserID, claims.Role == domain.UserRoleRoot, body.Content)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, test)
}

func (h *TestHandler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid test ID")
		return
	}

	if err := h.testService.Delete(id, claims.UserID, claims.Role == domain.UserRoleRoot); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}
