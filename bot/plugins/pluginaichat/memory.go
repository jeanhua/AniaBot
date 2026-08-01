package pluginaichat

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/tasklog"
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
	// Emb 内容的语义向量（与知识库共用 embedding 服务）；计算失败或服务未
	// 启用时为 nil。旧数据无此字段，检索时跳过语义加分，兼容性良好。
	Emb []float32 `json:"emb,omitempty"`
}

// ErrMemoryFull 单会话记忆条数达到上限时返回，提示 AI 先清理或合并旧记忆。
var ErrMemoryFull = errors.New("记忆条数已达上限")

// MaxContentRunes 单条记忆内容的符文数上限，超出部分截断。
// 每个 scope 的记忆是一个 key 存整个 JSON 数组，单条长度不设限会把 key 撑大。
const MaxContentRunes = 2000

// memoryManager 长期记忆管理器：按会话 scope 存取记忆条目。
//
// 每个 scope 的记忆是一个 JSON 数组整体读写（PersistentStorage 的 KV 语义，
// 单会话记忆量级在百级，全量读写开销可忽略）。所有变更在 mu 保护下串行落盘；
// 存储错误内部记录日志，不拖垮主对话流程（与 HistoryStore 风格一致）。
type memoryManager struct {
	store      storage.PersistentStorage
	logger     *slog.Logger
	maxEntries int // 单 scope 记忆条数上限，<=0 表示不限制
	// embedder 语义向量计算器：与知识库共享同一实例（复用 kb.embedding 配置）；
	// nil 时记忆检索保持纯关键词（与历史行为一致）。
	embedder *embedder

	mu sync.Mutex
}

func newMemoryManager(store storage.PersistentStorage, logger *slog.Logger, maxEntries int, embedder *embedder) *memoryManager {
	return &memoryManager{
		store:      store.Clone("memory:"),
		logger:     logger,
		maxEntries: maxEntries,
		embedder:   embedder,
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
// 达到 maxEntries 上限时返回 ErrMemoryFull；超长内容按 MaxContentRunes 截断。
func (m *memoryManager) add(scope, userID, content string, tags []string) (memoryEntry, error) {
	content = tasklog.Truncate(strings.TrimSpace(content), MaxContentRunes)
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
	// 入库时计算语义向量（记忆写入频率极低，锁内调用可接受；失败静默降级为纯关键词）
	m.embedEntry(&entry)
	entries = append(entries, entry)
	if ok := m.store.Set(context.Background(), scope, entries); !ok {
		m.logger.Error("保存记忆失败", "scope", scope)
		return memoryEntry{}, errors.New("记忆保存失败，请查看日志")
	}
	return entry, nil
}

// embedEntry 计算单条记忆的语义向量；embedder 未启用（nil）或计算失败时
// 保持 nil，检索时自动跳过语义加分（纯关键词），不阻断记忆写入。
func (m *memoryManager) embedEntry(entry *memoryEntry) {
	if m.embedder == nil {
		return
	}
	if vec := m.embedder.EmbedOne(context.Background(), entry.Content); len(vec) > 0 {
		entry.Emb = vec
	}
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

// scopes 列出当前已有记忆的全部会话 scope（g:群号 / f:QQ号），排序后返回。
// 供 Web 面板的记忆管理页使用。
func (m *memoryManager) scopes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys, err := m.store.Keys(context.Background(), "")
	if err != nil {
		m.logger.Error("列出记忆 scope 失败", "error", err)
		return nil
	}
	slices.Sort(keys)
	return keys
}

// update 按 ID 更新指定 scope 中一条记忆的内容、关联 QQ 与标签；
// ID 不存在时返回错误。创建时间保留不变；超长内容按 MaxContentRunes 截断。
func (m *memoryManager) update(scope, id, userID, content string, tags []string) error {
	content = tasklog.Truncate(strings.TrimSpace(content), MaxContentRunes)
	if content == "" {
		return errors.New("记忆内容不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.listLocked(scope)
	for i, e := range entries {
		if e.ID == id {
			entries[i].UserID = strings.TrimSpace(userID)
			entries[i].Content = content
			entries[i].Tags = tags
			// 内容变更后语义向量需要重新计算
			m.embedEntry(&entries[i])
			if ok := m.store.Set(context.Background(), scope, entries); !ok {
				m.logger.Error("更新记忆后落盘失败", "scope", scope, "id", id)
				return errors.New("记忆保存失败，请查看日志")
			}
			return nil
		}
	}
	return fmt.Errorf("记忆不存在: %s", id)
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
