package response

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorBody  `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

type Meta struct {
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalPages int   `json:"total_pages"`
}

func JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: status >= 200 && status < 300,
		Data:    data,
	})
}

func JSONWithMeta(w http.ResponseWriter, status int, data interface{}, meta Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    data,
		Meta:    &meta,
	})
}

func Error(w http.ResponseWriter, err error) {
	var status int
	var errorBody ErrorBody

	switch e := err.(type) {
	case *domain.AppError:
		status = e.StatusCode
		errorBody = ErrorBody{
			Code:    e.Code,
			Message: e.Message,
			Details: e.Details,
		}
	default:
		log.Printf("[ERROR] Unexpected error: %v", err)
		status = http.StatusInternalServerError
		errorBody = ErrorBody{
			Code:    "INTERNAL_ERROR",
			Message: "An unexpected error occurred",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   &errorBody,
	})
}

func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, data)
}

func OK(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, data)
}

func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

func BadRequest(w http.ResponseWriter, message string) {
	Error(w, domain.NewAppError("BAD_REQUEST", message, http.StatusBadRequest))
}

func NotFound(w http.ResponseWriter, resource string) {
	Error(w, domain.NewNotFoundError(resource))
}

func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, domain.NewUnauthorizedError(message))
}

func Forbidden(w http.ResponseWriter, message string) {
	Error(w, domain.NewForbiddenError(message))
}

func ValidationError(w http.ResponseWriter, details map[string]string) {
	Error(w, domain.NewValidationError(details))
}

func InternalError(w http.ResponseWriter) {
	Error(w, domain.NewAppError("INTERNAL_ERROR", "An unexpected error occurred", http.StatusInternalServerError))
}

func Paginated[T any](w http.ResponseWriter, result domain.PaginatedResult[T]) {
	JSONWithMeta(w, http.StatusOK, result.Data, Meta{
		Total:      result.Total,
		Page:       result.Page,
		PageSize:   result.PageSize,
		TotalPages: result.TotalPages,
	})
}
