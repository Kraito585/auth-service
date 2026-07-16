package migrate

import (
	"auth-service/core/config"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// УДАЛИЛИ: var FS embed.FS

// ДОБАВИЛИ: аргумент fs embed.FS
func Run(cfg *config.CoreConfig, fs embed.FS) error {
	slog.Info("Ожидание проверки миграций базы данных...")

	dsn := cfg.Postgres.DSN(cfg.Postgres.Name)

	// Используем переданную файловую систему fs
	d, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("ошибка чтения файлов миграций: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", d, dsn)
	if err != nil {
		return fmt.Errorf("ошибка инициализации мигратора: %w", err)
	}

	defer func() {
		sourceErr, dbErr := m.Close()
		if sourceErr != nil {
			slog.Warn("Предупреждение при закрытии источника миграций", slog.Any("error", sourceErr))
		}
		if dbErr != nil {
			slog.Warn("Предупреждение при закрытии БД мигратора", slog.Any("error", dbErr))
		}
	}()

	err = m.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("База данных актуальна, новых миграций нет.")
			return nil
		}
		return fmt.Errorf("ошибка при выполнении миграций: %w", err)
	}

	slog.Info("Миграции успешно применены!")
	return nil
}
