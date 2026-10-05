# Boiler-Go

A Dockerized Go API starter using Echo, PostgreSQL, sqlc, sqlx, and the pgx
database driver. Redis is an optional cache: the API continues serving when it
is unavailable and automatically resumes cache use after its background probe
finds Redis healthy again.

## Run with Docker

```bash
docker compose up --build
curl http://localhost:8080/health
```

Compose runs PostgreSQL, applies migrations once, starts the API, and also
starts Redis for caching. The API has no dependency on Redis startup.

## Local development

```bash
cp .env.example .env
make dev
make migrate-up
make api
```

## Architecture

- `cmd/api`: Echo HTTP server.
- `internal/handler`: routes, handlers, and the central error handler.
- `internal/middleware`: request logging, Clerk auth, security headers,
  and body-size limits.
- `internal/repository`: sqlc-generated `database/sql` queries used through
  `sqlx`, with `pgx/v5/stdlib` as the registered `pgx` driver.
- `internal/cache`: best-effort Redis cache interface and reconnect monitor.
- `internal/mailer`: small mail-sender interface with an optional Resend
  implementation enabled by `RESEND_API_KEY` and `RESEND_FROM`.

Run `make sqlc` after editing SQL and `make test` before committing.

## Debugging with logs

Every request is logged as structured JSON with a `request_id` that is also
returned in the `X-Request-ID` response header and in error bodies. Failures
are logged with their real HTTP status and their cause, and recovered panics
include a stack trace. See [LOGGING.md](LOGGING.md) for the field reference and a
step-by-step way to trace a reported failure.

## API docs (OpenAPI)

Annotated with [swaggo](https://github.com/swaggo/swag) — code is the source of truth.

```bash
make swagger-install  # one-time: installs swag CLI
make swagger          # regenerates docs/ from annotations
make api              # UI at http://localhost:8080/swagger/index.html
```

- Spec files: `docs/swagger.json`, `docs/swagger.yaml` (committed).
- Raw spec also served at `/swagger/doc.json`.
- Add a new endpoint: annotate the handler (`@Summary/@Tags/@Param/@Success/@Failure/@Router`), then rerun `make swagger`.
- Auth uses `BearerAuth` (`Authorization: Bearer <clerk-session-token>`), verified by the Clerk Go SDK. `POST /webhooks/clerk` (Svix-signed) syncs `user.created/updated/deleted` into `users.clerk_id`.

### Docs stay fresh automatically

- Local: the pre-commit hook (`.githooks/pre-commit`, via `make hooks`) reruns
  `make swagger` and stages `docs/`. Needs the CLI once: `make swagger-install`.
- Pull requests: `.github/workflows/openapi.yml` fails if `docs/` is stale —
  run `make swagger` and push.
- Pushes to `main`: the same workflow regenerates `docs/` and commits the
  update back, so merged annotations are always reflected.
- `make swagger-check` is the CI gate (regen + `git diff --exit-code -- docs/`).
