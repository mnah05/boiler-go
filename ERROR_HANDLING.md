# Error Handling Strategy

## Client-Facing Errors

**Rule:** Never expose internal error details (stack traces, SQL errors, dependency names) to API clients.

### Response Format

All error responses use a consistent JSON envelope:

```json
{
  "error": "error_code",
  "message": "Human-readable description",
  "details": ["optional", "field-level", "errors"]
}
```

### HTTP Status Codes

| Code | When to Use |
|------|-------------|
| 400  | Malformed JSON, missing required fields, invalid formats |
| 401  | Missing or invalid authentication |
| 403  | Authenticated but not authorized |
| 404  | Resource not found |
| 413  | Request body exceeds size limit (1MB default) |
| 422  | Struct validation failures (returned with `details` array) |
| 429  | Rate limit exceeded |
| 500  | Unexpected internal errors — log the real error, return generic message |
| 503  | Dependency unavailable (Redis, DB down, queue full) |

### Error Logging

- Always log the **real error** server-side with `log.Error().Err(err).Msg(...)`.
- Include `request_id` via the context logger for correlation.
- Never log sensitive data (passwords, tokens, PII) even at debug level.

## Retry Policies

### API Server

- **Database errors**: Do not retry at the handler level. Return 503 and let the client retry.
- **Redis errors**: Do not retry. Return 503 for queue operations.
- **Validation errors**: Never retry. Return 400/422 immediately.

### Worker (Asynq)

| Error Type | Max Retries | Backoff |
|------------|-------------|---------|
| Transient (network, timeout) | 3 | Exponential: 2s, 4s, 8s (capped at 64s) |
| Permanent (bad payload, validation) | 0 | Return error immediately, do not retry |
| Dependency down (DB, external API) | 5 | Exponential with cap |

### Exponential Backoff

The worker uses `1 << n` seconds with `n` capped at 6 to prevent overflow:

```
Retry 1: 2s
Retry 2: 4s
Retry 3: 8s
Retry 4: 16s
Retry 5: 32s
Retry 6+: 64s (capped)
```

## Circuit Breaker Pattern (Future)

For external dependency calls (third-party APIs, external databases), consider implementing a circuit breaker:

1. **Closed** (normal): Requests flow through. Track failure count.
2. **Open** (tripped): After N consecutive failures, reject requests immediately for a cooldown period. Return 503.
3. **Half-Open** (probing): After cooldown, allow one request through. If it succeeds, close the circuit. If it fails, reopen.

Recommended library: `sony/gobreaker` or `mercari/go-circuitbreaker`.

## Graceful Degradation

- Health endpoints (`/health`, `/worker/health`) are excluded from rate limiting so monitoring always works.
- If Redis is down, the API can still serve database-only endpoints (health reports partial status).
- Shutdown sequence: stop accepting new requests → drain in-flight requests → close dependencies.
