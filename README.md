# Boiler-Go

A production-ready Go backend boilerplate with clean architecture, PostgreSQL, Redis, and background job processing.

---

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/mnah05/boiler-go.git
cd boiler-go

# Copy environment file
cp .env.example .env

# Start services with Docker
make dev

# Run API server
make api

# Run background worker
make worker
```

---

## 📋 Features

- ✅ **Thread-Safe Database Pool** - Concurrent-safe PostgreSQL connection management with `pgx`
- ✅ **Repository Pattern** - Clean data access layer with base repository, query logging, and transaction support
- ✅ **Graceful Shutdown** - Shared utilities for proper resource cleanup and timeout handling
- ✅ **Background Jobs** - Redis-based task processing with Asynq
- ✅ **Worker Management** - API endpoints for worker status and ping testing
- ✅ **Health Checks** - Lightweight service health monitoring with duration tracking
- ✅ **Structured Logging** - JSON logging with request tracing and correlation IDs
- ✅ **Environment Configuration** - Flexible config with validation (returns errors, no logger injection)
- ✅ **CORS Support** - Configurable per-origin CORS (no wildcard)
- ✅ **Rate Limiting** - Token bucket rate limiter (10 req/sec per IP) via `httprate`
- ✅ **Request Size Limiting** - Global 1MB request body size limit
- ✅ **Input Validation** - Struct validation with `go-playground/validator`
- ✅ **Security Hardened** - Secure log permissions (0600), configurable CORS, health endpoints excluded from rate limiting
- ✅ **Error Handling** - Standardized JSON error responses with HTTP status codes
- ✅ **Database Migrations** - Schema versioning with golang-migrate
- ✅ **Docker Support** - Containerized development environment
- ✅ **Comprehensive Documentation** - Error handling strategy with retry policies

---

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│   API Server    │───▶│   PostgreSQL    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │                         ▲
                              │                         │
                              ▼                         │
                       ┌─────────────────┐              │
                       │  Task Scheduler │──────────────┘
                       │  (Redis Queue)  │
                       └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │  Worker Process │
                       │  (Job Consumer) │
                       └─────────────────┘
```

### Request Flow

```
HTTP Request
    │
    ▼
┌─────────────────┐
│ SecurityHeaders │ ← X-Content-Type-Options, X-Frame-Options, CSP
└─────────────────┘
    │
    ▼
┌─────────────────┐
│  CORS Handler   │ ← Configurable per-origin CORS
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ RequestLogger   │ ← Injects request_id and logger into context
└─────────────────┘
    │
    ▼
┌─────────────────┐
│ MaxBodySize     │ ← 1MB global limit
└─────────────────┘
    │
    ▼
┌─────────────────┐
│  HTTP Handler   │ ← Uses request-scoped logger, enqueues tasks with request_id
└─────────────────┘
    │
    ├──► Database (pgx pool)
    │
    └──► Redis Queue (Asynq)
              │
              ▼
         ┌─────────┐
         │  Worker │ ← Processes task, logs with original request_id
         └─────────┘
```

### Project Structure

```
boiler-go/
├── cmd/
│   ├── api/                 # HTTP API server entry point
│   └── worker/              # Background job processor entry point
├── internal/
│   ├── config/              # Environment configuration and validation
│   ├── handler/             # HTTP request handlers
│   ├── middleware/          # HTTP middleware (security, CORS, logging, auth, body limit)
│   ├── queue/               # Shared queue names and priority configuration
│   ├── repository/          # Data access layer
│   │   ├── pool/            # Database connection pool
│   │   ├── repo/            # Repository implementations with logging
│   │   └── db/              # sqlc-generated database code
│   ├── scheduler/           # Job scheduling client (Asynq wrapper)
│   ├── tasks/               # Shared task type constants
│   └── validator/           # Input validation utilities
├── pkg/
│   └── logger/              # Structured logging utilities with global fallback
├── migrations/              # Database migration files (golang-migrate)
├── sql/                     # SQL schema and queries for sqlc
└── docker-compose.yml
```

### Package Responsibilities

| Package | Purpose | Key Types/Functions |
|---------|---------|---------------------|
| `internal/config` | Environment parsing and validation | `Load()` → `(*Config, error)`, `Config` struct |
| `internal/repository/pool` | Thread-safe database pool | `Open(ctx, cfg)`, `Get()`, `Close()` |
| `internal/repository/repo` | Data access layer with logging | `BaseRepo`, `UserRepo` |
| `internal/handler` | HTTP request handlers | `HealthHandler`, `WorkerHandler`, `RepoTestHandler` |
| `internal/middleware` | HTTP middleware | `RequestLogger()`, `MaxBodySize()` |
| `internal/queue` | Queue configuration | `Names()`, `Priorities()` |
| `internal/scheduler` | Task enqueueing | `Client.Enqueue()`, `Client.EnqueueWithID()` |
| `internal/tasks` | Task type constants | `TypeWorkerPing` |
| `internal/validator` | Struct validation | `ValidateStruct()`, `GetValidationErrors()` |
| `pkg/jwtpkg` | JWT token generation and parsing | `GenerateToken()`, `ParseToken()`, `NewClaims()` |
| `pkg/logger` | Logging utilities | `New()`, `Global()`, `FromChiContext()` |

---

## ⚙️ Configuration

### Environment Variables

```bash
# Server
APP_PORT=8080

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5432/appdb?sslmode=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Worker
WORKER_CONCURRENCY=10

# Security
JWT_SECRET=your-secret-key-here-must-be-at-least-32-chars

# Timeouts
HEALTH_CHECK_TIMEOUT=2s
API_SHUTDOWN_TIMEOUT=10s
WORKER_SHUTDOWN_TIMEOUT=30s

# Logging
LOG_OUTPUT=stdout        # Options: stdout, file, both
LOG_FILE=logs/app.log    # Required when LOG_OUTPUT is file or both
LOG_LEVEL=info           # Options: debug, info, warn, error

# Security
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://example.com
```

#### Security Configuration

**CORS_ALLOWED_ORIGINS**: Comma-separated list of allowed origins. Never use `*` in production. Example:
```bash
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://myapp.com
```

**LOG_OUTPUT & LOG_FILE**: When set to `file` or `both`, log files are written with `0600` permissions and log directories with `0750` for security.

### Database Configuration

The database pool (`internal/repository/pool`) is configured with sensible defaults:

- **Max Connections**: 15
- **Min Connections**: 2
- **Connection Lifetime**: 30 minutes
- **Idle Timeout**: 5 minutes
- **Health Check Period**: 1 minute

The pool initialization accepts a `context.Context` for timeout control during startup.

### Repository Pattern

The codebase implements a clean repository pattern with the following features:

```go
// Base repository with query logging
baseRepo := repo.NewBaseRepo(pool, log)

// Specific repositories
userRepo := repo.NewUserRepo(pool, log)

// Transaction support via repo layer
userRepo := repo.NewUserRepo(pool, log)
tx, err := pool.Begin(ctx)
if err != nil { ... }
defer tx.Rollback(ctx)

txRepo := userRepo.WithTx(tx)
user, err := txRepo.Create(ctx, params)
if err != nil { ... }
// ... more operations

if err := tx.Commit(ctx); err != nil { ... }
```

All database operations include automatic query logging with duration tracking.

---

## 🔧 Development

### Prerequisites

- Go 1.24.0
- Docker & Docker Compose
- PostgreSQL
- Redis

### Setup

1. **Start Infrastructure**

   ```bash
   make dev
   ```

2. **Run Migrations**

   ```bash
   make migrate-up
   ```

3. **Generate SQL Code**
   ```bash
   make sqlc
   ```

### Running Services

```bash
# Start API server
make api

# Start background worker
make worker

# Stop services
make stop-api        # Stop API server
make stop-worker     # Stop worker process
make stop            # Stop all local services
make dev-down        # Stop Docker containers
```

---

## 🏥 Health Check

The `/health` endpoint provides service status with duration tracking:

```json
{
  "status": {
    "database": "up",
    "redis": "up"
  },
  "checked": "2024-02-21T20:41:00Z",
  "duration": 12
}
```

Health check completion is logged at `Info` level for operational visibility.

---

## 📦 Dependencies

### Core Backend

- **[Chi](https://github.com/go-chi/chi)** - Lightweight, idiomatic and composable HTTP router
- **[pgx/v5](https://github.com/jackc/pgx)** - PostgreSQL driver
- **[sqlc](https://sqlc.dev/)** - Type-safe SQL code generation
- **[go-playground/validator](https://github.com/go-playground/validator)** - Struct validation

### Background Jobs & Caching

- **[asynq](https://github.com/hibiken/asynq)** - Redis-based job queue
- **[go-redis](https://github.com/redis/go-redis)** - Redis client

### Configuration & Logging

- **[env/v11](https://github.com/caarlos0/env)** - Environment variable parsing
- **[zerolog](https://github.com/rs/zerolog)** - Structured JSON logging

---

## 🛡️ Production Readiness

This boilerplate includes several production-ready features:

### Thread Safety

- Database pool uses `sync.RWMutex` for concurrent access
- Configuration returns errors for safe initialization (no singleton pattern)
- All shared resources are properly synchronized

### Logging

- **Structured JSON logging** throughout the application
- **Request correlation** - HTTP `X-Request-ID` is propagated to worker logs via task payloads
- **Global fallback** - `FromChiContext` falls back to a global logger instead of silently dropping logs
- **Consistent format** - Config uses the same logger as the rest of the app

### Error Handling

- Comprehensive error checking and logging
- Graceful degradation on service failures
- Proper resource cleanup on errors

### Resource Management

- Connection pooling with configurable limits
- Context-aware database initialization with timeouts
- Automatic cleanup on shutdown via graceful shutdown with timeout handling
- Memory leak prevention

### Rate Limiting

IP-based token bucket rate limiter via `go-chi/httprate`:
- **Rate**: 10 requests per second
- **Key**: Client IP (via `httprate.KeyByIP`, strips port)
- **Excluded Endpoints**: `/health` and `/worker/health` (for monitoring)
- **Response**: HTTP 429 with JSON error when limit exceeded

### Request Size Limits

Global 1MB request body size limit enforced via `internal/middleware.MaxBodySize()`:
- Returns HTTP 413 (Request Entity Too Large) if exceeded
- Applied to all routes except health checks
- Prevents DoS attacks via large payloads

### Error Handling

Standardized error response format across all endpoints:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "details": ["optional", "field-level", "errors"]
}
```

**HTTP Status Codes:**

| Code | When to Use |
|------|-------------|
| 400  | Malformed JSON, missing required fields |
| 401  | Missing or invalid authentication |
| 403  | Authenticated but not authorized |
| 404  | Resource not found |
| 413  | Request body exceeds size limit |
| 422  | Struct validation failures (with details) |
| 429  | Rate limit exceeded |
| 500  | Unexpected internal errors (generic message) |
| 503  | Dependency unavailable (Redis, DB down) |

**Validation Errors:**

```json
{
  "error": "validation_failed",
  "details": ["Email must be a valid email", "Name is required"]
}
```

See [ERROR_HANDLING.md](ERROR_HANDLING.md) for complete documentation including retry policies and circuit breaker patterns.

### Monitoring

- Health check endpoints for all services
- Structured logging with request tracing and correlation IDs
- Error metrics and alerting ready

---

## 📝 API Endpoints

### Health Check

```
GET /health
```

Returns the status of database and Redis connections with response duration in milliseconds. This endpoint is safe for frequent polling by load balancers — it does not enqueue background jobs.

### Worker Management

```
GET  /worker/health
GET  /worker/status
POST /worker/ping
```

#### Worker Health

Returns the status of database and Redis connections with response duration in milliseconds. This endpoint is safe for frequent polling by load balancers.

```json
{
  "status": {
    "database": "up",
    "redis": "up"
  },
  "checked": "2024-02-21T20:41:00Z",
  "duration": 5
}
```

#### Worker Status

Returns scheduler connectivity and available queue information:

```json
{
  "scheduler": "connected",
  "queues": ["critical", "default", "low"],
  "note": "Use POST /worker/ping to test task processing"
}
```

Queue names are sourced from `internal/queue` package for consistency with the worker configuration.

#### Worker Ping

Enqueues a test task to verify worker is processing jobs. The request ID is propagated to the worker for end-to-end tracing.

**Request Validation:**
- `message` field: optional, max 500 characters

**Example Requests:**

```bash
# With custom message and request ID
curl -X POST http://localhost:8080/worker/ping \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: req-12345" \
  -d '{"message": "test from curl"}'

# Without message (uses default)
curl -X POST http://localhost:8080/worker/ping
```

Response:

```json
{
  "success": true,
  "data": {
    "task_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "task_type": "worker:ping",
    "queued_at": "2024-02-21T20:41:00.000000000Z"
  },
  "message": "task queued successfully"
}
```

Worker logs will include the original `request_id` for correlation.

**Validation Error Response:**

```json
{
  "error": "validation failed",
  "details": ["Message exceeds maximum length"]
}
```

**Rate Limit Error Response:**

```json
{
  "error": "rate limit exceeded",
  "message": "too many requests"
}
```

### Repository Test Endpoints

Test endpoints demonstrating the repository pattern:

```
GET    /repo-test/users       # List users (limit: 10)
POST   /repo-test/users       # Create user
GET    /repo-test/users/get   # Get user by ID (query: id)
```

**Create User:**

```bash
curl -X POST http://localhost:8080/repo-test/users \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "name": "John Doe"}'
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "name": "John Doe",
  "created_at": "2024-01-15T10:30:00.000000000Z",
  "updated_at": "2024-01-15T10:30:00.000000000Z"
}
```

**Get User:**

```bash
curl "http://localhost:8080/repo-test/users/get?id=550e8400-e29b-41d4-a716-446655440000"
```

**List Users:**

```bash
curl http://localhost:8080/repo-test/users
```

---

## 🧪 Testing

```bash
# Run all tests
make test

# Run with race detection
go test -race ./...

# Run integration tests
go test -tags=integration ./...
```

---

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t boiler-go .

# Run container
docker run -p 8080:8080 --env-file .env boiler-go
```

### Environment Variables

Ensure all required environment variables are set in production:

```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require
REDIS_ADDR=redis-host:6379
APP_PORT=8080
```

---

## 🏗️ Design Patterns

### Shared Constants Pattern

Task types and queue names are defined in dedicated packages (`internal/tasks`, `internal/queue`) to ensure consistency between handlers and workers:

```go
// internal/tasks/tasks.go
const TypeWorkerPing = "worker:ping"

// Used in both handler and worker
import "boiler-go/internal/tasks"
tasks.TypeWorkerPing
```

### Context-Aware Initialization

Database and other external connections accept a `context.Context` for timeout control:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := db.Open(ctx, cfg); err != nil {
    log.Fatal(err)
}
```

### Graceful Shutdown Pattern

Both API server and worker handle shutdown gracefully with timeout control:

```go
// API server - shutdown with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.APIShutdownTimeout)
defer cancel()

if err := server.Shutdown(shutdownCtx); err != nil {
    logg.Error().Err(err).Msg("server shutdown failed")
}

// Worker - stop accepting new tasks, then shutdown with timeout
srv.Stop()

shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.WorkerShutdownTimeout)
defer cancel()

done := make(chan struct{})
go func() {
    srv.Shutdown()
    close(done)
}()

select {
case <-done:
    logg.Info().Msg("worker shutdown completed gracefully")
case <-shutdownCtx.Done():
    logg.Warn().Msg("worker shutdown timed out")
}
```
