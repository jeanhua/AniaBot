package tasklog

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/storage"

	_ "modernc.org/sqlite"
)

// sqlFakeStore 包装 fakeStore 并附加 SQL 能力，使 New() 探测走 SQL 后端。
type sqlFakeStore struct {
	*fakeStore
	db *sql.DB
}

func (s *sqlFakeStore) SQLDB() *sql.DB                 { return s.db }
func (s *sqlFakeStore) SQLDialect() storage.SQLDialect { return storage.SQLDialectSQLite }

// newSQLLogger 构造走 SQL 后端的 Logger（sqlite :memory:，单连接）。
func newSQLLogger(t *testing.T, maxEntries int) (*Logger, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	return New(&sqlFakeStore{fakeStore: newFakeStore(), db: db}, maxEntries, nil), db
}

func TestSQLBackendRecordRecent(t *testing.T) {
	l, _ := newSQLLogger(t, 3)

	l.Record(Entry{TaskID: "t1", Status: StatusSuccess, TaskTitle: "A"})
	l.Record(Entry{TaskID: "t2", Status: StatusTimeout, TaskTitle: "B"})
	e3 := l.Record(Entry{TaskID: "t3", Status: StatusRunning, TaskTitle: "C"})

	got := l.Recent(10)
	if len(got) != 3 || got[0].TaskID != "t3" || got[2].TaskID != "t1" {
		t.Fatalf("Recent 顺序异常: %+v", got)
	}
	// running 状态不写 FinishedAt
	if !got[0].FinishedAt.IsZero() {
		t.Fatalf("running 状态不应有完成时间: %+v", got[0])
	}

	// Update 全字段往返
	l.Update(e3.ID, func(en *Entry) {
		en.Status = StatusSuccess
		en.DurationMs = 42
		en.Iterations = 2
		en.Reply = "任务完成"
		en.ToolCalls = []ToolCallRecord{{Name: "web_search", Arguments: `{"q":"x"}`, Result: "ok", DurationMs: 30}}
		en.PromptTokens, en.TotalTokens = 200, 260
		en.FinishedAt = time.Now()
	})
	for _, x := range l.Recent(0) {
		if x.ID == e3.ID {
			if x.Status != StatusSuccess || x.DurationMs != 42 || x.Iterations != 2 || x.Reply != "任务完成" {
				t.Fatalf("Update 未生效: %+v", x)
			}
			if len(x.ToolCalls) != 1 || x.ToolCalls[0].Name != "web_search" {
				t.Fatalf("工具调用明细未保存: %+v", x.ToolCalls)
			}
			if x.PromptTokens != 200 || x.TotalTokens != 260 {
				t.Fatalf("token 统计未保存: %+v", x)
			}
			return
		}
	}
	t.Fatalf("entry %s not found after update", e3.ID)
}

func TestSQLBackendRollingCap(t *testing.T) {
	l, db := newSQLLogger(t, 2)
	for i := 0; i < 5; i++ {
		l.Record(Entry{TaskID: "t", Status: StatusSuccess})
	}
	if got := l.Recent(0); len(got) != 2 {
		t.Fatalf("want capped to 2, got %d", len(got))
	}
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM ania_task_log`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("表中行数 = %d,%v want 2", n, err)
	}
}

func TestSQLBackendSeqAcrossReload(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	store := &sqlFakeStore{fakeStore: newFakeStore(), db: db}
	l1 := New(store, 10, nil)
	e1 := l1.Record(Entry{TaskID: "t1"})

	l2 := New(store, 10, nil)
	e2 := l2.Record(Entry{TaskID: "t2"})
	if e2.ID == e1.ID {
		t.Fatalf("重启后 ID 冲突: %q", e2.ID)
	}
	if got := l2.Recent(0); len(got) != 2 {
		t.Fatalf("重启后历史应保留, got %d 条", len(got))
	}
}

func TestSQLBackendQueryAndRecentFor(t *testing.T) {
	l, _ := newSQLLogger(t, 100)
	base := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	l.Record(Entry{TriggerTime: base.Add(-2 * time.Hour), TaskID: "a", TaskTitle: "每日新闻推送", TargetType: "group", TargetID: "1", Status: StatusSuccess})
	l.Record(Entry{TriggerTime: base.Add(-1 * time.Hour), TaskID: "b", TaskTitle: "周报总结", TargetType: "group", TargetID: "1", Status: StatusTimeout})
	l.Record(Entry{TriggerTime: base, TaskID: "a", TaskTitle: "每日新闻推送", TargetType: "friend", TargetID: "2", Status: StatusError})

	cases := []struct {
		name   string
		filter Filter
		want   int
	}{
		{"全部", Filter{Limit: 10}, 3},
		{"按任务", Filter{TaskID: "a"}, 2},
		{"按对象类型", Filter{TargetType: "group"}, 2},
		{"按对象", Filter{TargetType: "group", TargetID: "1"}, 2},
		{"按状态", Filter{Status: StatusTimeout}, 1},
		{"按起始时间", Filter{Start: base.Add(-90 * time.Minute)}, 2},
		{"按截止时间", Filter{End: base.Add(-90 * time.Minute)}, 1},
		{"按关键词", Filter{Keyword: "新闻"}, 2},
		{"组合条件", Filter{TaskID: "a", Status: StatusError, TargetType: "friend"}, 1},
		{"无命中", Filter{TaskID: "zzz"}, 0},
		{"limit 截断", Filter{Limit: 1}, 1},
	}
	for _, c := range cases {
		if got := len(l.Query(c.filter)); got != c.want {
			t.Errorf("%s: 期望 %d 条，实际 %d 条", c.name, c.want, got)
		}
	}

	// RecentForTask / RecentForTarget：新在前
	if got := l.RecentForTask("a", 0); len(got) != 2 || got[0].Status != StatusError {
		t.Fatalf("RecentForTask 异常: %+v", got)
	}
	if got := l.RecentForTarget("group", "1", 0); len(got) != 2 || got[0].TaskID != "b" {
		t.Fatalf("RecentForTarget 异常: %+v", got)
	}
}

func TestSQLBackendQueryBeforeCursor(t *testing.T) {
	l, _ := newSQLLogger(t, 10)
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		l.Record(Entry{TaskID: "t", Status: StatusSuccess, TaskTitle: title})
	}
	all := l.Query(Filter{Limit: 10})
	if len(all) != 5 {
		t.Fatalf("want 5 entries, got %d", len(all))
	}
	cursor := all[2].ID
	page := l.Query(Filter{Before: cursor, Limit: 10})
	if len(page) != 2 || page[0].ID != all[3].ID || page[1].ID != all[4].ID {
		t.Fatalf("cursor page wrong: %+v", page)
	}
}

// TestBackendConformance 同一操作序列分别作用于 KV 与 SQL 后端，结果应一致。
func TestBackendConformance(t *testing.T) {
	kv := New(newFakeStore(), 3, nil)
	sqlm, _ := newSQLLogger(t, 3)

	base := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	inputs := []Entry{
		{TriggerTime: base, TaskID: "a", TaskTitle: "任务一", TargetType: "group", TargetID: "1", Status: StatusSuccess},
		{TriggerTime: base.Add(time.Hour), TaskID: "b", TaskTitle: "任务二", TargetType: "friend", TargetID: "2", Status: StatusTimeout},
		{TriggerTime: base.Add(2 * time.Hour), TaskID: "a", TaskTitle: "任务一", TargetType: "group", TargetID: "1", Status: StatusError},
		{TriggerTime: base.Add(3 * time.Hour), TaskID: "c", TaskTitle: "任务三", TargetType: "group", TargetID: "3", Status: StatusSuccess},
	}
	var ids []string
	for _, e := range inputs {
		e1 := kv.Record(e)
		e2 := sqlm.Record(e)
		if e1.ID != e2.ID {
			t.Fatalf("ID 分配不一致: kv=%q sql=%q", e1.ID, e2.ID)
		}
		ids = append(ids, e1.ID)
	}

	compare := func(name string, a, b []Entry) {
		t.Helper()
		if len(a) != len(b) {
			t.Fatalf("%s: 条数不一致 kv=%d sql=%d", name, len(a), len(b))
		}
		for i := range a {
			if a[i].ID != b[i].ID || a[i].TaskID != b[i].TaskID || a[i].Status != b[i].Status {
				t.Fatalf("%s 第 %d 条不一致:\nkv  %+v\nsql %+v", name, i, a[i], b[i])
			}
		}
	}
	compare("Recent", kv.Recent(0), sqlm.Recent(0))
	compare("Query-任务", kv.Query(Filter{TaskID: "a"}), sqlm.Query(Filter{TaskID: "a"}))
	compare("RecentForTarget", kv.RecentForTarget("group", "1", 0), sqlm.RecentForTarget("group", "1", 0))

	for _, l := range []*Logger{kv, sqlm} {
		l.Update(ids[1], func(en *Entry) { en.Status = StatusSuccess })
	}
	compare("Update后", kv.Recent(0), sqlm.Recent(0))
}
