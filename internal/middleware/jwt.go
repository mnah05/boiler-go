package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"boiler-go/pkg/jwtpkg"
	"boiler-go/pkg/logger"
)

type contextKey string

const (
	ContextKeyUserID = contextKey("user_id")
	ContextKeyRoles  = contextKey("roles")
)

// JWTConfig holds the configuration for the JWT authentication middleware.
type JWTConfig struct {
	Secret []byte
}

// JWTAuth returns a middleware that validates Bearer tokens using the
// shared jwt package.
func JWTAuth(cfg JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.FromChiContext(r.Context())

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Warn().Msg("missing authorization header")
				writeAuthError(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				log.Warn().Msg("invalid authorization header format")
				writeAuthError(w, "invalid authorization header format")
				return
			}

			tokenString := parts[1]

			claims, err := jwtpkg.ParseToken(tokenString, cfg.Secret)
			if err != nil {
				log.Warn().Err(err).Msg("invalid jwt token")
				writeAuthError(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyRoles, claims.Roles)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func writeAuthError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "unauthorized",
		"message": message,
	})
}

// UserIDFromContext extracts the user ID stored by JWTAuth.
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(ContextKeyUserID).(string)
	return userID, ok
}

// RolesFromContext extracts the roles stored by JWTAuth.
func RolesFromContext(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(ContextKeyRoles).([]string)
	return roles, ok
}
