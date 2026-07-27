package adminpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func TestRunBalanceQuery(t *testing.T) {
	api := fakeBalanceAPI(t)
	defer api.Close()

	s := newBalanceTestServer(t)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", api.URL)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "test-key")

	got, err := s.runBalanceQuery()
	if err != nil {
		t.Fatalf("默认配置查询失败: %v", err)
	}
	if got != "¥ 42.50" {
		t.Fatalf("余额显示不符: got %q", got)
	}
}

func TestRunBalanceQueryCustom(t *testing.T) {
	// POST + 自定义请求头 + 自定义模板
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-Key") != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "gpt-test") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"quota":7.5,"unit":"USD"}`)
	}))
	defer api.Close()

	s := newBalanceTestServer(t)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", api.URL)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "test-key")
	_ = s.opt.Config.Set("plugin.ai_chat_bot.model", "gpt-test")
	_ = s.opt.Config.Set("bot.balance.method", "POST")
	_ = s.opt.Config.Set("bot.balance.headers", `{"X-Key":"${api_key}"}`)
	_ = s.opt.Config.Set("bot.balance.body", `{"model":"${model}"}`)
	_ = s.opt.Config.Set("bot.balance.format", `${quota} {unit}`)

	got, err := s.runBalanceQuery()
	if err != nil {
		t.Fatalf("自定义配置查询失败: %v", err)
	}
	if got != "$7.5 USD" {
		t.Fatalf("余额显示不符: got %q", got)
	}
}

func TestRunBalanceQueryError(t *testing.T) {
	s := newBalanceTestServer(t)

	// 鉴权失败（非 2xx）
	api := fakeBalanceAPI(t)
	defer api.Close()
	_ = s.opt.Config.Set("plugin.ai_chat_bot.base_url", api.URL)
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "wrong-key")
	if _, err := s.runBalanceQuery(); err == nil {
		t.Fatal("鉴权失败应当报错")
	}

	// 显示模板路径不存在
	_ = s.opt.Config.Set("plugin.ai_chat_bot.api_key", "test-key")
	_ = s.opt.Config.Set("bot.balance.format", "{data.nope}")
	if _, err := s.runBalanceQuery(); err == nil {
		t.Fatal("路径不存在应当报错")
	}

	// 响应不是合法 JSON
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer bad.Close()
	s2 := newBalanceTestServer(t)
	_ = s2.opt.Config.Set("plugin.ai_chat_bot.base_url", bad.URL)
	_ = s2.opt.Config.Set("bot.balance.url", "${base_url}")
	_ = s2.opt.Config.Set("bot.balance.headers", "")
	if _, err := s2.runBalanceQuery(); err == nil {
		t.Fatal("非法 JSON 应当报错")
	}
}

func TestRenderBalanceFormat(t *testing.T) {
	body := []byte(`{"a":{"b":42.50},"s":"hello","n":100}`)
	cases := []struct {
		format string
		want   string
	}{
		{"¥ {a.b}", "¥ 42.50"},   // 数字保留原始字面量
		{"{s}", "hello"},         // 字符串不带引号
		{"{n} 元", "100 元"},       // 整数
		{"{s} {n}", "hello 100"}, // 多占位符
		{"固定文本", "固定文本"},         // 无占位符原样输出
	}
	for _, c := range cases {
		got, err := renderBalanceFormat(c.format, body)
		if err != nil {
			t.Fatalf("renderBalanceFormat(%q) 报错: %v", c.format, err)
		}
		if got != c.want {
			t.Fatalf("renderBalanceFormat(%q) = %q, want %q", c.format, got, c.want)
		}
	}
	if _, err := renderBalanceFormat("{a.nope}", body); err == nil {
		t.Fatal("路径不存在应当报错")
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
