package core

import (
	"context"
	"fmt"
	"log/slog"
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

// newPersistentStorage 根据配置创建持久化存储（重启不丢失）。
// 默认驱动为 sqlite；可配置为 mysql。均使用纯 Go 驱动，无需 CGO。
func newPersistentStorage(ctx context.Context, cfg *viper.Viper, logger *slog.Logger) (storage.PersistentStorage, error) {
	driver := strings.ToLower(cfg.GetString("bot.store.persistent.driver"))
	if driver == "" {
		driver = "sqlite"
	}
	switch driver {
	case "sqlite":
		path := cfg.GetString("bot.store.persistent.sqlite.path")
		if path == "" {
			path = "./data/aniabot.db"
		}
		logger.Info("使用 SQLite 持久化存储引擎", "path", path)
		return NewAniaSqliteStorage(ctx, path, logger)
	case "mysql":
		dsn := cfg.GetString("bot.store.persistent.mysql.dsn")
		logger.Info("使用 MySQL 持久化存储引擎")
		return NewAniaMysqlStorage(ctx, dsn, MysqlPoolConfig{
			MaxOpenConns:       cfg.GetInt("bot.store.persistent.mysql.max_open_conns"),
			MaxIdleConns:       cfg.GetInt("bot.store.persistent.mysql.max_idle_conns"),
			ConnMaxLifetimeSec: cfg.GetInt("bot.store.persistent.mysql.conn_max_lifetime_sec"),
		}, logger)
	default:
		return nil, fmt.Errorf("未知的持久化存储驱动: %s（可选：sqlite / mysql）", driver)
	}
}
