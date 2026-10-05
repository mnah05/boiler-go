package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"boiler-go/internal/repository/db"

	"database/sql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepo struct {
	*BaseRepo
}

func NewUserRepo(pool *sqlx.DB, log zerolog.Logger) *UserRepo {
	return &UserRepo{
		BaseRepo: NewBaseRepo(pool, log),
	}
}

func (r *UserRepo) UpsertByClerkID(ctx context.Context, params db.UpsertUserByClerkIDParams) (db.User, error) {
	start := time.Now()
	user, err := r.queries.UpsertUserByClerkID(ctx, params)
	r.logQuery(ctx, "UpsertByClerkID", "users", err, start)

	if err != nil {
		return db.User{}, fmt.Errorf("user repo: upsert by clerk id: %w", err)
	}
	return user, nil
}

func (r *UserRepo) GetByClerkID(ctx context.Context, clerkID string) (db.User, error) {
	start := time.Now()
	user, err := r.queries.GetUserByClerkID(ctx, sql.NullString{String: clerkID, Valid: true})
	r.logQuery(ctx, "GetByClerkID", "users", err, start)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("user repo: get by clerk id: %w", err)
	}
	return user, nil
}

func (r *UserRepo) DeleteByClerkID(ctx context.Context, clerkID string) error {
	start := time.Now()
	rows, err := r.queries.DeleteUserByClerkID(ctx, sql.NullString{String: clerkID, Valid: true})
	r.logQuery(ctx, "DeleteByClerkID", "users", err, start)

	if err != nil {
		return fmt.Errorf("user repo: delete by clerk id: %w", err)
	}
	if rows == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	start := time.Now()
	user, err := r.queries.GetUserByID(ctx, id)
	r.logQuery(ctx, "GetByID", "users", err, start)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.User{}, ErrUserNotFound
		}
		return db.User{}, fmt.Errorf("user repo: get by id: %w", err)
	}
	return user, nil
}

func (r *UserRepo) List(ctx context.Context, limit int32) ([]db.User, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("user repo: list: limit must be positive")
	}
	if limit > 1000 {
		limit = 1000
	}

	start := time.Now()
	users, err := r.queries.ListUsers(ctx, limit)
	r.logQuery(ctx, "List", "users", err, start)

	if err != nil {
		return nil, fmt.Errorf("user repo: list: %w", err)
	}
	return users, nil
}
