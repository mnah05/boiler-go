package repo

import (
	"time"

	"boiler-go/internal/repository/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type BaseRepo struct {
	queries *db.Queries
	pool    *pgxpool.Pool
	log     zerolog.Logger
}

func NewBaseRepo(pool *pgxpool.Pool, log zerolog.Logger) *BaseRepo {
	return &BaseRepo{
		queries: db.New(pool),
		pool:    pool,
		log:     log,
	}
}

func (r *BaseRepo) Queries() *db.Queries {
	return r.queries
}

func (r *BaseRepo) logQuery(queryName, table string, err error, start time.Time) {
	elapsed := time.Since(start)

	if err != nil {
		r.log.Error().
			Err(err).
			Str("query", queryName).
			Str("table", table).
			Dur("duration", elapsed).
			Send()
		return
	}

	r.log.Debug().
		Str("query", queryName).
		Str("table", table).
		Dur("duration", elapsed).
		Send()
}
