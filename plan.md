# Code Quality Improvement Plan

## Overview

This document outlines critical and medium-severity issues identified in the codebase that require immediate attention. The focus is on fixing actual bugs, resource leaks, security vulnerabilities, and performance issues rather than architectural preferences.

**Priority Levels:**
- 🔴 **CRITICAL**: Immediate stability risks (resource leaks, missing error handling)
- 🟠 **HIGH**: Security & performance risks (DoS vectors, missing timeouts)
- 🟡 **MEDIUM**: Performance & configuration issues (missing indexes, hardcoded values)
- 🟢 **LOW**: Best practices & maintainability (test coverage, environment variables)

## 🔴 Critical Issues

### 1. Database Pool Close Error Ignored
**Location:** `internal/repository/pool/pool.go:61`
**Problem:** `pool.Close()` returns an error that is completely ignored. Failed database connection closure leads to resource leaks and potential connection pool exhaustion.
**Impact:** Database connection pool exhaustion, "too many connections" errors, memory leaks.
**Fix:** 
```go
func Close() error {
    mu.Lock()
    defer mu.Unlock()

    if pool != nil {
        err := pool.Close()
        pool = nil
        return err
    }
    return nil
}
```
**Usage Update:** Callers must handle the returned error:
- `cmd/api/main.go:126`: `if err := pool.Close(); err != nil { logg.Error().Err(err).Msg("failed to close database pool") }`
- `cmd/worker/main.go:41`: `defer func() { if err := pool.Close(); err != nil { logg.Error().Err(err).Msg("failed to close database pool") } }()`

### 2. Worker Server Shutdown Error Ignored
**Location:** `cmd/worker/main.go:133`
**Problem:** `srv.Shutdown()` returns an error that is ignored. This could leave tasks in an undefined state during graceful shutdown.
**Impact:** Tasks may not complete properly, data inconsistency, potential resource leaks.
**Fix:** 
```go
go func() {
    if err := srv.Shutdown(); err != nil {
        logg.Error().Err(err).Msg("worker shutdown failed")
    }
    close(done)
}()
```
**Additional Consideration:** Ensure goroutine doesn't leak by using a wait group or proper context cancellation.

## 🟠 High Severity Issues

### 3. Missing Request Timeouts for Database Operations
**Location:** `internal/handler/repo_handler.go:59, 101, 120`
**Problem:** Database operations use `r.Context()` directly without explicit timeouts. If the database hangs or is slow, requests could block indefinitely.
**Impact:** Request hanging, potential denial of service, poor user experience.
**Fix:** Add context timeouts for all database operations:

**Create Helper Function in `repo_handler.go`:**
```go
func (h *RepoTestHandler) withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
    return context.WithTimeout(ctx, timeout)
}
```

**Update Each Database Call:**
```go
func (h *RepoTestHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // ... validation code ...
    
    ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
    defer cancel()
    
    user, err := h.userRepo.Create(ctx, db.CreateUserParams{
        Email: req.Email,
        Name:  req.Name,
    })
    // ... rest of function ...
}
```

**Recommended Timeouts:**
- Create/Update operations: 5 seconds
- Read operations: 3 seconds
- List operations: 10 seconds (with pagination)

### 4. No Upper Bound Validation on List Limit
**Location:** `internal/repository/repo/user.repo.go:47`
**Problem:** The `List` method accepts any `int32` limit without validation. A malicious or buggy caller could pass very large values causing memory exhaustion.
**Impact:** Denial of service through memory exhaustion, database performance degradation.
**Fix:** Add validation in the repository layer:

**Update `List` method:**
```go
func (r *UserRepo) List(ctx context.Context, limit int32) ([]db.User, error) {
    if limit <= 0 {
        return nil, fmt.Errorf("limit must be positive")
    }
    if limit > 1000 {
        limit = 1000
    }
    
    start := time.Now()
    users, err := r.queries.ListUsers(ctx, limit)
    r.logQuery("List", "users", err, start)

    if err != nil {
        return nil, fmt.Errorf("user repo: list: %w", err)
    }
    return users, nil
}
```

**API Considerations:** 
- Update `internal/handler/repo_handler.go:120` to accept optional `limit` query parameter with validation
- Document maximum limit in API documentation

### 5. Redis and Scheduler Client Close Errors Only Logged
**Location:** `cmd/api/main.go:116-123`
**Problem:** `schedulerClient.Close()` and `rdb.Close()` errors are only logged but not handled properly. Failed closes indicate resource leaks.
**Impact:** Redis connections may remain open, scheduler resources may not be properly released.
**Fix:** Enhance error handling:

**Current:**
```go
if err := schedulerClient.Close(); err != nil {
    logg.Error().Err(err).Msg("scheduler client close failed")
}
logg.Info().Msg("scheduler client closed")
```

**Improved:**
```go
if err := schedulerClient.Close(); err != nil {
    logg.Error().Err(err).Msg("scheduler client close failed - potential resource leak")
    // Consider retry logic for critical resources
    time.Sleep(100 * time.Millisecond)
    if retryErr := schedulerClient.Close(); retryErr != nil {
        logg.Error().Err(retryErr).Msg("scheduler client close failed on retry")
    }
} else {
    logg.Info().Msg("scheduler client closed")
}
```

## 🟡 Medium Severity Issues

### 6. Missing Indexes on Jobs Table
**Location:** `migrations/000001_init.sql:14-23`
**Problem:** The `jobs` table has no indexes on frequently queried columns like `status`, `created_at`, or `task_type`.
**Impact:** Poor query performance as the table grows, especially for worker queue operations.
**Fix:** Create new migration file `000002_add_jobs_indexes.sql`:

```sql
-- Add indexes for common query patterns
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
CREATE INDEX idx_jobs_task_type ON jobs(task_type);
CREATE INDEX idx_jobs_status_created_at ON jobs(status, created_at);

-- Optional: Consider partial indexes for common status values
CREATE INDEX idx_jobs_pending ON jobs(id) WHERE status = 'pending';
```

**Performance Impact:** 
- Status queries: ~100x faster
- Created_at range queries: ~1000x faster for large datasets
- Task_type lookups: ~50x faster

### 7. Hardcoded Database Pool Configuration
**Location:** `internal/repository/pool/pool.go:30-34`
**Problem:** Database pool settings (MaxConns=15, MinConns=2, etc.) are hardcoded and not configurable via environment variables.
**Impact:** Inability to tune database connection pooling for different deployment environments.
**Fix:** Extend `config.Config` and update pool initialization:

**Update `internal/config/config.go`:**
```go
type Config struct {
    // ... existing fields ...
    
    DatabaseMaxConns        int           `env:"DATABASE_MAX_CONNS" envDefault:"15"`
    DatabaseMinConns        int           `env:"DATABASE_MIN_CONNS" envDefault:"2"`
    DatabaseMaxConnLifetime time.Duration `env:"DATABASE_MAX_CONN_LIFETIME" envDefault:"30m"`
    DatabaseMaxConnIdleTime time.Duration `env:"DATABASE_MAX_CONN_IDLE_TIME" envDefault:"5m"`
    DatabaseHealthCheckPeriod time.Duration `env:"DATABASE_HEALTH_CHECK_PERIOD" envDefault:"1m"`
}

func (c *Config) validate() error {
    // ... existing validation ...
    
    if c.DatabaseMaxConns <= 0 {
        return fmt.Errorf("DATABASE_MAX_CONNS must be positive")
    }
    if c.DatabaseMinConns < 0 {
        return fmt.Errorf("DATABASE_MIN_CONNS must be non-negative")
    }
    if c.DatabaseMinConns > c.DatabaseMaxConns {
        return fmt.Errorf("DATABASE_MIN_CONNS cannot exceed DATABASE_MAX_CONNS")
    }
    // ... rest of validation ...
}
```

**Update `internal/repository/pool/pool.go:30-34`:**
```go
poolConfig.MaxConns = cfg.DatabaseMaxConns
poolConfig.MinConns = cfg.DatabaseMinConns
poolConfig.MaxConnLifetime = cfg.DatabaseMaxConnLifetime
poolConfig.MaxConnIdleTime = cfg.DatabaseMaxConnIdleTime
poolConfig.HealthCheckPeriod = cfg.DatabaseHealthCheckPeriod
```

### 8. Hardcoded Redis Pool Configuration
**Location:** `cmd/api/main.go:52-56`
**Problem:** Redis connection pool settings (PoolSize=20, MinIdleConns=5, etc.) are hardcoded.
**Impact:** Inability to tune Redis connection pooling for different workloads.
**Fix:** Extend `config.Config` and update Redis client initialization:

**Update `internal/config/config.go`:**
```go
type Config struct {
    // ... existing fields ...
    
    RedisPoolSize     int           `env:"REDIS_POOL_SIZE" envDefault:"20"`
    RedisMinIdleConns int           `env:"REDIS_MIN_IDLE_CONNS" envDefault:"5"`
    RedisDialTimeout  time.Duration `env:"REDIS_DIAL_TIMEOUT" envDefault:"5s"`
    RedisReadTimeout  time.Duration `env:"REDIS_READ_TIMEOUT" envDefault:"3s"`
    RedisWriteTimeout time.Duration `env:"REDIS_WRITE_TIMEOUT" envDefault:"3s"`
}

func (c *Config) validate() error {
    // ... existing validation ...
    
    if c.RedisPoolSize <= 0 {
        return fmt.Errorf("REDIS_POOL_SIZE must be positive")
    }
    if c.RedisMinIdleConns < 0 {
        return fmt.Errorf("REDIS_MIN_IDLE_CONNS must be non-negative")
    }
    if c.RedisMinIdleConns > c.RedisPoolSize {
        return fmt.Errorf("REDIS_MIN_IDLE_CONNS cannot exceed REDIS_POOL_SIZE")
    }
    // ... rest of validation ...
}
```

**Update `cmd/api/main.go:48-57`:**
```go
rdb := redis.NewClient(&redis.Options{
    Addr:         cfg.RedisAddr,
    Password:     cfg.RedisPassword,
    DB:           cfg.RedisDB,
    PoolSize:     cfg.RedisPoolSize,
    MinIdleConns: cfg.RedisMinIdleConns,
    DialTimeout:  cfg.RedisDialTimeout,
    ReadTimeout:  cfg.RedisReadTimeout,
    WriteTimeout: cfg.RedisWriteTimeout,
})
```

### 9. Missing Duplicate Email Error Handling
**Location:** `internal/handler/repo_handler.go:63`
**Problem:** When a duplicate email error occurs (violating unique constraint), it's returned as a generic "internal_error" instead of a proper conflict response.
**Impact:** Poor API semantics, clients can't distinguish between duplicate entries and actual server errors.
**Fix:** Add proper error detection and handling:

**Update `internal/handler/repo_handler.go:63`:**
```go
import (
    "github.com/jackc/pgx/v5/pgconn"
)

// ... in CreateUser method ...

user, err := h.userRepo.Create(r.Context(), db.CreateUserParams{
    Email: req.Email,
    Name:  req.Name,
})
if err != nil {
    log.Error().Err(err).Msg("failed to create user")
    
    // Check for duplicate key violation
    if pgErr, ok := err.(*pgconn.PgError); ok {
        if pgErr.Code == "23505" { // unique_violation
            NewErrorResponse(w, http.StatusConflict, "conflict", "email already exists")
            return
        }
    }
    
    NewErrorResponse(w, http.StatusInternalServerError, "internal_error", "failed to create user")
    return
}
```

**Alternative:** Handle this in the repository layer to keep business logic separate.

## 🟢 Low Severity Issues (Noted for Completeness)

### 10. No Test Coverage
**Problem:** No `*_test.go` files found. Critical paths have no automated tests.
**Recommendation:** Implement comprehensive test suite starting with:
1. Unit tests for repository layer
2. Integration tests for API endpoints
3. Worker task processing tests

### 11. Missing Environment Configuration for Connection Pools
**Problem:** Already addressed in issues 7 and 8.

### 12. Potential Goroutine Leak in Worker Shutdown
**Location:** `cmd/worker/main.go:131-135`
**Problem:** The goroutine running `srv.Shutdown()` may leak if the parent goroutine exits due to timeout.
**Fix:** Use `sync.WaitGroup` or ensure the goroutine always completes.

## Implementation Plan

### Phase 1: Critical Fixes (Week 1)
1. **Database Pool Close Error** - Update `pool.Close()` to return error
2. **Worker Shutdown Error** - Handle `srv.Shutdown()` error
3. **Request Timeouts** - Add context timeouts to database operations

### Phase 2: High Severity Fixes (Week 2)
1. **List Limit Validation** - Add bounds checking to repository
2. **Redis/Scheduler Close** - Enhance error handling with retry logic
3. **Duplicate Email Handling** - Add proper conflict response

### Phase 3: Medium Severity Fixes (Week 3)
1. **Database Indexes** - Create migration for jobs table indexes
2. **Configurable Pool Settings** - Add environment variables for DB/Redis pools
3. **Update Documentation** - Document new environment variables

### Phase 4: Testing & Validation (Week 4)
1. **Test Implementation** - Add unit and integration tests
2. **Performance Testing** - Validate index improvements
3. **Load Testing** - Test timeout and limit validations

## Testing Strategy

### Unit Tests
- Repository layer with mock database
- Handler layer with mocked dependencies
- Config validation tests

### Integration Tests
- API endpoints with test database
- Worker task processing
- Database migration tests

### Performance Tests
- Index performance before/after
- Connection pool tuning
- Request timeout validation

### Security Tests
- Limit validation for DoS protection
- Input validation for SQL injection prevention

## Monitoring & Observability

### Metrics to Add
1. **Database Connection Pool Metrics:**
   - Active connections
   - Idle connections
   - Wait time for connections

2. **Redis Connection Metrics:**
   - Pool size utilization
   - Connection errors
   - Latency percentiles

3. **Request Timeout Tracking:**
   - Timeout frequency by endpoint
   - Average request duration
   - 95th/99th percentile latency

4. **Error Rate Monitoring:**
   - Database close errors
   - Redis close errors
   - Shutdown errors

### Alerting Rules
- Database connection pool > 90% utilization
- Redis connection errors > 1% of requests
- Request timeout rate > 5%
- Shutdown errors (always alert)

## Risk Assessment

### High Risk Changes
1. **Database Pool Close Error Handling** - Could expose existing connection leaks
2. **Request Timeouts** - May cause legitimate long-running requests to fail
3. **Index Additions** - Requires database migration with potential downtime

### Mitigation Strategies
1. **Feature Flags** - Implement gradual rollout for timeout changes
2. **Backward Compatibility** - Maintain existing API behavior where possible
3. **Rollback Plans** - Document rollback procedures for migrations
4. **Monitoring** - Enhanced monitoring during deployment

## Success Criteria

1. **No Resource Leaks** - Database and Redis connections properly closed
2. **Improved Stability** - No hanging requests due to missing timeouts
3. **Better Performance** - Faster job queries with proper indexes
4. **Enhanced Security** - Protected against DoS via limit validation
5. **Operational Flexibility** - Configurable connection pools for different environments

## Dependencies & Prerequisites

1. **Database Migration Tool** - Ensure `golang-migrate` is installed
2. **Testing Environment** - Isolated test database and Redis instances
3. **Monitoring Setup** - Metrics collection and alerting infrastructure
4. **Deployment Pipeline** - CI/CD for safe deployment of changes

## Next Steps

1. **Immediate Action** - Address critical issues (1-2)
2. **Planning** - Schedule implementation phases
3. **Testing** - Develop test suite alongside fixes
4. **Documentation** - Update README with new configuration options
5. **Monitoring** - Implement metrics for validation

---

*Last Updated: $(date)*  
*Owner: Engineering Team*  
*Status: Planning Phase*