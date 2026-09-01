package pluginaichat

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func newTestPromptManager(editor *fakeConfigEditor) *promptOverrideManager {
	return newPromptOverrideManager(editor, promptConfigKey, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestParsePromptOverrides(t *testing.T) {
	groups, friends, err := parsePromptOverrides(`{"groups":{"123":"群提示词","fs:oc_abc":"飞书群"},"friends":{"456":"好友提示词"}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 纯数字 QQ ID 规范化为 qq: 前缀，平台前缀 ID 原样保留
	if v, ok := groups[message.FromString("123")]; !ok || v != "群提示词" {
		t.Fatalf("纯数字群 ID 应规范化并命中, got ok=%v v=%q", ok, v)
	}
	if _, ok := groups[message.FromString("fs:oc_abc")]; !ok {
		t.Fatal("带前缀群 ID 应命中")
	}
	if v, ok := friends[message.FromString("456")]; !ok || v != "好友提示词" {
		t.Fatalf("好友覆盖应命中, got ok=%v v=%q", ok, v)
	}
	if len(groups) != 2 || len(friends) != 1 {
		t.Fatalf("数量不符: groups=%d friends=%d", len(groups), len(friends))
	}

	// 空配置返回空表
	g2, f2, err := parsePromptOverrides("")
	if err != nil || len(g2) != 0 || len(f2) != 0 {
		t.Fatalf("空配置应返回空表, err=%v", err)
	}

	// 非法 JSON 整体拒绝
	if _, _, err := parsePromptOverrides(`{bad`); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

// TestPromptOverrideManagerHotReload 面板直接改 files.prompt_json 后 TTL 重读生效；
// 配置损坏时沿用旧快照。
func TestPromptOverrideManagerHotReload(t *testing.T) {
	editor := newFakeConfigEditor()
	m := newTestPromptManager(editor)
	id := message.FromString("123")

	// 首次查询必加载（lastCheck 零值）；此后进入 TTL 窗口
	if _, ok := m.get(id, true); ok {
		t.Fatal("空配置不应命中")
	}

	// TTL 内不重读
	editor.Set(promptConfigKey, `{"groups":{"123":"新群提示词"}}`)
	if _, ok := m.get(id, true); ok {
		t.Fatal("TTL 内不应重读")
	}

	// 越过 TTL 生效
	m.lastCheck = time.Now().Add(-time.Minute)
	if v, ok := m.get(id, true); !ok || v != "新群提示词" {
		t.Fatalf("TTL 后应读到新配置, ok=%v v=%q", ok, v)
	}

	// 配置损坏：沿用旧快照
	editor.Set(promptConfigKey, `{bad json`)
	m.lastCheck = time.Now().Add(-time.Minute)
	if v, ok := m.get(id, true); !ok || v != "新群提示词" {
		t.Fatalf("配置损坏应沿用旧快照, ok=%v v=%q", ok, v)
	}
}

// TestPromptOverrideManagerInitSnapshot Start 时先用 viper 快照填充，之后
// 配置中心更新在 TTL 后覆盖内存（面板热生效的起点）。
func TestPromptOverrideManagerInitSnapshot(t *testing.T) {
	editor := newFakeConfigEditor()
	m := newTestPromptManager(editor)
	m.loadRaw(`{"friends":{"456":"快照提示词"}}`)

	id := message.FromString("456")
	if v, ok := m.get(id, false); !ok || v != "快照提示词" {
		t.Fatalf("快照应立即生效, ok=%v v=%q", ok, v)
	}

	editor.Set(promptConfigKey, `{"friends":{"456":"面板新提示词"}}`)
	m.lastCheck = time.Now().Add(-time.Minute)
	if v, ok := m.get(id, false); !ok || v != "面板新提示词" {
		t.Fatalf("配置中心更新应热生效, ok=%v v=%q", ok, v)
	}
}
