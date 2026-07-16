package pluginaichat

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/storage"
)

// pfake 前缀感知的进程内 PersistentStorage，各 Clone 共享底层 map 但以不同前缀隔离命名空间。
type pfake struct {
	prefix string
	data   map[string]string
	mu     *sync.Mutex
}

func newPFake() *pfake {
	return &pfake{data: map[string]string{}, mu: &sync.Mutex{}}
}

func (s *pfake) key(k string) string { return s.prefix + k }

func (s *pfake) GetString(_ context.Context, k string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[s.key(k)]
	return v, ok
}
func (s *pfake) SetString(_ context.Context, k, v string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[s.key(k)] = v
	return true
}
func (s *pfake) Get(ctx context.Context, k string, out any) bool {
	v, ok := s.GetString(ctx, k)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(v), out) == nil
}
func (s *pfake) Set(ctx context.Context, k string, v any) bool {
	b, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return s.SetString(ctx, k, string(b))
}
func (s *pfake) Has(ctx context.Context, k string) bool { _, ok := s.GetString(ctx, k); return ok }
func (s *pfake) Del(_ context.Context, k string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, s.key(k))
	return true
}
func (s *pfake) Keys(_ context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	full := s.key(prefix)
	var out []string
	for k := range s.data {
		if strings.HasPrefix(k, full) {
			out = append(out, strings.TrimPrefix(k, s.prefix))
		}
	}
	return out, nil
}
func (s *pfake) Clear(_ context.Context) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.data {
		if strings.HasPrefix(k, s.prefix) {
			delete(s.data, k)
		}
	}
	return true
}
func (s *pfake) Clone(prefix string) storage.PersistentStorage {
	return &pfake{prefix: s.prefix + prefix + ":", data: s.data, mu: s.mu}
}

func TestClockManagerCRUDAndPersist(t *testing.T) {
	p := &AIChatPlugin{}
	p.Logger = slog.Default()
	p.PersistentStorage = newPFake()

	m := newClockManager(p, 30*time.Second, 100)

	// 无效 cron 应失败
	if _, err := m.Add(&ClockTask{Cron: "not a cron", Content: "c", TargetType: "group", TargetID: 1}); err == nil {
		t.Fatal("expected error for invalid cron")
	}
	// 缺少内容应失败
	if _, err := m.Add(&ClockTask{Cron: "@every 1h", TargetType: "group", TargetID: 1}); err == nil {
		t.Fatal("expected error for empty content")
	}

	id, err := m.Add(&ClockTask{Cron: "@every 1h", Title: "喝水", Content: "提醒喝水", TargetType: "group", TargetID: 123, Enabled: true})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id == "" {
		t.Fatal("empty id")
	}

	// List / Get
	if len(m.List()) != 1 {
		t.Fatalf("want 1 task, got %d", len(m.List()))
	}
	got, ok := m.Get(id)
	if !ok || got.Title != "喝水" {
		t.Fatalf("Get failed: %+v %v", got, ok)
	}

	// Update 禁用
	dis := false
	if _, err := m.Update(id, ClockUpdateFields{Enabled: &dis}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if g, _ := m.Get(id); g.Enabled {
		t.Fatal("expected disabled")
	}

	// ListByTarget 过滤
	if len(m.ListByTarget("group", 123)) != 1 {
		t.Fatal("ListByTarget group/123 should have 1")
	}
	if len(m.ListByTarget("friend", 123)) != 0 {
		t.Fatal("ListByTarget friend/123 should have 0")
	}

	// 重启模拟：用同一存储新建 manager，任务应恢复
	m2 := newClockManager(p, 30*time.Second, 100)
	loaded := m2.List()
	if len(loaded) != 1 {
		t.Fatalf("after reload want 1 task, got %d", len(loaded))
	}
	if loaded[0].Title != "喝水" || loaded[0].Enabled {
		t.Fatalf("reload state wrong: %+v", loaded[0])
	}

	// Delete
	if !m2.Delete(id) {
		t.Fatal("Delete should return true")
	}
	if len(m2.List()) != 0 {
		t.Fatal("expected 0 after delete")
	}
	// 再次 reload 确认已落盘删除
	m3 := newClockManager(p, 30*time.Second, 100)
	if len(m3.List()) != 0 {
		t.Fatal("expected 0 after reload following delete")
	}
}

func TestBuildTriggerPrompt(t *testing.T) {
	p := &AIChatPlugin{}
	p.Logger = slog.Default()
	p.PersistentStorage = newPFake()
	m := newClockManager(p, 30*time.Second, 100)

	got := m.buildTriggerPrompt(&ClockTask{Title: "早安", Content: "大家早上好"})
	want := "【定时任务】早安\n大家早上好"
	if got != want {
		t.Fatalf("prompt mismatch:\n got: %q\nwant: %q", got, want)
	}

	// 无标题
	got2 := m.buildTriggerPrompt(&ClockTask{Content: "仅内容"})
	if got2 != "【定时任务】\n仅内容" {
		t.Fatalf("no-title prompt mismatch: %q", got2)
	}

	// 带备注
	got3 := m.buildTriggerPrompt(&ClockTask{Title: "t", Content: "c", Note: "n"})
	if !strings.Contains(got3, "（备注：n）") {
		t.Fatalf("note missing: %q", got3)
	}
}

func TestResolveTarget(t *testing.T) {
	b := clockToolBase{defType: clockTargetGroup, defID: message.QID(123)}
	// 默认回退到当前会话
	if tt, id := b.resolveTarget("", 0); tt != clockTargetGroup || id != 123 {
		t.Fatalf("default resolve wrong: %s %d", tt, id)
	}
	// 显式提供则采用
	if tt, id := b.resolveTarget(clockTargetFriend, 999); tt != clockTargetFriend || id != 999 {
		t.Fatalf("explicit resolve wrong: %s %d", tt, id)
	}
	// 类型对但 id 缺失 → 回退
	if tt, id := b.resolveTarget(clockTargetFriend, 0); tt != clockTargetGroup || id != 123 {
		t.Fatalf("half resolve wrong: %s %d", tt, id)
	}
}
