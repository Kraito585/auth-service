package app

import (
	coreconfig "auth-service/core/config"
	"context"
	"fmt"

	"auth-service/core/pkg/coretelemetry"

	"auth-service/core/pkg/health"

	"auth-service/migrations"
	"net"

	pkgkafka "auth-service/core/pkg/kafka"
	"auth-service/core/pkg/logger"

	"auth-service/core/pkg/migrate"
	"auth-service/core/pkg/postgres"

	coreredis "auth-service/core/pkg/redis"
	"auth-service/core/pkg/security"
	"auth-service/internal/middleware"

	"auth-service/internal/telemetry"

	appconfig "auth-service/pkg/config"

	"auth-service/core/pkg/response"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Microservice struct {
	FiberApp *fiber.App

	GRPCServer *grpc.Server

	CoreCfg *coreconfig.CoreConfig
	AppCfg  *appconfig.AppConfig

	Logger *slog.Logger

	DBPool *pgxpool.Pool

	RedisClient     *coreredis.Wrapper
	Encryptor       *security.Encryptor
	MigrationsReady *atomic.Bool

	OutboxRelay *pkgkafka.OutboxRelay

	JWTManager *security.JWTManager

	TracerShutdown func(context.Context) error

	HealthCheckers []health.Checker
}

type Builder struct {
	app *Microservice
	err error
}

func NewBuilder(configPath string) *Builder {
	coreCfg, err := coreconfig.LoadCoreConfig(configPath)
	if err != nil {
		return &Builder{err: err}
	}

	appCfg, err := appconfig.LoadAppConfig(configPath)
	if err != nil {
		return &Builder{err: err}
	}

	isPrometheusEnabled := coreCfg.Prometheus.Enabled
	promServiceName := coreCfg.Prometheus.ServiceName

	// Инициализируем базовую телеметрию
	coretelemetry.InitMetrics(promServiceName, isPrometheusEnabled)
	telemetry.InitAppMetrics(promServiceName, isPrometheusEnabled)

	fiberApp := fiber.New(fiber.Config{
		ErrorHandler: response.GlobalErrorHandler,
	})

	return &Builder{
		app: &Microservice{
			FiberApp:        fiberApp,
			CoreCfg:         coreCfg,
			AppCfg:          appCfg,
			MigrationsReady: &atomic.Bool{},
			HealthCheckers:  make([]health.Checker, 0),
		},
	}
}

func (b *Builder) WithLogger() *Builder {
	if b.err != nil {
		return b
	}

	// Инициализируем логгер на основе флага is_prod
	logger.Init(b.app.AppCfg.App.IsProd)

	// Сохраняем логгер в поле для использования в других методах
	b.app.Logger = slog.Default()

	// Делаем первую тестовую запись
	slog.Info("Логгер инициализирован", slog.Bool("is_prod", b.app.AppCfg.App.IsProd))

	return b
}

func (b *Builder) WithGRPCServer(interceptors ...grpc.UnaryServerInterceptor) *Builder {
	if b.err != nil {
		return b
	}

	// По умолчанию можно добавить системные интерцепторы ядра (метрики, логи, Jaeger)
	// Например, с использованием официальных пакетов для open-telemetry
	var opts []grpc.ServerOption

	// Если пользователь передал свои кастомные интерцепторы, объединяем их
	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	b.app.GRPCServer = grpc.NewServer(opts...)

	slog.Info("gRPC сервер успешно инициализирован")
	return b
}

func (b *Builder) WithJWT() *Builder {
	if b.err != nil {
		return b
	}

	cfg := b.app.CoreCfg.JWT
	if !cfg.Enabled {
		return b
	}

	jwtManager, err := security.NewJWTManager(cfg.PrivateKeyPath, cfg.PublicKeyPath, cfg.AccessTTL)
	if err != nil {
		if !b.app.AppCfg.App.IsProd {
			b.err = err
			return b
		}
		slog.Warn("Ошибка инициализации JWT Manager (Prod)", slog.Any("error", err))
	}

	b.app.JWTManager = jwtManager
	if jwtManager != nil {
		slog.Info("JWT Manager успешно инициализирован")
	}

	return b
}

func (b *Builder) WithMigrations() *Builder {
	if b.err != nil {
		return b
	}

	if err := migrate.Run(b.app.CoreCfg, migrations.FS); err != nil {
		if !b.app.AppCfg.App.IsProd {
			b.err = fmt.Errorf("критическая ошибка миграций (Dev): %w", err)
			return b
		}

		slog.Warn("Критическая ошибка миграций (Prod). Трафик заблокирован.", slog.Any("error", err))
		b.app.MigrationsReady.Store(false)
	} else {
		b.app.MigrationsReady.Store(true)
	}

	migChecker := health.NewMigrationChecker(b.app.MigrationsReady)
	b.app.HealthCheckers = append(b.app.HealthCheckers, migChecker)

	return b
}

func (b *Builder) WithDatabases() *Builder {
	if b.err != nil {
		return b
	}

	// 1. Безопасно определяем имя главной базы данных
	var mainDBName string
	if len(b.app.CoreCfg.Postgres.Names) > 0 {
		mainDBName = b.app.CoreCfg.Postgres.Names[0] // Берем из массива, если он есть
	} else {
		mainDBName = b.app.CoreCfg.Postgres.Name // Фолбэк на одиночное поле db_name
	}

	// Защита от ситуации, когда в конфиге вообще забыли указать базу
	if mainDBName == "" {
		b.err = fmt.Errorf("критическая ошибка: не указано имя базы данных (ни db_name, ни db_names)")
		return b
	}

	pool, err := postgres.NewPool(context.Background(), b.app.CoreCfg.Postgres, mainDBName)
	if err != nil {
		b.err = err
		return b
	}

	b.app.DBPool = pool

	pgChecker := health.NewPostgresChecker(b.app.DBPool)
	b.app.HealthCheckers = append(b.app.HealthCheckers, pgChecker)

	return b
}

func (b *Builder) WithRedis() *Builder {
	if b.err != nil {
		return b
	}

	ctx := context.Background()
	client, err := coreredis.NewRedisManager(ctx, b.app.CoreCfg.Redis)
	if err != nil {
		b.err = fmt.Errorf("ошибка инициализации пула Redis: %w", err)
		return b
	}

	b.app.RedisClient = client

	redisChecker := health.NewRedisChecker(b.app.RedisClient)
	b.app.HealthCheckers = append(b.app.HealthCheckers, redisChecker)

	return b
}

func (b *Builder) WithEncryptor() *Builder {
	if b.err != nil {
		return b
	}

	enc, err := security.NewEncryptor(b.app.CoreCfg.Security.MasterKey)
	if err != nil {
		b.err = fmt.Errorf("ошибка инициализации шифровальщика: %w", err)
		return b
	}

	b.app.Encryptor = enc
	return b
}

func (b *Builder) WithOutboxRelay() *Builder {
	if b.err != nil {
		return b
	}

	// 1. Проверяем глобальный рубильник
	if !b.app.CoreCfg.Kafka.Enabled {
		return b
	}

	// 2. Проверяем рубильник конкретного модуля
	if !b.app.CoreCfg.Kafka.OutboxRelay.Enabled {
		return b
	}

	if b.app.DBPool == nil {
		b.err = fmt.Errorf("критическая ошибка: Outbox Relay требует пул БД (вызови WithDatabases раньше)")
		return b
	}

	isProd := b.app.AppCfg.App.IsProd

	b.app.OutboxRelay = pkgkafka.NewOutboxRelay(b.app.DBPool, b.app.CoreCfg.Kafka, isProd)

	relayChecker := health.NewOutboxRelayChecker(b.app.OutboxRelay)
	b.app.HealthCheckers = append(b.app.HealthCheckers, relayChecker)

	return b
}

func (b *Builder) WithCORS() *Builder {
	if b.err != nil {
		return b
	}

	cfg := b.app.CoreCfg.CORS

	if !cfg.Enabled {
		return b
	}

	corsConfig := cors.Config{
		AllowOriginsFunc: func(origin string) bool {

			if origin == "" || origin == "http://localhost:3000" {
				return true
			}
			isAllowed, err := b.app.RedisClient.SIsMember(context.Background(), "cors:allowed_origins", origin).Result()

			if err != nil {
				slog.Error("Ошибка проверки CORS в Redis", "error", err, "origin", origin)
				return false
			}

			return isAllowed
		},

		AllowMethods:     cfg.AllowMethods,
		AllowHeaders:     cfg.AllowHeaders,
		AllowCredentials: cfg.AllowCredentials,
	}

	b.app.FiberApp.Use(cors.New(corsConfig))

	slog.Info("CORS middleware успешно активирован (Динамический режим)")
	return b
}

func (b *Builder) WithTracing() *Builder {
	if b.err != nil {
		return b
	}

	cfg := b.app.CoreCfg.Jaeger

	// Вызываем наш инициализатор из пакета telemetry
	shutdownFn, err := telemetry.InitJaeger(cfg.URL, cfg.ServiceName, cfg.Enabled)
	if err != nil {
		b.err = fmt.Errorf("ошибка инициализации Jaeger: %w", err)
		return b
	}

	if cfg.Enabled {
		slog.Info("Трейсинг Jaeger успешно активирован")
	}

	// Сохраняем функцию остановки
	b.app.TracerShutdown = shutdownFn
	return b
}

// Build завершает сборку и возвращает готовое приложение
// Build собирает итоговый инстанс микросервиса
func (b *Builder) Build() (*Microservice, error) {
	// 1. Проверяем, не было ли ошибок на предыдущих этапах сборки
	if b.err != nil {
		return nil, b.err
	}

	// 2. Достаем конфиги для удобства
	coreCfg := b.app.CoreCfg
	isPrometheusEnabled := coreCfg.Prometheus.Enabled
	isJaegerEnabled := coreCfg.Jaeger.Enabled

	// 3. ИНИЦИАЛИЗИРУЕМ MIDDLEWARE MANAGER ЗДЕСЬ!
	// Теперь b.app.JWTManager точно существует (если был вызван .WithJWT)
	midManager := middleware.NewManager(
		isPrometheusEnabled,
		isJaegerEnabled,
		coreCfg.Prometheus.Secure,
		coreCfg.Prometheus.User,
		coreCfg.Prometheus.Password,
		b.app.JWTManager, // <--- Прокидываем готовый JWT!
		b.app.RedisClient,
		b.app.AppCfg.App.IsProd,
	)

	// 4. Подключаем глобальные middlewares к Fiber
	b.app.FiberApp.Use(midManager.Tracing())
	b.app.FiberApp.Use(midManager.Logging())
	b.app.FiberApp.Use(midManager.Metrics())

	// Логирование статуса
	if isPrometheusEnabled {
		slog.Info("Мониторинг Prometheus успешно активирован")
	} else {
		slog.Info("Мониторинг Prometheus отключен")
	}

	// 5. Возвращаем готовое ядро
	return b.app, nil
}

func (m *Microservice) Run() error {
	// 1. Создаем контекст, привязанный к жизненному циклу приложения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // На случай непредвиденной паники

	var wg sync.WaitGroup

	// 2. Запускаем фоновые воркеры Кафки
	if m.OutboxRelay != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.OutboxRelay.Start(ctx)
		}()
	}

	// 3. Настраиваем перехват системных сигналов
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 4. Запускаем Fiber (HTTP сервер)
	go func() {
		port := m.AppCfg.App.Port
		slog.Info("HTTP Сервер запускается", slog.String("port", port))

		if err := m.FiberApp.Listen(":" + port); err != nil {
			slog.Error("Ошибка HTTP сервера", slog.Any("error", err))
		}
	}()

	// 4.1 Запускаем gRPC Сервер
	if m.GRPCServer != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			grpcPort := "50051" // Или брать из специального поля конфигурации
			lis, err := net.Listen("tcp", ":"+grpcPort)
			if err != nil {
				slog.Error("Ошибка запуска gRPC listener", slog.Any("error", err))
				return
			}
			slog.Info("gRPC Сервер запускается", slog.String("port", grpcPort))
			if err := m.GRPCServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
				slog.Error("Ошибка gRPC сервера", slog.Any("error", err))
			}
		}()
	}

	// ------------------------------------------------------------------
	// БЛОКИРОВКА: Ждем сигнала от ОС (Ctrl+C или Kubernetes SIGTERM)
	// ------------------------------------------------------------------
	<-sigChan
	slog.Info("Получен сигнал остановки, начинаем Graceful Shutdown...")

	// 5. Останавливаем прием новых HTTP-запросов
	if err := m.FiberApp.Shutdown(); err != nil {
		slog.Warn("Ошибка при остановке Fiber", slog.Any("error", err))
	}

	// 5.1 Останавливаем gRPC сервер (дает активным RPC завершиться)
	if m.GRPCServer != nil {
		slog.Info("Остановка gRPC сервера...")
		m.GRPCServer.GracefulStop()
	}

	// 6. Посылаем сигнал отмены в ctx (это завершит циклы Start() во всех воркерах)
	slog.Info("Остановка фоновых воркеров Kafka...")
	cancel()

	// 7. Даем воркерам время на безопасное завершение (с таймаутом, чтобы не зависнуть)
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		slog.Info("Все фоновые воркеры безопасно остановлены")
	case <-time.After(10 * time.Second): // Защитный таймаут 10 секунд
		slog.Warn("Истекло время ожидания остановки воркеров (Принудительное продолжение)")
	}
	// 8. Сбрасываем кэш трейсов в коллектор
	if m.TracerShutdown != nil {
		slog.Info("Остановка трейсера Jaeger...")
		ctxTimeout, cancelTrace := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelTrace()

		if err := m.TracerShutdown(ctxTimeout); err != nil {
			slog.Warn("Ошибка при остановке Jaeger", slog.Any("error", err))
		}
	}
	// 9. Закрываем базы данных СТРОГО В КОНЦЕ, когда никто к ним уже не обращается
	if m.DBPool != nil {
		slog.Info("Закрытие пула PostgreSQL...")
		m.DBPool.Close()
	}
	if m.RedisClient != nil {
		slog.Info("Закрытие соединений Redis...")
		if err := m.RedisClient.Close(); err != nil {
			slog.Warn("Ошибка при остановке Redis", slog.Any("error", err))
		}
	}

	slog.Info("Микросервис успешно остановлен. До свидания!")
	return nil
}
