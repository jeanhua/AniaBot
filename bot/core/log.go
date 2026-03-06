package core

import (
	"io"
	"log/slog"
	"os"
)

var logger *slog.Logger

func createLogger(level slog.Level, writer io.Writer) *slog.Logger {
	logger = slog.New(slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	}))
	return logger
}

func Logger() *slog.Logger {
	if logger == nil {
		logger = createLogger(slog.LevelDebug, os.Stderr)
	}
	return logger
}
