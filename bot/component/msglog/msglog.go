// Package msglog 提供进程内的消息日志记录能力。
//
// 与 tasklog（持久化执行日志）不同，msglog 面向高频的收消息 / 通知事件，
// 采用内存环形缓冲，不落盘：按容量上限滚动保留最近的若干条，重启后清空。
// 供 Web 控制面板的「消息日志」页实时展示群消息、好友消息与通知事件。
package msglog

import (
	"sync"
	"time"
)

// Type 日志类型
type Type string

const (
	TypeGroup  Type = "group"  // 群消息
	TypeFriend Type = "friend" // 好友（私聊）消息
	TypeNotice Type = "notice" // 通知事件（群成员变动、撤回、戳一戳等）
)

const defaultMax = 500

// Entry 一条消息日志
type Entry struct {
	ID       uint64    `json:"id"`
	Time     time.Time `json:"time"`
	Type     Type      `json:"type"`
	GroupId  string    `json:"group_id,omitempty"` // 群号（群消息 / 群通知）
	UserId   string    `json:"user_id,omitempty"`  // 相关 QQ 号
	Nickname string    `json:"nickname,omitempty"` // 发送者昵称 / 群名片
	Title    string    `json:"title,omitempty"`    // 通知标题（仅 notice 类型），如「群成员增加」
	Text     string    `json:"text"`               // 消息内容 / 通知描述
}

// Recorder 内存消息日志记录器（环形缓冲，新条目在前）。
type Recorder struct {
	mu      sync.Mutex
	entries []Entry
	max     int
	seq     uint64
}

// New 创建记录器。maxEntries<=0 时取默认值。
func New(maxEntries int) *Recorder {
	if maxEntries <= 0 {
		maxEntries = defaultMax
	}
	return &Recorder{max: maxEntries}
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
	r.entries = append([]Entry{e}, r.entries...)
	if len(r.entries) > r.max {
		r.entries = r.entries[:r.max]
	}
}

// Recent 返回最近 limit 条日志（新在前）。limit<=0 时返回全部。
func (r *Recorder) Recent(limit int) []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 || limit > len(r.entries) {
		limit = len(r.entries)
	}
	out := make([]Entry, limit)
	copy(out, r.entries[:limit])
	return out
}
