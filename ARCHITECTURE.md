# Boiler-Go Architecture Report

> **Generated:** April 25, 2026
>
> A production-ready Go backend boilerplate with clean architecture, PostgreSQL, Redis, and background job processing.

---

## 📋 Table of Contents

1. [System Overview](#1-system-overview)
2. [Project Structure](#2-project-structure)
3. [Configuration System](#3-configuration-system)
4. [Entry Points (cmd/)](#4-entry-points-cmd)
5. [HTTP Layer (internal/handler)](#5-http-layer-internalhandler)
6. [Middleware Stack (internal/middleware)](#6-middleware-stack-internalmiddleware)
7. [Authentication & Authorization](#7-authentication--authorization)
8. [Database Layer](#8-database-layer)
9. [Job Queue System](#9-job-queue-system)
10. [Logging System](#10-logging-system)
11. [Validation System](#11-validation-system)
12. [Request Lifecycle (Full Trace)](#12-request-lifecycle-full-trace)
13. [Security Posture](#13-security-posture)
14. [Startup & Shutdown](#14-startup--shutdown)
15. [Known Gaps & Limitations](#15-known-gaps--limitations)
16. [Key Design Decisions](#16-key-design-decisions)
17. [Quick Reference](#17-quick-reference)

---

## 1. System Overview

```
┌─────────────┐     ┌──────────────┐     ┌────────────┐
│ HTTP Client  │────▶│  API Server  │────▶│ PostgreSQL │
│             │     │  (cmd/api)   │     └────────────┘
└─────────────┘     └──────┬───────┘           ▲
                           │                    │
                           ▼                    │
                    ┌──────────────┐            │
                    │Task Scheduler│────────────┘
                    │(Asynq Client)│
                    └──────┬───────┘
                           │
                           ▼
                    ┌──────────────┐
                    │   Worker     │
                    │ (cmd/worker) │
                    └──────────────┘
```

### 🔑 Key Facts

| Aspect | Detail |
|--------|--------|
| **Language** | Go 1.24.0 |
| **Module** | `boiler-go` |
| **Architecture** | Two binaries sharing common internal packages |
| **Router** | go-chi/chi v5 |
| **Database** | PostgreSQL via pgx/v5 + sqlc code generation |
| **Job Queue** | Redis-backed Asynq (hibiken/asynq) |
| **Logging** | Structured JSON via rs/zerolog |
| **Auth** | JWT (HS256) via golang-jwt/jwt/v5 |

---

## 2. Project Structure

```
boiler-go/
├── cmd/                          # 📌 ENTRY POINTS
│   ├── api/main.go               #    HTTP API server
│   └── worker/main.go            #    Background job consumer
│
├── internal/                     # 📌 APPLICATION CORE (not importable externally)
│   ├── config/config.go          #    ⚙️ Env-var configuration + validation
│   │
│   ├── handler/                  #    🌐 HTTP request handlers
│   │   ├── router.go             #       Chi router + middleware wiring
│   │   ├── response.go           #       JSON response helpers
│   │   ├── health.go             #       GET /health
│   │   ├── worker.go             #       /worker/* endpoints
│   │   ├── repo_handler.go       #       /repo-test/* CRUD demo
│   │   └── not_found.go          #       Global 404
│   │
│   ├── middleware/               #    🛡️ HTTP middleware
│   │   ├── body_limit.go         #       1MB request body limit
│   │   ├── jwt.go                #       JWT Bearer token auth
│   │   ├── logger.go             #       Request-scoped logger
│   │   ├── rbac.go               #       Role-based access control
│   │   └── security.go           #       Security headers (CSP, HSTS)
│   │

## 3. Configuration System

**File:** `internal/config/config.go`

### ⚙️ How It Works

1. `.env` file loaded via `godotenv.Load()` (errors silently ignored)
2. Environment variables parsed into `Config` struct via `caarlos0/env/v11`
3. `validate()` method runs ~20 validation checks

### 🔑 Important Validation Rules

```go
// These will cause startup FAILURE:
JWT_SECRET           // Required, minimum 32 characters
DATABASE_URL         // Required, must be postgres:// scheme
REDIS_ADDR            // Required
APP_PORT              // Must be 1-65535
LOG_OUTPUT            // Must be "stdout", "file", or "both"
DB_MIN_CONNS          // Cannot exceed DB_MAX_CONNS
REDIS_MIN_IDLE_CONNS  // Cannot exceed REDIS_POOL_SIZE
```

### 🎯 Key Config Fields

| Category | Fields | Defaults |
|----------|--------|----------|
| Server | `AppHost`, `AppPort` | `127.0.0.1:8080` |
| Database | `DatabaseURL` | (required) |
| DB Pool | `DBMaxConns`, `DBMinConns`, `DBMaxConnLifetime`, `DBMaxConnIdleTime`, `DBHealthCheckPeriod` | 15, 2, 30m, 5m, 1m |
| Redis | `RedisAddr`, `RedisPassword`, `RedisDB` | (required), "", 0 |
| Redis Pool | `RedisPoolSize`, `RedisMinIdleConns`, `RedisDialTimeout`, `RedisReadTimeout`, `RedisWriteTimeout` | 20, 5, 5s, 3s, 3s |
| Worker | `WorkerConcurrency` | 10 |
| Timeouts | `HealthCheckTimeout`, `APIShutdownTimeout`, `WorkerShutdownTimeout` | 2s, 10s, 30s |
| Logging | `LogOutput`, `LogLevel`, `LogFile` | stdout, info, "" |
| Security | `JWTSecret`, `SecurityHSTSEnabled`, `CORSAllowedOrigins` | (required), false, localhost:3000 |
| Request | `RequestTimeout` | 30s |

---

## 4. Entry Points (cmd/)

Both binaries follow the same startup sequence:

### 🚀 Startup Flow

```
1. Create boot logger (early logging before config)
2. config.Load() — load & validate env vars
3. logger.NewLogger() — create application logger
4. pool.Open(ctx, cfg) — connect to PostgreSQL (10s timeout)
5. redis.NewClient() + rdb.Ping() — connect to Redis (5s timeout)
6. Initialize domain-specific dependencies
7. Start service (HTTP server or Asynq worker)
8. Wait for OS signal (SIGINT/SIGTERM)
9. Graceful shutdown
```

### ⚡ API Server (`cmd/api/main.go`)

```

## 5. HTTP Layer (internal/handler)

### 🗺️ Route Map

| Method | Path | Auth | Rate Limited | Description |
|--------|------|------|-------------|-------------|
| GET | `/health` | ❌ | ❌ | DB + Redis health check |
| GET | `/worker/health` | ❌ | ❌ | Worker health check |
| GET | `/worker/status` | ❌ | ✅ | Queue info + dependency status |
| POST | `/worker/ping` | ❌ | ✅ | Enqueue ping task |
| POST | `/repo-test/users` | ✅ JWT | ✅ | Create user |
| GET | `/repo-test/users` | ✅ JWT | ✅ | List users |
| GET | `/repo-test/users/get` | ✅ JWT | ✅ | Get user by ID |

### 📦 Response Envelope

**Error:**
```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "details": ["optional", "field-level", "errors"]
}
```

**Success:**
```json
{
  "success": true,
  "data": { ... },
  "message": "optional message"
}
```

### 🔄 Health Check Flow (`health.go`)

```
Request → context.WithTimeout(timeout) → concurrent pings (sync.WaitGroup)
  ├── db.Ping(ctx)  → "up" or "down"
  └── rdb.Ping(ctx) → "up" or "down"

Response: 200 if both up, 503 otherwise
{
  "status": { "database": "up", "redis": "up" },
  "checked": "2026-04-25T...",
  "duration": 12
}
```

### 🧪 Repo Test Handler (`repo_handler.go`)

Demonstration CRUD showcasing repository pattern:

- **CreateUser**: validates input, normalizes email (lowercase+trim), handles PG unique violation (23505 → 409)
- **GetUser**: parses UUID from query param, returns 404 on not found
- **ListUsers**: optional limit param (default 10), repo caps at 1000

All handlers use:
- Request-scoped logger via `logger.FromChiContext()`
- Context timeouts (3-10s)
- Proper error wrapping and status codes

---

## 6. Middleware Stack (internal/middleware)

### 🏗️ Order of Execution

```
1. chi.RequestID         → Inject X-Request-ID into context
2. chi.Recoverer         → Recover from panics (500 response)
3. SecurityHeaders       → Set X-Content-Type-Options, X-Frame-Options, CSP, HSTS
4. cors.Handler          → Validate origin, set CORS headers
5. RequestLogger         → Create request-scoped zerolog logger
6. MaxBodySize(1MB)     → Wrap request body with MaxBytesReader
7. httprate (10/s)      → Token bucket rate limiter by IP
   (skips /health, /worker/health)

## 7. Authentication & Authorization

### 🔐 JWT System (`pkg/jwtpkg`)

```go
// Claims structure stored in JWT
type Claims struct {
    UserID string   `json:"user_id"`
    Roles  []string `json:"roles"`
    jwt.RegisteredClaims  // includes IssuedAt, ExpiresAt
}
```

**Key Rules:**
- **HS256 only** — enforced via `jwt.WithValidMethods([]string{"HS256"})`
- Custom signing methods rejected
- Token generation: `GenerateToken(claims, secret)`
- Token parsing: `ParseToken(tokenString, secret)` — validates signature, expiry, method
- Secret generation: `GenerateSecret()` ← 32 bytes, base64-encoded

### 🎭 RBAC System (`internal/middleware/rbac.go`)

Two middleware functions:

- **`RequireRole("admin")`** — user must have exactly "admin" role
- **`RequireAnyRole("admin", "moderator")`** — user must have at least one

Both extract roles from context (set by `JWTAuth`), return 403 on failure.

### 📦 Context Access

```go
// Extract stored values
UserIDFromContext(ctx) → (string, bool)
RolesFromContext(ctx)  → ([]string, bool)
```

---

## 8. Database Layer

### 🏊 Thread-Safe Pool (`internal/repository/pool`)

```
Singleton pattern guarded by sync.RWMutex

Open(ctx, cfg) → ParseConfig → pool.NewWithConfig → Ping → store
Get()          → return pool (nil if uninitialized)
Begin(ctx)     → pool.Begin(ctx) → pgx.Tx
Close()        → pool.Close() + set to nil
```

**Pool Configuration:**

| Setting | Default |
|---------|---------|
| Max Connections | 15 |
| Min Connections | 2 |
| Max Lifetime | 30 minutes |
| Max Idle Time | 5 minutes |
| Health Check | 1 minute |

### ⚡ sqlc Code Generation

```yaml
# sqlc.yaml configuration
sql_package: "pgx/v5"
emit_json_tags: true
emit_prepared_queries: true
emit_interface: false
```

## 9. Job Queue System

### 📋 Task Types (`internal/tasks`)

```go
const TypeWorkerPing = "worker:ping"

type PingTaskPayload struct {
    Message   string    `json:"message"`
    RequestID string    `json:"request_id"`  // For end-to-end tracing
    QueuedAt  time.Time `json:"queued_at"`
}
```

### 📬 Queue Configuration (`internal/queue`)

| Queue Name | Priority | Description |
|-----------|----------|-------------|
| `critical` | 6 (highest) | Time-sensitive tasks |
| `default` | 3 | Standard tasks |
| `low` | 1 (lowest) | Batch/non-urgent tasks |

### 📤 Scheduler Client (`internal/scheduler`)

```go
type Client struct {
    client *asynq.Client
}

func (c *Client) Enqueue(ctx, taskType, payload, opts...) error
func (c *Client) EnqueueWithID(ctx, taskType, payload, opts...) (string, error)
```

### ⚙️ Worker Configuration

```
Concurrency: config.WorkerConcurrency (default: 10)
Queues:      critical:6, default:3, low:1

Retry Policy (exponential backoff):
  Retry 1: 2s
  Retry 2: 4s
  Retry 3: 8s
  Retry N: min(1<<N, 64)s
  Max retries: configurable per task (usually 3)

Error Handler:
  Logs task_type, task_id, error on failure
```

### 🔄 End-to-End Tracing

```
API Handler                    Worker
┌──────────────┐              ┌──────────────┐
│ X-Request-ID │──────────────▶│ request_id   │
│ (from header)│  payload      │ (in payload) │
└──────────────┘              └──────────────┘

The worker logs include the original request_id for correlation.
```

---

## 10. Logging System

### 📝 Logger Capabilities (`pkg/logger`)

## 12. Request Lifecycle (Full Trace)

```
HTTP Request
    │
    ▼
┌─────────────────────────────┐
│ 1. chi.RequestID            │
│    └─ injects X-Request-ID  │
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 2. chi.Recoverer            │ ← panics → 500
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 3. SecurityHeaders          │
│    ├─ X-Content-Type-Options│
│    ├─ X-Frame-Options       │
│    ├─ Referrer-Policy       │
│    ├─ CSP                   │
│    └─ HSTS (optional)       │
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 4. CORS Handler             │
│    └─ Validate origin       │
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 5. RequestLogger            │
│    ├─ Create logger with    │
│    │  request_id, method,   │
│    │  path                  │
│    └─ Inject into context   │
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 6. MaxBodySize(1MB)        │ ← too large → 413
└─────────────────────────────┘
    │
    ▼
┌─────────────────────────────┐
│ 7. Rate Limiter (10/s/IP)  │ ← exceeded → 429
│    (SKIPS /health,          │
│     /worker/health)         │
└─────────────────────────────┘
    │
    ├──▶ (If route needs auth)
    │    ┌─────────────────────┐
    │    │ 8. JWTAuth          │
    │    │    ├─ Missing → 401 │

## 15. Known Gaps & Limitations

| # | Gap | Impact |
|---|-----|--------|
| 1 | **No tests** | `make test` runs against empty suite |
| 2 | `pool.Close()` returns void | Errors not propagated in shutdown |
| 3 | Worker `srv.Shutdown()` returns void (asynq v0.26.0 API) | No error capture |
| 4 | **No global request timeout middleware** | Relies on `WriteTimeout` (10s) + handler contexts |
| 5 | **No circuit breaker** | Documented as future work in `ERROR_HANDLING.md` |
| 6 | **No metrics/prometheus** | Only health checks + logging for observability |
| 7 | **Single JWT algorithm (HS256)** | No RSA/ECDSA support |
| 8 | **No automatic route protection introspection** | `IsRouteProtected` was removed |

---

## 16. Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Two binaries** (api + worker) | Separation of concerns; worker scales independently |
| **sqlc code generation** | Type-safe queries, no ORM, compile-time SQL validation |
| **Singleton DB pool (RWMutex)** | Thread-safe without DI framework complexity |
| **Repository pattern** | Clean data access separation, testable via interfaces |
| **Asynq for jobs** | Redis-backed, built-in retry/priority/queues |
| **Context-based logger** | Request tracing without explicit parameter passing |
| **Env-var config (12-factor)** | No YAML complexity, cloud-native |
| **No DI framework** | Simplicity; dependencies wired manually in main() |
| **Shared constants pattern** | Task types and queue names defined once, imported everywhere |
| **Panic recovery in worker shutdown** | Prevents silent crashes from `srv.Shutdown()` |

---

## 17. Quick Reference

### 🔧 Make Commands

```bash
make dev              # Start PostgreSQL + Redis (Docker)
make dev-down         # Stop Docker containers
make migrate-up       # Apply DB migrations
make migrate-down     # Rollback one migration
make migrate-create name=...  # Create new migration
make sqlc             # Regenerate sqlc Go code
make api              # Start API server
make worker           # Start background worker
make test             # Run tests (currently empty)
make build            # Build both binaries
make fmt              # go fmt ./...
make vet              # go vet ./...
make lint             # golangci-lint run
make check            # fmt → vet → lint → test
make hooks            # Install git pre-commit hooks
make clean            # Stop everything + remove binaries
```

### ⚡ Dependencies Map

| Package | Version | Purpose |
|---------|---------|---------|
| go-chi/chi/v5 | v5.1.0 | HTTP router |
| go-chi/cors | v1.2.2 | CORS middleware |
| go-chi/httprate | v0.15.0 | Rate limiting |
| jackc/pgx/v5 | v5.8.0 | PostgreSQL driver + pool |
| hibiken/asynq | v0.26.0 | Redis job queue |
| redis/go-redis/v9 | v9.18.0 | Redis client |
| caarlos0/env/v11 | v11.3.1 | Env var parsing |
| joho/godotenv | v1.5.1 | .env file loading |
| rs/zerolog | v1.34.0 | Structured logging |
| go-playground/validator/v10 | v10.30.1 | Struct validation |
| golang-jwt/jwt/v5 | v5.3.1 | JWT implementation |
| google/uuid | v1.6.0 | UUID generation |
| sqlc (dev) | v1.30.0 | SQL code generation |
| golang-migrate (dev) | CLI | Database migrations |

---

> **End of Architecture Report**

    │    │    ├─ Invalid → 401 │
    │    │    └─ Valid → store │
    │    │       user_id+roles │
    │    └─────────────────────┘
    │         │
    │         ▼
    │    ┌─────────────────────┐
    │    │ 9. RequireRole      │
    │    │    └─ Insufficient  │
    │    │       → 403         │
    │    └─────────────────────┘
    │         │
    │         ▼
    └──▶ 10. Handler
          ├─ Decode JSON
          ├─ Validate struct
          ├─ Query DB (repo layer)
          ├─ Enqueue task (optional)
          └─ Write JSON response
```

---

## 13. Security Posture

### 🛡️ Defense Layers

| Layer | Protection |
|-------|-----------|
| Request Body Limit | 1MB max via `MaxBytesReader` (413) |
| Rate Limiting | 10 req/s per IP via token bucket (429) |
| JWT Validation | HS256 only, valid signing method enforced |
| CORS | Configurable origins (no wildcard in production) |
| Security Headers | CSP, X-Frame-Options, HSTS, nosniff |
| Error Handling | Internal errors never exposed to clients |
| Log Permissions | Files: 0600, Directories: 0750 |
| Input Validation | Struct validation with user-friendly messages |

### HTTP Status Code Mapping

| Code | When |
|------|------|
| 400 | Malformed JSON, missing required fields |
| 401 | Missing or invalid auth token |
| 403 | Authenticated but insufficient permissions |
| 404 | Resource not found |
| 409 | Duplicate key (PG code 23505) |
| 413 | Request body exceeds 1MB limit |
| 429 | Rate limit exceeded |
| 500 | Unexpected internal errors (generic message) |
| 503 | Dependency unavailable (DB, Redis down) |

---

## 14. Startup & Shutdown

### 🚀 Startup Order

```bash
make dev              # 1. Docker: PostgreSQL + Redis
make migrate-up       # 2. Apply DB migrations
make sqlc             # 3. (if SQL changed) Regenerate Go code
make api              # 4. Start API server on :8080
make worker           # 5. Start background worker
```

### 🛑 Graceful Shutdown

**API Server:**
```
Receive signal → context.WithTimeout(APIShutdownTimeout)
  → server.Shutdown(ctx)
  → if error: log, else: log success
  → log cleanup → exit
```

**Worker:**
```
Receive signal → srv.Stop()
  → drain workerErrors channel
  → Ping Redis (warn if down)
  → goroutine: srv.Shutdown() with panic recovery
  → select: done channel vs timeout
  → if timeout: os.Exit(1)
```


| Feature | Implementation |
|---------|---------------|
| Library | `rs/zerolog` — structured JSON logging |
| Output modes | `stdout` (JSON), `file` (0600 perms), `both` |
| Log levels | `debug`, `info`, `warn`, `error` |
| Request tracing | Logger injected into context with `request_id`, `method`, `path` |
| Global fallback | `FromChiContext()` falls back to global logger if none in context |

### 🔧 Logger Creation

```go
// Standard production logger (JSON to stdout)
func NewProduction(level string) zerolog.Logger

// File logger with optional console mirroring
func NewWithFile(filePath string, console bool, level string) (zerolog.Logger, cleanup func(), error)

// Selector (config-driven)
func NewLogger(cfg *config.Config, defaultFile string) (zerolog.Logger, cleanup func(), error)
```

### 🌐 Context Access

```go
// In any handler:
log := logger.FromChiContext(r.Context())  // → request-scoped logger
log.Info().Str("key", "value").Msg("message")

// If no logger in context → falls back to Global() with debug warning message
```

---

## 11. Validation System

**File:** `internal/validator/validator.go`

```go
// Singleton validator (initialized in init())
var validate *validator.Validate

func ValidateStruct(s any) error              // Wraps validate.Struct()
func GetValidationErrors(err error) []string  // User-friendly messages
```

**Supported Tags & Messages:**

| Tag | Message |
|-----|---------|
| `required` | "{Field} is required" |
| `max` | "{Field} exceeds maximum length" |
| `min` | "{Field} is below minimum length" |
| `email` | "{Field} must be a valid email" |
| `url` | "{Field} must be a valid URL" |
| (other) | "{Field} is invalid" |


> **⚠️ Important:** `internal/repository/db/` is **auto-generated** — never edit directly.

**Workflow:**
1. Edit `sql/schema.sql` (table definitions)
2. Edit `sql/queries/*.sql` (named queries)
3. Run `make sqlc` → regenerates Go code

### 📐 Repository Pattern (`internal/repository/repo`)

```
BaseRepo                    UserRepo
├── queries *db.Queries     ├── Create(ctx, params)
├── pool *pgxpool.Pool      ├── GetByID(ctx, uuid)
├── log zerolog.Logger      └── List(ctx, limit)
├── logQuery() ← auto-timing
└── WithTx(tx) → bound repo
```

**Features:**
- Automatic query duration logging (debug on success, error on failure)
- Transaction support via `WithTx(tx)` — returns new repo bound to tx
- `List()` caps limit at **1000**
- `GetByID()` returns `ErrUserNotFound` sentinel error on `pgx.ErrNoRows`

### 📊 Database Schema

```sql
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Auto-update updated_at on modification
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

### 🏗️ Migration History

| Migration | Description |
|-----------|-------------|
| `000001_init` | Create pgcrypto extension, users + jobs tables |
| `000003_drop_jobs_table` | Remove unused jobs table (moved to Asynq/Redis) |
| `000004_add_updated_at_trigger` | Add auto-update trigger for users.updated_at |

8. JWTAuth              → (on protected routes) Validate Bearer token
9. RequireRole          → (on protected routes) Check role permissions
```

### 🛡️ Middleware Details

| Middleware | File | What It Does |
|-----------|------|-------------|
| `MaxBodySize` | `body_limit.go` | Wraps `http.MaxBytesReader` — returns 413 automatically on exceed |
| `JWTAuth` | `jwt.go` | Extracts Bearer token, parses via `jwtpkg.ParseToken`, stores `user_id` + `roles` in context |
| `RequestLogger` | `logger.go` | Injects logger with `request_id`, `method`, `path` into context; wraps ResponseWriter to capture status |
| `RequireRole` | `rbac.go` | Single role check → 403 if missing |
| `RequireAnyRole` | `rbac.go` | Multi-role check → 403 if none match |
| `SecurityHeaders` | `security.go` | Sets security headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, CSP, optional HSTS |

### ⚠️ NOT Present (Important)

- **No global request timeout middleware**
- Use `http.Server.WriteTimeout` (10s) or handler-level `context.WithTimeout` instead

Dependencies created:
  - HTTP server with chi router
  - Scheduler client (Asynq) for enqueueing tasks

Shutdown:
  server.Shutdown(shutdownCtx) with APIShutdownTimeout
```

### ⚡ Worker (`cmd/worker/main.go`)

```
Dependencies created:
  - Asynq server with queue priorities
  - Task handlers registered on ServeMux
  - Middleware: loggingMiddleware

Shutdown (detailed):
  1. srv.Stop() → stop accepting new tasks
  2. Drain workerErrors channel
  3. Ping Redis (warn if unreachable)
  4. srv.Shutdown() in goroutine with panic recovery
  5. If timeout → os.Exit(1)

Task handler registered:
  tasks.TypeWorkerPing ("worker:ping")
```

│   ├── queue/queue.go            #    📬 Queue names & priorities
│   │
│   ├── repository/               #    🗄️ Data access layer
│   │   ├── pool/pool.go          #       Thread-safe DB pool (singleton)
│   │   ├── repo/                 #       Repository implementations
│   │   │   ├── base.repo.go      #          BaseRepo (query logging, WithTx)
│   │   │   └── user.repo.go      #          UserRepo (CRUD operations)
│   │   └── db/                   #       ⚠️  sqlc-generated — DO NOT EDIT
│   │       ├── db.go, models.go, users.sql.go
│   │
│   ├── scheduler/client.go       #    📤 Asynq task enqueueing
│   ├── tasks/tasks.go            #    📋 Shared task type constants
│   └── validator/validator.go    #    ✅ Struct validation wrapper
│
├── pkg/                          # 📌 REUSABLE LIBRARIES
│   └── logger/
│       ├── logger.go             #    Zerolog creation (file/console)
│       └── chicontext.go         #    Context-based logger access
│
├── sql/                          # 📌 SQLC SOURCE FILES
│   ├── schema.sql                #    Database schema
│   └── queries/users.sql         #    User queries
│
├── migrations/                   # 📌 DATABASE MIGRATIONS
│   ├── 000001_init.sql
│   ├── 000003_drop_jobs_table.sql
│   └── 000004_add_updated_at_trigger.sql
│
├── .githooks/pre-commit          # 🪝 Git pre-commit hook
├── docker-compose.yml            # 🐳 PostgreSQL + Redis
├── sqlc.yaml                     # ⚡ sqlc generation config
├── Makefile                      # 🔧 All dev commands
├── .env.example                  # 📝 Template environment
├── README.md                     # Full documentation
├── AGENTS.md                     # AI assistant context
├── ERROR_HANDLING.md             # Error handling strategy
└── ARCHITECTURE.md               # 🆕 This file
```
