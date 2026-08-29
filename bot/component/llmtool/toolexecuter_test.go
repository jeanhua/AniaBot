package llmtool

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// fakeTool 最小 Tool 实现，仅用于注册表顺序测试
type fakeTool struct{ name string }

func (t *fakeTool) Name() string        { return t.name }
func (t *fakeTool) Description() string { return t.name }
func (t *fakeTool) Params() any         { return &struct{}{} }
func (t *fakeTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	return "", nil
}

// TestToolsDeterministicOrder 验证 Tools() 输出顺序完全确定（按名称排序）。
// prompt 前缀缓存依赖请求序列化稳定，map 随机遍历会把缓存命中率打到 0。
func TestToolsDeterministicOrder(t *testing.T) {
	names := []string{"web_search", "time", "bash", "msg_history", "file"}
	e := NewToolExecuter()
	for _, n := range names {
		e.Register(&fakeTool{name: n})
	}

	want := []string{"bash", "file", "msg_history", "time", "web_search"}
	for round := range 10 {
		defs := e.Tools()
		if len(defs) != len(want) {
			t.Fatalf("round %d: got %d tools, want %d", round, len(defs), len(want))
		}
		for i, td := range defs {
			if td.Function.Name != want[i] {
				t.Fatalf("round %d: tools[%d] = %s, want %s", round, i, td.Function.Name, want[i])
			}
		}
	}
}

// TestSessionToolsDeterministicOrder 验证共享工具 + 会话工具合并后顺序仍确定
func TestSessionToolsDeterministicOrder(t *testing.T) {
	e := NewToolExecuter()
	e.Register(&fakeTool{name: "time"})
	e.Register(&fakeTool{name: "web_search"})

	session := e.NewSessionExecutor()
	session.RegisterSession(&fakeTool{name: "memory_save"})
	session.RegisterSession(&fakeTool{name: "clock_create"})

	first := session.Tools()
	for round := range 10 {
		defs := session.Tools()
		if len(defs) != len(first) {
			t.Fatalf("round %d: tool count changed: %d != %d", round, len(defs), len(first))
		}
		for i := range defs {
			if defs[i].Function.Name != first[i].Function.Name {
				t.Fatalf("round %d: tools[%d] = %s, want %s", round, i, defs[i].Function.Name, first[i].Function.Name)
			}
		}
	}
	// 合并列表按名称整体排序
	want := []string{"clock_create", "memory_save", "time", "web_search"}
	for i, td := range first {
		if td.Function.Name != want[i] {
			t.Fatalf("tools[%d] = %s, want %s", i, td.Function.Name, want[i])
		}
	}
}

// TestBuildAvailableSkillsPromptDeterministic 验证 skill 注册表注入 system prompt
// 的内容顺序确定（按 skill 名排序），不受 map 遍历随机性影响
func TestBuildAvailableSkillsPromptDeterministic(t *testing.T) {
	m := &SkillManager{skills: map[string]*Skill{
		"weather": {Meta: SkillMeta{Name: "weather", Description: "查天气"}},
		"code":    {Meta: SkillMeta{Name: "code", Description: "写代码"}},
		"news":    {Meta: SkillMeta{Name: "news", Description: "看新闻"}},
	}}

	first := m.BuildAvailableSkillsPrompt()
	for round := range 10 {
		if got := m.BuildAvailableSkillsPrompt(); got != first {
			t.Fatalf("round %d: skill prompt not deterministic:\n%s\n---\n%s", round, got, first)
		}
	}

	// 按名称排序：code < news < weather
	idxCode := strings.Index(first, "<name>code</name>")
	idxNews := strings.Index(first, "<name>news</name>")
	idxWeather := strings.Index(first, "<name>weather</name>")
	if !(idxCode >= 0 && idxCode < idxNews && idxNews < idxWeather) {
		t.Fatalf("skills not sorted by name: code=%d news=%d weather=%d", idxCode, idxNews, idxWeather)
	}
}

// TestToolsWithSessionDuplicateName 同名工具共享层与会话层并存时：
// tools 字段只保留一份定义且与 resolveTool 一致由会话层优先——
// 重复定义会被部分提供方以 400 拒绝（prompt 缓存也随之全失）。
func TestToolsWithSessionDuplicateName(t *testing.T) {
	e := NewToolExecuter()
	e.Register(&fakeTool{name: "time"})
	e.Register(&fakeTool{name: "bash"})

	session := e.NewSessionExecutor()
	// 会话层注册同名工具（如 MCP 服务器暴露的工具与内置工具重名）
	session.RegisterSession(&fakeTool{name: "time"})

	defs := session.Tools()
	count := 0
	for _, td := range defs {
		if td.Function.Name == "time" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("同名工具应只保留一份定义, got %d: %+v", count, defs)
	}
	if len(defs) != 2 {
		t.Fatalf("去重后应有 2 个工具, got %d", len(defs))
	}

	// 执行路径同样命中会话层（与 resolveTool 优先级一致）
	tool, ok := e.resolveTool("time", session.snapshotSessionTools())
	if !ok {
		t.Fatal("resolveTool 应找到 time")
	}
	if tool != session.sessionTools["time"] {
		t.Fatal("resolveTool 应优先返回会话层工具")
	}
}

// TestSessionExecutorConcurrentAccess 回归：并行工具执行时 mcp_load 等工具
// 并发 RegisterSession（写）与其他工具 Execute/Tools（读）不再产生 map 竞争
// （此前会触发不可恢复的 fatal error: concurrent map read and map write）。
// 需配合 -race 运行验证。
func TestSessionExecutorConcurrentAccess(t *testing.T) {
	e := NewToolExecuter()
	e.Register(&fakeTool{name: "time"})
	session := e.NewSessionExecutor()

	var wg sync.WaitGroup
	// 写方：模拟 mcp_load 动态加载工具
	for i := range 8 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := range 50 {
				session.RegisterSession(&fakeTool{name: fmt.Sprintf("loaded_tool_%d_%d", i, j)})
			}
		}(i)
	}
	// 读方：模拟并行工具执行与定义列表刷新
	for range 8 {
		wg.Go(func() {
			for range 50 {
				_ = session.Tools()
				_, _ = session.Execute(context.Background(), ToolCall{ID: "1", Name: "time", Arguments: "{}"}, CallBackFuncs{})
			}
		})
	}
	wg.Wait()

	if cleared := session.ClearDynamicMCPTools(); cleared != 0 {
		t.Fatalf("fakeTool 非 MCPTool，不应被清理, got %d", cleared)
	}
}
