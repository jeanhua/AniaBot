// Package tasklog 提供持久化的执行日志记录能力。
//
// 与控制台日志（log/slog）不同，tasklog 将结构化执行记录落盘到 PersistentStorage，
// 供事后查询与上报（如定时任务的触发 / 成功 / 超时 / 失败记录）。日志以 JSON 数组
// 整体读写（符合持久化存储的 KV 语义，不支持列表语义），按容量上限滚动保留最近的
// 若干条。命名空间由调用方在注入时隔离，本组件不再 Clone。
package tasklog

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

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

// Entry 一条执行日志
type Entry struct {
	ID               string    `json:"id"` // 日志 ID（自增序号的 base36）
	TaskID           string    `json:"task_id"`
	TaskTitle        string    `json:"task_title"`
	TargetType       string    `json:"target_type"` // group / friend
	TargetID         string    `json:"target_id"`
	TriggerTime      time.Time `json:"trigger_time"`
	Status           Status    `json:"status"`
	DurationMs       int64     `json:"duration_ms"`
	Error            string    `json:"error,omitempty"`
	PromptTokens     int       `json:"prompt_tokens,omitempty"`
	CompletionTokens int       `json:"completion_tokens,omitempty"`
	TotalTokens      int       `json:"total_tokens,omitempty"`
	FinishedAt       time.Time `json:"finished_at,omitempty"`
}

const (
	entriesKey = "entries"
	seqKey     = "seq"
	defaultMax = 500
)

// Logger 持久化执行日志记录器。
//
// 内部用互斥串行化 Record/Update，避免并发写入互相覆盖；读取（Recent 等）
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
	if e.TriggerTime.IsZero() {
		e.TriggerTime = time.Now()
	}
	if e.FinishedAt.IsZero() && e.Status != StatusRunning {
		e.FinishedAt = time.Now()
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

// RecentForTask 返回指定任务的最近 limit 条日志（新在前）。
func (l *Logger) RecentForTask(taskID string, limit int) []Entry {
	all := l.load(context.Background())
	if limit <= 0 {
		limit = len(all)
	}
	out := make([]Entry, 0, limit)
	for _, e := range all {
		if e.TaskID == taskID {
			out = append(out, e)
			if len(out) >= limit {
				break
			}
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
		l.logger.Error("tasklog 落盘失败", "err", fmt.Errorf("set entries failed"))
		return
	}
}
