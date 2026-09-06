package pluginaichat

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

// lockFake 最小内存 Storage 实现：仅支持 tryLock/release 需要的 SETNX、
// GetString、Expire 与 Del，其余方法经内嵌接口兜底（测试中不会触达）。
type lockFake struct {
	storage.Storage
	mu   sync.Mutex
	data map[string]string
}

func newLockFake() *lockFake { return &lockFake{data: map[string]string{}} }

func (f *lockFake) SetString(_ context.Context, key, val string, opts ...storage.Option) bool {
	var cfg storage.StorageConfig
	for _, o := range opts {
		o(&cfg)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if cfg.CheckExist {
		if _, ok := f.data[key]; ok {
			return false
		}
	}
	f.data[key] = val
	return true
}

func (f *lockFake) GetString(_ context.Context, key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[key]
	return v, ok
}

func (f *lockFake) Expire(_ context.Context, key string, _ time.Duration) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.data[key]
	return ok
}

func (f *lockFake) Del(_ context.Context, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, key)
	return true
}

// newEvictTestPlugin 构造带锁存储与并发槽位的测试插件（回收逻辑只依赖这两项与 Logger）。
func newEvictTestPlugin() *AIChatPlugin {
	p := &AIChatPlugin{
		lockStorage: newLockFake(),
		rateCh:      make(chan struct{}, 4),
	}
	p.Logger = testLogger()
	return p
}

func storeIdleEntry(p *AIChatPlugin, key string, id message.QID, isGroup bool, idle time.Duration) {
	e := newChatEntry(nil, id, isGroup, "")
	e.lastActive.Store(time.Now().Add(-idle).Unix())
	p.chats.Store(key, e)
}

func TestEvictChatsIdle(t *testing.T) {
	p := newEvictTestPlugin()
	id := message.FromUint64(1001)
	storeIdleEntry(p, "g:qq:1001", id, true, 3*time.Hour)
	storeIdleEntry(p, "g:qq:1002", message.FromUint64(1002), true, 10*time.Minute)

	p.evictChats(2*time.Hour, 0)

	if _, ok := p.chats.Load("g:qq:1001"); ok {
		t.Fatal("闲置 3 小时的会话应被淘汰")
	}
	if _, ok := p.chats.Load("g:qq:1002"); !ok {
		t.Fatal("最近活跃的会话不应被淘汰")
	}
}

func TestEvictChatsSkipsLockedSession(t *testing.T) {
	p := newEvictTestPlugin()
	id := message.FromUint64(1003)
	storeIdleEntry(p, "g:qq:1003", id, true, 3*time.Hour)

	// 模拟该会话正在响应（持有会话锁）
	lock := p.tryLock(id, true)
	if lock == nil {
		t.Fatal("预取会话锁失败")
	}
	defer lock.release()

	p.evictChats(2*time.Hour, 0)
	if _, ok := p.chats.Load("g:qq:1003"); !ok {
		t.Fatal("响应中的会话不应被淘汰")
	}
}

func TestEvictChatsSkipsPendingQueue(t *testing.T) {
	p := newEvictTestPlugin()
	id := message.FromUint64(1004)
	storeIdleEntry(p, "g:qq:1004", id, true, 3*time.Hour)

	// 遗留排队消息：淘汰窗口内到达的消息只能排队，不得连会话一起丢
	p.enqueuePending(id, true, message.Message{})

	p.evictChats(2*time.Hour, 0)
	if _, ok := p.chats.Load("g:qq:1004"); !ok {
		t.Fatal("有排队消息的会话不应被淘汰")
	}
}

func TestEvictChatsLRU(t *testing.T) {
	p := newEvictTestPlugin()
	storeIdleEntry(p, "g:qq:1", message.FromUint64(1), true, 1*time.Hour)
	storeIdleEntry(p, "g:qq:2", message.FromUint64(2), true, 2*time.Hour)
	storeIdleEntry(p, "f:qq:3", message.FromUint64(3), false, 10*time.Minute)

	p.evictChats(0, 2)

	if _, ok := p.chats.Load("g:qq:2"); ok {
		t.Fatal("最久未活跃的会话应被淘汰")
	}
	if _, ok := p.chats.Load("g:qq:1"); !ok {
		t.Fatal("次旧会话应保留")
	}
	if _, ok := p.chats.Load("f:qq:3"); !ok {
		t.Fatal("最活跃会话应保留")
	}
}

func TestEvictChatsDisabled(t *testing.T) {
	p := newEvictTestPlugin()
	storeIdleEntry(p, "g:qq:1", message.FromUint64(1), true, 100*time.Hour)

	p.evictChats(0, 0)
	if _, ok := p.chats.Load("g:qq:1"); !ok {
		t.Fatal("策略均为 0 时不应淘汰任何会话")
	}
}

func TestTouchChat(t *testing.T) {
	p := newEvictTestPlugin()
	id := message.FromUint64(1005)
	storeIdleEntry(p, "g:qq:1005", id, true, 3*time.Hour)

	p.touchChat("g:qq:1005")

	v, _ := p.chats.Load("g:qq:1005")
	e := v.(*chatEntry)
	if time.Since(time.Unix(e.lastActive.Load(), 0)) > time.Minute {
		t.Fatal("touchChat 应刷新活跃时间")
	}
	// 不存在的键：空操作不 panic
	p.touchChat("g:nonexist")
}
