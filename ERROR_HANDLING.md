# Error handling

The API returns errors as:

```json
{"error":"code","message":"human-readable message","request_id":"3f2b1c9d..."}
```

`request_id` matches the `X-Request-ID` response header and every log line
for that request, so a caller can quote it in a bug report. See
[LOGGING.md](LOGGING.md) for how to trace a failure through the logs.

## Where errors are logged

`handler.HTTPErrorHandler` is the single place a failed request is recorded:

- **5xx** at error level, with the underlying cause.
- **4xx** at warn level.
- A caller that disconnected is labelled `request aborted by client` so it is
  not mistaken for a server fault.

Handlers therefore do not need to log before returning an error. Internal
details stay in the logs: the response body only ever carries the error code
and a client-safe message.

Authentication follows the same path: `middleware.ClerkAuth` rejects with a 401
rendered as `{"error":"unauthorized","message":...}`, and a 403 renders as
`{"error":"forbidden",...}`. Both carry `request_id`.

## Other behaviour

Database unavailability produces a 503 health response. Redis is an optional
cache: cache failures are logged and treated as cache misses rather than API
failures. The cache reconnect monitor restores it after Redis is healthy.
