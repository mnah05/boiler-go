package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	custommiddleware "boiler-go/internal/middleware"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"boiler-go/internal/repository/db"
	"boiler-go/internal/repository/repo"
)

type RepoTestHandler struct {
	userRepo *repo.UserRepo
	log      zerolog.Logger
}

func NewRepoTestHandler(pool *sqlx.DB, log zerolog.Logger) *RepoTestHandler {
	return &RepoTestHandler{userRepo: repo.NewUserRepo(pool, log), log: log}
}

type UserResponse struct {
	ID        string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ClerkID   *string `json:"clerk_id,omitempty" example:"user_2abcXYZ"`
	Email     string  `json:"email" example:"jane@example.com"`
	Name      string  `json:"name" example:"Jane Doe"`
	CreatedAt string  `json:"created_at" example:"2026-10-05T12:00:00Z"`
	UpdatedAt string  `json:"updated_at" example:"2026-10-05T12:00:00Z"`
}

// GetMe godoc
// @Summary		Current user
// @Description	Returns the local user linked to the Clerk session. Requires a Clerk session token.
// @Tags			users
// @Produce		json
// @Security		BearerAuth
// @Success		200	{object}	handler.UserResponse
// @Failure		401	{object}	handler.ErrorResponse
// @Failure		404	{object}	handler.ErrorResponse
// @Router			/me [get]
func (h *RepoTestHandler) GetMe(c echo.Context) error {
	clerkID, ok := custommiddleware.ClerkUserIDFromContext(c.Request().Context())
	if !ok {
		return NewEchoError(http.StatusUnauthorized, "unauthorized", "invalid or expired token")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	user, err := h.userRepo.GetByClerkID(ctx, clerkID)
	if errors.Is(err, repo.ErrUserNotFound) || errors.Is(err, sql.ErrNoRows) {
		return NewEchoError(http.StatusNotFound, "not_found", "user not synced yet")
	}
	if err != nil {
		return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to get user")
	}
	return c.JSON(http.StatusOK, userResponse(user))
}

// GetUser godoc
// @Summary		Get user
// @Description	Get a single user by id query param. Requires a Clerk session token.
// @Tags			users
// @Produce		json
// @Security		BearerAuth
// @Param			id	query		string	true	"User UUID"
// @Success		200	{object}	handler.UserResponse
// @Failure		400	{object}	handler.ErrorResponse
// @Failure		401	{object}	handler.ErrorResponse
// @Failure		404	{object}	handler.ErrorResponse
// @Failure		500	{object}	handler.ErrorResponse
// @Router			/repo-test/users/get [get]
func (h *RepoTestHandler) GetUser(c echo.Context) error {
	id, err := uuid.Parse(c.QueryParam("id"))
	if err != nil {
		return NewEchoError(http.StatusBadRequest, "bad_request", "invalid or missing id parameter")
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	user, err := h.userRepo.GetByID(ctx, id)
	if errors.Is(err, repo.ErrUserNotFound) || errors.Is(err, sql.ErrNoRows) {
		return NewEchoError(http.StatusNotFound, "not_found", "user not found")
	}
	if err != nil {
		return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to get user")
	}
	return c.JSON(http.StatusOK, userResponse(user))
}

// ListUsers godoc
// @Summary		List users
// @Description	List users newest-first. Requires a Clerk session token.
// @Tags			users
// @Produce		json
// @Security		BearerAuth
// @Param			limit	query		int	false	"Max rows"	minimum(1)	default(10)
// @Success		200	{array}		handler.UserResponse
// @Failure		400	{object}	handler.ErrorResponse
// @Failure		401	{object}	handler.ErrorResponse
// @Failure		500	{object}	handler.ErrorResponse
// @Router			/repo-test/users [get]
func (h *RepoTestHandler) ListUsers(c echo.Context) error {
	limit := int32(10)
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 32)
		if err != nil || n <= 0 {
			return NewEchoError(http.StatusBadRequest, "bad_request", "limit must be a positive integer")
		}
		limit = int32(n)
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()
	users, err := h.userRepo.List(ctx, limit)
	if err != nil {
		return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to list users")
	}
	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = userResponse(user)
	}
	return c.JSON(http.StatusOK, response)
}

func userResponse(user db.User) UserResponse {
	var clerkID *string
	if user.ClerkID.Valid {
		v := user.ClerkID.String
		clerkID = &v
	}
	return UserResponse{ID: user.ID.String(), ClerkID: clerkID, Email: user.Email, Name: user.Name, CreatedAt: user.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: user.UpdatedAt.Format(time.RFC3339Nano)}
}
