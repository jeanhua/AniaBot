package oplog

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"
)

// fakeStore 进程内 PersistentStorage 实现，仅用于单测。
type fakeStore struct {
	data map[string]string
}

func newFakeStore() *fakeStore { return &fakeStore{data: map[string]string{}} }

func (s *fakeStore) GetString(_ context.Context, key string) (string, bool) {
	v, ok := s.data[key]
	return v, ok
}
func (s *fakeStore) SetString(_ context.Context, key, val string) bool {
	s.data[key] = val
	return true
}
func (s *fakeStore) Get(_ context.Context, key string, out any) bool {
	v, ok := s.data[key]
	if !ok {
		return false
	}
	return json.Unmarshal([]byte(v), out) == nil
}
func (s *fakeStore) Set(_ context.Context, key string, val any) bool {
	b, err := json.Marshal(val)
	if err != nil {
		return false
	}
	s.data[key] = string(b)
	return true
}
func (s *fakeStore) Has(_ context.Context, key string) bool { _, ok := s.data[key]; return ok }
func (s *fakeStore) Del(_ context.Context, key string) bool {
	delete(s.data, key)
	return true
}
func (s *fakeStore) Keys(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	for k := range s.data {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	return keys, nil
}
func (s *fakeStore) Clear(_ context.Context) bool             { s.data = map[string]string{}; return true }
func (s *fakeStore) Clone(_ string) storage.PersistentStorage { return s } // 测试不复用前缀

func TestRecordAndQuery(t *testing.T) {
	Init(newFakeStore(), 10, nil)

	e1 := Record(CategoryAuth, "login", "面板登录成功，IP: 127.0.0.1")
	e2 := Record(CategoryConfig, "config_update", "面板更新配置: plugin.ai_chat_bot.model")
	Record(CategoryAI, "config_set", "AI 修改配置 plugin.ai_chat_bot.model")

	if e1.ID == "" || e2.ID == "" || e1.ID == e2.ID {
		t.Fatalf("ID 分配异常: %q %q", e1.ID, e2.ID)
	}
	if e1.Time.IsZero() {
		t.Fatal("时间未填充")
	}

	all := Query(Filter{})
	if len(all) != 3 {
		t.Fatalf("期望 3 条日志，实际 %d", len(all))
	}
	// 新在前
	if all[0].Action != "config_set" || all[2].Action != "login" {
		t.Fatalf("排序错误（应新在前）: %v", []string{all[0].Action, all[1].Action, all[2].Action})
	}

	// 分类过滤
	authOnly := Query(Filter{Category: CategoryAuth})
	if len(authOnly) != 1 || authOnly[0].Action != "login" {
		t.Fatalf("分类过滤异常: %v", authOnly)
	}

	// 关键词过滤（不区分大小写，命中操作名或详情）
	kw := Query(Filter{Keyword: "ai "})
	if len(kw) != 1 || kw[0].Category != CategoryAI {
		t.Fatalf("关键词过滤异常: %v", kw)
	}

	// 时间过滤
	future := Query(Filter{Start: time.Now().Add(time.Hour)})
	if len(future) != 0 {
		t.Fatalf("时间过滤异常: %v", future)
	}

	// 分页游标：仅返回比 before 更旧的记录
	page := Query(Filter{Before: all[0].ID})
	if len(page) != 2 || page[0].ID == all[0].ID {
		t.Fatalf("游标分页异常: %v", page)
	}
}

func TestEvict(t *testing.T) {
	Init(newFakeStore(), 5, nil)
	for i := 0; i < 8; i++ {
		Record(CategorySystem, "start", "启动")
	}
	all := Query(Filter{})
	if len(all) != 5 {
		t.Fatalf("容量淘汰异常：期望 5 条，实际 %d", len(all))
	}
}

func TestRecordBeforeInit(t *testing.T) {
	// 未初始化（be 重置为 nil 模拟）时 Record 静默丢弃
	mu.Lock()
	saved := be
	be = nil
	mu.Unlock()
	defer func() {
		mu.Lock()
		be = saved
		mu.Unlock()
	}()

	if e := Record(CategorySystem, "start", "x"); e.ID != "" {
		t.Fatal("未初始化时 Record 应返回零值 Entry")
	}
	if got := Query(Filter{}); got != nil {
		t.Fatal("未初始化时 Query 应返回 nil")
	}
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("字", MaxDetailRunes+10)
	got := Truncate(s, MaxDetailRunes)
	if len([]rune(got)) != MaxDetailRunes+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("截断异常: rune 数 %d", len([]rune(got)))
	}
	if Truncate("短", 10) != "短" {
		t.Fatal("未超限不应截断")
	}
}
