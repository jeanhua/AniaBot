package core

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

type memItem struct {
	val      string
	expireAt time.Time
}

type memList struct {
	items    []string
	expireAt time.Time
}

func (m *memItem) isExpired() bool {
	if m.expireAt.IsZero() {
		return false
	}
	return time.Now().After(m.expireAt)
}

func (m *memList) isExpired() bool {
	if m.expireAt.IsZero() {
		return false
	}
	return time.Now().After(m.expireAt)
}

type AniaMemoryStorage struct {
	prefix string
	data   map[string]*memItem
	lists  map[string]*memList
	mu     *sync.RWMutex
	logger *slog.Logger
}

func NewAniaMemoryStorage(logger *slog.Logger) *AniaMemoryStorage {
	return &AniaMemoryStorage{
		data:   make(map[string]*memItem),
		lists:  make(map[string]*memList),
		mu:     &sync.RWMutex{},
		logger: logger,
	}
}

func (store *AniaMemoryStorage) Clone(prefix string) storage.Storage {
	return &AniaMemoryStorage{
		prefix: store.prefix + prefix + ":",
		data:   store.data,
		lists:  store.lists,
		mu:     store.mu, // 共享同一把锁，避免各 clone 持有独立锁而对共享 map 产生并发竞争
		logger: store.logger,
	}
}

func (store *AniaMemoryStorage) fullKey(key string) string {
	return store.prefix + key
}

func (store *AniaMemoryStorage) cleanExpired() {
	now := time.Now()
	for k, v := range store.data {
		if !v.expireAt.IsZero() && now.After(v.expireAt) {
			delete(store.data, k)
		}
	}
	for k, v := range store.lists {
		if !v.expireAt.IsZero() && now.After(v.expireAt) {
			delete(store.lists, k)
		}
	}
}

func (store *AniaMemoryStorage) GetString(ctx context.Context, key string) (string, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	item, ok := store.data[store.fullKey(key)]
	if !ok {
		return "", false
	}
	if item.isExpired() {
		return "", false
	}
	return item.val, true
}

func (store *AniaMemoryStorage) SetString(ctx context.Context, key, val string, option ...storage.Option) bool {
	cfg := storage.StorageConfig{}
	for _, f := range option {
		f(&cfg)
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)

	if cfg.CheckExist {
		if item, ok := store.data[fullKey]; ok && !item.isExpired() {
			return false
		}
	}

	var expireAt time.Time
	if cfg.TTL > 0 {
		expireAt = time.Now().Add(cfg.TTL)
	}

	store.data[fullKey] = &memItem{
		val:      val,
		expireAt: expireAt,
	}
	return true
}

func (store *AniaMemoryStorage) Get(ctx context.Context, key string, out any) bool {
	val, ok := store.GetString(ctx, key)
	if !ok {
		return false
	}
	if err := json.Unmarshal([]byte(val), out); err != nil {
		store.logger.Error("JSON unmarshal failed", "error", err)
		return false
	}
	return true
}

func (store *AniaMemoryStorage) Set(ctx context.Context, key string, val any, option ...storage.Option) bool {
	data, err := json.Marshal(val)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return false
	}
	return store.SetString(ctx, key, string(data), option...)
}

func (store *AniaMemoryStorage) Del(ctx context.Context, key string) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.data, store.fullKey(key))
	delete(store.lists, store.fullKey(key))
	return true
}

func (store *AniaMemoryStorage) Clear(ctx context.Context) bool {
	store.mu.Lock()
	defer store.mu.Unlock()
	prefix := store.prefix
	for k := range store.data {
		if strings.HasPrefix(k, prefix) {
			delete(store.data, k)
		}
	}
	for k := range store.lists {
		if strings.HasPrefix(k, prefix) {
			delete(store.lists, k)
		}
	}
	return true
}

func (store *AniaMemoryStorage) ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error) {
	// cleanExpired 会删除共享 map 中的过期键，必须持写锁：
	// RLock 下写 map 会触发 runtime 级 fatal（recover 无法捕获）
	store.mu.Lock()
	defer store.mu.Unlock()

	store.cleanExpired()

	var keys []string
	fullPattern := store.prefix + pattern

	for k := range store.data {
		if matched, _ := matchPattern(k, fullPattern); matched {
			keys = append(keys, strings.TrimPrefix(k, store.prefix))
		}
		if int64(len(keys)) >= count && count > 0 {
			break
		}
	}

	for k := range store.lists {
		if matched, _ := matchPattern(k, fullPattern); matched {
			trimmed := strings.TrimPrefix(k, store.prefix)
			found := false
			for _, existing := range keys {
				if existing == trimmed {
					found = true
					break
				}
			}
			if !found {
				keys = append(keys, trimmed)
			}
		}
		if int64(len(keys)) >= count && count > 0 {
			break
		}
	}

	return keys, nil
}

func (store *AniaMemoryStorage) LPush(ctx context.Context, key string, values ...any) int64 {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || list.isExpired() {
		list = &memList{}
		store.lists[fullKey] = list
	}

	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			store.logger.Error("JSON marshal failed", "error", err)
			continue
		}
		list.items = append([]string{string(data)}, list.items...)
	}

	return int64(len(list.items))
}

func (store *AniaMemoryStorage) RPush(ctx context.Context, key string, values ...any) int64 {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || list.isExpired() {
		list = &memList{}
		store.lists[fullKey] = list
	}

	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			store.logger.Error("JSON marshal failed", "error", err)
			continue
		}
		list.items = append(list.items, string(data))
	}

	return int64(len(list.items))
}

func (store *AniaMemoryStorage) LPop(ctx context.Context, key string) (any, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return nil, false
	}

	val := list.items[0]
	list.items = list.items[1:]

	var out any
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaMemoryStorage) RPop(ctx context.Context, key string) (any, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return nil, false
	}

	val := list.items[len(list.items)-1]
	list.items = list.items[:len(list.items)-1]

	var out any
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaMemoryStorage) LRange(ctx context.Context, key string, start, stop int64) ([]any, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return nil, false
	}

	length := int64(len(list.items))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		return nil, true
	}

	var results []any
	for i := start; i <= stop; i++ {
		var out any
		if err := json.Unmarshal([]byte(list.items[i]), &out); err != nil {
			results = append(results, list.items[i])
		} else {
			results = append(results, out)
		}
	}
	return results, true
}

func (store *AniaMemoryStorage) LLen(ctx context.Context, key string) int64 {
	store.mu.RLock()
	defer store.mu.RUnlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || list.isExpired() {
		return 0
	}
	return int64(len(list.items))
}

func (store *AniaMemoryStorage) LRem(ctx context.Context, key string, count int64, value any) int64 {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return 0
	}

	data, err := json.Marshal(value)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return 0
	}
	target := string(data)

	var removed int64
	if count == 0 {
		newItems := make([]string, 0, len(list.items))
		for _, item := range list.items {
			if item == target {
				removed++
			} else {
				newItems = append(newItems, item)
			}
		}
		list.items = newItems
	} else if count > 0 {
		newItems := make([]string, 0, len(list.items))
		for _, item := range list.items {
			if item == target && removed < count {
				removed++
			} else {
				newItems = append(newItems, item)
			}
		}
		list.items = newItems
	} else {
		count = -count
		newItems := make([]string, 0, len(list.items))
		for i := len(list.items) - 1; i >= 0; i-- {
			if list.items[i] == target && removed < count {
				removed++
			} else {
				newItems = append(newItems, list.items[i])
			}
		}
		for i, j := 0, len(newItems)-1; i < j; i, j = i+1, j-1 {
			newItems[i], newItems[j] = newItems[j], newItems[i]
		}
		list.items = newItems
	}

	return removed
}

func (store *AniaMemoryStorage) LSet(ctx context.Context, key string, index int64, value any) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return false
	}

	length := int64(len(list.items))
	if index < 0 || index >= length {
		return false
	}

	data, err := json.Marshal(value)
	if err != nil {
		store.logger.Error("JSON marshal failed", "error", err)
		return false
	}

	list.items[index] = string(data)
	return true
}

func (store *AniaMemoryStorage) LIndex(ctx context.Context, key string, index int64) (any, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return nil, false
	}

	length := int64(len(list.items))
	if index < 0 || index >= length {
		return nil, false
	}

	var out any
	if err := json.Unmarshal([]byte(list.items[index]), &out); err != nil {
		return nil, false
	}
	return out, true
}

func (store *AniaMemoryStorage) LTrim(ctx context.Context, key string, start, stop int64) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	list, ok := store.lists[fullKey]
	if !ok || len(list.items) == 0 || list.isExpired() {
		return true
	}

	length := int64(len(list.items))
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}

	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop {
		list.items = nil
		return true
	}

	list.items = list.items[start : stop+1]
	return true
}

func (store *AniaMemoryStorage) Expire(ctx context.Context, key string, ttl time.Duration) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	fullKey := store.fullKey(key)
	if item, ok := store.data[fullKey]; ok {
		item.expireAt = time.Now().Add(ttl)
		return true
	}
	if list, ok := store.lists[fullKey]; ok {
		list.expireAt = time.Now().Add(ttl)
		return true
	}
	return false
}

// matchPattern 实现与 Redis 后端一致的 '*' 通配匹配语义：
// 首段锚定开头、尾段锚定结尾，中间字面段须按顺序且不重叠地出现。
func matchPattern(s, pattern string) (bool, error) {
	if pattern == "*" {
		return true, nil
	}

	if !strings.Contains(pattern, "*") {
		return s == pattern, nil
	}

	parts := strings.Split(pattern, "*")

	if !strings.HasPrefix(s, parts[0]) {
		return false, nil
	}
	s = s[len(parts[0]):]

	last := parts[len(parts)-1]
	if !strings.HasSuffix(s, last) {
		return false, nil
	}
	s = s[:len(s)-len(last)]

	for _, part := range parts[1 : len(parts)-1] {
		if part == "" {
			continue
		}
		idx := strings.Index(s, part)
		if idx < 0 {
			return false, nil
		}
		s = s[idx+len(part):]
	}
	return true, nil
}
