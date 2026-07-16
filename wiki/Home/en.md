# Auth Service

**Auth Service** is a high-performance authentication and authorization microservice built in Go using the [Fiber v3](https://docs.gofiber.io/) framework and gRPC.

## Features

- **Registration & Login** — multi-factor authentication support (password, email code, TOTP)
- **JWT Tokens** — RS256 signing, access/refresh tokens with short TTL
- **SSO (Single Sign-On)** — one-time token generation for passing sessions to partner services
- **S2S (Server-to-Server) API** — secured token exchange between services via API keys
- **Outbox Relay** — guaranteed event delivery to Kafka via the Outbox pattern
- **Observability** — Distributed Tracing (Jaeger), Metrics (Prometheus), Structured Logging (slog)
- **Graceful Shutdown** — clean shutdown of HTTP/gRPC servers, Kafka workers, DB connection pools
- **Health Check** — endpoints for PostgreSQL, Redis, migrations, and Kafka worker health
- **DB Migrations** — embedded migration system via Go embed file system

## Technology Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.22+ |
| HTTP Framework | Fiber v3 |
| gRPC | google.golang.org/grpc |
| Database | PostgreSQL (pgx v5) |
| Cache / Storage | Redis (go-redis v9) |
| Message Queue | Kafka (segmentio/kafka-go) |
| Tracing | OpenTelemetry + Jaeger |
| Metrics | Prometheus (client_golang) |
| Logging | log/slog (Go standard library) |
| JWT | golang-jwt/jwt v5 |
| TOTP | pquerna/otp |
| Migrations | golang-migrate/migrate v4 |
| Containerization | Docker |

## Quick Start

```bash
# Clone
git clone https://git.kraito.ru/social/auth-service.git
cd auth-service

# Build
go build -o auth-service ./cmd/api

# Run (requires PostgreSQL, Redis, Kafka)
./auth-service -config=config.yml