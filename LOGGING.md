# Logging and debugging requests

This guide explains how the API logs a request, and how to use those logs to
answer the only question that matters during an incident: **what happened to
this request?**

## The mental model

Every request gets a `request_id` and exactly one logger carrying it. That
logger is created in internal/middleware/logger.go and placed on the request
context. Anything that logs with `logger.FromContext(ctx)` contributes to the
same request's story.

```
client -> RequestID -> Recover -> RequestLogger -> route -> your handler
                                      |                        |
                                      | puts request logger    | logs with
                                      +-- on the context ------+ FromContext
```

Two rules follow from this design:

- **Never build your own logger inside a handler.** Use `logger.FromContext`
  so your line carries the `request_id`.
- **Do not log-and-return just to record an error.** The central error handler
  does that for you (see below).

## What a request produces

A successful request produces one line:

```json
{"level":"info","request_id":"SKib...","method":"POST","path":"/repo-test/users","status":201,"duration":0.249,"remote_ip":"192.0.2.1","message":"request completed"}
```

A failed request produces two: the completion line, then the cause.

```json
{"level":"error","request_id":"ydUv...","method":"GET","path":"/repo-test/users/get","status":500,"duration":0.004,"remote_ip":"192.0.2.1","message":"request completed"}
{"level":"error","request_id":"ydUv...","method":"GET","path":"/repo-test/users/get","status":500,"error":"failed to get user","message":"request failed"}
```

Both lines share a `request_id`, which is how you connect them.

### Why two lines, and why the status comes from the error

Echo renders a returned error *after* the middleware chain has unwound. When
`RequestLogger` regains control the response has not been written yet, so
`c.Response().Status` is still its default of `200`. Reading it would log
every failure as a success.

So `RequestLogger` derives the status from the returned error instead
(`statusFromError`), and `HTTPErrorHandler` - which runs later, when the
response is genuinely written - logs the cause. That split is deliberate:

- the `request completed` line is the request's ledger entry (status,
  duration, caller);
- the `request failed` line is the diagnostic (the error and its chain).

If you add a new error type that maps to a specific status, give it a
`StatusCode() int` method (see `handler.APIError`). The middleware finds it
through an interface, which is how it reads the status without importing
`handler` and creating an import cycle.

## Debugging a reported failure

1. **Get the ID.** The caller has it in the `X-Request-ID` response header, or
   in the `request_id` field of the error body.
2. **Find the request:** `grep <request_id> logs/api.log`
3. **Read it in order.** The completion line gives status and duration. The
   `request failed` line gives the cause. Any `query failed` lines carry the
   same ID and show which query broke and how long it ran.

## Field reference

| Field | Appears on | Meaning |
|---|---|---|
| `request_id` | everything | Correlates every line belonging to one request |
| `method`, `path` | completion, errors | Route pattern (`/repo-test/users`), or the raw URL when no route matched |
| `status` | completion, errors | The real HTTP status |
| `duration` | completion, queries | Elapsed time in **seconds** (zerolog's `Dur` format) |
| `remote_ip` | completion | Caller address; aware of `X-Forwarded-For`, so it is spoofable |
| `error` | errors | Failure cause, including wrapped context |
| `panic`, `stack` | panics | Recovered panic value and goroutine stack |
| `query`, `table` | repositories | Which query ran, against which table |

## Panics

A recovered panic is logged as structured JSON including the stack:

```json
{"level":"error","request_id":"nbiL...","method":"GET","path":"/panic","status":500,"panic":"nil map write","stack":"goroutine 1 [running]: ..."}
```

The panic value is never sent to the client; the caller gets a generic 500. Note
that a panicking request has **no** `request completed` line, because `Recover`
sits outside `RequestLogger` and unwinds past it. The panic line is that
request's record.

## Logging from your own code

In a handler:

```go
func (h *RepoTestHandler) GetUser(c echo.Context) error {
	log := logger.FromContext(c.Request().Context())

	log.Info().Str("user_id", id.String()).Msg("loading user")

	user, err := h.userRepo.GetByID(ctx, id)
	if err != nil {
		// No need to log here: returning the error is enough for the central
		// handler to record it with the right status. Log only to add context
		// the error itself does not carry.
		return NewEchoError(http.StatusInternalServerError, "internal_error", "failed to get user")
	}
	return c.JSON(http.StatusOK, userResponse(user))
}
```

In a repository, pass the context so the query log inherits the request ID:

```go
r.logQuery(ctx, "GetByID", "users", err, start)
```

## Things worth knowing

- **The inbound `X-Request-ID` is trusted.** Echo reuses a client-supplied
  header when present, so a caller can choose its own ID. Fine for a single
  instance; revisit before exposing the API to untrusted clients at scale.
- **`logger.FromContext` falls back to a global logger** when a context carries
  none. `main` installs the configured logger with `logger.SetGlobal`, so
  background logs (the Redis monitor, for example) reach the same sink instead of
  a separate stdout logger.
- **`LOG_LEVEL=error` hides completion lines.** They are `info` for 2xx,
  `warn` for 4xx and `error` for 5xx, so a stricter level silences the
  ledger, not just the noise.
- **Output is JSON on stdout by default** (`LOG_OUTPUT=stdout`). Set
  `LOG_OUTPUT=file` with `LOG_FILE=...` to write to a file, or `both`.
