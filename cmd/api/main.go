package main

import (
	"auth-service/api/proto"
	"auth-service/core/pkg/corehandler"
	"auth-service/internal/app"
	"auth-service/internal/handler"
	"auth-service/internal/middleware"
	"auth-service/internal/repository"
	"auth-service/internal/router"
	"auth-service/internal/service"
	"log/slog"
	"os"
)

func main() {
	// 1. Собираем ядро со всеми новыми компонентами
	ms, err := app.NewBuilder("config.yml").
		WithLogger().
		WithCORS().
		WithTracing().
		WithJWT().
		WithMigrations().
		WithDatabases().
		WithRedis().
		WithEncryptor().
		WithOutboxRelay().
		WithGRPCServer().
		Build()

	if err != nil {
		slog.Error("Критическая ошибка при сборке", slog.Any("error", err))
		os.Exit(1)
	}

	// 2. Инициализируем бизнес-логику (Слои чистой архитектуры)
	DefaultRepo := repository.NewDefaultRepository(
		ms.DBPool,
		ms.RedisClient,
	)
	DefaultService := service.NewDefaultService(DefaultRepo, ms.Encryptor, ms.JWTManager, ms.AppCfg.App.IsProd)
	DefaultHandler := handler.NewDefaultHandler(DefaultService, ms.AppCfg.App.IsProd)

	CoreHandler := corehandler.NewDefaultHandler(DefaultService)

	myUserService := &service.UserServer{}
	proto.RegisterUserServiceServer(ms.GRPCServer, myUserService)

	// 5. Настраиваем HTTP Роутер Fiber
	// Обрати внимание: мы передаем FiberApp и хендлеры в отдельный пакет роутера,
	// чтобы не засорять main.go тысячей строк с эндпоинтами.
	//

	midManager := middleware.NewManager(
		ms.CoreCfg.Prometheus.Enabled,
		ms.CoreCfg.Jaeger.Enabled,
		ms.CoreCfg.Prometheus.Secure,
		ms.CoreCfg.Prometheus.User,
		ms.CoreCfg.Prometheus.Password,
		ms.JWTManager,
		ms.RedisClient,
		ms.AppCfg.App.IsProd,
	)

	router.SetupRoutes(
		ms.FiberApp,
		midManager,
		CoreHandler,
		DefaultHandler,
		ms.HealthCheckers,
		ms.CoreCfg.Prometheus.Enabled,
	)

	// 6. ЗАПУСК! (Эта функция блокирует поток и держит приложение живым)
	if err := ms.Run(); err != nil {
		slog.Error("Ошибка при работе микросервиса", slog.Any("error", err))
		os.Exit(1)
	}
}
