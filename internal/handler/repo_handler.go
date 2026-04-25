package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"boiler-go/internal/repository/db"
	"boiler-go/internal/repository/repo"
	"boiler-go/internal/validator"
	"boiler-go/pkg/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type RepoTestHandler struct {
	userRepo *repo.UserRepo
	log      zerolog.Logger
}

func NewRepoTestHandler(pool *pgxpool.Pool, log zerolog.Logger) *RepoTestHandler {
	return &RepoTestHandler{
		userRepo: repo.NewUserRepo(pool, log),
		log:      log,
	}
}

type CreateUserRequest struct {
	Email string `json:"email" validate:"required,email"`
	Name  string `json:"name" validate:"required,min=1,max=255"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (h *RepoTestHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("failed to decode request")
		if wErr := NewErrorResponse(w, http.StatusBadRequest, "bad_request", "invalid request body"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	if err := validator.ValidateStruct(&req); err != nil {
		validationErrors := validator.GetValidationErrors(err)
		if wErr := NewErrorResponseWithDetails(w, http.StatusBadRequest, "validation_error", "validation failed", validationErrors); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	user, err := h.userRepo.Create(ctx, db.CreateUserParams{
		Email: strings.ToLower(strings.TrimSpace(req.Email)),
		Name:  strings.TrimSpace(req.Name),
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create user")

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if wErr := NewErrorResponse(w, http.StatusConflict, "conflict", "email already exists"); wErr != nil {
				log.Error().Err(wErr).Msg("failed to write error response")
			}
			return
		}

		if wErr := NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to create user"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	log.Info().Str("user_id", uuid.UUID(user.ID.Bytes).String()).Msg("user created via repo")

	if wErr := NewSuccessResponse(w, http.StatusCreated, UserResponse{
		ID:        uuid.UUID(user.ID.Bytes).String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Time.Format(time.RFC3339Nano),
		UpdatedAt: user.UpdatedAt.Time.Format(time.RFC3339Nano),
	}, "user created"); wErr != nil {
		log.Error().Err(wErr).Msg("failed to write success response")
	}
}

func (h *RepoTestHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())

	userIDStr := r.URL.Query().Get("id")
	if userIDStr == "" {
		if wErr := NewErrorResponse(w, http.StatusBadRequest, "bad_request", "missing id parameter"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	userUUID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid uuid")
		if wErr := NewErrorResponse(w, http.StatusBadRequest, "bad_request", "invalid uuid format"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	pgUUID := pgtype.UUID{
		Bytes: userUUID,
		Valid: true,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	user, err := h.userRepo.GetByID(ctx, pgUUID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			if wErr := NewErrorResponse(w, http.StatusNotFound, "not_found", "user not found"); wErr != nil {
				log.Error().Err(wErr).Msg("failed to write error response")
			}
			return
		}
		log.Error().Err(err).Msg("failed to get user")
		if wErr := NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to get user"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	if wErr := WriteJSON(w, http.StatusOK, UserResponse{
		ID:        uuid.UUID(user.ID.Bytes).String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Time.Format(time.RFC3339Nano),
		UpdatedAt: user.UpdatedAt.Time.Format(time.RFC3339Nano),
	}); wErr != nil {
		log.Error().Err(wErr).Msg("failed to write JSON response")
	}
}

func (h *RepoTestHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())

	var limit int32 = 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.ParseInt(limitStr, 10, 32)
		if err != nil || parsed <= 0 {
			if wErr := NewErrorResponse(w, http.StatusBadRequest, "bad_request", "limit must be a positive integer"); wErr != nil {
				log.Error().Err(wErr).Msg("failed to write error response")
			}
			return
		}
		limit = int32(parsed)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	users, err := h.userRepo.List(ctx, limit)
	if err != nil {
		log.Error().Err(err).Msg("failed to list users")
		if wErr := NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to list users"); wErr != nil {
			log.Error().Err(wErr).Msg("failed to write error response")
		}
		return
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = UserResponse{
			ID:        uuid.UUID(user.ID.Bytes).String(),
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Time.Format(time.RFC3339Nano),
			UpdatedAt: user.UpdatedAt.Time.Format(time.RFC3339Nano),
		}
	}

	log.Info().Int("count", len(users)).Msg("users listed via repo")

	if wErr := WriteJSON(w, http.StatusOK, response); wErr != nil {
		log.Error().Err(wErr).Msg("failed to write JSON response")
	}
}
