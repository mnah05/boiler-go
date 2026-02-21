package config

import (
	"log"
	"sync"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	// server
	AppPort string `env:"APP_PORT" envDefault:"8080"`

	// database
	DatabaseURL string `env:"DATABASE_URL,required"`

	// redis / asynq
	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	// timeouts
	HealthCheckTimeout    time.Duration `env:"HEALTH_CHECK_TIMEOUT" envDefault:"2s"`
	APIShutdownTimeout    time.Duration `env:"API_SHUTDOWN_TIMEOUT" envDefault:"10s"`
	WorkerShutdownTimeout time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"30s"`
}

var (
	cfg  *Config
	once sync.Once
)

// Load initializes configuration and FAILS FAST if anything is wrong.
func Load() *Config {
	once.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Println("no .env file found (using system environment)")
		}

		c := Config{}

		if err := env.Parse(&c); err != nil {
			log.Fatalf("failed to load config: %v", err)
		}

		// Basic validation
		if c.DatabaseURL == "" {
			log.Fatalf("DATABASE_URL is required")
		}
		if c.RedisAddr == "" {
			log.Fatalf("REDIS_ADDR is required")
		}
		if c.HealthCheckTimeout <= 0 {
			log.Fatalf("HEALTH_CHECK_TIMEOUT must be positive")
		}
		if c.APIShutdownTimeout <= 0 {
			log.Fatalf("API_SHUTDOWN_TIMEOUT must be positive")
		}
		if c.WorkerShutdownTimeout <= 0 {
			log.Fatalf("WORKER_SHUTDOWN_TIMEOUT must be positive")
		}

		cfg = &c
	})

	return cfg
}
