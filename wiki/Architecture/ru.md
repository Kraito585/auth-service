# Архитектура

## Слои приложения

Проект следует принципам **Чистой Архитектуры** (Clean Architecture) с разделением на слои:

```
┌──────────────────────────────────────────────────┐
│                   cmd/api/main.go                 │  ← Точка входа
├──────────────────────────────────────────────────┤
│                internal/app/app.go                │  ← Сборка (Builder)
├──────────────────────────────────────────────────┤
│  internal/handler  │  internal/router             │  ← Транспортный слой
├──────────────────────────────────────────────────┤
│  internal/service                                │  ← Бизнес-логика
├──────────────────────────────────────────────────┤
│  internal/repository                             │  ← Доступ к данным
├──────────────────────────────────────────────────┤
│  internal/model                                  │  ← Доменные модели
├──────────────────────────────────────────────────┤
│  core/pkg/*  │  pkg/config                        │  ← Инфраструктура
└──────────────────────────────────────────────────┘
```

## Структура директорий

| Путь | Назначение |
|------|------------|
| `cmd/api/main.go` | Точка входа. Читает конфиг, запускает Builder |
| `internal/app/app.go` | Паттерн **Builder** для инициализации микросервиса |
| `internal/handler/` | HTTP/gRPC handlers (контроллеры) |
| `internal/router/` | Регистрация маршрутов Fiber |
| `internal/service/` | Бизнес-логика аутентификации |
| `internal/repository/` | Слой доступа к PostgreSQL |
| `internal/model/` | DTO и доменные структуры |
| `internal/middleware/` | Менеджер middleware + реализации |
| `core/pkg/` | Переиспользуемые пакеты ядра |
| `pkg/config/` | Парсинг app-специфичного конфига |
| `config/certs/` | RSA ключи для JWT |
| `migrations/` | SQL миграции (embed) |
| `api/proto/` | Protobuf определения и сгенерированный gRPC код |

## Паттерн Builder

Инициализация микросервиса построена на цепочке методов Builder'а (fluent interface). Каждый метод `With*()` отвечает за инициализацию одного компонента:

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

Сборка ленивая (lazy): каждый шаг проверяет `b.err` и пропускается при наличии ошибки на предыдущем шаге. Финальный `Build()` собирает middleware manager и возвращает готовый экземпляр `*Microservice`.

## Жизненный цикл микросервиса

Метод `Microservice.Run()` управляет полным жизненным циклом:

1. Запуск Kafka-воркеров (OutboxRelay, StateReplicator) в горутинах
2. Запуск HTTP сервера (Fiber) в горутине
3. Запуск gRPC сервера в горутине
4. Ожидание сигнала ОС (SIGINT, SIGTERM)
5. Graceful Shutdown: остановка Fiber → остановка gRPC → отмена контекста воркеров → ожидание воркеров (с таймаутом 10с) → сброс Jaeger → закрытие пула PostgreSQL → закрытие Redis