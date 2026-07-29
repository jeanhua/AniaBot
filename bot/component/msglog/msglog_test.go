package msglog_test

import (
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/msglog"
	"github.com/jeanhua/AniaBot/bot/core"
	"log/slog"
)

func newStore() *core.AniaMemoryStorage {
	return core.NewAniaMemoryStorage(slog.Default())
}

func TestAddAndRecent(t *testing.T) {
	r := msglog.New(newStore(), 0)
	r.Add(msglog.Entry{Type: msglog.TypeGroup, Text: "第一条"})
	r.Add(msglog.Entry{Type: msglog.TypeFriend, Text: "第二条"})
	r.Add(msglog.Entry{Type: msglog.TypeNotice, Title: "戳一戳", Text: "第三条"})

	entries := r.Recent(0)
	if len(entries) != 3 {
		t.Fatalf("期望 3 条日志，实际 %d", len(entries))
	}
	// 新条目在前
	if entries[0].Text != "第三条" || entries[2].Text != "第一条" {
		t.Fatalf("顺序错误: %+v", entries)
	}
	// ID 单调递增
	for i, e := range entries {
		if e.ID != uint64(3-i) {
			t.Fatalf("ID 不连续: %+v", entries)
		}
	}
	if entries[0].Time.IsZero() {
		t.Fatal("Time 未自动填充")
	}

	if got := r.Recent(2); len(got) != 2 || got[0].Text != "第三条" {
		t.Fatalf("Recent(2) 错误: %+v", got)
	}
}

func TestMaxEntries(t *testing.T) {
	r := msglog.New(newStore(), 3)
	for i := 0; i < 5; i++ {
		r.Add(msglog.Entry{Type: msglog.TypeGroup, Text: "msg"})
	}
	entries := r.Recent(0)
	if len(entries) != 3 {
		t.Fatalf("容量上限 3，实际 %d", len(entries))
	}
	// 最旧的两条被淘汰，保留 ID 3/4/5
	if entries[0].ID != 5 || entries[2].ID != 3 {
		t.Fatalf("淘汰错误: %+v", entries)
	}
}

func TestPage(t *testing.T) {
	r := msglog.New(newStore(), 10)
	for _, text := range []string{"a", "b", "c", "d", "e"} {
		r.Add(msglog.Entry{Type: msglog.TypeGroup, Text: text})
	}

	// 第一页：最新两条
	p1 := r.Page(2, 0)
	if len(p1) != 2 || p1[0].Text != "e" || p1[1].Text != "d" {
		t.Fatalf("page1 wrong: %+v", p1)
	}
	// 第二页：以 p1 最旧一条为游标
	p2 := r.Page(2, p1[1].ID)
	if len(p2) != 2 || p2[0].Text != "c" || p2[1].Text != "b" {
		t.Fatalf("page2 wrong: %+v", p2)
	}
	// 第三页：只剩一条
	p3 := r.Page(2, p2[1].ID)
	if len(p3) != 1 || p3[0].Text != "a" {
		t.Fatalf("page3 wrong: %+v", p3)
	}
	// 游标超过最新 ID 时从最新开始
	p4 := r.Page(2, 999)
	if len(p4) != 2 || p4[0].Text != "e" {
		t.Fatalf("page with future cursor wrong: %+v", p4)
	}
}

func TestPageWithEviction(t *testing.T) {
	r := msglog.New(newStore(), 3)
	for _, text := range []string{"a", "b", "c", "d", "e"} {
		r.Add(msglog.Entry{Type: msglog.TypeGroup, Text: text})
	}
	// 容量 3，只剩 e/d/c；游标指向已淘汰的 b（ID=2）时换算偏移越界，应为空
	if page := r.Page(2, 2); len(page) != 0 {
		t.Fatalf("evicted cursor should yield empty page, got %+v", page)
	}
	// 游标指向现存最旧的 c（ID=3）时，更旧的已被淘汰，应为空
	if page := r.Page(2, 3); len(page) != 0 {
		t.Fatalf("oldest cursor should yield empty page, got %+v", page)
	}
}

// TestRestartWithSameStore 模拟 redis 驱动下的重启：同一存储新建 Recorder，
// 日志与 ID 计数器都应续接。
func TestRestartWithSameStore(t *testing.T) {
	store := newStore()
	r1 := msglog.New(store, 0)
	r1.Add(msglog.Entry{Type: msglog.TypeGroup, Text: "重启前"})

	r2 := msglog.New(store, 0)
	r2.Add(msglog.Entry{Type: msglog.TypeFriend, Text: "重启后"})

	entries := r2.Recent(0)
	if len(entries) != 2 {
		t.Fatalf("重启后期望 2 条日志，实际 %d", len(entries))
	}
	if entries[0].Text != "重启后" || entries[1].Text != "重启前" {
		t.Fatalf("重启后顺序错误: %+v", entries)
	}
	if entries[0].ID != 2 || entries[1].ID != 1 {
		t.Fatalf("重启后 ID 未续接: %+v", entries)
	}
}
