package aichat

import "context"

// HistoryStore 对话历史的持久化存储抽象。
//
// messageWindow 仅持有内存中的消息，重启后丢失；通过注入 HistoryStore
// 可在每次变更后落盘、启动时回放，从而让按群聊/好友独立的对话跨重启延续。
//
// 实现需保证并发安全（同一会话已由调用方加锁，但存储后端本身可能被
// 多个会话共享，如 SQLite 单连接）。错误应内部记录日志后返回，
// 与 common/storage 的风格保持一致，避免拖垮主对话流程。
type HistoryStore interface {
	// Load 读取已保存的历史消息；无记录时返回 nil, nil。
	Load(ctx context.Context) ([]Message, error)
	// Save 覆盖写入当前完整历史。
	Save(ctx context.Context, messages []Message) error
	// Clear 清除历史（删除对应键）。
	Clear(ctx context.Context) error
}
