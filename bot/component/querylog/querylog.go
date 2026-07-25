// Package querylog 提供 AI 查询（Query）执行日志的持久化记录能力。
//
// 一次 Query 指从用户触发 AI 回复（群里 @ 机器人或私聊）到 AI 最终响应的完整过程。
// 与 tasklog（定时任务执行日志）结构类似：日志以 JSON 数组整体读写（符合持久化存储
// 的 KV 语义），按容量上限滚动保留最近若干条。命名空间由调用方在注入时隔离，
// 本组件不再 Clone。
package querylog

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/common/storage"
)

// Status 查询执行状态
type Status string

const (
	StatusRunning Status = "running" // 已触发，执行中
	StatusSuccess Status = "success" // 执行成功
	StatusStopped Status = "stopped" // 用户主动停止（/stop）
	StatusTimeout Status = "timeout" // 请求超时
	StatusError   Status = "error"   // 执行出错
)

// 字段截断上限（符文数），避免单条日志体积失控
const (
	MaxQueryRunes  = 500  // 用户输入
	MaxReplyRunes  = 1000 // 最终回复
	MaxArgsRunes   = 500  // 工具调用参数
	MaxResultRunes = 1000 // 工具执行结果
)

// ToolCallRecord 一次工具调用的执行记录（如 bash 命令的执行详情）
type ToolCallRecord struct {
	Name       string `json:"name"`
	Arguments  string `json:"arguments,omitempty"`
	Result     string `json:"result,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Entry 一条 Query 日志
type Entry struct {
	ID               string           `json:"id"`
	Time             time.Time        `json:"time"`
	ChatType         string           `json:"chat_type"` // group / friend
	TargetID         string           `json:"target_id"` // 群号 / 好友 QQ
	Senders          []string         `json:"senders,omitempty"`
	Query            string           `json:"query"` // 合并后的用户输入（可能含排队消息）
	Status           Status           `json:"status"`
	DurationMs       int64            `json:"duration_ms"`
	Iterations       int              `json:"iterations"` // LLM 调用轮数
	ToolCalls        []ToolCallRecord `json:"tool_calls,omitempty"`
	PromptTokens     int              `json:"prompt_tokens,omitempty"`
	CompletionTokens int              `json:"completion_tokens,omitempty"`
	TotalTokens      int              `json:"total_tokens,omitempty"`
	Reply            string           `json:"reply,omitempty"` // 最终回复（截断）
	Error            string           `json:"error,omitempty"`
}

const (
	entriesKey = "entries"
	seqKey     = "seq"
	defaultMax = 200
)

// Truncate 按符文数截断字符串，超出时追加省略标记。max<=0 时不截断。
func Truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

// Logger 持久化 Query 日志记录器。
//
// 内部用互斥串行化 Record/Update，避免并发写入互相覆盖；读取（Recent）
// 每次直接从存储加载最新快照，不持有内存缓存，保证与落盘状态一致。
type Logger struct {
	store      storage.PersistentStorage
	maxEntries int
	logger     *slog.Logger
	mu         sync.Mutex
	seq        uint64
}

// New 创建日志记录器。store 应为已隔离好命名空间的子存储；maxEntries<=0 时取默认值。
func New(store storage.PersistentStorage, maxEntries int, logger *slog.Logger) *Logger {
	if maxEntries <= 0 {
		maxEntries = defaultMax
	}
	l := &Logger{store: store, maxEntries: maxEntries, logger: logger}
	if l.logger == nil {
		l.logger = slog.Default()
	}
	l.seq = l.loadSeq()
	return l
}

func (l *Logger) loadSeq() uint64 {
	s, ok := l.store.GetString(context.Background(), seqKey)
	if !ok {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// Record 追加一条日志并落盘，返回写入的 Entry（含分配的 ID）。
func (l *Logger) Record(e Entry) Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	e.ID = strconv.FormatUint(l.seq, 36)
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if e.Status == "" {
		e.Status = StatusRunning
	}

	entries := l.loadLocked()
	entries = append([]Entry{e}, entries...) // newest first
	if len(entries) > l.maxEntries {
		entries = entries[:l.maxEntries]
	}
	l.saveLocked(entries)
	l.store.SetString(context.Background(), seqKey, strconv.FormatUint(l.seq, 10))
	return e
}

// Update 按日志 ID 更新一条已存在的记录（如 running → success）。未找到时忽略。
func (l *Logger) Update(id string, mutate func(*Entry)) {
	if id == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	entries := l.loadLocked()
	for i := range entries {
		if entries[i].ID == id {
			mutate(&entries[i])
			l.saveLocked(entries)
			return
		}
	}
}

// Recent 返回最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (l *Logger) Recent(limit int) []Entry {
	entries := l.load(context.Background())
	if limit <= 0 {
		limit = len(entries)
	}
	if limit > len(entries) {
		limit = len(entries)
	}
	out := make([]Entry, limit)
	copy(out, entries[:limit])
	return out
}

// Filter Query 日志的查询条件，零值字段不参与过滤。
type Filter struct {
	ChatType string    // group / friend
	TargetID string    // 群号 / 好友 QQ（精确匹配）
	Sender   string    // 触发人 QQ（精确匹配，命中批次内任一发言者）
	Start    time.Time // 起始时间（含），零值不限
	End      time.Time // 截止时间（含），零值不限
	Keyword  string    // 用户输入内容包含的关键词（不区分大小写）
	Limit    int       // 返回条数上限，<=0 时取默认值
}

// match 判断一条日志是否满足过滤条件。
func (f Filter) match(e Entry) bool {
	if f.ChatType != "" && e.ChatType != f.ChatType {
		return false
	}
	if f.TargetID != "" && e.TargetID != f.TargetID {
		return false
	}
	if f.Sender != "" {
		hit := false
		for _, s := range e.Senders {
			if s == f.Sender {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if !f.Start.IsZero() && e.Time.Before(f.Start) {
		return false
	}
	if !f.End.IsZero() && e.Time.After(f.End) {
		return false
	}
	if f.Keyword != "" && !strings.Contains(strings.ToLower(e.Query), strings.ToLower(f.Keyword)) {
		return false
	}
	return true
}

// Query 按条件过滤日志（新在前），最多返回 f.Limit 条（<=0 时取默认值）。
func (l *Logger) Query(f Filter) []Entry {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultMax
	}
	entries := l.load(context.Background())
	out := make([]Entry, 0, limit)
	for _, e := range entries {
		if !f.match(e) {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func (l *Logger) load(ctx context.Context) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadLocked()
}

func (l *Logger) loadLocked() []Entry {
	var entries []Entry
	if ok := l.store.Get(context.Background(), entriesKey, &entries); !ok {
		return nil
	}
	return entries
}

func (l *Logger) saveLocked(entries []Entry) {
	if ok := l.store.Set(context.Background(), entriesKey, entries); !ok {
		l.logger.Error("querylog 落盘失败", "err", fmt.Errorf("set entries failed"))
		return
	}
}
