package pluginaichat

import (
	"context"
	"log/slog"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/common/storage"
)

// persistentHistoryStore 基于 PersistentStorage 实现的对话历史存储（KV 回退方案）。
//
// 每个群聊/好友会话拥有独立实例（key 已含 g:/f: + id），命名空间在注入时
// 已按插件名隔离。历史以 JSON 数组整体读写，符合持久化存储的 KV 语义
// （不支持列表语义）；SQL 后端下由 newSQLHistoryStore 的行级存储替代，
// 本实现用于非 SQL 后端的回退路径。错误内部记录日志后吞掉，避免拖垮主对话流程。
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

func (h *persistentHistoryStore) Load(ctx context.Context) ([]aichat.Message, error) {
	var msgs []aichat.Message
	if ok := h.store.Get(ctx, h.key, &msgs); !ok {
		return nil, nil
	}
	return msgs, nil
}

// Append 增量追加：KV 语义下退化为读-改-写整段数组。
// 同一会话由插件层会话锁串行，无需额外加锁。
func (h *persistentHistoryStore) Append(ctx context.Context, messages []aichat.Message) error {
	if len(messages) == 0 {
		return nil
	}
	var msgs []aichat.Message
	h.store.Get(ctx, h.key, &msgs)
	msgs = append(msgs, messages...)
	if ok := h.store.Set(ctx, h.key, msgs); !ok {
		h.logger.Error("追加对话历史失败", "key", h.key)
	}
	return nil
}

func (h *persistentHistoryStore) Replace(ctx context.Context, messages []aichat.Message) error {
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
