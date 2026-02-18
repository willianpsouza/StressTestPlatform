package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/willianpsouza/StressTestPlatform/internal/adapters/http/response"
	"github.com/willianpsouza/StressTestPlatform/internal/app"
	"github.com/willianpsouza/StressTestPlatform/internal/domain"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "claims"
)

func Auth(authService *app.AuthService) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				response.Error(w, domain.NewUnauthorizedError("Missing authorization header"))
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				response.Error(w, domain.NewUnauthorizedError("Invalid authorization header format"))
				return
			}

			claims, err := authService.ValidateToken(parts[1])
			if err != nil {
				response.Error(w, domain.NewUnauthorizedError("Invalid or expired token"))
				return
			}

			ctx := context.WithValue(r.Context(), ClaimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(roles ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				response.Error(w, domain.NewUnauthorizedError("Authentication required"))
				return
			}

			hasRole := false
			for _, role := range roles {
				if string(claims.Role) == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				response.Error(w, domain.NewForbiddenError("Insufficient permissions"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetClaims(ctx context.Context) *domain.TokenClaims {
	claims, ok := ctx.Value(ClaimsContextKey).(*domain.TokenClaims)
	if !ok {
		return nil
	}
	return claims
}
