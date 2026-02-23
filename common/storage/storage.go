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
}
