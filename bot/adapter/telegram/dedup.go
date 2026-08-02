package telegram

import (
	"sync"
	"time"
)

// eventDedupTTL 去重键保留时长：与 core 层事件去重 TTL 一致。
const eventDedupTTL = 10 * time.Minute

// updateDedup 长轮询 update_id 幂等去重。
// 长轮询为 at-least-once 投递：进程崩溃时 offset 未推进的更新会被重新投递，
// 以 update_id 为键去重避免重复响应（与 core 层按 message_id 的去重互不冲突，
// 本层还保护成员变动等无 core 去重键的通知事件）。
type updateDedup struct {
	mu   sync.Mutex
	seen map[int]time.Time // update_id -> claim 时间
	ttl  time.Duration
}

func newUpdateDedup(ttl time.Duration) *updateDedup {
	return &updateDedup{seen: map[int]time.Time{}, ttl: ttl}
}

// Claim 尝试占用去重键：首次占用返回 true，重复投递返回 false。
// 窗口内（TTL 未过）的键不可重复占用；过期键视为可重新占用。
// 惰性清理过期项（仅当 map 膨胀时），避免无限增长。
func (d *updateDedup) Claim(id int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	if t, ok := d.seen[id]; ok && now.Sub(t) <= d.ttl {
		return false
	}
	if len(d.seen) > 4096 {
		for k, v := range d.seen {
			if now.Sub(v) > d.ttl {
				delete(d.seen, k)
			}
		}
	}
	d.seen[id] = now
	return true
}
