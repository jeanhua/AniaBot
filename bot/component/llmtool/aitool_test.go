package llmtool

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jeanhua/AniaBot/common/aitool"
)

// fakeProviderTool 平台注入工具测试桩：带参数结构体，记录最近一次执行。
type fakeProviderTool struct {
	name     string
	desc     string
	lastText string
}

func (t *fakeProviderTool) Name() string        { return t.name }
func (t *fakeProviderTool) Description() string { return t.desc }
func (t *fakeProviderTool) Params() any         { return &fakeProviderParams{} }
func (t *fakeProviderTool) Execute(ctx context.Context, params any) (string, error) {
	p := params.(*fakeProviderParams)
	t.lastText = p.Text
	return "ok:" + p.Text, nil
}

type fakeProviderParams struct {
	Text   string `json:"text" desc:"测试文本"`
	Number int    `json:"number,omitempty" desc:"可选数字"`
}

// TestAdaptProviderToolExecute 桥接后工具经标准解析路径执行：
// LLM 实参 JSON → 参数结构体 → 平台工具 Execute。
func TestAdaptProviderToolExecute(t *testing.T) {
	inner := &fakeProviderTool{name: "qq_send_poke", desc: "测试工具"}
	tool := AdaptProviderTool(inner)

	exec := NewToolExecuter()
	session := exec.NewSessionExecutor()
	session.RegisterSession(tool)

	if _, err := session.Execute(context.Background(), ToolCall{
		Name:      "qq_send_poke",
		Arguments: `{"text":"你好","number":3}`,
	}, CallBackFuncs{}); err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	if inner.lastText != "你好" {
		t.Errorf("参数未正确传递: %q", inner.lastText)
	}
}

// TestProviderToolsSerializeDeterministic 平台注入工具与内置/会话工具合并后，
// tools 字段序列化逐请求完全一致（按工具名排序兜底，与注册顺序无关）——
// 顺序抖动会打失上游 prompt 前缀缓存。
func TestProviderToolsSerializeDeterministic(t *testing.T) {
	providerTools := func() []aitool.Tool {
		return []aitool.Tool{
			&fakeProviderTool{name: "qq_send_poke", desc: "d1"},
			&fakeProviderTool{name: "qq_get_group_list", desc: "d2"},
			&fakeProviderTool{name: "qq_send_ai_voice", desc: "d3"},
		}
	}

	build := func(reverseRegister bool) []byte {
		exec := NewToolExecuter()
		// 共享层内置工具（同样经桥接构造，等价于任意 llmtool.Tool）
		exec.Register(AdaptProviderTool(&fakeProviderTool{name: "time", desc: "builtin"}))
		session := exec.NewSessionExecutor()
		tools := providerTools()
		if reverseRegister {
			for i, j := 0, len(tools)-1; i < j; i, j = i+1, j-1 {
				tools[i], tools[j] = tools[j], tools[i]
			}
		}
		for _, pt := range tools {
			session.RegisterSession(AdaptProviderTool(pt))
		}
		b, err := json.Marshal(session.Tools())
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		return b
	}

	first := build(false)
	reversed := build(true)
	again := build(false)
	if string(first) != string(reversed) {
		t.Errorf("注册顺序不同导致 tools 序列化不同（缓存打失风险）:\n%s\n%s", first, reversed)
	}
	if string(first) != string(again) {
		t.Errorf("同组工具多次序列化结果不一致:\n%s\n%s", first, again)
	}

	// 定义列表里平台工具与共享工具各出现一次，名字齐全
	var defs []ToolDef
	if err := json.Unmarshal(first, &defs); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	names := map[string]int{}
	for _, d := range defs {
		names[d.Function.Name]++
	}
	for _, want := range []string{"time", "qq_get_group_list", "qq_send_ai_voice", "qq_send_poke"} {
		if names[want] != 1 {
			t.Errorf("工具 %s 出现次数 = %d, want 1", want, names[want])
		}
	}
}

// TestToolExecuterHas 重名检查：平台注入工具注册前据此跳过与内置工具的冲突。
func TestToolExecuterHas(t *testing.T) {
	exec := NewToolExecuter()
	exec.Register(AdaptProviderTool(&fakeProviderTool{name: "time", desc: "builtin"}))
	if !exec.Has("time") {
		t.Error("time 应已注册")
	}
	if exec.Has("qq_send_poke") {
		t.Error("未注册的工具不应报告存在")
	}
}
