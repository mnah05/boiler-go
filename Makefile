APP_NAME=app

# PID files for process management
API_PID=.api.pid
WORKER_PID=.worker.pid

# ---------- run ----------
api:
	go run ./cmd/api

worker:
	go run ./cmd/worker

# Run both API and worker in background
run-all:
	@echo "Starting API..."
	@go run ./cmd/api > api.log 2>&1 & echo $$! > $(API_PID)
	@echo "API started (PID: $$(cat $(API_PID)))"
	@echo "Starting Worker..."
	@go run ./cmd/worker > worker.log 2>&1 & echo $$! > $(WORKER_PID)
	@echo "Worker started (PID: $$(cat $(WORKER_PID)))"
	@echo "Both services running. Check api.log and worker.log for output."
	@echo "Run 'make stop-all' to stop both services."

# Stop both API and worker
stop-all:
	@echo "Stopping services..."
	@if [ -f $(API_PID) ]; then \
		kill $$(cat $(API_PID)) 2>/dev/null && echo "API stopped" || echo "API already stopped"; \
		rm -f $(API_PID); \
	else \
		echo "API not running"; \
	fi
	@if [ -f $(WORKER_PID) ]; then \
		kill $$(cat $(WORKER_PID)) 2>/dev/null && echo "Worker stopped" || echo "Worker already stopped"; \
		rm -f $(WORKER_PID); \
	else \
		echo "Worker not running"; \
	fi
	@echo "All services stopped."

# Check status of services
status:
	@echo "Service Status:"
	@if [ -f $(API_PID) ]; then \
		if ps -p $$(cat $(API_PID)) > /dev/null 2>&1; then \
			echo "  API: Running (PID: $$(cat $(API_PID)))"; \
		else \
			echo "  API: Not running (stale PID file)"; \
			rm -f $(API_PID); \
		fi \
	else \
		echo "  API: Not running"; \
	fi
	@if [ -f $(WORKER_PID) ]; then \
		if ps -p $$(cat $(WORKER_PID)) > /dev/null 2>&1; then \
			echo "  Worker: Running (PID: $$(cat $(WORKER_PID)))"; \
		else \
			echo "  Worker: Not running (stale PID file)"; \
			rm -f $(WORKER_PID); \
		fi \
	else \
		echo "  Worker: Not running"; \
	fi

# View logs
logs:
	@echo "=== API Logs ==="
	@tail -f api.log 2>/dev/null || echo "No API logs found"

logs-worker:
	@echo "=== Worker Logs ==="
	@tail -f worker.log 2>/dev/null || echo "No worker logs found"

logs-all:
	@tail -f api.log worker.log 2>/dev/null || echo "No logs found"

# Clean up log and PID files
clean:
	@rm -f $(API_PID) $(WORKER_PID) api.log worker.log
	@echo "Cleaned up PID and log files"

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

.PHONY: api worker run-all stop-all status logs logs-worker logs-all clean migrate-up migrate-down migrate-create sqlc dev dev-down test tidy
