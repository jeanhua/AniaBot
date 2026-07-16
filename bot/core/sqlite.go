package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jeanhua/AniaBot/common/storage"
	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动，无 CGO，便于跨平台交叉编译
)

// NewAniaSqliteStorage 创建一个基于 SQLite 的持久化存储实例。
// path 为数据库文件路径（如 "./data/aniabot.db"）；传入 ":memory:" 使用内存库（主要用于测试）。
// 使用纯 Go 驱动 modernc.org/sqlite，无需 CGO，交叉编译友好。
func NewAniaSqliteStorage(ctx context.Context, path string, logger *slog.Logger) (storage.PersistentStorage, error) {
	if path == "" {
		path = "./data/aniabot.db"
	}
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create sqlite dir %q: %w", dir, err)
			}
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	// SQLite 单连接：避免多连接写冲突导致的 "database is locked"
	db.SetMaxOpenConns(1)
	// WAL 提升并发读与崩溃恢复能力；busy_timeout 在遇到锁时等待而非立即失败
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("apply sqlite pragma %q: %w", pragma, err)
		}
	}
	store, err := newSqlPersistentStorage(ctx, db, sqliteDialect, logger)
	if err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}
