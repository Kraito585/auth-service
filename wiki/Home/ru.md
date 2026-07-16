# Auth Service

**Auth Service** — это высокопроизводительный микросервис аутентификации и авторизации, построенный на Go с использованием фреймворка [Fiber v3](https://docs.gofiber.io/) и gRPC.

## Возможности

- **Регистрация и вход** — поддержка многофакторной аутентификации (пароль, email-код, TOTP)
- **JWT токены** — RS256 подпись, access/refresh токены с коротким TTL
- **SSO (Single Sign-On)** — генерация одноразовых токенов для передачи сессии партнёрским сервисам
- **S2S (Server-to-Server) API** — защищённый обмен токенами между сервисами по API-ключам
- **Outbox Relay** — гарантированная доставка событий в Kafka через паттерн Outbox
- **Наблюдаемость** — Distributed Tracing (Jaeger), метрики (Prometheus), структурированное логирование (slog)
- **Graceful Shutdown** — корректная остановка HTTP/gRPC серверов, воркеров Kafka, закрытие пулов БД
- **Health Check** — эндпоинты проверки состояния PostgreSQL, Redis, миграций, Kafka-воркеров
- **Миграции БД** — встроенная система миграций через embed-файловую систему

## Технологический стек

| Компонент | Технология |
|-----------|------------|
| Язык | Go 1.22+ |
| HTTP фреймворк | Fiber v3 |
| gRPC | google.golang.org/grpc |
| База данных | PostgreSQL (pgx v5) |
| Кэш / Хранилище | Redis (go-redis v9) |
| Очередь сообщений | Kafka (segmentio/kafka-go) |
| Трейсинг | OpenTelemetry + Jaeger |
| Метрики | Prometheus (client_golang) |
| Логирование | log/slog (стандартная библиотека Go) |
| JWT | golang-jwt/jwt v5 |
| TOTP | pquerna/otp |
| Миграции | golang-migrate/migrate v4 |
| Контейнеризация | Docker |

## Быстрый старт

```bash
# Клонирование
git clone https://git.kraito.ru/social/auth-service.git
cd auth-service

# Сборка
go build -o auth-service ./cmd/api

# Запуск (требуется PostgreSQL, Redis, Kafka)
./auth-service -config=config.yml