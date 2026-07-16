package logger

import (
	"log/slog"
	"os"
)

// Init настраивает глобальный логгер slog
func Init(isProd bool) {
	var handler slog.Handler

	if isProd {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	logger := slog.New(handler)

	slog.SetDefault(logger)
}
