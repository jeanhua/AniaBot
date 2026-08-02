// Package msglog 提供消息日志记录能力。
//
// 与 tasklog（持久化执行日志）不同，msglog 面向高频的收消息 / 通知事件，
// 底层使用缓存存储（storage.Storage）：缓存驱动为 memory 时仅保留在进程内，
// 重启后清空（与旧版内存环形缓冲行为一致）；驱动为 redis 时日志随 Redis
// 保存，重启后仍可回看。供 Web 控制面板的「消息日志」页实时展示群消息、
// 好友消息与通知事件。
//
// 存储结构刻意拆分为多个键，避免「一个大 JSON 塞一个 key」的读写放大：
//   - msglog:seq     自增 ID 计数器（十进制字符串）
//   - msglog:entries 列表，每个元素是一条独立日志（LPush 新条目在前，
//     LTrim 按容量上限滚动淘汰最旧条目）
//
// 命名空间由调用方在注入存储时隔离，本组件不再 Clone。
package msglog

import (
	"context"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

// Type 日志类型
type Type string

const (
	TypeGroup  Type = "group"  // 群消息
	TypeFriend Type = "friend" // 好友（私聊）消息
	TypeNotice Type = "notice" // 通知事件（群成员变动、撤回、戳一戳等）
)

const (
	seqKey     = "msglog:seq"     // 自增 ID 计数器
	entriesKey = "msglog:entries" // 日志列表（新在前，每条日志一个元素）
	defaultMax = 500
)

// Entry 一条消息日志
type Entry struct {
	ID       uint64    `json:"id"`
	Time     time.Time `json:"time"`
	Type     Type      `json:"type"`
	GroupId  string    `json:"group_id,omitempty"` // 会话 ID（群消息 / 群通知）
	UserId   string    `json:"user_id,omitempty"`  // 相关用户 ID
	Nickname string    `json:"nickname,omitempty"` // 发送者昵称 / 群名片
	Title    string    `json:"title,omitempty"`    // 通知标题（仅 notice 类型），如「群成员增加」
	Text     string    `json:"text"`               // 消息内容 / 通知描述
}

// Recorder 消息日志记录器。
//
// 内部用互斥串行化 Add，保证 ID 单调递增；读取（Recent）每次直接从存储
// 加载最新数据，不持有内存副本，保证与存储状态一致（redis 驱动下重启不丢）。
type Recorder struct {
	store storage.Storage
	max   int64
	mu    sync.Mutex
	seq   uint64
}

// New 创建记录器。store 应为已隔离好命名空间的缓存存储；maxEntries<=0 时取默认值。
func New(store storage.Storage, maxEntries int) *Recorder {
	if maxEntries <= 0 {
		maxEntries = defaultMax
	}
	r := &Recorder{store: store, max: int64(maxEntries)}
	r.seq = r.loadSeq()
	return r
}

// loadSeq 从存储恢复自增 ID 计数器（redis 驱动下重启后续接之前的 ID）。
func (r *Recorder) loadSeq() uint64 {
	s, ok := r.store.GetString(context.Background(), seqKey)
	if !ok {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// Add 追加一条日志。Time 为零值时取当前时间。
func (r *Recorder) Add(e Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	e.ID = r.seq
	if e.Time.IsZero() {
		e.Time = time.Now()
	}

	ctx := context.Background()
	r.store.LPush(ctx, entriesKey, e)
	r.store.LTrim(ctx, entriesKey, 0, r.max-1)
	r.store.SetString(ctx, seqKey, strconv.FormatUint(r.seq, 10))
}

// Recent 返回最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (r *Recorder) Recent(limit int) []Entry {
	if limit <= 0 || limit > int(r.max) {
		limit = int(r.max)
	}
	items, ok := r.store.LRange(context.Background(), entriesKey, 0, int64(limit)-1)
	if !ok {
		return nil
	}
	return decodeEntries(items)
}

// Page 返回一页日志（新在前），供面板滚动分页使用：beforeID>0 时仅返回
// ID 小于它的更旧日志，否则从最新开始。limit<=0 时取容量上限。
//
// 列表按新在前排列且 ID 连续自增，索引 i 处的条目 ID 为 seq-i，
// 因此可直接把游标换算为列表偏移，无需全量扫描。
func (r *Recorder) Page(limit int, beforeID uint64) []Entry {
	r.mu.Lock()
	seq := r.seq
	r.mu.Unlock()

	if limit <= 0 || limit > int(r.max) {
		limit = int(r.max)
	}
	var start int64
	if beforeID > 0 && beforeID <= seq {
		start = int64(seq - beforeID + 1)
	}
	items, ok := r.store.LRange(context.Background(), entriesKey, start, start+int64(limit)-1)
	if !ok {
		return nil
	}
	return decodeEntries(items)
}

// decodeEntries 把存储层按 any 解码的列表元素（map）经 JSON 往返还原为 Entry。
func decodeEntries(items []any) []Entry {
	out := make([]Entry, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err != nil {
			continue
		}
		var e Entry
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out
}
