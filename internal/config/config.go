package config

import (
	"log"
	"sync"

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

		cfg = &c
	})

	return cfg
}
