package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	"github.com/labstack/echo/v4"
)

// ClerkAuth returns an Echo middleware that authenticates requests with a Clerk
// session token from the Authorization: Bearer <token> header, storing verified
// session claims on the request context.
//
// It deliberately does not use clerkhttp.RequireHeaderAuthorization: that
// wrapper answers a missing token with a bare 403 and an empty body, which
// bypasses both the shared error envelope and the central logger. Here every
// rejection returns an *echo.HTTPError, so handler.HTTPErrorHandler renders the
// {"error","message","request_id"} envelope and records the reason.
func ClerkAuth() echo.MiddlewareFunc {
	// The SDK's failure handler must not write: this middleware reports the
	// outcome itself. Token verification is still performed by the SDK.
	verify := clerkhttp.WithHeaderAuthorization(
		clerkhttp.AuthorizationFailureHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})),
	)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !hasBearerToken(c.Request()) {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing or malformed authorization header")
			}

			// Run verification against an inner handler that captures the
			// request it is given. The inner handler is only reached when
			// verification succeeded; a failure calls the no-op handler above
			// and leaves verified nil. The response writer is discarded so the
			// SDK can never write to the real response behind Echo's back.
			var verified *http.Request
			inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				verified = r
			})
			verify(inner).ServeHTTP(discardWriter{}, c.Request())

			if verified == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}
			claims, ok := clerk.SessionClaimsFromContext(verified.Context())
			if !ok || claims == nil || claims.Subject == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
			}

			// Handlers read the claims with
			// ClerkUserIDFromContext(c.Request().Context()).
			c.SetRequest(verified)
			return next(c)
		}
	}
}

// hasBearerToken reports whether the request carries a non-empty bearer token.
// Requests without one are rejected before any verification work.
func hasBearerToken(r *http.Request) bool {
	scheme, token, found := strings.Cut(strings.TrimSpace(r.Header.Get(echo.HeaderAuthorization)), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return false
	}
	return strings.TrimSpace(token) != ""
}

// discardWriter swallows writes so a verification failure handler cannot commit
// a response behind Echo's back.
type discardWriter struct{}

func (discardWriter) Header() http.Header         { return http.Header{} }
func (discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (discardWriter) WriteHeader(int)             {}

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
