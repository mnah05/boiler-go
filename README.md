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
- `internal/repository`: sqlc-generated `database/sql` queries used through
  `sqlx`, with `pgx/v5/stdlib` as the registered `pgx` driver.
- `internal/cache`: best-effort Redis cache interface and reconnect monitor.
- `internal/mailer`: small mail-sender interface with an optional Resend
  implementation enabled by `RESEND_API_KEY` and `RESEND_FROM`.

Run `make sqlc` after editing SQL and `make test` before committing.
