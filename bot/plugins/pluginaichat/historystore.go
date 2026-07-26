package pluginaichat

import (
	"context"
	"log/slog"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/storage"
)

// persistentHistoryStore 基于 PersistentStorage 实现的对话历史存储。
//
// 每个群聊/好友会话拥有独立实例（key 已含 g:/f: + id），命名空间在注入时
// 已按插件名隔离。历史以 JSON 数组整体读写，符合持久化存储的 KV 语义
// （不支持列表语义）。错误内部记录日志后吞掉，避免拖垮主对话流程。
type persistentHistoryStore struct {
	store  storage.PersistentStorage
	key    string
	logger *slog.Logger
}

func newPersistentHistoryStore(store storage.PersistentStorage, key string, logger *slog.Logger) *persistentHistoryStore {
	return &persistentHistoryStore{
		// 再 Clone 一层 history: 子空间，避免与插件其它持久化数据混淆
		store:  store.Clone("history:"),
		key:    key,
		logger: logger,
	}
}

// migrateLegacyHistory 将旧版不带 g:/f: 前缀的历史键迁移到新键。
// 新键已有数据或旧键不存在时不做任何事；迁移成功后删除旧键。
// 同号群聊与私聊在旧版共享同一条记录，迁移归先创建会话的一方。
func migrateLegacyHistory(root storage.PersistentStorage, legacyKey, newKey string) {
	store := root.Clone("history:")
	ctx := context.Background()
	if store.Has(ctx, newKey) || !store.Has(ctx, legacyKey) {
		return
	}
	if raw, ok := store.GetString(ctx, legacyKey); ok {
		if store.SetString(ctx, newKey, raw) {
			store.Del(ctx, legacyKey)
		}
	}
}

func (h *persistentHistoryStore) Load(ctx context.Context) ([]aichat.Message, error) {
	var msgs []aichat.Message
	if ok := h.store.Get(ctx, h.key, &msgs); !ok {
		return nil, nil
	}
	return msgs, nil
}

func (h *persistentHistoryStore) Save(ctx context.Context, messages []aichat.Message) error {
	if ok := h.store.Set(ctx, h.key, messages); !ok {
		h.logger.Error("保存对话历史失败", "key", h.key)
	}
	return nil
}

func (h *persistentHistoryStore) Clear(ctx context.Context) error {
	if ok := h.store.Del(ctx, h.key); !ok {
		h.logger.Error("清除对话历史失败", "key", h.key)
	}
	return nil
}
