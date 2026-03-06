package core

import (
	"log/slog"
	"os"
)

var inlogger *slog.Logger

func createLogger() *slog.Logger {
	inlogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return inlogger
}

func Logger() *slog.Logger {
	if inlogger == nil {
		inlogger = createLogger()
	}
	return inlogger
}
