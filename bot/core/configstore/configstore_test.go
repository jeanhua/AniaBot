package configstore

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	// 独立临时目录，避免触达真实工作目录下的旧配置文件
	t.Chdir(t.TempDir())
	return New(newMemPersistent(), nil)
}

func TestSeedDefaults(t *testing.T) {
	s := newTestStore(t)
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	v := s.ToViper()
	if got := v.GetString("plugin.ai_chat_bot.model"); got != "deepseek-chat" {
		t.Fatalf("model = %q", got)
	}
	if got := v.GetInt("plugin.ai_chat_bot.max_token"); got != 8192 {
		t.Fatalf("max_token = %d", got)
	}
	if got := v.GetFloat64("plugin.ai_chat_bot.temperature"); got != 1.2 {
		t.Fatalf("temperature = %v", got)
	}
	if got := v.GetStringSlice("plugin.interceptor.whitelist.users"); len(got) != 1 || got[0] != "all" {
		t.Fatalf("whitelist.users = %v", got)
	}
	if !v.GetBool("bot.admin_panel.enable") {
		t.Fatal("bot.admin_panel.enable should default to true")
	}
	if v.IsSet("bot.admin_id") {
		t.Fatal("admin_id 为空时不应被写入（IsSet 语义）")
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

func TestMigrateFromLegacyYAML(t *testing.T) {
	s := newTestStore(t)
	legacy := `bot:
  admin_id: 10001
  adapter:
    ws:
      address: ws://example:4455
  store:
    persistent:
      driver: mysql
      mysql:
        dsn: "root@tcp(localhost)/x"
plugin:
  ai_chat_bot:
    base_url: "https://api.example.com"
    api_key: "sk-test"
    model: "my-model"
`
	if err := os.WriteFile("config.yaml", []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("aniabot.mcp.json", []byte(`{"servers":[{"name":"demo","transport":"stdio","command":"echo"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	v := s.ToViper()
	if got := v.GetInt64("bot.admin_id"); got != 10001 {
		t.Fatalf("admin_id = %d", got)
	}
	if got := v.GetString("plugin.ai_chat_bot.api_key"); got != "sk-test" {
		t.Fatalf("api_key = %q", got)
	}
	// bot.store.persistent.* 已改由环境变量引导，迁移时应丢弃
	if v.IsSet("bot.store.persistent.driver") {
		t.Fatal("persistent 配置不应迁入配置中心")
	}
	// 旧配置缺失的新增键应以默认值补齐
	if !v.GetBool("bot.admin_panel.enable") {
		t.Fatal("迁移后应以默认值补齐 bot.admin_panel.enable")
	}
	if got := v.GetInt("plugin.ai_chat_bot.max_token"); got != 8192 {
		t.Fatalf("迁移后应以默认值补齐 max_token, got %d", got)
	}
	// MCP JSON 原文应迁入 files.mcp_json
	if got := v.GetString(KeyMCPJSON); got == "" {
		t.Fatal("files.mcp_json 应包含迁移内容")
	}
	// 旧文件应更名为 .bak
	if _, err := os.Stat("config.yaml.bak"); err != nil {
		t.Fatal("config.yaml 应更名为 .bak")
	}
	if _, err := os.Stat("aniabot.mcp.json.bak"); err != nil {
		t.Fatal("aniabot.mcp.json 应更名为 .bak")
	}

	// 再次 Init 应为幂等（不重复迁移）
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
}

func TestMigratePrefersDevConfig(t *testing.T) {
	s := newTestStore(t)
	if err := os.WriteFile("config.dev.yaml", []byte("plugin:\n  ai_chat_bot:\n    model: dev-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("config.yaml", []byte("plugin:\n  ai_chat_bot:\n    model: prod-model\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Init(); err != nil {
		t.Fatal(err)
	}
	if got := s.ToViper().GetString("plugin.ai_chat_bot.model"); got != "dev-model" {
		t.Fatalf("应优先迁移 config.dev.yaml, model = %q", got)
	}
	// 两份文件都应更名，避免下次启动混淆
	for _, f := range []string{"config.dev.yaml.bak", "config.yaml.bak"} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("%s 应存在", filepath.Base(f))
		}
	}
}
