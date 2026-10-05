# AGENTS.md

This repo is a generic boilerplate for spinning up services fast. Keep changes
reusable — no hardcoded domain logic.

## Commands

```bash
make dev             # Start Postgres + Redis via Docker (API runs separately)
make api             # Run the API locally (auto-loads .env via godotenv)
make test            # go test -race ./... (no DB needed)
make sqlc            # Regenerate sqlc code after SQL changes
make migrate-up      # Apply migrations (reads .env manually; needs migrate CLI)
make swagger         # Regen docs/ from handler annotations (required after change)
make build           # Build the API binary
make hooks           # Enable .githooks (pre-commit: fmt + vet + swagger regen)
```

Single package: `go test -race ./internal/handler/`. Full gate: `make check`
(fmt + vet + lint + test). `migrate-create` needs `make migrate-create name=...`.

## Architecture

- One binary: `cmd/api`, an Echo HTTP server wired in `internal/handler/router.go`.
- Auth is Clerk (`clerk-sdk-go/v2`): `middleware.ClerkAuth()` protects routes,
  user ID comes from `ClerkUserIDFromContext` (session claims Subject).
  `ClerkAuth` is `net/http` style — attach with `echo.WrapMiddleware`.
  `POST /webhooks/clerk` is public and Svix-verified: read the raw body first,
  verify signature, then parse. Never `c.Bind` before verifying.
- Users sync from Clerk webhooks (`user.created/updated/deleted`) into
  `users` via `clerk_id TEXT UNIQUE`. No local signup endpoint by design.
- Postgres via sqlc `database/sql` output + sqlx with the pgx stdlib driver.
  Never edit `internal/repository/db/`; edit `sql/schema.sql` + `sql/queries/`,
  then `make sqlc`. `sql/` is the sqlc source, `migrations/` is the deploy
  source — update both for schema changes. Migration numbers have a gap
  (`000002` missing); don't renumber, just use the next sequence.
- Migrations use the `migrate` CLI, not app code. `make migrate-up` exports
  `.env` itself, so `.env` must exist with `DATABASE_URL`.
- Redis is optional cache with background reconnect; the API serves degraded
  without it. `RESEND_API_KEY`/`RESEND_FROM` must be set together or both blank.
- Errors: `handler.HTTPErrorHandler` is the single render/log point; envelope is
  `{"error","message","request_id"}` matching `X-Request-ID`. In handlers log
  with `logger.FromContext`; in `net/http`-style middleware use
  `logger.FromChiContext`. Details: `ERROR_HANDLING.md`, `LOGGING.md`.
- OpenAPI: annotations on handlers are the source of truth; `docs/` is
  committed. Pre-commit hook regens + stages `docs/` (skipped if `swag` CLI
  missing); PR CI fails on stale docs via `make swagger-check`; pushes to main
  auto-commit regen. Pinned CLI: `SWAG_VERSION` in Makefile.
- Required env: `DATABASE_URL`, `CLERK_SECRET_KEY`, `CLERK_WEBHOOK_SECRET`
  (see `.env.example`). Local Clerk webhooks need a public tunnel URL.
