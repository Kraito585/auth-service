# ==========================================
# Этап 1: Сборка (Builder)
# ==========================================
FROM golang:1.25-alpine AS builder

# Устанавливаем зависимости ОС, которые могут понадобиться для сборки
RUN apk add --no-cache git

WORKDIR /app

# Сначала копируем только файлы зависимостей для кэширования слоя
COPY go.mod go.sum ./
RUN go mod download

# Копируем весь остальной исходный код
COPY . .

# Собираем бинарник. 
# CGO_ENABLED=0 отключает C-библиотеки, делая бинарник полностью статичным.
# Если твой main.go лежит не в корне, а в cmd/api, замени "." на "./cmd/api"
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/auth-service ./cmd/api

# ==========================================
# Этап 2: Финальный легковесный образ (Runtime)
# ==========================================
FROM alpine:latest

WORKDIR /app

# Добавляем корневые сертификаты (критически важно для HTTPS и подключения к Kafka с SSL)
# и tzdata для правильной работы со временем
RUN apk --no-cache add ca-certificates tzdata

# Копируем готовый бинарник из первого этапа
COPY --from=builder /bin/auth-service ./auth-service

# ВАЖНО: Копируем папку с миграциями, чтобы хендлер RecreateDatabase ее нашел!
COPY migrations/ ./migrations/

# Если тебе нужен .env файл внутри контейнера, раскомментируй строку ниже.
# Но обычно в проде переменные передаются через docker-compose (environment:)
# COPY .env .env

# Открываем порт, на котором работает твой Fiber (судя по твоим логам, это 7912)
EXPOSE 7912

# Команда запуска
CMD ["./auth-service"]