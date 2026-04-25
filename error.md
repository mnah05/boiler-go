# Extensive Codebase Review — Errors & Issues

> Generated: 2026-04-25

---

## 1. Bugs & Logic Errors

### 1.1 `GetUser` returns 404 for ALL database errors
- **File:** `internal/handler/repo_handler.go:119-123`
- **Severity:** High
- **Code:**
```go
user, err := h.userRepo.GetByID(ctx, pgUUID)
if err != nil {
    log.Error().Err(err).Msg("failed to get user")
    NewErrorResponse(w, http.StatusNotFound, "not_found", "user not found")
    return
}
```
- **Problem:** This treats a database connection failure, timeout, or any other error as "not found". It should distinguish `pgx.ErrNoRows` from real errors.
- **Fix:**
```go
if errors.Is(err, pgx.ErrNoRows) {
    NewErrorResponse(w, http.StatusNotFound, "not_found", "user not found")
    return
}
NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to get user")
```

---

### 1.2 `Ping` handler silently ignores chunked request bodies
- **File:** `internal/handler/worker.go:50`
- **Severity:** High
- **Code:**
```go
if r.ContentLength > 0 {
```
- **Problem:** `r.ContentLength` is `-1` for chunked transfer encoding or when the length is unknown. Clients sending JSON with `Transfer-Encoding: chunked` will have their payload silently discarded, and the task will be enqueued with the default message `"ping from API"`.
- **Fix:** Always attempt to decode if the method is POST and body is present:
```go
if r.Body != http.NoBody {
    // decode
}
```

---

### 1.3 `updated_at` column never updates
- **File:** `migrations/000001_init.sql:11` and `sql/schema.sql:9`
- **Severity:** High
- **Code:**
```sql
updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
```
- **Problem:** PostgreSQL has no native `ON UPDATE` trigger. `updated_at` will always remain the insertion timestamp.
- **Fix:** Add an update trigger:
```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

---

### 1.4 `PingResponse.QueuedAt` uses a different timestamp than the payload
- **File:** `internal/handler/worker.go:71-107`
- **Severity:** Low
- **Problem:** The response timestamp is captured milliseconds after the payload timestamp. If a consumer correlates these, they'll differ.
- **Fix:** Reuse the same `time.Now().UTC()` value for both payload and response.

---

### 1.5 `uuid.UUID(user.ID.Bytes[:])` is unnecessarily convoluted
- **File:** `internal/handler/repo_handler.go:84,87,127,161`
- **Severity:** Low
- **Code:**
```go
uuid.UUID(user.ID.Bytes[:]).String()
```
- **Problem:** `pgtype.UUID.Bytes` is already `[16]byte`. The `[:]` slice conversion is redundant and confusing.
- **Fix:** Use `uuid.UUID(user.ID.Bytes).String()`.

---

## 2. Security Issues

### 2.1 `.env` file committed to repository
- **File:** `.env`
- **Severity:** Critical
- **Problem:** `.env` is present in the repo. Even though it only contains local dev values, this trains developers to commit environment files. If someone later adds a real secret, it will be leaked to git history.
- **Fix:** `git rm --cached .env`, add `.env` to `.gitignore`, and rely on `.env.example`.

---

### 2.2 Shell injection risk in Makefile migrations
- **File:** `Makefile:27-28`
- **Severity:** Critical
- **Code:**
```makefile
@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
migrate -path ./migrations -database $$DATABASE_URL up
```
- **Problem:** `xargs` parsing of `.env` values is fragile. Values with spaces, quotes, or shell metacharacters will break the command or execute arbitrary code.
- **Fix:** Use `set -a; source .env; set +a` or a proper env loader.

---

### 2.3 `RepoTestHandler` endpoints are exposed without authentication
- **File:** `internal/handler/router.go:55-60`
- **Severity:** Medium
- **Problem:** `/repo-test/users` allows anyone to create and list users. In a boilerplate this is expected, but there is **no comment, middleware, or build tag** preventing this from being deployed to production accidentally.

---

### 2.4 Rate limiting trusts client headers blindly
- **File:** `internal/handler/router.go:43`
- **Severity:** Medium
- **Code:**
```go
httprate.WithKeyByIP()
```
- **Problem:** `KeyByIP` reads `X-Forwarded-For` and `X-Real-Ip`. If the API is deployed behind a reverse proxy, an attacker can rotate these headers to bypass rate limits entirely. If it's NOT behind a proxy, the client can still spoof them.
- **Fix:** If behind a proxy, strip/spoof those headers at the edge. If not, use `httprate.KeyByEndpoint` or a custom function that ignores those headers.

---

### 2.5 API binds to all network interfaces
- **File:** `cmd/api/main.go:76`
- **Severity:** Medium
- **Code:**
```go
Addr: ":" + cfg.AppPort,
```
- **Problem:** This binds to `0.0.0.0` (all interfaces). In development or containerized environments, this can expose the API to the local network unintentionally. There is no env var to bind to `127.0.0.1` only.
---

## 3. Reliability & Concurrency Issues

### 3.1 Worker has no panic recovery
- **File:** `cmd/worker/main.go:57-82`
- **Severity:** Medium
- **Problem:** The asynq server is created without `asynq.RecoverPanic`. If any task handler panics, the **entire worker process crashes**.
- **Fix:** Add `asynq.RecoverPanic` to the config:
```go
asynq.Config{
    // ...
    RecoverPanic: true,
}
```

---

### 3.2 Worker shutdown deadlock if `Run()` returns unexpectedly
- **File:** `cmd/worker/main.go:109-114`
- **Severity:** Medium
- **Code:**
```go
go func() {
    if err := srv.Run(mux); err != nil {
        workerErrors <- fmt.Errorf("worker failed to start: %w", err)
    }
}()
```
- **Problem:** If `srv.Run()` returns `nil` (possible during internal shutdown or after `Stop()` from another path), nothing is sent to `workerErrors`. The main goroutine remains blocked on the first `select` forever if no OS signal is received. This is a goroutine leak / deadlock.
- **Fix:**
```go
go func() {
    err := srv.Run(mux)
    if err != nil {
        workerErrors <- fmt.Errorf("worker failed: %w", err)
    } else {
        workerErrors <- nil // sentinel for clean exit
    }
}()
```

---

### 3.3 `Fatal()` logs bypass all defers on startup failure
- **File:** `cmd/api/main.go:100` and `cmd/worker/main.go:121`
- **Severity:** Medium
- **Code:**
```go
logg.Fatal().Err(err).Msg("server startup failed")
```
- **Problem:** `zerolog.Fatal()` calls `os.Exit(1)` immediately. All deferred cleanups (`pool.Close`, `rdb.Close`, `logCleanup`, context cancels) are skipped. While the OS frees resources, log buffers may not be flushed and file handles may not be closed cleanly.

---

### 3.4 `responseWriter` middleware breaks `http.Flusher` / `http.Hijacker`
- **File:** `internal/middleware/logger.go:46-54`
- **Severity:** Medium
- **Problem:** The custom `responseWriter` only embeds `http.ResponseWriter`. If any downstream handler or middleware tries to flush (SSE) or hijack (WebSockets), the type assertion fails because the wrapper doesn't implement those interfaces.
- **Fix:** Implement the optional interfaces:
```go
func (rw *responseWriter) Flush() {
    if f, ok := rw.ResponseWriter.(http.Flusher); ok {
        f.Flush()
    }
}
// similar for Hijacker
```

---

### 3.5 Health checks run sequentially instead of concurrently
- **File:** `internal/handler/health.go:42-52` and `internal/handler/worker.go:168-178`
- **Severity:** Low
- **Problem:** DB ping and Redis ping run one after another. If DB is down, you wait `timeout`, then Redis ping fails immediately. Worst-case health check latency is `2 * timeout`.

---

### 3.6 `Global()` fallback logger logs at Trace level
- **File:** `pkg/logger/logger.go:113-118`
- **Severity:** Low
- **Code:**
```go
func Global() zerolog.Logger {
    globalOnce.Do(func() {
        global = New() // No Level() set!
    })
    return global
}
```
- **Problem:** `New()` creates a logger with no level restriction, defaulting to `TraceLevel`. If `FromChiContext` ever falls back to `Global()`, every single trace log will be emitted. This can be extremely noisy and performance-heavy.

---

## 4. Code Quality & Maintainability

### 4.1 JSON encode errors are silently swallowed
- **File:** `internal/handler/response.go:20-53`
- **Severity:** Low
- **Problem:** All response helpers ignore `json.NewEncoder(w).Encode(data)` errors. If the client disconnects mid-response, you never know.

---

### 4.2 Duplicate health check code
- **File:** `internal/handler/health.go` and `internal/handler/worker.go:154-193`
- **Severity:** Low
- **Problem:** `HealthHandler.Check` and `WorkerHandler.Health` are almost identical. `WorkerHandler.Status` also pings DB/Redis. This violates DRY.

---

### 4.3 `TxManager` is dead code
- **File:** `internal/repository/repo/tx.repo.go`
- **Severity:** Low
- **Problem:** The transaction manager is defined but **never used anywhere** in the application. Either add a usage example or remove it.

---

### 4.4 `jobs` table and indexes are completely unused
- **File:** `migrations/000002_add_jobs_indexes.sql` and `internal/repository/db/models.go`
- **Severity:** Low
- **Problem:** The `jobs` table is defined and migrated, but there are zero queries, handlers, or workers that use it. This is dead schema.

---

### 4.5 `bootLog` and `logg` use different output formats
- **File:** `cmd/api/main.go:23` and `cmd/worker/main.go:23`
- **Severity:** Low
- **Problem:** `bootLog` is created with `logger.New()` (console format), while `logg` uses `NewLogger(cfg, ...)` (possibly JSON or file). If config loading fails, you get console output. If the server fails after that, you might get JSON output. This inconsistency makes log aggregation harder.

---

### 4.6 `logQuery` uses `Send()` with empty message
- **File:** `internal/repository/repo/base.repo.go:30-48`
- **Severity:** Low
- **Code:**
```go
r.log.Debug().Str("query", queryName).Str("table", table).Dur("duration", elapsed).Send()
```
- **Problem:** `Send()` is equivalent to `Msg("")`. While valid, it produces log entries with empty `message` fields, which is inconsistent with the rest of the codebase that always uses `Msg("something")`.

---

### 4.7 Redundant nil checks after `pool.Open`
- **File:** `cmd/api/main.go:44-46` and `cmd/worker/main.go:46-48`
- **Severity:** Low
- **Code:**
```go
dbPool := pool.Get()
if dbPool == nil {
    logg.Fatal().Msg("database pool is nil")
}
```
- **Problem:** If `pool.Open()` succeeded, `pool.Get()` will never be nil. These checks are defensive but unnecessary noise.

---

### 4.8 `DatabaseURL` validated twice
- **File:** `internal/config/config.go:16` and `62-68`
- **Severity:** Low
- **Problem:** The `env` tag has `required`, which already fails parsing if empty. The manual `if c.DatabaseURL == ""` check is redundant.

---

## 5. Missing Features / Gaps

### 5.1 No `offset` parameter in `ListUsers`
- **File:** `internal/handler/repo_handler.go:135-172`
- **Severity:** Low
- **Problem:** There is no pagination offset. You can only get the first N users.

---

### 5.2 No `DisallowUnknownFields` on JSON decoders
- **File:** `internal/handler/repo_handler.go:52`, `internal/handler/worker.go:52`
- **Severity:** Low
- **Problem:** Extra/unknown fields in request bodies are silently ignored. If a client typos a field name, they won't get an error.

---

### 5.3 No strict request validation on `PingRequest`
- **File:** `internal/handler/worker.go:35-36`
- **Severity:** Low
- **Code:**
```go
type PingRequest struct {
    Message string `json:"message,omitempty" validate:"omitempty,max=500"`
}
```
- **Problem:** `omitempty` on the JSON tag combined with `omitempty` on validation means an empty body is accepted. That's fine, but there's no `min` validation or content sanitization.

---

### 5.4 `ListUsers` limit defaults to 10 with no upper bound from caller
- **File:** `internal/handler/repo_handler.go:138-146`
- **Severity:** Low
- **Problem:** The handler parses `limit` but only the repo enforces a hard max of 1000. The handler should probably enforce a smaller default max (e.g., 100) to prevent accidental large queries.

---

### 5.5 `scheduler.Client` doesn't expose queue inspection
- **File:** `internal/scheduler/client.go`
- **Severity:** Low
- **Problem:** There is no way to check queue depths, pending task counts, or worker status. The `Status` endpoint only pings DB/Redis.

---

## 6. Minor Issues

| # | Issue | File | Detail |
|---|-------|------|--------|
| 6.1 | `DBTX` uses `interface{}` instead of `any` | `internal/repository/db/db.go:14-18` | Pre-Go 1.18 style |
| 6.2 | `errors` variable shadows stdlib package | `internal/validator/validator.go:21` | Harmless but confusing |
| 6.3 | `make stop-api` is fragile | `Makefile:10` | `pkill -f "go run ./cmd/api"` can match unintended processes |
| 6.4 | `CORSAllowedOrigins` defaults to `localhost:3000` | `internal/config/config.go:44` | Risky if deployed without changing |
| 6.5 | `createUser` query missing semicolon | `sql/queries/users.sql:9` | Works fine, but inconsistent |
| 6.6 | Hardcoded task timeout & retry | `internal/handler/worker.go:88-89` | `30s` timeout and `3` retries should be configurable |
| 6.7 | `validator` doesn't include parameter values | `internal/validator/validator.go:30-48` | `"name exceeds maximum length"` doesn't say what the max is |

---

## Summary by Severity

| Severity | Count | Examples |
|----------|-------|----------|
| **Critical** | 3 | Worker panic crash, `.env` in repo, shell injection in Makefile |
| **High** | 3 | 404 on all DB errors, chunked body ignored, `updated_at` never updates |
| **Medium** | 6 | No panic recovery, worker deadlock, missing `http.Flusher`, health checks sequential |
| **Low** | 10+ | Dead code, DRY violations, missing `DisallowUnknownFields`, ignored encode errors |


