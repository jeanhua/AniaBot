package pluginaichat

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jeanhua/AniaBot/common/plugininfo"
)

// newTestMemoryPlugin 构造一个挂载了记忆管理器的插件实例（maxEntries=0 不限条数）
func newTestMemoryPlugin() *AIChatPlugin {
	p := &AIChatPlugin{memoryManager: newTestMemoryManager(0)}
	p.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return p
}

func TestMemoryPanelCRUD(t *testing.T) {
	p := newTestMemoryPlugin()

	// 新增
	id, err := p.MemoryCreate(plugininfo.MemoryEntryUpsert{
		Scope:   "g:123",
		UserID:  "456",
		Content: "小明喜欢熬夜打榜",
		Tags:    []string{"偏好"},
	})
	if err != nil {
		t.Fatalf("MemoryCreate 失败: %v", err)
	}
	if id == "" {
		t.Fatal("MemoryCreate 未返回 ID")
	}

	// scope 列表
	scopes := p.MemoryScopes()
	if len(scopes) != 1 {
		t.Fatalf("scope 数量不符: %+v", scopes)
	}
	s := scopes[0]
	if s.Scope != "g:123" || s.Kind != "group" || s.Target != "123" || s.Count != 1 {
		t.Fatalf("scope 信息不符: %+v", s)
	}

	// 列表
	entries, err := p.MemoryList("g:123")
	if err != nil {
		t.Fatalf("MemoryList 失败: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != id || entries[0].Content != "小明喜欢熬夜打榜" {
		t.Fatalf("记忆列表不符: %+v", entries)
	}

	// 编辑
	if err := p.MemoryUpdate(plugininfo.MemoryEntryUpsert{
		Scope:   "g:123",
		ID:      id,
		Content: "小明改作息了",
		Tags:    []string{"近况"},
	}); err != nil {
		t.Fatalf("MemoryUpdate 失败: %v", err)
	}
	entries, _ = p.MemoryList("g:123")
	if len(entries) != 1 || entries[0].Content != "小明改作息了" || entries[0].UserID != "" {
		t.Fatalf("更新后内容不符: %+v", entries)
	}

	// 删除
	if err := p.MemoryDelete("g:123", id); err != nil {
		t.Fatalf("MemoryDelete 失败: %v", err)
	}
	if got := p.MemoryScopes(); len(got) != 1 || got[0].Count != 0 {
		t.Fatalf("删除后条数不符: %+v", got)
	}
	// 删除不存在的 ID
	if err := p.MemoryDelete("g:123", id); err == nil {
		t.Fatal("删除不存在的 ID 应报错")
	}
}

func TestMemoryPanelScopeValidation(t *testing.T) {
	p := newTestMemoryPlugin()

	for _, bad := range []string{"", "g:", "x:123", "g:abc", "g:123/../../", "../admin", "f:12 3"} {
		if _, err := p.MemoryList(bad); err == nil {
			t.Fatalf("非法 scope %q 应被拒绝（MemoryList）", bad)
		}
		if _, err := p.MemoryCreate(plugininfo.MemoryEntryUpsert{Scope: bad, Content: "x"}); err == nil {
			t.Fatalf("非法 scope %q 应被拒绝（MemoryCreate）", bad)
		}
		if err := p.MemoryDelete(bad, "deadbeef"); err == nil {
			t.Fatalf("非法 scope %q 应被拒绝（MemoryDelete）", bad)
		}
	}

	// 更新时 id 为空应报错
	if err := p.MemoryUpdate(plugininfo.MemoryEntryUpsert{Scope: "g:123", Content: "x"}); err == nil {
		t.Fatal("MemoryUpdate 缺少 id 应报错")
	}
}

func TestMemoryPanelDisabled(t *testing.T) {
	p := &AIChatPlugin{}
	p.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))

	if got := p.MemoryScopes(); got != nil {
		t.Fatalf("功能未启用时 MemoryScopes 应返回 nil，实际 %v", got)
	}
	if _, err := p.MemoryList("g:123"); err == nil {
		t.Fatal("功能未启用时 MemoryList 应报错")
	}
	if _, err := p.MemoryCreate(plugininfo.MemoryEntryUpsert{Scope: "g:123", Content: "x"}); err == nil {
		t.Fatal("功能未启用时 MemoryCreate 应报错")
	}
	if err := p.MemoryUpdate(plugininfo.MemoryEntryUpsert{Scope: "g:123", ID: "x", Content: "y"}); err == nil {
		t.Fatal("功能未启用时 MemoryUpdate 应报错")
	}
	if err := p.MemoryDelete("g:123", "x"); err == nil {
		t.Fatal("功能未启用时 MemoryDelete 应报错")
	}
}
