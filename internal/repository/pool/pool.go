package pool

import (
	"boiler-go/internal/config"
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool *pgxpool.Pool
	mu   sync.RWMutex
)

func Open(ctx context.Context, cfg *config.Config) error {
	mu.Lock()
	defer mu.Unlock()

	if pool != nil {
		return fmt.Errorf("database pool already initialized")
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database config: %w", err)
	}
	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MinConns = cfg.DBMinConns
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolConfig.MaxConnIdleTime = cfg.DBMaxConnIdleTime
	poolConfig.HealthCheckPeriod = cfg.DBHealthCheckPeriod

	newPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to create pool: %w", err)
	}

	if err := newPool.Ping(ctx); err != nil {
		newPool.Close() // Clean up the pool on ping failure
		return fmt.Errorf("database unreachable: %w", err)
	}

	pool = newPool
	return nil
}

func Get() *pgxpool.Pool {
	mu.RLock()
	defer mu.RUnlock()
	return pool
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if pool != nil {
		pool.Close()
		pool = nil
	}
}
