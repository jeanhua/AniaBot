package storage

import "context"

type Storage interface {
	GetString(ctx context.Context, key string) (string, bool)
	SetString(ctx context.Context, key, val string) bool

	Get(ctx context.Context, key string, out any) bool
	Set(ctx context.Context, key string, val any) bool

	Del(ctx context.Context, key string) bool
	Clear(ctx context.Context) bool

	Clone(prefix string) Storage
}
