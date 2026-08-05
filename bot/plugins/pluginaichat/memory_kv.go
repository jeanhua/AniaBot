package pluginaichat

import (
	"context"
	"slices"

	"github.com/jeanhua/AniaBot/common/storage"
)

// kvMemoryStore KV 版记忆存储（回退方案）：每个 scope 一个 JSON 数组整体读写，
// 符合 PersistentStorage 的 KV 语义；单 scope 记忆量级在百级，全量读写可忽略。
type kvMemoryStore struct {
	store storage.PersistentStorage
}

func newKVMemoryStore(store storage.PersistentStorage) *kvMemoryStore {
	return &kvMemoryStore{
		// 再 Clone 一层 memory: 子空间，避免与插件其它持久化数据混淆
		store: store.Clone("memory:"),
	}
}

func (s *kvMemoryStore) list(scope string) []memoryEntry {
	var entries []memoryEntry
	if ok := s.store.Get(context.Background(), scope, &entries); !ok {
		return nil
	}
	return entries
}

func (s *kvMemoryStore) insert(scope string, e memoryEntry) bool {
	entries := append(s.list(scope), e)
	return s.store.Set(context.Background(), scope, entries)
}

func (s *kvMemoryStore) update(scope string, e memoryEntry) bool {
	entries := s.list(scope)
	for i := range entries {
		if entries[i].ID == e.ID {
			entries[i] = e
			return s.store.Set(context.Background(), scope, entries)
		}
	}
	return false
}

func (s *kvMemoryStore) remove(scope, id string) bool {
	entries := s.list(scope)
	for i := range entries {
		if entries[i].ID == id {
			entries = append(entries[:i], entries[i+1:]...)
			s.store.Set(context.Background(), scope, entries)
			return true
		}
	}
	return false
}

func (s *kvMemoryStore) scopes() []string {
	keys, err := s.store.Keys(context.Background(), "")
	if err != nil {
		return nil
	}
	slices.Sort(keys)
	return keys
}
