package querylog

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
	// 完整字段往返（含工具调用明细与 token 统计）
	l.Update(e1.ID, func(en *Entry) {
		en.Status = StatusSuccess
		en.DurationMs = 1234
		en.Iterations = 3
		en.ToolCalls = []ToolCallRecord{{Name: "bash", Arguments: "ls", Result: "ok", DurationMs: 12}}
		en.PromptTokens, en.CompletionTokens, en.TotalTokens = 100, 50, 150
	})
	got := l.Recent(0)[1]
	if got.Status != StatusSuccess || got.DurationMs != 1234 || got.Iterations != 3 {
		t.Fatalf("Update 未生效: %+v", got)
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "bash" {
		t.Fatalf("工具调用明细未保存: %+v", got.ToolCalls)
	}
	if got.PromptTokens != 100 || got.TotalTokens != 150 {
		t.Fatalf("token 统计未保存: %+v", got)
	}
}

func TestSQLBackendCapacity(t *testing.T) {
	l, db := newSQLLogger(t, 2)
	l.Record(Entry{Query: "q1"})
	l.Record(Entry{Query: "q2"})
	l.Record(Entry{Query: "q3"})

	recent := l.Recent(0)
	if len(recent) != 2 || recent[0].Query != "q3" || recent[1].Query != "q2" {
		t.Fatalf("应滚动保留最新两条: %+v", recent)
	}
	// 旧行已从表中删除
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM ania_query_log`).Scan(&n); err != nil || n != 2 {
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
	e1 := l1.Record(Entry{Query: "q1"})

	// 模拟重启：同一数据库重建 Logger，序号继续递增而非重置
	l2 := New(store, 10, nil)
	e2 := l2.Record(Entry{Query: "q2"})
	if e2.ID == e1.ID {
		t.Fatalf("重启后 ID 冲突: %q", e2.ID)
	}
	if got := l2.Recent(0); len(got) != 2 {
		t.Fatalf("重启后历史应保留, got %d 条", len(got))
	}
}

func TestSQLBackendQueryFilter(t *testing.T) {
	l, _ := newSQLLogger(t, 100)
	base := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	l.Record(Entry{Time: base.Add(-2 * time.Hour), ChatType: "group", TargetID: "10001", Senders: []string{"111"}, Query: "今天天气怎么样"})
	l.Record(Entry{Time: base.Add(-1 * time.Hour), ChatType: "group", TargetID: "10001", Senders: []string{"222", "111"}, Query: "帮我看看新闻 News"})
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
		{"触发人不误中子串", Filter{Sender: "11"}, 0},
		{"按起始时间", Filter{Start: base.Add(-90 * time.Minute)}, 2},
		{"按截止时间", Filter{End: base.Add(-90 * time.Minute)}, 1},
		{"按时间区间", Filter{Start: base.Add(-2 * time.Hour), End: base.Add(-1 * time.Hour)}, 2},
		{"按关键词", Filter{Keyword: "天气"}, 2},
		{"关键词不区分大小写", Filter{Keyword: "news"}, 1},
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

func TestSQLBackendQueryBeforeCursor(t *testing.T) {
	l, _ := newSQLLogger(t, 10)
	for _, q := range []string{"a", "b", "c", "d", "e"} {
		l.Record(Entry{ChatType: "group", TargetID: "1", Query: q})
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

// TestSQLBackendMarkRunningInterrupted SQL 后端的 running → interrupted 标记，
// 并与 KV 后端做一致性对比。
func TestSQLBackendMarkRunningInterrupted(t *testing.T) {
	kv := New(newFakeStore(), 10, nil)
	sqlm, _ := newSQLLogger(t, 10)

	now := time.Now()
	inputs := []Entry{
		{Query: "q1", Status: StatusRunning, Time: now.Add(-time.Minute)},
		{Query: "q2", Status: StatusSuccess, Time: now.Add(-3 * time.Minute)},
		{Query: "q3", Status: StatusRunning, Time: now.Add(-2 * time.Minute)},
	}
	for _, e := range inputs {
		kv.Record(e)
		sqlm.Record(e)
	}

	kvN, sqlN := kv.MarkRunningInterrupted(), sqlm.MarkRunningInterrupted()
	if kvN != 2 || sqlN != 2 {
		t.Fatalf("want 2 interrupted, kv=%d sql=%d", kvN, sqlN)
	}
	kvAll, sqlAll := kv.Recent(0), sqlm.Recent(0)
	if len(kvAll) != len(sqlAll) {
		t.Fatalf("条数不一致 kv=%d sql=%d", len(kvAll), len(sqlAll))
	}
	for i := range kvAll {
		if kvAll[i].Status != sqlAll[i].Status {
			t.Fatalf("第 %d 条状态不一致: kv=%q sql=%q", i, kvAll[i].Status, sqlAll[i].Status)
		}
		if (kvAll[i].Query == "q1" || kvAll[i].Query == "q3") && kvAll[i].Status != StatusInterrupted {
			t.Fatalf("running 记录未标记中断: %+v", kvAll[i])
		}
	}
	// 二次调用无新增更新
	if n := sqlm.MarkRunningInterrupted(); n != 0 {
		t.Fatalf("二次调用应返回 0，实际 %d", n)
	}
}

// TestBackendConformance 同一操作序列分别作用于 KV 与 SQL 后端，
// Recent/Query 结果应一致（ID 序列与字段）。
func TestBackendConformance(t *testing.T) {
	kv := New(newFakeStore(), 3, nil)
	sqlm, _ := newSQLLogger(t, 3)

	base := time.Date(2026, 7, 26, 12, 0, 0, 0, time.Local)
	inputs := []Entry{
		{Time: base, ChatType: "group", TargetID: "1", Senders: []string{"u1"}, Query: "第一条"},
		{Time: base.Add(time.Hour), ChatType: "friend", TargetID: "2", Senders: []string{"u2"}, Query: "第二条"},
		{Time: base.Add(2 * time.Hour), ChatType: "group", TargetID: "1", Senders: []string{"u3"}, Query: "第三条"},
		{Time: base.Add(3 * time.Hour), ChatType: "group", TargetID: "3", Senders: []string{"u1"}, Query: "第四条"},
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
			if a[i].ID != b[i].ID || a[i].Query != b[i].Query || a[i].Status != b[i].Status {
				t.Fatalf("%s 第 %d 条不一致:\nkv  %+v\nsql %+v", name, i, a[i], b[i])
			}
		}
	}
	compare("Recent", kv.Recent(0), sqlm.Recent(0))
	compare("Query-群聊", kv.Query(Filter{ChatType: "group"}), sqlm.Query(Filter{ChatType: "group"}))
	compare("Query-触发人", kv.Query(Filter{Sender: "u1"}), sqlm.Query(Filter{Sender: "u1"}))
	compare("Query-关键词", kv.Query(Filter{Keyword: "条"}), sqlm.Query(Filter{Keyword: "条"}))

	// Update 后一致
	for _, l := range []*Logger{kv, sqlm} {
		l.Update(ids[1], func(en *Entry) { en.Status = StatusSuccess })
	}
	compare("Update后", kv.Recent(0), sqlm.Recent(0))
}
