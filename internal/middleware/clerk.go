package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

// ClerkAuth returns a middleware that validates Clerk session tokens from the
// Authorization: Bearer <token> header. Invalid or missing tokens get a 401
// JSON response matching the API error envelope.
func ClerkAuth() func(http.Handler) http.Handler {
	failure := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":   "unauthorized",
			"message": "invalid or expired token",
		})
	})
	return clerkhttp.RequireHeaderAuthorization(
		clerkhttp.AuthorizationFailureHandler(failure),
	)
}

// ClerkUserIDFromContext extracts the Clerk user ID (claims Subject) stored
// by ClerkAuth.
func ClerkUserIDFromContext(ctx context.Context) (string, bool) {
	claims, ok := clerk.SessionClaimsFromContext(ctx)
	if !ok || claims.Subject == "" {
		return "", false
	}
	return claims.Subject, true
}

// SessionClaimsFromContext re-exports the Clerk SDK helper for handlers that
// need full session claims.
func SessionClaimsFromContext(ctx context.Context) (*clerk.SessionClaims, bool) {
	return clerk.SessionClaimsFromContext(ctx)
}

// UserIDFromContext is kept for handler compatibility; it resolves the Clerk
// user ID from session claims.
func UserIDFromContext(ctx context.Context) (string, bool) {
	return ClerkUserIDFromContext(ctx)
}
