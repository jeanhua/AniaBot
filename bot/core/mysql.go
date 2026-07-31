package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/go-sql-driver/mysql" // 纯 Go MySQL 驱动
	"github.com/jeanhua/AniaBot/common/storage"
)

// MysqlPoolConfig MySQL 连接池配置。
type MysqlPoolConfig struct {
	MaxOpenConns       int // 最大打开连接数
	MaxIdleConns       int // 最大空闲连接数
	ConnMaxLifetimeSec int // 连接最长存活时间（秒）
}

// NewAniaMysqlStorage 创建一个基于 MySQL 的持久化存储实例。
// dsn 为标准 go-sql-driver/mysql DSN，如
// "user:password@tcp(localhost:3306)/aniabot?charset=utf8mb4&parseTime=true&loc=Local"。
// 使用纯 Go 驱动，无需 CGO。
func NewAniaMysqlStorage(ctx context.Context, dsn string, pool MysqlPoolConfig, logger *slog.Logger) (storage.PersistentStorage, error) {
	if dsn == "" {
		return nil, fmt.Errorf("mysql dsn 不能为空")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	// 未配置（<=0）时应用合理默认值，避免连接池无上限耗尽 MySQL 连接
	maxOpen := pool.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := pool.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen // 空闲连接数不应超过最大连接数
	}
	connMaxLifetime := pool.ConnMaxLifetimeSec
	if connMaxLifetime <= 0 {
		connMaxLifetime = 1800 // 30 分钟
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetMaxIdleConns(maxIdle)
	db.SetConnMaxLifetime(time.Duration(connMaxLifetime) * time.Second)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	store, err := newSqlPersistentStorage(ctx, db, mysqlDialect, logger)
	if err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}
