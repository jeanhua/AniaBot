package querylog

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

func TestRecordAndRecent(t *testing.T) {
	l := New(newFakeStore(), 3, nil)

	e1 := l.Record(Entry{ChatType: "group", TargetID: "10001", Query: "你好"})
	e2 := l.Record(Entry{ChatType: "friend", TargetID: "20002", Query: "在吗"})

	if e1.ID == "" || e2.ID == "" || e1.ID == e2.ID {
		t.Fatalf("ID 分配异常: %q %q", e1.ID, e2.ID)
	}
	if e1.Status != StatusRunning {
		t.Fatalf("默认状态应为 running，实际 %q", e1.Status)
	}

	recent := l.Recent(0)
	if len(recent) != 2 || recent[0].ID != e2.ID || recent[1].ID != e1.ID {
		t.Fatalf("Recent 顺序异常: %+v", recent)
	}
}

func TestRecordCapacity(t *testing.T) {
	l := New(newFakeStore(), 2, nil)
	l.Record(Entry{Query: "q1"})
	l.Record(Entry{Query: "q2"})
	l.Record(Entry{Query: "q3"})

	recent := l.Recent(0)
	if len(recent) != 2 {
		t.Fatalf("应滚动保留 2 条，实际 %d 条", len(recent))
	}
	if recent[0].Query != "q3" || recent[1].Query != "q2" {
		t.Fatalf("应保留最新的两条: %+v", recent)
	}
}

func TestUpdate(t *testing.T) {
	l := New(newFakeStore(), 10, nil)
	e := l.Record(Entry{Query: "q"})

	l.Update(e.ID, func(en *Entry) {
		en.Status = StatusSuccess
		en.DurationMs = 1234
		en.ToolCalls = []ToolCallRecord{{Name: "bash", Arguments: "ls", Result: "ok", DurationMs: 12}}
	})

	got := l.Recent(1)[0]
	if got.Status != StatusSuccess || got.DurationMs != 1234 {
		t.Fatalf("Update 未生效: %+v", got)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "bash" {
		t.Fatalf("工具调用明细未保存: %+v", got.ToolCalls)
	}

	// 未找到的 ID 应静默忽略
	l.Update("不存在的id", func(en *Entry) { en.Status = StatusError })
	if l.Recent(1)[0].Status != StatusSuccess {
		t.Fatal("不存在的 ID 不应影响已有记录")
	}
}

func TestSeqPersistAcrossReload(t *testing.T) {
	store := newFakeStore()
	l1 := New(store, 10, nil)
	e := l1.Record(Entry{Query: "q"})

	// 模拟重启：同一存储重建 Logger，序号应继续递增而非重置
	l2 := New(store, 10, nil)
	e2 := l2.Record(Entry{Query: "q2"})
	if e2.ID == e.ID {
		t.Fatalf("重启后 ID 冲突: %q", e2.ID)
	}
}

func TestMigrateLegacyEntries(t *testing.T) {
	store := newFakeStore()
	// 模拟旧版数据：entries 键存整体数组，ID 为序号 base36（1→"1", 2→"2"）
	legacy := []Entry{
		{ID: "2", Query: "较新", Status: StatusSuccess},
		{ID: "1", Query: "较旧", Status: StatusSuccess},
	}
	store.Set(context.Background(), "entries", legacy)
	store.SetString(context.Background(), "seq", "2")

	l := New(store, 10, nil)

	// 旧键应被删除，数据拆分为逐条记录
	if store.Has(context.Background(), "entries") {
		t.Fatal("迁移后旧 entries 键应被删除")
	}
	recent := l.Recent(0)
	if len(recent) != 2 || recent[0].Query != "较新" || recent[1].Query != "较旧" {
		t.Fatalf("迁移后数据异常: %+v", recent)
	}

	// 序号应延续，新记录不与旧记录冲突
	e := l.Record(Entry{Query: "新"})
	if e.ID == "1" || e.ID == "2" {
		t.Fatalf("迁移后 ID 冲突: %q", e.ID)
	}
	if got := l.Recent(1)[0].Query; got != "新" {
		t.Fatalf("新记录应在最前，实际 %q", got)
	}

	// Update 应能命中迁移过来的记录
	l.Update("1", func(en *Entry) { en.Status = StatusError })
	for _, en := range l.Recent(0) {
		if en.ID == "1" && en.Status != StatusError {
			t.Fatalf("迁移记录的 Update 未生效: %+v", en)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("你好世界", 10); got != "你好世界" {
		t.Fatalf("未超长不应截断: %q", got)
	}
	if got := Truncate("你好世界啊", 3); got != "你好世…" {
		t.Fatalf("应按符文截断: %q", got)
	}
	if got := Truncate("abc", 0); got != "abc" {
		t.Fatalf("max<=0 不应截断: %q", got)
	}
}

func TestQueryFilter(t *testing.T) {
	l := New(newFakeStore(), 100, nil)
	base := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	l.Record(Entry{Time: base.Add(-2 * time.Hour), ChatType: "group", TargetID: "10001", Senders: []string{"111"}, Query: "今天天气怎么样"})
	l.Record(Entry{Time: base.Add(-1 * time.Hour), ChatType: "group", TargetID: "10001", Senders: []string{"222", "111"}, Query: "帮我看看新闻"})
	l.Record(Entry{Time: base, ChatType: "friend", TargetID: "333", Senders: []string{"333"}, Query: "天气如何"})

	cases := []struct {
		name   string
		filter Filter
		want   int
	}{
		{"全部", Filter{Limit: 10}, 3},
		{"按类型群聊", Filter{ChatType: "group"}, 2},
		{"按类型私聊", Filter{ChatType: "friend"}, 1},
		{"按目标", Filter{TargetID: "333"}, 1},
		{"按触发人（命中批次任一发言者）", Filter{Sender: "222"}, 1},
		{"按触发人（多条命中）", Filter{Sender: "111"}, 2},
		{"按起始时间", Filter{Start: base.Add(-90 * time.Minute)}, 2},
		{"按截止时间", Filter{End: base.Add(-90 * time.Minute)}, 1},
		{"按时间区间", Filter{Start: base.Add(-2 * time.Hour), End: base.Add(-1 * time.Hour)}, 2},
		{"按关键词", Filter{Keyword: "天气"}, 2},
		{"关键词不区分大小写", Filter{Keyword: "NEWS"}, 0},
		{"组合条件", Filter{ChatType: "group", Sender: "111", Keyword: "新闻"}, 1},
		{"无命中", Filter{Sender: "999"}, 0},
		{"limit 截断", Filter{Limit: 1}, 1},
	}
	for _, c := range cases {
		if got := len(l.Query(c.filter)); got != c.want {
			t.Errorf("%s: 期望 %d 条，实际 %d 条", c.name, c.want, got)
		}
	}

	// 结果保持新在前
	got := l.Query(Filter{ChatType: "group"})
	if len(got) != 2 || !got[0].Time.After(got[1].Time) {
		t.Errorf("Query 结果应按时间倒序: %+v", got)
	}
}

func TestQueryBeforeCursor(t *testing.T) {
	l := New(newFakeStore(), 10, nil)
	for _, q := range []string{"a", "b", "c", "d", "e"} {
		l.Record(Entry{ChatType: "group", TargetID: "1", Query: q})
	}
	all := l.Query(Filter{Limit: 10})
	if len(all) != 5 {
		t.Fatalf("want 5 entries, got %d", len(all))
	}
	// 以中间记录为游标，应只返回比它更旧的两条
	cursor := all[2].ID
	page := l.Query(Filter{Before: cursor, Limit: 10})
	if len(page) != 2 || page[0].ID != all[3].ID || page[1].ID != all[4].ID {
		t.Fatalf("cursor page wrong: %+v", page)
	}
	// 非法游标不生效，从最新开始
	bad := l.Query(Filter{Before: "!!", Limit: 1})
	if len(bad) != 1 || bad[0].ID != all[0].ID {
		t.Fatalf("invalid cursor should be ignored: %+v", bad)
	}
}
