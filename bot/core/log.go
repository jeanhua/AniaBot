package core

import (
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/consollog"
	"github.com/lmittmann/tint"
)

var inlogger *slog.Logger

func createLogger() *slog.Logger {
	inner := tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	})
	// 日志先写入控制台环形缓冲（供 Web 面板「控制台日志」页查看），
	// 再透传给 tint 输出到 stderr，原控制台行为不变。
	inlogger = slog.New(consollog.Attach(inner))
	// 标准库 log（适配器 / 工具类代码）同样捕获：仍输出到 stderr，
	// 同时按行写入环形缓冲。
	log.SetOutput(io.MultiWriter(os.Stderr, consollog.Writer()))
	return inlogger
}

func Logger() *slog.Logger {
	if inlogger == nil {
		inlogger = createLogger()
	}
	return inlogger
}
