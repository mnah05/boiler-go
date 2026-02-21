# Boiler-Go

A production-ready Go backend boilerplate with clean architecture, PostgreSQL, Redis, and background job processing.

---

## 🚀 Quick Start

```bash
# Clone the repository
git clone https://github.com/mnah05/boiler-go.git
cd boiler-go

# Copy environment file
cp .env.example .env

# Start services with Docker
make dev

# Run API server
make api

# Run background worker
make worker
```

---

## 📋 Features

- ✅ **Thread-Safe Database Pool** - Concurrent-safe PostgreSQL connection management
- ✅ **Graceful Shutdown** - Proper resource cleanup and timeout handling
- ✅ **Background Jobs** - Redis-based task processing with Asynq
- ✅ **Health Checks** - Comprehensive service health monitoring
- ✅ **Structured Logging** - JSON logging with request tracing
- ✅ **Environment Configuration** - Flexible config with validation
- ✅ **Error Handling** - Robust error management and recovery
- ✅ **Docker Support** - Containerized development environment

---

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │───▶│   API Server    │───▶│   PostgreSQL    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐    ┌─────────────────┐
                       │ Background Jobs │───▶│     Redis       │
                       │    (Worker)     │    │   (Queue/Cache) │
                       └─────────────────┘    └─────────────────┘
```

### Project Structure

```
boiler-go/
├── cmd/
│   ├── api/          # HTTP API server
│   └── worker/       # Background job processor
├── internal/
│   ├── config/       # Environment configuration
│   ├── db/          # Database connection and queries
│   ├── handler/     # HTTP request handlers
│   ├── middleware/   # HTTP middleware
│   └── scheduler/    # Job scheduling client
├── pkg/
│   └── logger/      # Structured logging utilities
├── sql/             # Database migration files
└── docker-compose.yml
```

---

## ⚙️ Configuration

### Environment Variables

```bash
# Server
APP_PORT=8080

# Database
DATABASE_URL=postgres://postgres:postgres@localhost:5432/appdb?sslmode=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# Timeouts
HEALTH_CHECK_TIMEOUT=2s
API_SHUTDOWN_TIMEOUT=10s
WORKER_SHUTDOWN_TIMEOUT=30s
```

### Database Configuration

The database pool is configured with sensible defaults:

- **Max Connections**: 15
- **Min Connections**: 2
- **Connection Lifetime**: 30 minutes
- **Idle Timeout**: 5 minutes
- **Health Check Period**: 1 minute

---

## 🔧 Development

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- PostgreSQL
- Redis

### Setup

1. **Start Infrastructure**

   ```bash
   make dev
   ```

2. **Run Migrations**

   ```bash
   make migrate-up
   ```

3. **Generate SQL Code**
   ```bash
   make sqlc
   ```

### Running Services

```bash
# Start API server
make api

# Start background worker
make worker

# Stop all services
make dev-down
```

---

## 🏥 Health Check

The `/health` endpoint provides service status:

```json
{
  "status": {
    "database": "up",
    "redis": "up"
  },
  "checked": "2024-02-21T20:41:00Z"
}
```

---

## 📦 Dependencies

### Core Backend

- **[chi](https://github.com/go-chi/chi)** - Lightweight HTTP router
- **[pgx/v5](https://github.com/jackc/pgx)** - PostgreSQL driver
- **[sqlc](https://sqlc.dev/)** - Type-safe SQL code generation

### Background Jobs & Caching

- **[asynq](https://github.com/hibiken/asynq)** - Redis-based job queue
- **[go-redis](https://github.com/redis/go-redis)** - Redis client

### Configuration & Logging

- **[env/v11](https://github.com/caarlos0/env)** - Environment variable parsing
- **[zerolog](https://github.com/rs/zerolog)** - Structured JSON logging

---

## 🛡️ Production Readiness

This boilerplate includes several production-ready features:

### Thread Safety

- Database pool uses `sync.RWMutex` for concurrent access
- Configuration loading uses `sync.Once` for safe singleton pattern
- All shared resources are properly synchronized

### Error Handling

- Comprehensive error checking and logging
- Graceful degradation on service failures
- Proper resource cleanup on errors

### Resource Management

- Connection pooling with configurable limits
- Automatic cleanup on shutdown
- Memory leak prevention

### Monitoring

- Health check endpoints for all services
- Structured logging with request tracing
- Error metrics and alerting ready

---

## 📝 API Endpoints

### Health Check

```
GET /health
```

Returns the status of all connected services.

---

## 🧪 Testing

```bash
# Run all tests
make test

# Run with race detection
go test -race ./...

# Run integration tests
go test -tags=integration ./...
```

---

## 🚀 Deployment

### Docker

```bash
# Build image
docker build -t boiler-go .

# Run container
docker run -p 8080:8080 --env-file .env boiler-go
```

### Environment Variables

Ensure all required environment variables are set in production:

```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require
REDIS_ADDR=redis-host:6379
APP_PORT=8080
```

---

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

---

## 📄 License

This project is licensed under the MIT License.

---

## 🔗 Links

- [GitHub Repository](https://github.com/mnah05/boiler-go)
- [Documentation](https://github.com/mnah05/boiler-go/wiki)
- [Issues](https://github.com/mnah05/boiler-go/issues)
