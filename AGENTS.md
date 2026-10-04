# AGENTS.md

## Commands

```bash
make dev             # Start the Dockerized API, PostgreSQL, and Redis cache
make dev-down        # Stop Docker services
make api             # Run the API locally
make test            # Run Go tests with race detection
make sqlc            # Regenerate sqlc code after SQL changes
make migrate-up      # Apply migrations from .env
make build           # Build the API binary
```

## Architecture

- One binary: `cmd/api`, an Echo HTTP server.
- PostgreSQL access uses sqlc's `database/sql` output with sqlx and the pgx
  standard-library driver (`sqlx.Open("pgx", ...)`).
- Redis supports optional cache features only; requests continue when it is
  unavailable, and the cache reconnects in the background.
- Resend is exposed through `internal/mailer` as a provider abstraction.
- Never edit `internal/repository/db/` directly; edit `sql/` then run
  `make sqlc`.
