package agenthook

import (
	"strings"
	"testing"
)

func TestCompileHooks(t *testing.T) {
	t.Run("nil 配置产出空表", func(t *testing.T) {
		out, err := compileHooks(nil)
		if err != nil || len(out) != 0 {
			t.Fatalf("compileHooks(nil) = %v, %v", out, err)
		}
	})
	t.Run("未知事件名报错", func(t *testing.T) {
		_, err := compileHooks(&FileConfig{Hooks: map[Event][]ShellHookSpec{
			"NotAnEvent": {{Command: "echo hi"}},
		}})
		if err == nil || !strings.Contains(err.Error(), "未知钩子事件") {
			t.Fatalf("应报未知事件错误, got %v", err)
		}
	})
	t.Run("非法正则以整体无效报错", func(t *testing.T) {
		_, err := compileHooks(&FileConfig{Hooks: map[Event][]ShellHookSpec{
			EventPreToolUse: {{Command: "echo hi", Matcher: "(["}},
		}})
		if err == nil || !strings.Contains(err.Error(), "PreToolUse") {
			t.Fatalf("应报正则编译错误且含事件名, got %v", err)
		}
	})
	t.Run("空命令跳过", func(t *testing.T) {
		out, err := compileHooks(&FileConfig{Hooks: map[Event][]ShellHookSpec{
			EventPreToolUse: {{Command: "  "}, {Command: "echo hi"}},
		}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out[EventPreToolUse]) != 1 {
			t.Fatalf("空命令应被跳过, got %d", len(out[EventPreToolUse]))
		}
	})
}

func TestCompiledHookMatches(t *testing.T) {
	out, err := compileHooks(&FileConfig{Hooks: map[Event][]ShellHookSpec{
		EventPreToolUse: {
			{Command: "a"},                             // 空 matcher = 全部
			{Command: "b", Matcher: "^(bash|file)$"},   // 精确子集
			{Command: "c", Matcher: "^mcp_.*weather$"}, // 前缀模式
		},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hooks := out[EventPreToolUse]
	if len(hooks) != 3 {
		t.Fatalf("expected 3 hooks, got %d", len(hooks))
	}
	cases := []struct {
		tool string
		want []bool // 各 hook 是否匹配
	}{
		{"bash", []bool{true, true, false}},
		{"webSearch", []bool{true, false, false}},
		{"mcp_xx_weather", []bool{true, false, true}},
	}
	for _, c := range cases {
		for i, want := range c.want {
			if got := hooks[i].matches(c.tool); got != want {
				t.Fatalf("hook[%d].matches(%q) = %v, want %v", i, c.tool, got, want)
			}
		}
	}
	// 非工具事件：toolName 为空串，带 matcher 的钩子不匹配，空 matcher 匹配
	if hooks[0].matches("") != true || hooks[1].matches("") != false {
		t.Fatalf("非工具事件匹配语义错误")
	}
}
