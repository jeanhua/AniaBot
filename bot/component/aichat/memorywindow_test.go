package aichat

import (
	"context"
	"encoding/json"
	"strings"
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
	// 落盘副本中的图片片段应降级为文本标记（避免大 key 与写放大）
	if store.saved[1].Parts[1].Type != ContentPartText || store.saved[1].Parts[1].Text != "[图片]" {
		t.Fatalf("落盘历史中的图片应降级为文本标记: %+v", store.saved[1].Parts)
	}
	// 内存中的当前会话消息仍保留图片供本轮对话使用
	if w.history()[1].Parts[1].Type != ContentPartImageURL || w.history()[1].Parts[1].ImageURL != "https://example.com/a.png" {
		t.Fatalf("内存历史应保留原始图片: %+v", w.history()[1].Parts)
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
	// 回放后图片片段应为落盘时的文本标记
	if len(got.Parts) != 2 || got.Parts[1].Type != ContentPartText || got.Parts[1].Text != "[图片]" {
		t.Fatalf("回放后图片应为文本标记: %+v", got.Parts)
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

func TestPersistDegradesDataURI(t *testing.T) {
	// data URI（base64 内联图片）体积可达 MB 级，落盘副本必须剔除，
	// 但内存中当前会话的消息应保留供本轮对话使用
	store := &fakeHistoryStore{}
	w := newMessageWindow(1000, nil, nil, store)

	w.append(Message{
		Role: RoleUser,
		Parts: []ContentPart{
			TextPart("看这张本地图"),
			ImageURLPart("data:image/png;base64,iVBORw0KGgo="),
		},
	})

	if store.saved[0].Parts[1].Type != ContentPartText || store.saved[0].Parts[1].Text != "[图片]" {
		t.Fatalf("落盘副本的 data URI 应降级为文本标记: %+v", store.saved[0].Parts[1])
	}
	if w.history()[0].Parts[1].Type != ContentPartImageURL {
		t.Fatalf("内存消息应保留 data URI 图片: %+v", w.history()[0].Parts[1])
	}
	// 落盘降级不应修改原消息切片
	if w.history()[0].Parts[1].ImageURL != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("原消息被落盘降级修改: %+v", w.history()[0].Parts[1])
	}
}

func TestNeedsCompressionFallbackWithoutUsage(t *testing.T) {
	// 上游未上报 usage（lastPromptTokens == 0）时，字符数粗估超阈值也应触发压缩
	w := newMessageWindow(100, nil, nil, nil)
	if w.needsCompression() {
		t.Fatal("空历史不应触发压缩")
	}
	// 阈值 = 100 * 0.8 = 80 token ≈ 160 字符
	w.append(TextMessage(RoleUser, strings.Repeat("啊", 200)))
	if !w.needsCompression() {
		t.Fatal("usage 缺失时字符数超阈值应触发压缩")
	}

	// 上报了真实 usage 时以 usage 为准：字符数超阈值但 usage 很低则不压缩
	w2 := newMessageWindow(100, nil, nil, nil)
	w2.append(TextMessage(RoleUser, strings.Repeat("啊", 200)))
	w2.RecordUsage(TokenUsage{LastPromptTokens: 10})
	if w2.needsCompression() {
		t.Fatal("已有真实 usage 时不应走字符兜底")
	}
}
