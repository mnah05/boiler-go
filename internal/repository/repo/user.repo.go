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

func (r *UserRepo) WithTx(tx *sql.Tx) *UserRepo {
	return &UserRepo{BaseRepo: r.BaseRepo.WithTx(tx)}
}

func (r *UserRepo) Create(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	start := time.Now()
	user, err := r.queries.CreateUser(ctx, params)
	r.logQuery(ctx, "Create", "users", err, start)

	if err != nil {
		return db.User{}, fmt.Errorf("user repo: create: %w", err)
	}
	return user, nil
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
