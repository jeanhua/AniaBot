package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/redis/go-redis/v9"
)

type AniaRedisStorage struct {
	prefix string
	rdb    *redis.Client
	logger *slog.Logger
}

func NewAniaRedisStorage(ctx context.Context, addr, passwd string, db int, logger *slog.Logger) (*AniaRedisStorage, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: passwd,
		DB:       db,
	})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("ping redis %s: %w", addr, err)
	}
	return &AniaRedisStorage{
		rdb:    rdb,
		logger: logger,
	}, nil
}

func (store *AniaRedisStorage) Clone(prefix string) storage.Storage {
	return &AniaRedisStorage{
		prefix: store.prefix + prefix + ":",
		rdb:    store.rdb,
		logger: store.logger,
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
	fullKey := store.prefix + key
	var err error
	if cfg.CheckExist {
		setArgs := redis.SetArgs{
			Mode: "NX",
			TTL:  cfg.TTL,
		}
		err = store.rdb.SetArgs(ctx, fullKey, val, setArgs).Err()
	} else {
		err = store.rdb.Set(ctx, fullKey, val, cfg.TTL).Err()
	}

	if err != nil {
		if cfg.CheckExist && err == redis.Nil {
			return false
		}
		store.logger.Error("Redis operation failed", "key", fullKey, "error", err)
		return false
	}
	return true
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

	data, err := json.Marshal(val)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return false
	}
	return store.SetString(ctx, key, string(data), option...)
}

func (store *AniaRedisStorage) Del(ctx context.Context, key string) bool {
	if _, err := store.rdb.Del(ctx, store.prefix+key).Result(); err != nil {
		store.logger.Error("Redis del failed", "key", store.prefix+key, "error", err)
		return false
	}
	return true
}

func (store *AniaRedisStorage) Clear(ctx context.Context) bool {
	iter := store.rdb.Scan(ctx, 0, store.prefix+"*", 0).Iterator()
	for iter.Next(ctx) {
		if _, err := store.rdb.Del(ctx, iter.Val()).Result(); err != nil {
			store.logger.Error("Redis del failed", "key", iter.Val(), "error", err)
			return false
		}
	}
	if err := iter.Err(); err != nil {
		return false
	}
	return true
}

func (store *AniaRedisStorage) ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error) {
	var keys []string
	var cursor uint64 = 0
	for {
		var err error
		var scanned []string
		scanned, cursor, err = store.rdb.Scan(ctx, cursor, store.prefix+pattern, count).Result()
		if err != nil {
			return nil, err
		}
		for i := range scanned {
			scanned[i] = strings.TrimPrefix(scanned[i], store.prefix)
		}
		keys = append(keys, scanned...)
		// count 语义与内存后端一致：作为返回结果的数量上限（>0 时生效），
		// 而非仅作为 SCAN 每批的数量提示透传
		if count > 0 && int64(len(keys)) >= count {
			return keys[:int(count)], nil
		}
		if cursor == 0 {
			break
		}
	}
	return keys, nil
}

func (store *AniaRedisStorage) LPush(ctx context.Context, key string, values ...any) int64 {
	fullKey := store.prefix + key
	var args []any
	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			store.logger.Error("JSON marshal failed", "error", err)
			continue
		}
		args = append(args, string(data))
	}
	resu, err := store.rdb.LPush(ctx, fullKey, args...).Result()
	if err != nil {
		store.logger.Error("Redis LPush failed", "key", fullKey, "error", err)
		return 0
	}
	return resu
}

func (store *AniaRedisStorage) RPush(ctx context.Context, key string, values ...any) int64 {
	fullKey := store.prefix + key
	var args []any
	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			store.logger.Error("JSON marshal failed", "error", err)
			continue
		}
		args = append(args, string(data))
	}
	resu, err := store.rdb.RPush(ctx, fullKey, args...).Result()
	if err != nil {
		store.logger.Error("Redis RPush failed", "key", fullKey, "error", err)
		return 0
	}
	return resu
}

func (store *AniaRedisStorage) LPop(ctx context.Context, key string) (any, bool) {
	fullKey := store.prefix + key
	resu, err := store.rdb.LPop(ctx, fullKey).Result()
	if err != nil {
		store.logger.Error("Redis LPop failed", "key", fullKey, "error", err)
		return nil, false
	}
	var out any
	if err := json.Unmarshal([]byte(resu), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaRedisStorage) RPop(ctx context.Context, key string) (any, bool) {
	fullKey := store.prefix + key
	resu, err := store.rdb.RPop(ctx, fullKey).Result()
	if err != nil {
		store.logger.Error("Redis RPop failed", "key", fullKey, "error", err)
		return nil, false
	}
	var out any
	if err := json.Unmarshal([]byte(resu), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaRedisStorage) LRange(ctx context.Context, key string, start, stop int64) ([]any, bool) {
	fullKey := store.prefix + key
	resu, err := store.rdb.LRange(ctx, fullKey, start, stop).Result()
	if err != nil {
		store.logger.Error("Redis LRange failed", "key", fullKey, "error", err)
		return nil, false
	}
	var results []any
	for _, v := range resu {
		var out any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			store.logger.Error("JSON unmarshal failed", "error", err)
			results = append(results, v)
		} else {
			results = append(results, out)
		}
	}
	return results, true
}

func (store *AniaRedisStorage) LLen(ctx context.Context, key string) int64 {
	fullKey := store.prefix + key
	resu, err := store.rdb.LLen(ctx, fullKey).Result()
	if err != nil {
		store.logger.Error("Redis LLen failed", "key", fullKey, "error", err)
		return 0
	}
	return resu
}

func (store *AniaRedisStorage) LRem(ctx context.Context, key string, count int64, value any) int64 {
	fullKey := store.prefix + key
	data, err := json.Marshal(value)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return 0
	}
	resu, err := store.rdb.LRem(ctx, fullKey, count, string(data)).Result()
	if err != nil {
		store.logger.Error("Redis LRem failed", "key", fullKey, "error", err)
		return 0
	}
	return resu
}

func (store *AniaRedisStorage) LSet(ctx context.Context, key string, index int64, value any) bool {
	fullKey := store.prefix + key
	data, err := json.Marshal(value)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return false
	}
	err = store.rdb.LSet(ctx, fullKey, index, string(data)).Err()
	if err != nil {
		store.logger.Error("Redis LSet failed", "key", fullKey, "error", err)
		return false
	}
	return true
}

func (store *AniaRedisStorage) LIndex(ctx context.Context, key string, index int64) (any, bool) {
	fullKey := store.prefix + key
	resu, err := store.rdb.LIndex(ctx, fullKey, index).Result()
	if err != nil {
		store.logger.Error("Redis LIndex failed", "key", fullKey, "error", err)
		return nil, false
	}
	var out any
	if err := json.Unmarshal([]byte(resu), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaRedisStorage) LTrim(ctx context.Context, key string, start, stop int64) bool {
	fullKey := store.prefix + key
	_, err := store.rdb.LTrim(ctx, fullKey, start, stop).Result()
	if err != nil {
		store.logger.Error("Redis LTrim failed", "key", fullKey, "error", err)
		return false
	}
	return true
}

func (store *AniaRedisStorage) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	fullKey := store.prefix + key
	ok, err := store.rdb.Expire(ctx, fullKey, ttl).Result()
	if err != nil {
		store.logger.Error("Redis Expire failed", "key", fullKey, "error", err)
		return false
	}
	return ok
}
