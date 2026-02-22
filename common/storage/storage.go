package storage

import (
	"context"
	"time"
)

type Option func(*StorageConfig)

type StorageConfig struct {
	TTL time.Duration
}

func WithTTL(ttl time.Duration) Option {
	return func(sc *StorageConfig) {
		sc.TTL = ttl
	}
}

type Storage interface {
	GetString(ctx context.Context, key string) (string, bool)
	SetString(ctx context.Context, key, val string, option ...Option) bool

	Get(ctx context.Context, key string, out any) bool
	Set(ctx context.Context, key string, val any, option ...Option) bool

	Del(ctx context.Context, key string) bool
	Clear(ctx context.Context) bool

	Clone(prefix string) Storage
}
