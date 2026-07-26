//go:build !windows

package adminpanel

import (
	"log/slog"
	"os"
	"syscall"
)

// restartSelf 以相同参数原地替换当前进程（Unix exec，PID 不变，句柄/控制台无缝衔接）。
// 使用启动时缓存的 selfExe：Linux 的 os.Executable() 读 /proc/self/exe（跟随 inode），
// 更新把二进制 rename 为 <exe>.old 后再取会指向旧版本，导致 exec 回旧二进制。
func restartSelf(logger *slog.Logger) {
	exe := selfExe
	if exe == "" {
		logger.Error("重启失败：无法获取可执行文件路径")
		return
	}
	logger.Info("正在重启 AniaBot...")
	if err := syscall.Exec(exe, os.Args, os.Environ()); err != nil {
		logger.Error("重启失败", "error", err)
	}
}
