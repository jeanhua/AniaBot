package core

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

var inlogger *slog.Logger

func createLogger() *slog.Logger {
	inlogger = slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	}))
	return inlogger
}

func Logger() *slog.Logger {
	if inlogger == nil {
		inlogger = createLogger()
	}
	return inlogger
}
