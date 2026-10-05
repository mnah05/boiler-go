package repo

import (
	"context"
	"time"

	"boiler-go/internal/repository/db"
	"boiler-go/pkg/logger"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog"
)

type BaseRepo struct {
	queries *db.Queries
	log     zerolog.Logger
}

func NewBaseRepo(pool *sqlx.DB, log zerolog.Logger) *BaseRepo {
	return &BaseRepo{
		queries: db.New(pool),
		log:     log,
	}
}

// logQuery records query timing on the request-scoped logger when the context
// carries one, so a failed or slow query can be tied back to the request that
// caused it. It falls back to the repository's own logger for background work.
func (r *BaseRepo) logQuery(ctx context.Context, queryName, table string, err error, start time.Time) {
	log := r.log
	if requestLog, ok := logger.FromContextOK(ctx); ok {
		log = requestLog
	}

	elapsed := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Str("query", queryName).
			Str("table", table).
			Dur("duration", elapsed).
			Msg("query failed")
		return
	}

	log.Debug().
		Str("query", queryName).
		Str("table", table).
		Dur("duration", elapsed).
		Msg("query executed")
}
