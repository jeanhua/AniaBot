package pluginaichat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

// memoryEntry 一条长期记忆。
//
// 与会话内上下文（messageWindow）不同，长期记忆跨会话、跨重启保留，
// 由 AI 通过 memory_save / memory_search / memory_forget 工具自行管理。
// 记忆按会话 scope（g:群号 / f:QQ号）隔离，群与群、群与私聊之间互不可见，
// 避免跨会话信息泄露。
type memoryEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"` // 关联的群成员 QQ 号；空表示属于整个会话（群规、共同约定等）
	Content   string    `json:"content"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// ErrMemoryFull 单会话记忆条数达到上限时返回，提示 AI 先清理或合并旧记忆。
var ErrMemoryFull = errors.New("记忆条数已达上限")

// memoryManager 长期记忆管理器：按会话 scope 存取记忆条目。
//
// 每个 scope 的记忆是一个 JSON 数组整体读写（PersistentStorage 的 KV 语义，
// 单会话记忆量级在百级，全量读写开销可忽略）。所有变更在 mu 保护下串行落盘；
// 存储错误内部记录日志，不拖垮主对话流程（与 HistoryStore 风格一致）。
type memoryManager struct {
	store      storage.PersistentStorage
	logger     *slog.Logger
	maxEntries int // 单 scope 记忆条数上限，<=0 表示不限制

	mu sync.Mutex
}

func newMemoryManager(store storage.PersistentStorage, logger *slog.Logger, maxEntries int) *memoryManager {
	return &memoryManager{
		store:      store.Clone("memory:"),
		logger:     logger,
		maxEntries: maxEntries,
	}
}

// normalizeMemoryContent 规范化记忆内容用于去重比较。
func normalizeMemoryContent(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// list 读取指定 scope 的全部记忆；无记录或读取失败时返回 nil。
func (m *memoryManager) list(scope string) []memoryEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listLocked(scope)
}

func (m *memoryManager) listLocked(scope string) []memoryEntry {
	var entries []memoryEntry
	if ok := m.store.Get(context.Background(), scope, &entries); !ok {
		return nil
	}
	return entries
}

// add 追加一条记忆，返回写入后的条目（含生成的 ID）。
// 内容与已有记忆重复（规范化后相同）时不重复写入，返回已有条目；
// 达到 maxEntries 上限时返回 ErrMemoryFull。
func (m *memoryManager) add(scope, userID, content string, tags []string) (memoryEntry, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return memoryEntry{}, errors.New("记忆内容不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.listLocked(scope)
	norm := normalizeMemoryContent(content)
	for _, e := range entries {
		if normalizeMemoryContent(e.Content) == norm {
			// 已存在相同记忆，不重复写入
			return e, nil
		}
	}
	if m.maxEntries > 0 && len(entries) >= m.maxEntries {
		return memoryEntry{}, fmt.Errorf("%w（%d 条），请先调用 memory_forget 删除或合并旧记忆", ErrMemoryFull, m.maxEntries)
	}

	entry := memoryEntry{
		ID:        newMemoryID(),
		UserID:    strings.TrimSpace(userID),
		Content:   content,
		Tags:      tags,
		CreatedAt: time.Now(),
	}
	entries = append(entries, entry)
	if ok := m.store.Set(context.Background(), scope, entries); !ok {
		m.logger.Error("保存记忆失败", "scope", scope)
		return memoryEntry{}, errors.New("记忆保存失败，请查看日志")
	}
	return entry, nil
}

// remove 按 ID 删除指定 scope 中的一条记忆；ID 不存在时返回 false。
func (m *memoryManager) remove(scope, id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.listLocked(scope)
	for i, e := range entries {
		if e.ID == id {
			entries = append(entries[:i], entries[i+1:]...)
			if ok := m.store.Set(context.Background(), scope, entries); !ok {
				m.logger.Error("删除记忆后落盘失败", "scope", scope, "id", id)
			}
			return true
		}
	}
	return false
}

// newMemoryID 生成短随机 ID（8 位十六进制）。
// 单 scope 条数有限（百级），随机碰撞概率可忽略；即便碰撞也仅表现为
// memory_forget 误删同 ID 的另一条，影响可控。
func newMemoryID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 加密随机数不可用时退化为时间戳，仍然可用
		return fmt.Sprintf("%08x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b[:])
}
