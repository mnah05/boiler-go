package config

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	AppHost string `env:"APP_HOST" envDefault:"127.0.0.1"`
	AppPort string `env:"APP_PORT" envDefault:"8080"`

	DatabaseURL string `env:"DATABASE_URL,required"`

	RedisAddr     string `env:"REDIS_ADDR,required"`
	RedisPassword string `env:"REDIS_PASSWORD"`
	RedisDB       int    `env:"REDIS_DB" envDefault:"0"`

	WorkerConcurrency int `env:"WORKER_CONCURRENCY" envDefault:"10"`

	DBMaxConns          int32         `env:"DB_MAX_CONNS" envDefault:"15"`
	DBMinConns          int32         `env:"DB_MIN_CONNS" envDefault:"2"`
	DBMaxConnLifetime   time.Duration `env:"DB_MAX_CONN_LIFETIME" envDefault:"30m"`
	DBMaxConnIdleTime   time.Duration `env:"DB_MAX_CONN_IDLE_TIME" envDefault:"5m"`
	DBHealthCheckPeriod time.Duration `env:"DB_HEALTH_CHECK_PERIOD" envDefault:"1m"`

	RedisPoolSize     int           `env:"REDIS_POOL_SIZE" envDefault:"20"`
	RedisMinIdleConns int           `env:"REDIS_MIN_IDLE_CONNS" envDefault:"5"`
	RedisDialTimeout  time.Duration `env:"REDIS_DIAL_TIMEOUT" envDefault:"5s"`
	RedisReadTimeout  time.Duration `env:"REDIS_READ_TIMEOUT" envDefault:"3s"`
	RedisWriteTimeout time.Duration `env:"REDIS_WRITE_TIMEOUT" envDefault:"3s"`

	HealthCheckTimeout    time.Duration `env:"HEALTH_CHECK_TIMEOUT" envDefault:"2s"`
	APIShutdownTimeout    time.Duration `env:"API_SHUTDOWN_TIMEOUT" envDefault:"10s"`
	WorkerShutdownTimeout time.Duration `env:"WORKER_SHUTDOWN_TIMEOUT" envDefault:"30s"`

	LogOutput string `env:"LOG_OUTPUT" envDefault:"stdout"`
	LogFile   string `env:"LOG_FILE"`
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`

	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:"," envDefault:"http://localhost:3000"`

	JWTSecret         string        `env:"JWT_SECRET,required"`
	RequestTimeout    time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
	SecurityHSTSEnabled bool        `env:"SECURITY_HSTS_ENABLED" envDefault:"false"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		// Only treat it as an error if the file exists but is malformed.
		// If it doesn't exist, that's fine.
		if _, ok := err.(*url.Error); !ok && err.Error() != "open .env: no such file or directory" {
			// godotenv returns generic errors, so we just log and continue
		}
	}

	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, fmt.Errorf("parse env: %w", err)
	}

	if err := c.validate(); err != nil {
		return nil, err
	}

	return &c, nil
}

func (c *Config) validate() error {
	if err := validateDatabaseURL(c.DatabaseURL); err != nil {
		return fmt.Errorf("invalid DATABASE_URL: %w", err)
	}
	if c.RedisAddr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	if c.RedisDB < 0 || c.RedisDB > 15 {
		return fmt.Errorf("REDIS_DB must be between 0 and 15")
	}
	if err := validatePort(c.AppPort); err != nil {
		return fmt.Errorf("invalid APP_PORT: %w", err)
	}
	if c.HealthCheckTimeout <= 0 {
		return fmt.Errorf("HEALTH_CHECK_TIMEOUT must be positive")
	}
	if c.APIShutdownTimeout <= 0 {
		return fmt.Errorf("API_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.WorkerShutdownTimeout <= 0 {
		return fmt.Errorf("WORKER_SHUTDOWN_TIMEOUT must be positive")
	}
	if c.WorkerConcurrency <= 0 {
		return fmt.Errorf("WORKER_CONCURRENCY must be positive")
	}
	if c.DBMaxConns <= 0 {
		return fmt.Errorf("DB_MAX_CONNS must be positive")
	}
	if c.DBMinConns < 0 {
		return fmt.Errorf("DB_MIN_CONNS must be non-negative")
	}
	if c.DBMinConns > c.DBMaxConns {
		return fmt.Errorf("DB_MIN_CONNS cannot exceed DB_MAX_CONNS")
	}
	if c.DBMaxConnLifetime <= 0 {
		return fmt.Errorf("DB_MAX_CONN_LIFETIME must be positive")
	}
	if c.DBMaxConnIdleTime <= 0 {
		return fmt.Errorf("DB_MAX_CONN_IDLE_TIME must be positive")
	}
	if c.DBHealthCheckPeriod <= 0 {
		return fmt.Errorf("DB_HEALTH_CHECK_PERIOD must be positive")
	}
	if c.RedisPoolSize <= 0 {
		return fmt.Errorf("REDIS_POOL_SIZE must be positive")
	}
	if c.RedisMinIdleConns < 0 {
		return fmt.Errorf("REDIS_MIN_IDLE_CONNS must be non-negative")
	}
	if c.RedisMinIdleConns > c.RedisPoolSize {
		return fmt.Errorf("REDIS_MIN_IDLE_CONNS cannot exceed REDIS_POOL_SIZE")
	}
	if c.RedisDialTimeout <= 0 {
		return fmt.Errorf("REDIS_DIAL_TIMEOUT must be positive")
	}
	if c.RedisReadTimeout <= 0 {
		return fmt.Errorf("REDIS_READ_TIMEOUT must be positive")
	}
	if c.RedisWriteTimeout <= 0 {
		return fmt.Errorf("REDIS_WRITE_TIMEOUT must be positive")
	}
	if c.LogOutput != "stdout" && c.LogOutput != "file" && c.LogOutput != "both" {
		return fmt.Errorf("LOG_OUTPUT must be one of: stdout, file, both")
	}
	if (c.LogOutput == "file" || c.LogOutput == "both") && c.LogFile == "" {
		return fmt.Errorf("LOG_FILE is required when LOG_OUTPUT is file or both")
	}
	if len(c.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("REQUEST_TIMEOUT must be positive")
	}
	return nil
}

func validatePort(port string) error {
	if port == "" {
		return fmt.Errorf("port cannot be empty")
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if p < 1 || p > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}

func validateDatabaseURL(dbURL string) error {
	u, err := url.Parse(dbURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("URL scheme must be 'postgres' or 'postgresql', got '%s'", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must contain a host")
	}
	if u.Path == "" || u.Path == "/" {
		return fmt.Errorf("URL must contain a database name")
	}
	return nil
}
