package consollog

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// discard 返回丢弃所有输出、级别放行到 Debug 的底层 handler，
// 模拟真实核心 logger（tint 设为 Debug 级别）的 Enabled 行为。
func discard() slog.Handler {
	return slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
}

// slogEntries 通过真实 slog.Logger 写日志，返回捕获到的 Entry（新在前）。
func slogEntries(max int) (*Ring, []Entry) {
	r := NewRing(max)
	l := slog.New(r.Handler(discard()))
	ctx := context.Background()
	l.Debug("dbg msg", "a", 1)
	l.Info("hello", "k", "v")
	l.Warn("warned", "n", 2)
	l.Error("boom", "err", "oops")
	l.With("scope", "test").Info("scoped", "x", true)
	// 分组展开：键名应带点分前缀
	l.LogAttrs(ctx, slog.LevelInfo, "grouped", slog.Group("g", slog.Int("n", 7)))
	return r, r.Page(0, 0)
}

func TestCaptureLevelsAndMessage(t *testing.T) {
	_, es := slogEntries(0)
	want := []string{"dbg msg", "hello", "warned", "boom", "scoped", "grouped"}
	wantLevel := []string{"debug", "info", "warn", "error", "info", "info"}
	if len(es) != len(want) {
		t.Fatalf("捕获到 %d 条日志，期望 %d", len(es), len(want))
	}
	for i, w := range want {
		// es 新在前，与写入顺序相反
		e := es[len(want)-1-i]
		if e.Message != w {
			t.Errorf("第 %d 条消息 = %q，期望 %q", i, e.Message, w)
		}
		if e.Level != wantLevel[i] {
			t.Errorf("第 %d 条级别 = %q，期望 %q", i, e.Level, wantLevel[i])
		}
		if e.ID != uint64(i+1) {
			t.Errorf("第 %d 条 ID = %d，期望 %d", i, e.ID, i+1)
		}
	}
}

func TestCaptureAttrs(t *testing.T) {
	_, es := slogEntries(0)
	last := es[len(es)-1]
	if last.Level != "debug" || last.Message != "dbg msg" {
		t.Fatalf("首条日志不符: %+v", last)
	}
	if len(last.Attrs) != 1 || last.Attrs[0].Key != "a" || last.Attrs[0].Value != "1" {
		t.Fatalf("首条 attrs 不符: %+v", last.Attrs)
	}
	// grouped 记录：g.n 应展开为键 g.n
	for _, e := range es {
		if e.Message == "grouped" {
			if len(e.Attrs) != 1 || e.Attrs[0].Key != "g.n" || e.Attrs[0].Value != "7" {
				t.Fatalf("分组 attrs 不符: %+v", e.Attrs)
			}
		}
		if e.Message == "scoped" {
			keys := map[string]string{}
			for _, a := range e.Attrs {
				keys[a.Key] = a.Value
			}
			if keys["scope"] != "test" || keys["x"] != "true" {
				t.Fatalf("WithAttrs/With 累积不符: %+v", e.Attrs)
			}
		}
	}
}

func TestPageCursor(t *testing.T) {
	r := NewRing(10)
	for i := 0; i < 6; i++ {
		r.Add(Entry{Message: "m"})
	}
	all := r.Page(0, 0)
	if len(all) != 6 {
		t.Fatalf("Page(0,0) 应返回全部 6 条，实际 %d", len(all))
	}
	// before=3 时仅返回 ID<3 的更旧日志
	older := r.Page(0, 3)
	if len(older) != 2 || older[0].ID != 2 || older[1].ID != 1 {
		t.Fatalf("before 游标过滤不符: %+v", older)
	}
	// 分页大小限制
	page := r.Page(3, 0)
	if len(page) != 3 || page[0].ID != 6 || page[2].ID != 4 {
		t.Fatalf("分页取最新 3 条不符: %+v", page)
	}
}

func TestRingOverflow(t *testing.T) {
	r := NewRing(3)
	for i := 0; i < 5; i++ {
		r.Add(Entry{Message: "m"})
	}
	es := r.Page(0, 0)
	if len(es) != 3 {
		t.Fatalf("容量溢出后应保留 3 条，实际 %d", len(es))
	}
	// 淘汰最旧两条：保留 ID 3/4/5，且新在前
	if es[0].ID != 5 || es[1].ID != 4 || es[2].ID != 3 {
		t.Fatalf("淘汰最旧条目后保留集不符: %+v", es)
	}
}

func TestLogWriter(t *testing.T) {
	r := NewRing(0)
	// 模拟标准库 log：分多次写入同一行
	if _, err := r.Write([]byte("连接失败")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Write([]byte("（第 3 次）\n已启动\n")); err != nil {
		t.Fatal(err)
	}
	es := r.Page(0, 0)
	if len(es) != 2 {
		t.Fatalf("log 行切分应得到 2 条，实际 %d", len(es))
	}
	if es[0].Message != "已启动" || es[1].Message != "连接失败（第 3 次）" {
		t.Fatalf("log 行内容不符: %+v", es)
	}
	for _, e := range es {
		if e.Level != "log" {
			t.Fatalf("log 输出级别应为 log，实际 %q", e.Level)
		}
	}
}
