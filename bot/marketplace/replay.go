package marketplace

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jeanhua/AniaBot/common/pluginmeta"
)

// ApplyInstalled 把持久插件目录（pluginDir）中已安装的插件源码同步到源码树
// 的 custom/plugins/ 下，并重新生成注册代码（cmd/marketplace_plugins.go）。
//
// 供自动更新流水线在 git reset --hard 之后、go mod tidy 之前调用：
// 更新会清空源码树中的 custom/plugins 与生成的注册文件，这里负责重放，
// 保证容器重建 / 自动更新后市场插件不丢失。
//
// 插件目录不存在时视为无插件，仍会重置注册代码为空。单个插件同步失败只告警
// 不阻断（该插件不会进入新二进制，其余插件不受影响）。
func ApplyInstalled(ctx context.Context, srcDir, pluginDir string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	// 1. 收集插件目录（以磁盘为准，manifest 仅用于展示）
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("插件市场：无已安装插件，跳过重放")
		} else {
			logger.Warn("插件市场：读取插件目录失败", "dir", pluginDir, "error", err)
		}
		return resetRegistry(ctx, srcDir, logger)
	}
	installed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifestPath := filepath.Join(pluginDir, e.Name(), "plugin.json")
		m, err := pluginmeta.LoadManifest(manifestPath)
		if err != nil {
			logger.Warn("插件市场：跳过无效插件", "id", e.Name(), "error", err)
			continue
		}
		dst := filepath.Join(srcDir, pluginmeta.PluginRoot, m.ID)
		if err := replaceDir(filepath.Join(pluginDir, m.ID), dst); err != nil {
			logger.Warn("插件市场：同步插件到源码树失败，该插件不会进入新版本", "id", m.ID, "error", err)
			continue
		}
		logger.Info("插件市场：已重放插件", "id", m.ID, "version", m.Version)
		installed++
	}
	if installed == 0 {
		logger.Info("插件市场：无可重放插件")
	}
	return resetRegistry(ctx, srcDir, logger)
}

// resetRegistry 运行 plugingen 重新生成注册代码（无插件时重置为空文件）。
func resetRegistry(ctx context.Context, srcDir string, logger *slog.Logger) error {
	cmd := exec.CommandContext(ctx, "go", "run", "./tools/plugingen")
	cmd.Dir = srcDir
	if out, err := cmd.CombinedOutput(); err != nil {
		logger.Warn("插件市场：生成注册代码失败", "error", err, "output", string(out))
		return fmt.Errorf("生成插件注册代码失败: %w", err)
	}
	return nil
}
