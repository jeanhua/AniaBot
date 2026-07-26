package configstore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/jeanhua/AniaBot/common/storage"
)

// memPersistent 内存版 PersistentStorage，命名空间语义与 SQL 实现一致
// （Clone 追加 "prefix:"），供测试使用。
type memPersistent struct {
	mu   sync.RWMutex
	ns   string
	data map[string]string
}

func newMemPersistent() *memPersistent {
	return &memPersistent{data: map[string]string{}}
}

func (m *memPersistent) fullKey(key string) string { return m.ns + key }

func (m *memPersistent) GetString(_ context.Context, key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[m.fullKey(key)]
	return v, ok
}

func (m *memPersistent) SetString(_ context.Context, key, val string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[m.fullKey(key)] = val
	return true
}

func (m *memPersistent) Get(ctx context.Context, key string, out any) bool {
	raw, ok := m.GetString(ctx, key)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(raw), out) == nil
}

func (m *memPersistent) Set(ctx context.Context, key string, val any) bool {
	data, err := json.Marshal(val)
	if err != nil {
		return false
	}
	return m.SetString(ctx, key, string(data))
}

func (m *memPersistent) Has(ctx context.Context, key string) bool {
	_, ok := m.GetString(ctx, key)
	return ok
}

func (m *memPersistent) Del(_ context.Context, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, m.fullKey(key))
	return true
}

func (m *memPersistent) Keys(_ context.Context, prefix string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []string
	full := m.fullKey(prefix)
	for k := range m.data {
		if strings.HasPrefix(k, full) {
			out = append(out, strings.TrimPrefix(k, m.ns))
		}
	}
	return out, nil
}

func (m *memPersistent) Clear(_ context.Context) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.data {
		if strings.HasPrefix(k, m.ns) {
			delete(m.data, k)
		}
	}
	return true
}

func (m *memPersistent) Clone(prefix string) storage.PersistentStorage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &memPersistent{ns: m.ns + prefix + ":", data: m.data}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return New(newMemPersistent(), nil)
}

func TestSeedDefaults(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	v := s.ToViper()
	if got := v.GetString("bot.adapter.ws.address"); got != "ws://localhost:4455" {
		t.Fatalf("ws.address = %q", got)
	}
	if got := v.GetString("plugin.interceptor.mode"); got != "blacklist" {
		t.Fatalf("interceptor.mode = %q", got)
	}
	if !v.GetBool("bot.admin_panel.enable") {
		t.Fatal("bot.admin_panel.enable should default to true")
	}
	if v.IsSet("bot.admin_id") {
		t.Fatal("admin_id 为空时不应被写入（IsSet 语义）")
	}
	// 全新安装应标记待完成设置向导
	if !s.SetupPending() {
		t.Fatal("全新安装应标记 SetupPending")
	}
	s.CompleteSetup()
	if s.SetupPending() {
		t.Fatal("CompleteSetup 后应清除标记")
	}
	// 再次 Init 应幂等
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureDefaults(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	// 键不存在时写入默认值
	s.EnsureDefaults(map[string]any{
		"plugin.test.model":       "deepseek-chat",
		"plugin.test.max_token":   8192,
		"plugin.test.temperature": 1.2,
		"plugin.test.tags":        []string{"a", "b"},
		"plugin.test.nodefault":   nil, // nil 默认值跳过
	})
	v := s.ToViper()
	if got := v.GetString("plugin.test.model"); got != "deepseek-chat" {
		t.Fatalf("model = %q", got)
	}
	if got := v.GetInt("plugin.test.max_token"); got != 8192 {
		t.Fatalf("max_token = %d", got)
	}
	if got := v.GetFloat64("plugin.test.temperature"); got != 1.2 {
		t.Fatalf("temperature = %v", got)
	}
	if got := v.GetStringSlice("plugin.test.tags"); len(got) != 2 || got[0] != "a" {
		t.Fatalf("tags = %v", got)
	}
	if v.IsSet("plugin.test.nodefault") {
		t.Fatal("nil 默认值不应写入")
	}
	// 已存在的键不覆盖
	if err := s.Set("plugin.test.model", "gpt-4o"); err != nil {
		t.Fatal(err)
	}
	s.EnsureDefaults(map[string]any{"plugin.test.model": "other"})
	if got := s.ToViper().GetString("plugin.test.model"); got != "gpt-4o" {
		t.Fatalf("已存在的键被覆盖: %q", got)
	}
}

func TestSetGetRoundTrip(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("plugin.ai_chat_bot.model", "gpt-4o"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("plugin.dailyNews.groups", []any{float64(123), float64(456)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("plugin.ai_chat_bot.multimodal", true); err != nil {
		t.Fatal(err)
	}
	v := s.ToViper()
	if got := v.GetString("plugin.ai_chat_bot.model"); got != "gpt-4o" {
		t.Fatalf("model = %q", got)
	}
	if got := v.GetIntSlice("plugin.dailyNews.groups"); len(got) != 2 || got[0] != 123 || got[1] != 456 {
		t.Fatalf("groups = %v", got)
	}
	if !v.GetBool("plugin.ai_chat_bot.multimodal") {
		t.Fatal("multimodal should be true")
	}
}
