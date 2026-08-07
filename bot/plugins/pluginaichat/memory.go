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
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/common/storage"
)

// memoryEntry 一条长期记忆。
//
// 与会话内上下文（messageWindow）不同，长期记忆跨会话、跨重启保留，
// 由 AI 通过 memory_save / memory_search / memory_forget 工具自行管理。
// 记忆按会话 scope（g:会话ID / f:用户ID）隔离，群与群、群与私聊之间互不可见，
// 避免跨会话信息泄露。
type memoryEntry struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id,omitempty"` // 关联的用户 ID；空表示属于整个会话（群规、共同约定等）
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

// memoryInjectMaxRunes 主动注入块的字符数上限：注入内容追加在消息尾部，
// 超限会白白占用上下文，从分数最低的条目开始截断。
const memoryInjectMaxRunes = 1500

// memoryStore 记忆存储后端：KV 整段读写（回退）或 SQL 逐行（ania_memory 表）。
// 去重、上限、截断与语义向量计算等逻辑留在 memoryManager 层，后端只做存取。
type memoryStore interface {
	// list 读取指定 scope 的全部记忆（按创建时间升序）；无记录或失败时返回 nil。
	list(scope string) []memoryEntry
	// insert 追加一条记忆（调用方已完成去重与上限检查）。
	insert(scope string, e memoryEntry) bool
	// update 按 ID 覆盖一条记忆的可变字段；ID 不存在时返回 false。
	update(scope string, e memoryEntry) bool
	// remove 按 ID 删除一条记忆；ID 不存在时返回 false。
	remove(scope, id string) bool
	// scopes 列出已有记忆的全部 scope（排序后返回）。
	scopes() []string
}

// memoryManager 长期记忆管理器：按会话 scope 存取记忆条目。
//
// SQL 后端下每条记忆一行（ania_memory 表），非 SQL 后端回退为每 scope 一个
// JSON 数组整体读写（kvMemoryStore，单会话记忆量级在百级，全量读写开销可忽略）。
// 所有变更在 mu 保护下串行落盘；存储错误内部记录日志，不拖垮主对话流程
// （与 HistoryStore 风格一致）。
type memoryManager struct {
	store      memoryStore
	logger     *slog.Logger
	maxEntries int // 单 scope 记忆条数上限，<=0 表示不限制
	// embedder 语义向量计算器：与知识库共享同一实例（复用 kb.embedding 配置）；
	// nil 时记忆检索保持纯关键词（与历史行为一致）。
	embedder *embedder

	mu sync.Mutex
}

func newMemoryManager(store storage.PersistentStorage, logger *slog.Logger, maxEntries int, embedder *embedder) *memoryManager {
	m := &memoryManager{
		logger:     logger,
		maxEntries: maxEntries,
		embedder:   embedder,
	}
	// SQL 后端走行级存储（ania_memory 表）；探测或建表失败回退 KV 整段 JSON
	if db, dialect, ok := storage.SQLBackend(store); ok {
		if err := storage.EnsureTables(context.Background(), db, dialect, memoryTables...); err != nil {
			logger.Error("创建长期记忆表失败，回退 KV 存储", "error", err.Error())
		} else {
			m.store = newSQLMemoryStore(db, logger)
			return m
		}
	}
	m.store = newKVMemoryStore(store)
	return m
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
	return m.store.list(scope)
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
		CreatedAt: time.Now().UTC(),
	}
	// 入库时计算语义向量（记忆写入频率极低，锁内调用可接受；失败静默降级为纯关键词）
	m.embedEntry(&entry)
	if ok := m.store.insert(scope, entry); !ok {
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
	return m.store.remove(scope, id)
}

// scopes 列出当前已有记忆的全部会话 scope（g:会话ID / f:用户ID），排序后返回。
// 供 Web 面板的记忆管理页使用。
func (m *memoryManager) scopes() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.store.scopes()
}

// update 按 ID 更新指定 scope 中一条记忆的内容、关联用户 ID 与标签；
// ID 不存在时返回错误。创建时间保留不变；超长内容按 MaxContentRunes 截断。
func (m *memoryManager) update(scope, id, userID, content string, tags []string) error {
	content = tasklog.Truncate(strings.TrimSpace(content), MaxContentRunes)
	if content == "" {
		return errors.New("记忆内容不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entries := m.listLocked(scope)
	for _, e := range entries {
		if e.ID == id {
			e.UserID = strings.TrimSpace(userID)
			e.Content = content
			e.Tags = tags
			// 内容变更后语义向量需要重新计算
			m.embedEntry(&e)
			if ok := m.store.update(scope, e); !ok {
				m.logger.Error("更新记忆后落盘失败", "scope", scope, "id", id)
				return errors.New("记忆保存失败，请查看日志")
			}
			return nil
		}
	}
	return fmt.Errorf("记忆不存在: %s", id)
}

// autoInject 对用户消息做纯关键词检索，把相关记忆拼成一段「【长期记忆】…」
// 上下文块返回，供调用方注入到用户消息前（尾部注入：system 保持不变，
// 不影响上游前缀缓存；用户消息不落盘，注入内容不会污染持久化历史）。
//
// 与知识库自动注入同策略：只走关键词（复用 queryTerms 的中文切词），
// 每轮对话都触发，不承担 embedding API 成本；语义检索仍由 AI 按需
// 调 memory_search 完成。无命中返回空串。
func (m *memoryManager) autoInject(scope, userMsg string, max int) string {
	if strings.TrimSpace(userMsg) == "" {
		return ""
	}
	if max <= 0 {
		max = 3
	}
	entries := m.list(scope)
	if len(entries) == 0 {
		return ""
	}
	matched := filterMemoryByRelevance(entries, queryTerms(userMsg), nil)
	if len(matched) == 0 {
		return ""
	}
	if len(matched) > max {
		matched = matched[:max]
	}

	var sb strings.Builder
	sb.WriteString("【长期记忆】以下记忆可能与当前话题相关，可参考（与话题无关可忽略）：\n")
	budget := memoryInjectMaxRunes
	for _, e := range matched {
		if budget <= 0 {
			break
		}
		line := tasklog.Truncate(formatMemoryLine(e), budget)
		sb.WriteString(line)
		sb.WriteString("\n")
		budget -= utf8.RuneCountInString(line) + 1
	}
	return strings.TrimRight(sb.String(), "\n")
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
