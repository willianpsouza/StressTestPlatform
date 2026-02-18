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

type AuthHandler struct {
	authService *app.AuthService
}

func NewAuthHandler(authService *app.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input domain.RegisterInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	result, err := h.authService.Register(input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Created(w, result)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input domain.LoginInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = fwd
	}
	userAgent := r.UserAgent()

	result, err := h.authService.Login(input, ip, userAgent)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, result)
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	result, err := h.authService.RefreshToken(body.RefreshToken)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, result)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	if err := h.authService.Logout(body.RefreshToken); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.Unauthorized(w, "Authentication required")
		return
	}

	user, err := h.authService.GetCurrentUser(claims.UserID)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, user)
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.Unauthorized(w, "Authentication required")
		return
	}

	var input domain.UpdateProfileInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	user, err := h.authService.UpdateProfile(claims.UserID, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, user)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.Unauthorized(w, "Authentication required")
		return
	}

	var input domain.ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	if err := h.authService.ChangePassword(claims.UserID, input); err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, map[string]string{"message": "Password changed successfully"})
}

// Admin: List users
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	filter := domain.UserFilter{
		Pagination: domain.DefaultPagination(),
	}

	if q := r.URL.Query(); q.Get("page") != "" {
		filter.Page = queryInt(q, "page", 1)
	}
	if q := r.URL.Query(); q.Get("page_size") != "" {
		filter.PageSize = queryInt(q, "page_size", 20)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		filter.Search = &search
	}

	users, total, err := h.authService.ListUsers(filter)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.Paginated(w, domain.NewPaginatedResult(users, total, filter.Pagination))
}

// Admin: Get user by ID
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid user ID")
		return
	}

	user, err := h.authService.GetUser(id)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, user)
}

// Admin: Update user
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid user ID")
		return
	}

	var input domain.UpdateUserInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, "Invalid request body")
		return
	}

	user, err := h.authService.UpdateUser(id, input)
	if err != nil {
		response.Error(w, err)
		return
	}

	response.OK(w, user)
}

// Admin: Delete user
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, "Invalid user ID")
		return
	}

	if err := h.authService.DeleteUser(id); err != nil {
		response.Error(w, err)
		return
	}

	response.NoContent(w)
}

func queryInt(q interface{ Get(string) string }, key string, defaultValue int) int {
	val := q.(interface{ Get(string) string }).Get(key)
	if val == "" {
		return defaultValue
	}
	var v int
	for _, c := range val {
		if c >= '0' && c <= '9' {
			v = v*10 + int(c-'0')
		} else {
			return defaultValue
		}
	}
	return v
}
