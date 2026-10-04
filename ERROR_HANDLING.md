# Error handling

The API returns errors as:

```json
{"error":"code","message":"human-readable message"}
```

Database unavailability produces a 503 health response. Redis is an optional
cache: cache failures are logged and treated as cache misses rather than API
failures. The cache reconnect monitor restores it after Redis is healthy.
