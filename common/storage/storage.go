package storage

import (
	"context"
	"time"
)

type Option func(*StorageConfig)

type StorageConfig struct {
	TTL        time.Duration
	CheckExist bool
}

// WithTTL 设置键的过期时间
func WithTTL(ttl time.Duration) Option {
	return func(sc *StorageConfig) {
		sc.TTL = ttl
	}
}

// WithCheckExist 设置在设置值时检查键是否存在
func WithCheckExist() Option {
	return func(sc *StorageConfig) {
		sc.CheckExist = true
	}
}

type Storage interface {
	// GetString 获取字符串值
	GetString(ctx context.Context, key string) (string, bool)
	// SetString 设置字符串值
	SetString(ctx context.Context, key, val string, option ...Option) bool

	// Get 获取任意类型值
	Get(ctx context.Context, key string, out any) bool
	// Set 设置任意类型值
	Set(ctx context.Context, key string, val any, option ...Option) bool

	// ScanKeys 扫描匹配模式的键
	ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)

	// Del 删除键
	Del(ctx context.Context, key string) bool
	// Clear 清空所有键
	Clear(ctx context.Context) bool

	// Clone 创建一个新的存储实例，使用指定的前缀
	Clone(prefix string) Storage

	// LPush 从左侧插入值
	LPush(ctx context.Context, key string, values ...any) int64
	// RPush 从右侧插入值
	RPush(ctx context.Context, key string, values ...any) int64
	// LPop 从左侧弹出值
	LPop(ctx context.Context, key string) (any, bool)
	// RPop 从右侧弹出值
	RPop(ctx context.Context, key string) (any, bool)
	// LRange 获取列表范围内的值
	LRange(ctx context.Context, key string, start, stop int64) ([]any, bool)
	// LLen 获取列表长度
	LLen(ctx context.Context, key string) int64
	// LRem 移除列表中的值
	LRem(ctx context.Context, key string, count int64, value any) int64
	// LSet 设置列表中指定索引的值
	LSet(ctx context.Context, key string, index int64, value any) bool
	// LIndex 获取列表中指定索引的值
	LIndex(ctx context.Context, key string, index int64) (any, bool)
	// LTrim 修剪列表，保留指定范围内的值
	LTrim(ctx context.Context, key string, start, stop int64) bool

	// Expire 设置键的过期时间
	Expire(ctx context.Context, key string, ttl time.Duration) bool
}
