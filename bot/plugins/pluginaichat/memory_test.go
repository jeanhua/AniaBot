package pluginaichat

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func newTestMemoryManager(maxEntries int) *memoryManager {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return newMemoryManager(newPFake(), logger, maxEntries)
}

func TestMemoryAddAndList(t *testing.T) {
	m := newTestMemoryManager(0)

	e, err := m.add("g:123", "456", "小明喜欢熬夜打榜", []string{"偏好"})
	if err != nil {
		t.Fatalf("add 失败: %v", err)
	}
	if e.ID == "" {
		t.Fatal("add 未生成 ID")
	}

	entries := m.list("g:123")
	if len(entries) != 1 {
		t.Fatalf("期望 1 条记忆，实际 %d 条", len(entries))
	}
	if entries[0].Content != "小明喜欢熬夜打榜" || entries[0].UserID != "456" {
		t.Fatalf("记忆内容不符: %+v", entries[0])
	}

	// scope 隔离：其它群/私聊看不到
	if got := m.list("g:999"); len(got) != 0 {
		t.Fatalf("scope 隔离失效，g:999 看到 %d 条记忆", len(got))
	}
	if got := m.list("f:123"); len(got) != 0 {
		t.Fatalf("scope 隔离失效，f:123 看到 %d 条记忆", len(got))
	}
}

func TestMemoryAddDedup(t *testing.T) {
	m := newTestMemoryManager(0)

	first, err := m.add("g:123", "", "群规：不许发广告", nil)
	if err != nil {
		t.Fatalf("首次 add 失败: %v", err)
	}
	// 空白差异应被规范化去重
	second, err := m.add("g:123", "", "  群规：不许发广告 \n", nil)
	if err != nil {
		t.Fatalf("重复 add 失败: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("重复内容应返回已有条目，ID 不同: %s vs %s", first.ID, second.ID)
	}
	if got := m.list("g:123"); len(got) != 1 {
		t.Fatalf("去重失效，实际 %d 条", len(got))
	}
}

func TestMemoryAddEmpty(t *testing.T) {
	m := newTestMemoryManager(0)
	if _, err := m.add("g:123", "", "   ", nil); err == nil {
		t.Fatal("空内容应报错")
	}
}

func TestMemoryMaxEntries(t *testing.T) {
	m := newTestMemoryManager(2)

	if _, err := m.add("g:123", "", "第一条", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.add("g:123", "", "第二条", nil); err != nil {
		t.Fatal(err)
	}
	_, err := m.add("g:123", "", "第三条", nil)
	if !errors.Is(err, ErrMemoryFull) {
		t.Fatalf("达到上限应返回 ErrMemoryFull，实际: %v", err)
	}

	// 删除一条后可继续写入
	entries := m.list("g:123")
	if !m.remove("g:123", entries[0].ID) {
		t.Fatal("remove 失败")
	}
	if _, err := m.add("g:123", "", "第三条", nil); err != nil {
		t.Fatalf("删除后 add 仍失败: %v", err)
	}
}

func TestMemoryRemove(t *testing.T) {
	m := newTestMemoryManager(0)

	e, _ := m.add("g:123", "", "待删除", nil)
	if !m.remove("g:123", e.ID) {
		t.Fatal("remove 已存在 ID 应返回 true")
	}
	if got := m.list("g:123"); len(got) != 0 {
		t.Fatalf("删除后仍有 %d 条", len(got))
	}
	if m.remove("g:123", e.ID) {
		t.Fatal("remove 不存在 ID 应返回 false")
	}
	// 不影响其它 scope
	other, _ := m.add("g:999", "", "别的群的记忆", nil)
	if m.remove("g:123", other.ID) {
		t.Fatal("不应跨 scope 删除")
	}
}

func TestMemoryUpdate(t *testing.T) {
	m := newTestMemoryManager(0)

	e, _ := m.add("g:123", "456", "旧内容", []string{"旧标签"})
	created := e.CreatedAt

	if err := m.update("g:123", e.ID, "789", "新内容", []string{"新标签"}); err != nil {
		t.Fatalf("update 失败: %v", err)
	}
	entries := m.list("g:123")
	if len(entries) != 1 {
		t.Fatalf("update 后条目数不符: %d", len(entries))
	}
	got := entries[0]
	if got.Content != "新内容" || got.UserID != "789" || len(got.Tags) != 1 || got.Tags[0] != "新标签" {
		t.Fatalf("update 后内容不符: %+v", got)
	}
	if !got.CreatedAt.Equal(created) {
		t.Fatalf("update 不应改变创建时间: %v -> %v", created, got.CreatedAt)
	}

	// ID 不存在
	if err := m.update("g:123", "deadbeef", "", "x", nil); err == nil {
		t.Fatal("更新不存在的 ID 应报错")
	}
	// 空内容
	if err := m.update("g:123", e.ID, "", "   ", nil); err == nil {
		t.Fatal("空内容应报错")
	}
	// 不影响其它 scope
	if err := m.update("g:999", e.ID, "", "跨 scope", nil); err == nil {
		t.Fatal("不应跨 scope 更新")
	}
}

func TestMemoryScopes(t *testing.T) {
	m := newTestMemoryManager(0)

	if got := m.scopes(); len(got) != 0 {
		t.Fatalf("空管理器应无 scope，实际 %v", got)
	}

	if _, err := m.add("g:123", "", "群记忆", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := m.add("f:456", "", "私聊记忆", nil); err != nil {
		t.Fatal(err)
	}

	got := m.scopes()
	if len(got) != 2 || got[0] != "f:456" || got[1] != "g:123" {
		t.Fatalf("scopes 不符（应排序）: %v", got)
	}
}

func TestFilterMemoryByRelevance(t *testing.T) {
	entries := []memoryEntry{
		{ID: "a", Content: "完全无关的内容"},
		{ID: "b", Content: "用户喜欢喝咖啡"},
		{ID: "c", Content: "随便记的一条", Tags: []string{"咖啡"}},
	}
	matched := filterMemoryByRelevance(entries, []string{"咖啡"})
	if len(matched) != 2 {
		t.Fatalf("零分条目应被过滤，期望 2 条，实际 %d 条", len(matched))
	}
	// tag 命中权重(20)高于正文命中(10)，c 应排最前
	if matched[0].ID != "c" || matched[1].ID != "b" {
		t.Fatalf("排序不符: %v", matched)
	}

	// 全部零分时返回空
	if got := filterMemoryByRelevance(entries, []string{"奶茶"}); len(got) != 0 {
		t.Fatalf("无命中应返回空，实际 %d 条", len(got))
	}
}

func TestFormatMemoryLine(t *testing.T) {
	m := newTestMemoryManager(0)
	e, _ := m.add("g:123", "456", "内容", []string{"标签"})
	line := formatMemoryLine(e)
	for _, want := range []string{e.ID, "456", "标签", "内容"} {
		if !strings.Contains(line, want) {
			t.Fatalf("格式化结果缺少 %q: %s", want, line)
		}
	}
}
