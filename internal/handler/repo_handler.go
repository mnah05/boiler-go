package handler

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"boiler-go/internal/repository/db"
	"boiler-go/internal/repository/repo"
	"boiler-go/pkg/logger"
	"boiler-go/pkg/validator"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type RepoTestHandler struct {
	userRepo *repo.UserRepo
	log      zerolog.Logger
}

func NewRepoTestHandler(pool *sqlx.DB, log zerolog.Logger) *RepoTestHandler {
	return &RepoTestHandler{userRepo: repo.NewUserRepo(pool, log), log: log}
}

type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email" example:"jane@example.com"`
	Name  string `json:"name" validate:"required,min=1,max=255" example:"Jane Doe"`
}
type UserResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Email     string `json:"email" example:"jane@example.com"`
	Name      string `json:"name" example:"Jane Doe"`
	CreatedAt string `json:"created_at" example:"2026-10-05T12:00:00Z"`
	UpdatedAt string `json:"updated_at" example:"2026-10-05T12:00:00Z"`
}

// CreateUserSuccessResponse documents the POST /repo-test/users success envelope.
type CreateUserSuccessResponse struct {
	Success bool         `json:"success" example:"true"`
	Data    UserResponse `json:"data"`
	Message string       `json:"message" example:"user created"`
}

// CreateUser godoc
// @Summary		Create user
// @Description	Create a user by email and name. Requires a Bearer token.
// @Tags			users
// @Accept			json
// @Produce		json
// @Security		BearerAuth
// @Param			body	body		handler.CreateUserRequest	true	"User payload"
// @Success		201		{object}	handler.CreateUserSuccessResponse
// @Failure		400		{object}	handler.ErrorResponse
// @Failure		401		{object}	handler.ErrorResponse
// @Failure		409		{object}	handler.ErrorResponse
// @Failure		500		{object}	handler.ErrorResponse
// @Router			/repo-test/users [post]
func (h *RepoTestHandler) CreateUser(c echo.Context) error {
	log := logger.FromContext(c.Request().Context())
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return NewEchoError(http.StatusBadRequest, "bad_request", "invalid request body")
	}
	if err := validator.ValidateStruct(&req); err != nil {
		return NewEchoErrorWithDetails(http.StatusBadRequest, "validation_error", "validation failed", validator.GetValidationErrors(err))
	}
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()
	user, err := h.userRepo.Create(ctx, db.CreateUserParams{Email: strings.ToLower(strings.TrimSpace(req.Email)), Name: strings.TrimSpace(req.Name)})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return NewEchoError(http.StatusConflict, "conflict", "email already exists")
		}
		log.Error().Err(err).Msg("create user failed")
		return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to create user")
	}
	return c.JSON(http.StatusCreated, SuccessResponse{Success: true, Data: userResponse(user), Message: "user created"})
}

// GetUser godoc
// @Summary		Get user
// @Description	Get a single user by id query param. Requires a Bearer token.
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
// @Description	List users newest-first. Requires a Bearer token.
// @Tags			users
// @Produce		json
// @Security		BearerAuth
// @Param			limit	query		int	false	"Max rows"	minimum(1)	default(10)
// @Success		200		{array}		handler.UserResponse
// @Failure		400		{object}	handler.ErrorResponse
// @Failure		401		{object}	handler.ErrorResponse
// @Failure		500		{object}	handler.ErrorResponse
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
	return UserResponse{ID: user.ID.String(), Email: user.Email, Name: user.Name, CreatedAt: user.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: user.UpdatedAt.Format(time.RFC3339Nano)}
}
