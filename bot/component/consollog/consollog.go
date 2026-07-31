// Package consollog 提供 Bot 运行时日志（控制台日志）的内存环形缓冲，
// 供 Web 控制面板的「控制台日志」页实时查看。
//
// 与 msglog / tasklog / querylog（业务日志）不同，这里捕获的是传统意义上
// 「控制台」上的全部输出：slog 结构化日志（核心 / 插件 / 面板）与标准库
// log 输出（适配器 / 工具类代码）。两者都会先写入环形缓冲，再透传给底层
// handler（如 tint 的彩色控制台输出）或标准输出，因此原有控制台行为不变，
// 只是多了一份可回看的副本。重启后清空（纯内存，不持久化）。
//
// 环形缓冲按「新在前」供面板滚动分页：Page(limit, beforeID) 返回一页日志，
// beforeID>0 时仅返回 ID 比它更旧的记录。Entry.ID 在进程内单调递增。
package consollog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// defaultMax 环形缓冲默认容量（条数上限，超出后淘汰最旧条目）
const defaultMax = 2000

// printLevel 标准库 log 输出在环形缓冲中使用的伪级别
const printLevel = "log"

// Attr 一条结构化属性（键值对，用于渲染 slog 记录的附加字段）
type Attr struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Entry 一条控制台日志。
//
// Level 取值：debug / info / warn / error（slog 级别），或 log（标准库
// log 输出，无级别概念）。Attrs 保存 slog 记录的附加字段（含 WithGroup /
// WithAttrs 累积的、以及 slog.Group 展开后的字段），面板按行内 key=value 渲染。
type Entry struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Attrs   []Attr    `json:"attrs,omitempty"`
}

// Ring 线程安全的环形缓冲。
//
// 存储为定长循环数组（旧在前、新在后），写入频率高时无需整体搬移；
// 超过容量后覆盖最旧条目。ID 单调递增，供面板分页游标使用。
type Ring struct {
	mu   sync.Mutex
	buf  []Entry      // 定长循环数组
	cap  int          // 容量
	head int          // 最旧一条所在下标
	size int          // 当前条数
	seq  uint64       // 最新已分配 ID
	line bytes.Buffer // 标准库 log 输出的行缓冲（按换行切分）
}

// NewRing 创建容量为 max 的环形缓冲；max<=0 时取默认值。
func NewRing(max int) *Ring {
	if max <= 0 {
		max = defaultMax
	}
	return &Ring{buf: make([]Entry, max), cap: max}
}

// Add 追加一条日志（供 slog handler 调用，或构造 Entry 后直接写入）。
func (r *Ring) Add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.push(e)
}

// Page 返回一页日志（新在前），供面板滚动分页使用：beforeID>0 时仅返回
// ID 比它更旧的日志，否则从最新开始。limit<=0 时取容量上限。
func (r *Ring) Page(limit int, beforeID uint64) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > r.cap {
		limit = r.cap
	}
	out := make([]Entry, 0, limit)
	// 从新到旧遍历；ID 单调递增且与插入顺序一致，可安全用游标跳过新条目
	for i := 0; i < r.size && len(out) < limit; i++ {
		e := r.buf[(r.head+r.size-1-i)%r.cap]
		if beforeID > 0 && e.ID >= beforeID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Handler 返回包装 inner 的 slog.Handler：每条记录先写入环形缓冲，
// 再透传给 inner 输出（控制台行为不变）。WithGroup / WithAttrs 同时
// 作用于透传与捕获两侧，捕获侧按点分前缀展开分组键。
func (r *Ring) Handler(inner slog.Handler) slog.Handler {
	return &captureHandler{ring: r, inner: inner}
}

// Write 实现 io.Writer，供标准库 log.SetOutput 捕获原生 log 输出：
// 内部按换行切分，每行作为一条 Level=log 的日志写入环形缓冲。
func (r *Ring) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(p)
	for len(p) > 0 {
		if i := bytes.IndexByte(p, '\n'); i >= 0 {
			r.line.Write(p[:i])
			r.pushLine()
			p = p[i+1:]
		} else {
			r.line.Write(p)
			p = nil
		}
	}
	return n, nil
}

// pushLine 把行缓冲中已切分好的一行写入环形缓冲（调用方须持有锁）。
func (r *Ring) pushLine() {
	s := strings.TrimSpace(r.line.String())
	r.line.Reset()
	if s == "" {
		return
	}
	r.push(Entry{Time: time.Now(), Level: printLevel, Message: s})
}

// push 写入一条日志（调用方须持有锁）。
func (r *Ring) push(e Entry) {
	r.seq++
	e.ID = r.seq
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if r.size < r.cap {
		r.buf[(r.head+r.size)%r.cap] = e
		r.size++
	} else {
		// 已满：覆盖最旧条目
		r.buf[r.head] = e
		r.head = (r.head + 1) % r.cap
	}
}

// captureHandler 捕获 slog 记录到 Ring 并透传给底层 handler。
type captureHandler struct {
	ring  *Ring
	inner slog.Handler
	attrs []slog.Attr // WithAttrs 累积的属性
	group string      // 捕获侧完整分组前缀（点分，用于展开键名）
}

func (h *captureHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *captureHandler) Handle(ctx context.Context, r slog.Record) error {
	var attrs []Attr
	for _, a := range h.attrs {
		appendAttrs(&attrs, h.group, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttrs(&attrs, h.group, a)
		return true
	})
	h.ring.Add(Entry{
		Time:    r.Time,
		Level:   levelName(r.Level),
		Message: r.Message,
		Attrs:   attrs,
	})
	return h.inner.Handle(ctx, r)
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{
		ring:  h.ring,
		inner: h.inner.WithAttrs(attrs),
		attrs: append(append([]slog.Attr{}, h.attrs...), attrs...),
		group: h.group,
	}
}

func (h *captureHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	prefix := name
	if h.group != "" {
		prefix = h.group + "." + name
	}
	return &captureHandler{
		ring:  h.ring,
		inner: h.inner.WithGroup(name), // 透传侧只传本层组名，由底层 handler 自行嵌套
		attrs: h.attrs,
		group: prefix,
	}
}

// appendAttrs 把一条属性展开进 out：分组属性（KindGroup）递归展开，
// 键名带上前缀（如 group.key）。
func appendAttrs(out *[]Attr, prefix string, a slog.Attr) {
	key := a.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, child := range a.Value.Group() {
			appendAttrs(out, key, child)
		}
		return
	}
	*out = append(*out, Attr{Key: key, Value: a.Value.String()})
}

// levelName 把 slog 级别映射为面板友好的小写名称。
func levelName(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "error"
	case l >= slog.LevelWarn:
		return "warn"
	case l >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// ---- 包级默认环形缓冲（核心与面板共用） ----

var defaultRing = NewRing(0)

// Default 返回全局默认环形缓冲，供核心捕获日志、面板查询日志共用。
func Default() *Ring { return defaultRing }

// Attach 返回包装 inner 的 slog.Handler，把日志写入默认环形缓冲。
func Attach(inner slog.Handler) slog.Handler { return defaultRing.Handler(inner) }

// Writer 返回写入默认环形缓冲的 io.Writer，供标准库 log.SetOutput 使用。
func Writer() io.Writer { return defaultRing }

// Page 返回默认环形缓冲的一页日志（新在前）。
func Page(limit int, beforeID uint64) []Entry { return defaultRing.Page(limit, beforeID) }
