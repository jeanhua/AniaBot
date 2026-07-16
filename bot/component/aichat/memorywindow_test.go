package aichat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// fakeHistoryStore 用于测试的内存历史存储，模拟持久化的读写语义。
type fakeHistoryStore struct {
	saved   []Message
	cleared bool
}

func (f *fakeHistoryStore) Load(_ context.Context) ([]Message, error) {
	if f.saved == nil {
		return nil, nil
	}
	// 返回副本，模拟从外部存储反序列化得到的新切片
	out := make([]Message, len(f.saved))
	copy(out, f.saved)
	return out, nil
}

func (f *fakeHistoryStore) Save(_ context.Context, messages []Message) error {
	f.saved = make([]Message, len(messages))
	copy(f.saved, messages)
	return nil
}

func (f *fakeHistoryStore) Clear(_ context.Context) error {
	f.saved = nil
	f.cleared = true
	return nil
}

func TestMessageWindowPersistAndLoad(t *testing.T) {
	store := &fakeHistoryStore{}
	w := newMessageWindow(1000, nil, nil, store)

	// append 后应自动落盘
	w.append(TextMessage(RoleUser, "你好"))
	w.append(Message{
		Role: RoleAssistant,
		Parts: []ContentPart{
			TextPart("这是图片"),
			ImageURLPart("https://example.com/a.png"),
		},
		ToolCalls: []llmtool.ToolCall{
			{ID: "call_1", Name: "get_time", Arguments: `{}`},
		},
		ReasoningContent: "推理过程",
	})

	if len(store.saved) != 2 {
		t.Fatalf("持久化消息数 = %d, want 2", len(store.saved))
	}
	// 落盘的原始历史应保留图片 URL（无损存储）
	if store.saved[1].Parts[1].Type != ContentPartImageURL || store.saved[1].Parts[1].ImageURL != "https://example.com/a.png" {
		t.Fatalf("落盘历史未保留原始图片 URL: %+v", store.saved[1].Parts)
	}

	// 模拟重启：新建窗口并回放
	w2 := newMessageWindow(1000, nil, nil, store)
	w2.load(context.Background())
	if len(w2.history()) != 2 {
		t.Fatalf("回放后历史长度 = %d, want 2", len(w2.history()))
	}

	// 校验工具调用与推理内容均能正确还原
	got := w2.history()[1]
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].Name != "get_time" {
		t.Fatalf("工具调用未还原: %+v", got.ToolCalls)
	}
	if got.ToolCalls[0].ID != "call_1" || got.ToolCalls[0].Arguments != `{}` {
		t.Fatalf("工具调用字段未还原: %+v", got.ToolCalls[0])
	}
	if got.ReasoningContent != "推理过程" {
		t.Fatalf("推理内容未还原: %q", got.ReasoningContent)
	}
	// 回放后 http 图片 URL 应降级为文本标记（避免失效链接发给 LLM）
	if len(got.Parts) != 2 || got.Parts[1].Type != ContentPartText || got.Parts[1].Text != "[图片，链接已失效]" {
		t.Fatalf("http 图片应降级为文本标记: %+v", got.Parts)
	}
}

func TestMessageWindowClearDeletesStore(t *testing.T) {
	store := &fakeHistoryStore{}
	w := newMessageWindow(1000, nil, nil, store)
	w.append(TextMessage(RoleUser, "hi"))

	w.clear()

	if !store.cleared {
		t.Fatal("clear 未触发存储删除")
	}
	if len(store.saved) != 0 {
		t.Fatalf("clear 后存储非空: %d", len(store.saved))
	}
	// 模拟重启回放，应为空
	w2 := newMessageWindow(1000, nil, nil, store)
	w2.load(context.Background())
	if len(w2.history()) != 0 {
		t.Fatalf("清除后回放非空: %d", len(w2.history()))
	}
}

func TestMessageJSONRoundTrip(t *testing.T) {
	orig := []Message{
		TextMessage(RoleSystem, "系统提示"),
		TextMessage(RoleUser, "用户消息"),
		{
			Role: RoleAssistant,
			Parts: []ContentPart{
				TextPart("回复"),
				ImageURLPart("https://example.com/b.png"),
			},
			ToolCalls: []llmtool.ToolCall{
				{ID: "id1", Name: "web_search", Arguments: `{"q":"x"}`},
			},
			ReasoningContent: "思考",
		},
		{Role: RoleTool, ToolCallID: "id1", Parts: []ContentPart{TextPart("结果")}},
	}

	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got []Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(got) != len(orig) {
		t.Fatalf("消息数 = %d, want %d", len(got), len(orig))
	}
	// 抽查 assistant 消息的工具调用与图片片段
	asst := got[2]
	if len(asst.ToolCalls) != 1 || asst.ToolCalls[0].Name != "web_search" {
		t.Fatalf("工具调用还原错误: %+v", asst.ToolCalls)
	}
	if asst.ReasoningContent != "思考" {
		t.Fatalf("推理内容还原错误: %q", asst.ReasoningContent)
	}
	if len(asst.Parts) != 2 || asst.Parts[1].Type != ContentPartImageURL {
		t.Fatalf("图片片段还原错误: %+v", asst.Parts)
	}
	// 抽查 tool 消息的 ToolCallID
	if got[3].ToolCallID != "id1" {
		t.Fatalf("ToolCallID 还原错误: %q", got[3].ToolCallID)
	}
}

func TestDegradeImagesKeepsDataURI(t *testing.T) {
	// data URI（base64 内联，如本地图片）不依赖外部链接、重启不失效，回放后应保留原样
	msgs := []Message{
		{
			Role: RoleUser,
			Parts: []ContentPart{
				TextPart("看这张本地图"),
				ImageURLPart("data:image/png;base64,iVBORw0KGgo="),
			},
		},
		{
			Role: RoleUser,
			Parts: []ContentPart{
				TextPart("看这张 QQ 图"),
				ImageURLPart("https://qpic.cn/expire-soon.png"),
			},
		},
	}

	got := degradeImagesToText(msgs)

	// data URI 图片保留
	if got[0].Parts[1].Type != ContentPartImageURL || got[0].Parts[1].ImageURL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("data URI 应保留原样: %+v", got[0].Parts[1])
	}
	// http URL 降级为文本标记
	if got[1].Parts[1].Type != ContentPartText || got[1].Parts[1].Text != "[图片，链接已失效]" {
		t.Fatalf("http URL 应降级为文本标记: %+v", got[1].Parts[1])
	}
}
