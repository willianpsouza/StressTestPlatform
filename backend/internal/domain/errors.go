package domain

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrNotFound           = errors.New("resource not found")
	ErrConflict           = errors.New("resource already exists")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrValidation         = errors.New("validation error")
	ErrInternal           = errors.New("internal server error")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("invalid token")
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailExists        = errors.New("email already exists")
	ErrDomainNotFound     = errors.New("domain not found")
	ErrTestNotFound       = errors.New("test not found")
	ErrExecutionNotFound  = errors.New("execution not found")
	ErrScheduleNotFound   = errors.New("schedule not found")
	ErrTooManyConcurrent  = errors.New("too many concurrent tests")
)

type AppError struct {
	Code       string            `json:"code"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
	StatusCode int               `json:"-"`
	Err        error             `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func NewAppError(code, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

func (e *AppError) WithError(err error) *AppError {
	return &AppError{
		Code:       e.Code,
		Message:    e.Message,
		Details:    e.Details,
		StatusCode: e.StatusCode,
		Err:        err,
	}
}

func (e *AppError) WithDetails(details map[string]string) *AppError {
	return &AppError{
		Code:       e.Code,
		Message:    e.Message,
		Details:    details,
		StatusCode: e.StatusCode,
		Err:        e.Err,
	}
}

func NewValidationError(details map[string]string) *AppError {
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    "Validation failed",
		Details:    details,
		StatusCode: http.StatusUnprocessableEntity,
	}
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{
		Code:       "CONFLICT",
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:       "FORBIDDEN",
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

func NewTooManyRequestsError(message string) *AppError {
	return &AppError{
		Code:       "TOO_MANY_REQUESTS",
		Message:    message,
		StatusCode: http.StatusTooManyRequests,
	}
}

func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode == http.StatusNotFound
	}
	return false
}

func IsConflict(err error) bool {
	if errors.Is(err, ErrConflict) {
		return true
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode == http.StatusConflict
	}
	return false
}
