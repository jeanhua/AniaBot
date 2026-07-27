package adminpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jeanhua/AniaBot/bot/core/configstore"
	"github.com/jeanhua/AniaBot/common/storage"
)

// ---- 测试用内存持久化存储（core 包的实现会引入 import 环，这里写个极简版） ----

type fakePersistent struct {
	mu     *sync.Mutex
	data   map[string]string
	prefix string
}

func newFakePersistent() *fakePersistent {
	return &fakePersistent{mu: &sync.Mutex{}, data: map[string]string{}}
}

func (f *fakePersistent) key(k string) string { return f.prefix + k }

func (f *fakePersistent) GetString(_ context.Context, key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.data[f.key(key)]
	return v, ok
}

func (f *fakePersistent) SetString(_ context.Context, key, val string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data[f.key(key)] = val
	return true
}

func (f *fakePersistent) Get(ctx context.Context, key string, out any) bool {
	raw, ok := f.GetString(ctx, key)
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(raw), out) == nil
}

func (f *fakePersistent) Set(ctx context.Context, key string, val any) bool {
	data, err := json.Marshal(val)
	if err != nil {
		return false
	}
	return f.SetString(ctx, key, string(data))
}

func (f *fakePersistent) Has(ctx context.Context, key string) bool {
	_, ok := f.GetString(ctx, key)
	return ok
}

func (f *fakePersistent) Del(_ context.Context, key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.data, f.key(key))
	return true
}

func (f *fakePersistent) Keys(_ context.Context, prefix string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.data {
		if rel, ok := strings.CutPrefix(k, f.prefix); ok && strings.HasPrefix(rel, prefix) {
			out = append(out, rel)
		}
	}
	return out, nil
}

func (f *fakePersistent) Clear(_ context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.data {
		if strings.HasPrefix(k, f.prefix) {
			delete(f.data, k)
		}
	}
	return true
}

func (f *fakePersistent) Clone(prefix string) storage.PersistentStorage {
	return &fakePersistent{mu: f.mu, data: f.data, prefix: f.prefix + prefix + "/"}
}

// ---- 测试 ----

func newBalanceTestServer(t *testing.T) *Server {
	t.Helper()
	store := configstore.New(newFakePersistent(), nil)
	return &Server{opt: Options{Config: store, Logger: slog.Default()}}
}

// 模拟 DeepSeek 风格余额接口
func fakeBalanceAPI(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"balances":[{"currency":"CNY","total_balance":"42.50"}]}}`)
	}))
}

func TestRunBalanceScript(t *testing.T) {
	api := fakeBalanceAPI(t)
	defer api.Close()

	s := newBalanceTestServer(t)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", api.URL)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "test-key")

	got, err := s.runBalanceScript(DefaultBalanceJS)
	if err != nil {
		t.Fatalf("默认脚本执行失败: %v", err)
	}
	if got != "¥ 42.50" {
		t.Fatalf("余额显示不符: got %q", got)
	}
}

func TestRunBalanceScriptError(t *testing.T) {
	s := newBalanceTestServer(t)

	// 无返回值
	if _, err := s.runBalanceScript(`const a = 1`); err == nil {
		t.Fatal("无返回值应当报错")
	}
	// 脚本抛异常
	if _, err := s.runBalanceScript(`throw new Error("boom")`); err == nil {
		t.Fatal("脚本抛异常应当报错")
	}
	// HTTP 错误被脚本捕获并抛出
	api := fakeBalanceAPI(t)
	defer api.Close()
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", api.URL)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "wrong-key")
	if _, err := s.runBalanceScript(DefaultBalanceJS); err == nil {
		t.Fatal("鉴权失败应当报错")
	}
}

func TestBalanceDisplay(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{float64(42.5), "42.5"},
		{float64(100), "100"},
		{int64(7), "7"},
		{"¥ 9.9", "¥ 9.9"},
		{map[string]any{"a": 1}, `{"a":1}`},
	}
	for _, c := range cases {
		got, err := balanceDisplay(c.in)
		if err != nil {
			t.Fatalf("balanceDisplay(%v) 报错: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("balanceDisplay(%v) = %q, want %q", c.in, got, c.want)
		}
	}
	if _, err := balanceDisplay(nil); err == nil {
		t.Fatal("nil 应当报错")
	}
}

func TestQueryBalanceDisabledAndCache(t *testing.T) {
	s := newBalanceTestServer(t)

	// 未启用
	if res := s.queryBalance(true); res.Enabled {
		t.Fatal("未启用时 Enabled 应为 false")
	}

	// 启用但 base_url 指向不可用地址：报错不 panic，且写入缓存
	_ = s.opt.Config.Set("bot.balance.enable", true)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", "http://127.0.0.1:1")
	_ = s.opt.Config.Set("bot.balance.cache_sec", 60)
	res := s.queryBalance(true)
	if !res.Enabled || res.Error == "" {
		t.Fatalf("查询失败应带 error: %+v", res)
	}
	if res2 := s.queryBalance(false); !res2.Cached {
		t.Fatal("TTL 内第二次查询应命中缓存")
	}
}
