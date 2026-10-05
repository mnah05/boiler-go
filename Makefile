# ---------- run ----------
api:
	go run ./cmd/api

# ---------- stop ----------
stop-api:
	pkill -f "go run ./cmd/api" || echo "API not running"

stop-migrate:
	pkill -f "migrate" || echo "No migrations running"

stop: stop-api stop-migrate
	@echo "All services stopped"

# ---------- format / lint ----------
fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Run: brew install golangci-lint"; \
		exit 1; \
	fi

check: fmt vet lint test
	@echo "All checks passed"

# ---------- test ----------
test:
	go test -race ./...

# ---------- database ----------
migrate-up:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	migrate -path ./migrations -database $$DATABASE_URL up

migrate-down:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	migrate -path ./migrations -database $$DATABASE_URL down 1

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

migrate-force:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	migrate -path ./migrations -database $$DATABASE_URL force $(version)

migrate-version:
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	migrate -path ./migrations -database $$DATABASE_URL version

# ---------- sqlc ----------
sqlc:
	sqlc generate

# ---------- swagger / openapi ----------
SWAG_VERSION ?= v1.16.4

swagger-install:
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

swagger:
	swag init -g cmd/api/main.go -o docs
	@echo "OpenAPI generated in docs/. UI at /swagger/index.html"

# CI-friendly: regenerates docs and fails if committed docs are stale.
swagger-check:
	swag init -g cmd/api/main.go -o docs
	git diff --exit-code -- docs/ || (echo "docs/ is stale. Run 'make swagger' and commit the result."; exit 1)

# ---------- dev ----------
dev:
	docker compose up -d

dev-down:
	docker compose down

dev-stop: dev-down
	@echo "Docker services stopped"

# ---------- build ----------
build-api:
	go build -o bin/api ./cmd/api

build: build-api
	@echo "Build complete"

# ---------- hooks ----------
hooks:
	git config core.hooksPath .githooks
	@echo "Git hooks configured from .githooks/"

# ---------- clean ----------
clean: stop dev-down
	@rm -rf bin/
	@echo "Cleaned up"

.PHONY: api stop-api stop-migrate stop fmt vet lint check test hooks migrate-up migrate-down migrate-create migrate-force migrate-version sqlc swagger swagger-install swagger-check dev dev-down dev-stop build-api build clean
