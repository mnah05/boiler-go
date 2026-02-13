APP_NAME=app

# ---------- run ----------
api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

# ---------- database ----------
migrate-up:
	migrate -path ./migrations -database $$DATABASE_URL up

migrate-down:
	migrate -path ./migrations -database $$DATABASE_URL down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

# ---------- sqlc ----------
sqlc:
	sqlc generate

# ---------- dev ----------
dev:
	docker compose up -d

dev-down:
	docker compose down

# ---------- quality ----------
test:
	go test ./...

tidy:
	go mod tidy

.PHONY: api worker migrate-up migrate-down migrate-create sqlc dev dev-down test tidy
