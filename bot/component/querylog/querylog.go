// Package querylog 提供 AI 查询（Query）执行日志的持久化记录能力。
//
// 一次 Query 指从用户触发 AI 回复（群里 @ 机器人或私聊）到 AI 最终响应的完整过程。
// 与 tasklog（定时任务执行日志）结构类似：每条日志独立占用一个 KV 记录
// （key 为 e:<序号>），写入与单条更新均为 O(1)，避免整体数组读写的放大；
// 按容量上限滚动淘汰最旧记录。命名空间由调用方在注入时隔离，本组件不再 Clone。
//
// 早期版本曾将全部日志打包为单个 JSON 数组存于 entries 键，New 时会自动迁移
// 为逐条记录并删除旧键。
package querylog

import (
	"context"
	"log/slog"
	"sort"
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

// MaxToolCallRecords 单条日志最多保留的工具调用明细条数。
// 超出部分丢弃，实际总数见 Entry.ToolCallsTotal。
const MaxToolCallRecords = 20

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
	ToolCallsTotal   int              `json:"tool_calls_total,omitempty"` // 工具调用总数（超过 MaxToolCallRecords 时大于 len(ToolCalls)）
	PromptTokens     int              `json:"prompt_tokens,omitempty"`
	CompletionTokens int              `json:"completion_tokens,omitempty"`
	TotalTokens      int              `json:"total_tokens,omitempty"`
	CachedTokens     int              `json:"cached_tokens,omitempty"` // 命中上游 prompt 缓存的 token 数（提供方支持时才有）
	Reply            string           `json:"reply,omitempty"`         // 最终回复（截断）
	Error            string           `json:"error,omitempty"`
}

const (
	entryKeyPrefix   = "e:"      // 逐条日志的键前缀：e:<序号>
	seqKey           = "seq"     // 自增序号（十进制）
	legacyEntriesKey = "entries" // 旧版整体 JSON 数组键，仅用于迁移
	defaultMax       = 200
)

// Truncate 按符文数截断字符串，超出时追加省略标记。max<=0 时不截断。
func Truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

// entryKey 由序号生成日志记录键。
func entryKey(seq uint64) string {
	return entryKeyPrefix + strconv.FormatUint(seq, 10)
}

// Logger 持久化 Query 日志记录器。
//
// 内部用互斥串行化 Record/Update，避免并发写入互相覆盖；读取（Recent/Query）
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
	l.migrateLegacy()
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

// migrateLegacy 将旧版 entries 键中的整体 JSON 数组拆分为逐条记录。
// 旧 ID 是序号的 base36，可解析回序号复用为记录键；解析失败的分配新序号。
func (l *Logger) migrateLegacy() {
	l.mu.Lock()
	defer l.mu.Unlock()

	ctx := context.Background()
	var entries []Entry
	if !l.store.Get(ctx, legacyEntriesKey, &entries) {
		return
	}
	for _, e := range entries {
		n, err := strconv.ParseUint(e.ID, 36, 64)
		if err != nil || n == 0 {
			l.seq++
			n = l.seq
			e.ID = strconv.FormatUint(n, 36)
		}
		l.store.Set(ctx, entryKey(n), e)
		if n > l.seq {
			l.seq = n
		}
	}
	l.store.SetString(ctx, seqKey, strconv.FormatUint(l.seq, 10))
	l.store.Del(ctx, legacyEntriesKey)
	l.evictLocked()
	l.logger.Info("querylog 旧格式已迁移为逐条存储", "count", len(entries))
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

	ctx := context.Background()
	if !l.store.Set(ctx, entryKey(l.seq), e) {
		l.logger.Error("querylog 落盘失败", "id", e.ID)
	}
	l.store.SetString(ctx, seqKey, strconv.FormatUint(l.seq, 10))
	l.evictLocked()
	return e
}

// evictLocked 淘汰超出容量上限的最旧记录。
func (l *Logger) evictLocked() {
	seqs := l.listLocked()
	if len(seqs) <= l.maxEntries {
		return
	}
	ctx := context.Background()
	for _, n := range seqs[:len(seqs)-l.maxEntries] {
		l.store.Del(ctx, entryKey(n))
	}
}

// Update 按日志 ID 更新一条已存在的记录（如 running → success）。未找到时忽略。
func (l *Logger) Update(id string, mutate func(*Entry)) {
	if id == "" {
		return
	}
	// ID 即序号的 base36，直接定位记录键，无需全量扫描
	n, err := strconv.ParseUint(id, 36, 64)
	if err != nil || n == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	ctx := context.Background()
	key := entryKey(n)
	var e Entry
	if !l.store.Get(ctx, key, &e) {
		return
	}
	mutate(&e)
	if !l.store.Set(ctx, key, e) {
		l.logger.Error("querylog 落盘失败", "id", id)
	}
}

// Recent 返回最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (l *Logger) Recent(limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	seqs := l.listLocked() // 升序，新在后
	if limit <= 0 || limit > len(seqs) {
		limit = len(seqs)
	}
	ctx := context.Background()
	out := make([]Entry, 0, limit)
	for i := len(seqs) - 1; i >= len(seqs)-limit; i-- {
		var e Entry
		if l.store.Get(ctx, entryKey(seqs[i]), &e) {
			out = append(out, e)
		}
	}
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

// load 返回全部日志（新在前）。
func (l *Logger) load(ctx context.Context) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.loadLocked()
}

func (l *Logger) loadLocked() []Entry {
	seqs := l.listLocked() // 升序，新在后
	ctx := context.Background()
	entries := make([]Entry, 0, len(seqs))
	for i := len(seqs) - 1; i >= 0; i-- {
		var e Entry
		if l.store.Get(ctx, entryKey(seqs[i]), &e) {
			entries = append(entries, e)
		}
	}
	return entries
}

// listLocked 返回全部日志记录的序号，按升序排列（旧在前）。
func (l *Logger) listLocked() []uint64 {
	keys, err := l.store.Keys(context.Background(), entryKeyPrefix)
	if err != nil {
		l.logger.Error("querylog 列举键失败", "err", err)
		return nil
	}
	seqs := make([]uint64, 0, len(keys))
	for _, k := range keys {
		n, err := strconv.ParseUint(strings.TrimPrefix(k, entryKeyPrefix), 10, 64)
		if err != nil {
			continue
		}
		seqs = append(seqs, n)
	}
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	return seqs
}
