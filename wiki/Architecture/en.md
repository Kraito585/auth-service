# Architecture

## Application Layers

The project follows **Clean Architecture** principles with layered separation:

```
┌──────────────────────────────────────────────────┐
│                   cmd/api/main.go                 │  ← Entry Point
├──────────────────────────────────────────────────┤
│                internal/app/app.go                │  ← Assembly (Builder)
├──────────────────────────────────────────────────┤
│  internal/handler  │  internal/router             │  ← Transport Layer
├──────────────────────────────────────────────────┤
│  internal/service                                │  ← Business Logic
├──────────────────────────────────────────────────┤
│  internal/repository                             │  ← Data Access
├──────────────────────────────────────────────────┤
│  internal/model                                  │  ← Domain Models
├──────────────────────────────────────────────────┤
│  core/pkg/*  │  pkg/config                        │  ← Infrastructure
└──────────────────────────────────────────────────┘
```

## Directory Structure

| Path | Purpose |
|------|---------|
| `cmd/api/main.go` | Entry point. Reads config, starts Builder |
| `internal/app/app.go` | **Builder** pattern for microservice initialization |
| `internal/handler/` | HTTP/gRPC handlers (controllers) |
| `internal/router/` | Fiber route registration |
| `internal/service/` | Authentication business logic |
| `internal/repository/` | PostgreSQL data access layer |
| `internal/model/` | DTOs and domain structures |
| `internal/middleware/` | Middleware manager + implementations |
| `core/pkg/` | Reusable core packages |
| `pkg/config/` | App-specific config parsing |
| `config/certs/` | RSA keys for JWT |
| `migrations/` | SQL migrations (embed) |
| `api/proto/` | Protobuf definitions and generated gRPC code |

## Builder Pattern

Microservice initialization uses a chain of Builder methods (fluent interface). Each `With*()` method handles initialization of a single component:

```
NewBuilder(configPath)
  .WithLogger()
  .WithGRPCServer()
  .WithJWT()
  .WithMigrations()
  .WithDatabases()
  .WithRedis()
  .WithEncryptor()
  .WithOutboxRelay()
  .WithStateReplicator()
  .WithCORS()
  .WithTracing()
  .Build()
```

Lazy assembly: each step checks `b.err` and is skipped if a previous step failed. The final `Build()` assembles the middleware manager and returns a ready `*Microservice` instance.

## Microservice Lifecycle

The `Microservice.Run()` method manages the full lifecycle:

1. Start Kafka workers (OutboxRelay, StateReplicator) in goroutines
2. Start HTTP server (Fiber) in a goroutine
3. Start gRPC server in a goroutine
4. Wait for OS signal (SIGINT, SIGTERM)
5. Graceful Shutdown: stop Fiber → stop gRPC → cancel worker context → wait for workers (10s timeout) → flush Jaeger → close PostgreSQL pool → close Redis