package pool

import (
	"boiler-go/internal/config"
	"context"
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var (
	pool *sqlx.DB
	mu   sync.RWMutex
)

func Open(ctx context.Context, cfg *config.Config) error {
	mu.Lock()
	defer mu.Unlock()

	if pool != nil {
		return fmt.Errorf("database pool already initialized")
	}

	newPool, err := sqlx.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	newPool.SetMaxOpenConns(int(cfg.DBMaxConns))
	newPool.SetMaxIdleConns(int(cfg.DBMaxIdleConns))
	newPool.SetConnMaxLifetime(cfg.DBMaxConnLifetime)
	newPool.SetConnMaxIdleTime(cfg.DBMaxConnIdleTime)

	if err := newPool.DB.PingContext(ctx); err != nil {
		newPool.Close() // Clean up the pool on ping failure
		return fmt.Errorf("database unreachable: %w", err)
	}

	pool = newPool
	return nil
}

func Get() *sqlx.DB {
	mu.RLock()
	defer mu.RUnlock()
	return pool
}

func Begin(ctx context.Context) (*sql.Tx, error) {
	p := Get()
	if p == nil {
		return nil, fmt.Errorf("database pool not initialized")
	}
	return p.DB.BeginTx(ctx, nil)
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if pool != nil {
		pool.Close()
		pool = nil
	}
}
