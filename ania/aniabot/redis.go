package aniabot

import (
	"context"
	"encoding/json"

	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/redis/go-redis/v9"
)

type AniaRedisStorage struct {
	prefix string
	rdb    *redis.Client
}

func NewAniaRedisStorage(ctx context.Context, addr, passwd string, db int) *AniaRedisStorage {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: passwd,
		DB:       db,
	})
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	return &AniaRedisStorage{
		rdb: rdb,
	}
}

func (store *AniaRedisStorage) Clone(prefix string) storage.Storage {
	return &AniaRedisStorage{
		prefix: store.prefix + prefix + ":",
		rdb:    store.rdb,
	}
}

func (store *AniaRedisStorage) GetString(ctx context.Context, key string) (string, bool) {
	resu, err := store.rdb.Get(ctx, store.prefix+key).Result()
	if err != nil {
		return "", false
	}
	return resu, true
}

func (store *AniaRedisStorage) SetString(ctx context.Context, key, val string, option ...storage.Option) bool {
	cfg := storage.StorageConfig{}
	for _, f := range option {
		f(&cfg)
	}
	if _, err := store.rdb.Set(ctx, store.prefix+key, val, cfg.TTL).Result(); err != nil {
		return false
	} else {
		return true
	}
}

func (store *AniaRedisStorage) Get(ctx context.Context, key string, out any) bool {
	resu, err := store.rdb.Get(ctx, store.prefix+key).Result()
	if err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(resu), out); err != nil {
		return false
	}
	return true
}

func (store *AniaRedisStorage) Set(ctx context.Context, key string, val any, option ...storage.Option) bool {
	cfg := storage.StorageConfig{}
	for _, f := range option {
		f(&cfg)
	}
	if b, err := json.Marshal(val); err != nil {
		return false
	} else {
		if _, err := store.rdb.Set(ctx, store.prefix+key, b, cfg.TTL).Result(); err != nil {
			return false
		} else {
			return true
		}
	}
}

func (store *AniaRedisStorage) Del(ctx context.Context, key string) bool {
	if _, err := store.rdb.Del(ctx, store.prefix+key).Result(); err != nil {
		return false
	}
	return true
}

func (store *AniaRedisStorage) Clear(ctx context.Context) bool {
	iter := store.rdb.Scan(ctx, 0, store.prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		store.rdb.Del(ctx, iter.Val()).Result()
	}
	if err := iter.Err(); err != nil {
		return false
	}
	return true
}
