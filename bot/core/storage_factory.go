package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/spf13/viper"
)

// newCacheStorage 根据配置创建缓存存储（易失，支持 TTL/列表）。
// 默认驱动为 redis；可配置为 memory 以脱离 Redis 运行。
func newCacheStorage(ctx context.Context, cfg *viper.Viper, logger *slog.Logger) (storage.Storage, error) {
	driver := strings.ToLower(cfg.GetString("bot.store.cache.driver"))
	if driver == "" {
		driver = "redis"
	}
	switch driver {
	case "memory", "mem":
		logger.Info("使用内存缓存存储引擎")
		return NewAniaMemoryStorage(logger), nil
	case "redis":
		addr := cfg.GetString("bot.store.cache.redis.address")
		if addr == "" {
			addr = "localhost:6379"
		}
		logger.Info("使用 Redis 缓存存储引擎", "address", addr)
		return NewAniaRedisStorage(ctx, addr,
			cfg.GetString("bot.store.cache.redis.password"),
			cfg.GetInt("bot.store.cache.redis.db"),
			logger)
	default:
		return nil, fmt.Errorf("未知的缓存存储驱动: %s（可选：redis / memory）", driver)
	}
}

// newPersistentStorage 创建持久化存储（重启不丢失）。
//
// 持久化存储是配置中心的载体，必须在读取任何配置之前打开，因此其驱动与
// 位置通过环境变量引导（不经过配置中心）：
//   - ANIABOT_STORE_DRIVER: sqlite（默认） | mysql
//   - ANIABOT_SQLITE_PATH: sqlite 文件路径，默认 ./data/aniabot.db
//   - ANIABOT_MYSQL_DSN:   mysql 标准 go-sql-driver DSN
//
// 均使用纯 Go 驱动，无需 CGO。
func newPersistentStorage(ctx context.Context, logger *slog.Logger) (storage.PersistentStorage, error) {
	driver := strings.ToLower(os.Getenv("ANIABOT_STORE_DRIVER"))
	if driver == "" {
		driver = "sqlite"
	}
	switch driver {
	case "sqlite":
		path := os.Getenv("ANIABOT_SQLITE_PATH")
		if path == "" {
			path = "./data/aniabot.db"
		}
		logger.Info("使用 SQLite 持久化存储引擎", "path", path)
		return NewAniaSqliteStorage(ctx, path, logger)
	case "mysql":
		dsn := os.Getenv("ANIABOT_MYSQL_DSN")
		if dsn == "" {
			return nil, fmt.Errorf("使用 MySQL 持久化存储需要设置环境变量 ANIABOT_MYSQL_DSN")
		}
		logger.Info("使用 MySQL 持久化存储引擎")
		return NewAniaMysqlStorage(ctx, dsn, MysqlPoolConfig{}, logger)
	default:
		return nil, fmt.Errorf("未知的持久化存储驱动: %s（可选：sqlite / mysql）", driver)
	}
}
