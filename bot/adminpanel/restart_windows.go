//go:build windows

package adminpanel

import (
	"log/slog"
	"os"
	"os/exec"
)

// restartSelf 以相同参数启动新进程后退出当前进程
// （Windows 不支持 exec 语义，子进程继承控制台与标准流）。
func restartSelf(logger *slog.Logger) {
	exe, err := os.Executable()
	if err != nil {
		logger.Error("重启失败：无法获取可执行文件路径", "error", err)
		return
	}
	wd, _ := os.Getwd()
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Dir = wd
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	logger.Info("正在重启 AniaBot...")
	if err := cmd.Start(); err != nil {
		logger.Error("重启失败", "error", err)
		return
	}
	os.Exit(0)
}
