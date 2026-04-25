package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"boiler-go/internal/repository/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepo struct {
	*BaseRepo
}

func NewUserRepo(pool *pgxpool.Pool, log zerolog.Logger) *UserRepo {
	return &UserRepo{
		BaseRepo: NewBaseRepo(pool, log),
	}
}

func (r *UserRepo) WithTx(tx pgx.Tx) *UserRepo {
	return &UserRepo{BaseRepo: r.BaseRepo.WithTx(tx)}
}

func (r *UserRepo) Create(ctx context.Context, params db.CreateUserParams) (db.User, error) {
	start := time.Now()
	user, err := r.queries.CreateUser(ctx, params)
	r.logQuery("Create", "users", err, start)

	if err != nil {
		return db.User{}, fmt.Errorf("user repo: create: %w", err)
	}
	return user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id pgtype.UUID) (db.User, error) {
	start := time.Now()
	user, err := r.queries.GetUserByID(ctx, id)
	r.logQuery("GetByID", "users", err, start)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	r.logQuery("List", "users", err, start)

	if err != nil {
		return nil, fmt.Errorf("user repo: list: %w", err)
	}
	return users, nil
}
