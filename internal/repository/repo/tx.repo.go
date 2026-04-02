package repo

import (
	"context"
	"fmt"

	"boiler-go/internal/repository/db"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
)

type TxManager struct {
	pool *pgxpool.Pool
	log  zerolog.Logger
}

func NewTxManager(pool *pgxpool.Pool, log zerolog.Logger) *TxManager {
	return &TxManager{
		pool: pool,
		log:  log,
	}
}

func (tm *TxManager) Execute(ctx context.Context, fn func(*db.Queries) error) error {
	tx, err := tm.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("tx manager: begin: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				tm.log.Error().Err(rbErr).Msg("tx manager: rollback failed")
			}
		}
	}()

	if err := fn(db.New(tx)); err != nil {
		return fmt.Errorf("tx manager: execute: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("tx manager: commit: %w", err)
	}
	committed = true

	tm.log.Debug().Msg("tx manager: transaction completed")
	return nil
}
