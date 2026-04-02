package handler

import (
	"encoding/json"
	"net/http"

	"boiler-go/internal/repository/db"
	"boiler-go/internal/repository/repo"
	"boiler-go/pkg/logger"

	"github.com/google/uuid"
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
	Email string `json:"email"`
	Name  string `json:"name"`
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
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.userRepo.Create(r.Context(), db.CreateUserParams{
		Email: req.Email,
		Name:  req.Name,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to create user")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	log.Info().Str("user_id", uuid.UUID(user.ID.Bytes[:]).String()).Msg("user created via repo")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(UserResponse{
		ID:        uuid.UUID(user.ID.Bytes[:]).String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Time.String(),
		UpdatedAt: user.UpdatedAt.Time.String(),
	})
}

func (h *RepoTestHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())

	userIDStr := r.URL.Query().Get("id")
	if userIDStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "missing id parameter"})
		return
	}

	userUUID, err := uuid.Parse(userIDStr)
	if err != nil {
		log.Error().Err(err).Msg("invalid uuid")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid uuid"})
		return
	}

	pgUUID := pgtype.UUID{
		Bytes: userUUID,
		Valid: true,
	}

	user, err := h.userRepo.GetByID(r.Context(), pgUUID)
	if err != nil {
		log.Error().Err(err).Msg("failed to get user")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UserResponse{
		ID:        uuid.UUID(user.ID.Bytes[:]).String(),
		Email:     user.Email,
		Name:      user.Name,
		CreatedAt: user.CreatedAt.Time.String(),
		UpdatedAt: user.UpdatedAt.Time.String(),
	})
}

func (h *RepoTestHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	log := logger.FromChiContext(r.Context())

	users, err := h.userRepo.List(r.Context(), 10)
	if err != nil {
		log.Error().Err(err).Msg("failed to list users")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = UserResponse{
			ID:        uuid.UUID(user.ID.Bytes[:]).String(),
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Time.String(),
			UpdatedAt: user.UpdatedAt.Time.String(),
		}
	}

	log.Info().Int("count", len(users)).Msg("users listed via repo")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
