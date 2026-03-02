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

func WithTTL(ttl time.Duration) Option {
	return func(sc *StorageConfig) {
		sc.TTL = ttl
	}
}

func WithCheckExist() Option {
	return func(sc *StorageConfig) {
		sc.CheckExist = true
	}
}

type Storage interface {
	GetString(ctx context.Context, key string) (string, bool)
	SetString(ctx context.Context, key, val string, option ...Option) bool

	Get(ctx context.Context, key string, out any) bool
	Set(ctx context.Context, key string, val any, option ...Option) bool

	ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error)

	Del(ctx context.Context, key string) bool
	Clear(ctx context.Context) bool

	Clone(prefix string) Storage

	LPush(ctx context.Context, key string, values ...any) int64
	RPush(ctx context.Context, key string, values ...any) int64
	LPop(ctx context.Context, key string) (any, bool)
	RPop(ctx context.Context, key string) (any, bool)
	LRange(ctx context.Context, key string, start, stop int64) ([]any, bool)
	LLen(ctx context.Context, key string) int64
	LRem(ctx context.Context, key string, count int64, value any) int64
	LSet(ctx context.Context, key string, index int64, value any) bool
	LIndex(ctx context.Context, key string, index int64) (any, bool)
	LTrim(ctx context.Context, key string, start, stop int64) bool
}
