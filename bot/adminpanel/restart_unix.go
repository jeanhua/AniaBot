//go:build !windows

package adminpanel

import (
	"log/slog"
	"os"
	"syscall"
)

// restartSelf 以相同参数原地替换当前进程（Unix exec，PID 不变，句柄/控制台无缝衔接）。
func restartSelf(logger *slog.Logger) {
	exe, err := os.Executable()
	if err != nil {
		logger.Error("重启失败：无法获取可执行文件路径", "error", err)
		return
	}
	logger.Info("正在重启 AniaBot...")
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		logger.Error("重启失败", "error", err)
	}
}
