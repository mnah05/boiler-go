// Package cache provides a best-effort Redis cache. Redis outages never make
// HTTP requests fail: operations return ErrUnavailable and a monitor restores
// the cache automatically once Redis becomes reachable again.
package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"boiler-go/internal/config"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

var ErrUnavailable = errors.New("cache unavailable")

type Store interface {
	Get(context.Context, string) (string, error)
	Set(context.Context, string, any, time.Duration) error
	Delete(context.Context, ...string) error
	Available() bool
	Close() error
}

type RedisStore struct {
	client    *redis.Client
	available atomic.Bool
	interval  time.Duration
	log       zerolog.Logger
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

func New(cfg *config.Config, log zerolog.Logger) *RedisStore {
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB, PoolSize: cfg.RedisPoolSize, MinIdleConns: cfg.RedisMinIdleConns, DialTimeout: cfg.RedisDialTimeout, ReadTimeout: cfg.RedisReadTimeout, WriteTimeout: cfg.RedisWriteTimeout})
	ctx, cancel := context.WithCancel(context.Background())
	s := &RedisStore{client: client, interval: cfg.RedisReconnectInterval, log: log, cancel: cancel, done: make(chan struct{})}
	go s.monitor(ctx)
	return s
}

func (s *RedisStore) monitor(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		s.probe(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *RedisStore) probe(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, min(s.interval, 3*time.Second))
	err := s.client.Ping(ctx).Err()
	cancel()
	wasAvailable := s.available.Swap(err == nil)
	if err != nil && wasAvailable {
		s.log.Warn().Err(err).Msg("redis cache unavailable; continuing without cache")
	}
	if err == nil && !wasAvailable {
		s.log.Info().Msg("redis cache available")
	}
}

func (s *RedisStore) Available() bool { return s.available.Load() }

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	if !s.Available() {
		return "", ErrUnavailable
	}
	value, err := s.client.Get(ctx, key).Result()
	if err != nil && err != redis.Nil {
		s.available.Store(false)
	}
	return value, err
}

func (s *RedisStore) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	if !s.Available() {
		return ErrUnavailable
	}
	err := s.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		s.available.Store(false)
	}
	return err
}

func (s *RedisStore) Delete(ctx context.Context, keys ...string) error {
	if !s.Available() {
		return ErrUnavailable
	}
	err := s.client.Del(ctx, keys...).Err()
	if err != nil {
		s.available.Store(false)
	}
	return err
}

func (s *RedisStore) Close() error {
	var err error
	s.closeOnce.Do(func() { s.cancel(); <-s.done; err = s.client.Close() })
	return err
}
