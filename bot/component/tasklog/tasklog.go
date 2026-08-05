// Package tasklog 提供持久化的执行日志记录能力。
//
// 与控制台日志（log/slog）不同，tasklog 将结构化执行记录落盘到 PersistentStorage，
// 供事后查询与上报（如定时任务的触发 / 成功 / 超时 / 失败记录）。存储为双后端
// 结构：SQL 后端下每条日志一行（ania_task_log 表，过滤条件下推 WHERE、容量淘汰
// 走范围删除）；非 SQL 后端回退为逐条 KV 记录（key 为 e:<序号>，写入与单条更新
// 均为 O(1)）。日志 ID 为自增序号的 base36，两种后端一致。命名空间由调用方在
// 注入时隔离（KV 后端），本组件不再 Clone。
package tasklog

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/common/storage"
)

// Status 执行状态
type Status string

const (
	StatusRunning Status = "running" // 已触发，执行中
	StatusSuccess Status = "success" // 执行成功
	StatusTimeout Status = "timeout" // 执行超时
	StatusError   Status = "error"   // 执行出错
)

// 字段截断上限（符文数），避免单条日志体积失控
const (
	MaxContentRunes = 500  // 触发时发送给 AI 的内容
	MaxReplyRunes   = 1000 // 最终回复
	MaxArgsRunes    = 500  // 工具调用参数
	MaxResultRunes  = 1000 // 工具执行结果
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

// Entry 一条执行日志
type Entry struct {
	ID               string           `json:"id"` // 日志 ID（自增序号的 base36）
	TaskID           string           `json:"task_id"`
	TaskTitle        string           `json:"task_title"`
	TargetType       string           `json:"target_type"` // group / friend
	TargetID         string           `json:"target_id"`
	TriggerTime      time.Time        `json:"trigger_time"`
	TriggerContent   string           `json:"trigger_content,omitempty"` // 触发时发送给 AI 的内容（截断）
	Status           Status           `json:"status"`
	DurationMs       int64            `json:"duration_ms"`
	Iterations       int              `json:"iterations,omitempty"` // LLM 调用轮数
	ToolCalls        []ToolCallRecord `json:"tool_calls,omitempty"`
	ToolCallsTotal   int              `json:"tool_calls_total,omitempty"` // 工具调用总数（超过 MaxToolCallRecords 时大于 len(ToolCalls)）
	Error            string           `json:"error,omitempty"`
	Reply            string           `json:"reply,omitempty"` // 最终回复（截断）
	PromptTokens     int              `json:"prompt_tokens,omitempty"`
	CompletionTokens int              `json:"completion_tokens,omitempty"`
	TotalTokens      int              `json:"total_tokens,omitempty"`
	CachedTokens     int              `json:"cached_tokens,omitempty"` // 命中上游 prompt 缓存的 token 数（提供方支持时才有）
	FinishedAt       time.Time        `json:"finished_at,omitempty"`
}

const defaultMax = 500

// Truncate 按符文数截断字符串，超出时追加省略标记。max<=0 时不截断。
func Truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

// backend 日志存储后端。调用方（Logger）持有 mu 并保证写路径串行，
// 后端实现无需额外加锁。错误内部记录日志（后端持有 logger）。
type backend interface {
	// maxSeq 返回已分配的最大序号（启动初始化计数器用）；无记录时返回 0。
	maxSeq() uint64
	// insert 追加一条日志。
	insert(seq uint64, e Entry)
	// load 按序号读取一条日志（Update 定位用）。
	load(seq uint64) (Entry, bool)
	// overwrite 按序号覆盖一条日志（Update 落盘用）。
	overwrite(seq uint64, e Entry)
	// evict 淘汰旧记录，仅保留序号最大的 maxEntries 条；maxSeq 为当前最大序号。
	evict(maxSeq uint64, maxEntries int)
	// recent 返回最近 limit 条日志（新在前）；limit<=0 时返回全部。
	recent(limit int) []Entry
	// query 按条件过滤日志（新在前），beforeSeq>0 时仅返回序号更小的记录，
	// 最多 limit 条（limit<=0 不限）。f.match 为最终判定。
	query(f Filter, beforeSeq uint64, limit int) []Entry
}

// Logger 持久化执行日志记录器。
//
// 内部用互斥串行化 Record/Update；读取（Recent 等）每次直接从存储加载
// 最新快照，不持有内存缓存，保证与落盘状态一致。SQL 后端与非 SQL 后端
// （KV）行为一致，ID 均为自增序号的 base36。
type Logger struct {
	backend    backend
	maxEntries int
	logger     *slog.Logger
	mu         sync.Mutex
	seq        uint64
}

// New 创建日志记录器。store 应为已隔离好命名空间的子存储；maxEntries<=0 时取默认值。
// SQL 后端（storage.SQLBackend 探测成功且建表成功）走 ania_task_log 行级存储，
// 否则回退逐条 KV 记录。
func New(store storage.PersistentStorage, maxEntries int, logger *slog.Logger) *Logger {
	if maxEntries <= 0 {
		maxEntries = defaultMax
	}
	l := &Logger{maxEntries: maxEntries, logger: logger}
	if l.logger == nil {
		l.logger = slog.Default()
	}
	var b backend = newKVBackend(store, l.logger)
	if db, dialect, ok := storage.SQLBackend(store); ok {
		if err := storage.EnsureTables(context.Background(), db, dialect, taskLogTables...); err != nil {
			l.logger.Error("创建任务日志表失败，回退 KV 存储", "error", err.Error())
		} else {
			b = newSQLBackend(db, l.logger)
		}
	}
	l.backend = b
	l.seq = b.maxSeq()
	return l
}

// Record 追加一条日志并落盘，返回写入的 Entry（含分配的 ID）。
func (l *Logger) Record(e Entry) Entry {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.seq++
	e.ID = strconv.FormatUint(l.seq, 36)
	if e.TriggerTime.IsZero() {
		e.TriggerTime = time.Now()
	}
	if e.FinishedAt.IsZero() && e.Status != StatusRunning {
		e.FinishedAt = time.Now()
	}

	l.backend.insert(l.seq, e)
	l.backend.evict(l.seq, l.maxEntries)
	return e
}

// Update 按日志 ID 更新一条已存在的记录（如 running → success）。未找到时忽略。
func (l *Logger) Update(id string, mutate func(*Entry)) {
	if id == "" {
		return
	}
	// ID 即序号的 base36，直接定位记录，无需全量扫描
	n, err := strconv.ParseUint(id, 36, 64)
	if err != nil || n == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.backend.load(n)
	if !ok {
		return
	}
	mutate(&e)
	l.backend.overwrite(n, e)
}

// Recent 返回最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (l *Logger) Recent(limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.backend.recent(limit)
}

// RecentForTask 返回指定任务的最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (l *Logger) RecentForTask(taskID string, limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.backend.query(Filter{TaskID: taskID}, 0, limit)
}

// RecentForTarget 返回指定触发对象的最近 limit 条日志（新在前）。limit<=0 时返回全部。
// 供会话视角查询使用：AI 工具与 /clock 命令按当前会话过滤，不暴露其他会话的任务日志。
func (l *Logger) RecentForTarget(targetType, targetID string, limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.backend.query(Filter{TargetType: targetType, TargetID: targetID}, 0, limit)
}

// Filter 执行日志的查询条件，零值字段不参与过滤。
type Filter struct {
	TargetType string    // group / friend
	TargetID   string    // 目标会话 ID / 用户 ID（精确匹配）
	TaskID     string    // 任务 ID（精确匹配）
	Status     Status    // 执行状态（精确匹配）
	Start      time.Time // 触发起始时间（含），零值不限
	End        time.Time // 触发截止时间（含），零值不限
	Keyword    string    // 任务标题包含的关键词（不区分大小写）
	Before     string    // 分页游标：仅返回比该日志 ID 更旧的记录（空为从最新开始）
	Limit      int       // 返回条数上限，<=0 时取默认值
}

// match 判断一条日志是否满足过滤条件。
func (f Filter) match(e Entry) bool {
	if f.TargetType != "" && e.TargetType != f.TargetType {
		return false
	}
	if f.TargetID != "" && e.TargetID != f.TargetID {
		return false
	}
	if f.TaskID != "" && e.TaskID != f.TaskID {
		return false
	}
	if f.Status != "" && e.Status != f.Status {
		return false
	}
	if !f.Start.IsZero() && e.TriggerTime.Before(f.Start) {
		return false
	}
	if !f.End.IsZero() && e.TriggerTime.After(f.End) {
		return false
	}
	if f.Keyword != "" && !strings.Contains(strings.ToLower(e.TaskTitle), strings.ToLower(f.Keyword)) {
		return false
	}
	return true
}

// Query 按条件过滤日志（新在前），最多返回 f.Limit 条（<=0 时取默认值）。
// f.Before 非空时作为分页游标，仅返回比它更旧的记录。
func (l *Logger) Query(f Filter) []Entry {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultMax
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.backend.query(f, parseBeforeSeq(f.Before), limit)
}

// parseBeforeSeq 把分页游标（日志 ID，序号的 base36）解析为序号；非法时为 0（不生效）。
func parseBeforeSeq(before string) uint64 {
	if before == "" {
		return 0
	}
	n, err := strconv.ParseUint(before, 36, 64)
	if err != nil {
		return 0
	}
	return n
}
