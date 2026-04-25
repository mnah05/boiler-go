package middleware

import (
	"encoding/json"
	"net/http"

	"boiler-go/pkg/logger"
)

func RequireRole(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.FromChiContext(r.Context())

			roles, ok := RolesFromContext(r.Context())
			if !ok {
				log.Warn().Msg("roles not found in context")
				writeRBACError(w, "unauthorized")
				return
			}

			for _, role := range roles {
				if role == requiredRole {
					next.ServeHTTP(w, r)
					return
				}
			}

			log.Warn().
				Str("required_role", requiredRole).
				Strs("user_roles", roles).
				Msg("insufficient permissions")

			writeRBACError(w, "forbidden: insufficient permissions")
		})
	}
}

func RequireAnyRole(requiredRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.FromChiContext(r.Context())

			roles, ok := RolesFromContext(r.Context())
			if !ok {
				log.Warn().Msg("roles not found in context")
				writeRBACError(w, "unauthorized")
				return
			}

			roleSet := make(map[string]bool, len(roles))
			for _, role := range roles {
				roleSet[role] = true
			}

			for _, required := range requiredRoles {
				if roleSet[required] {
					next.ServeHTTP(w, r)
					return
				}
			}

			log.Warn().
				Strs("required_roles", requiredRoles).
				Strs("user_roles", roles).
				Msg("insufficient permissions")

			writeRBACError(w, "forbidden: insufficient permissions")
		})
	}
}

func writeRBACError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "forbidden",
		"message": message,
	})
}
