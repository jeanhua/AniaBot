package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// anthropicUsageJSON 构造 Anthropic usage JSON 片段。
func anthropicUsageJSON(input, output, cacheRead int) string {
	return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d,`+
		`"cache_creation_input_tokens":0,"cache_read_input_tokens":%d}`, input, output, cacheRead)
}

// anthropicStreamEvent 构造一条 Anthropic SSE 帧。
func anthropicStreamEvent(eventType, payload string) string {
	return "event: " + eventType + "\ndata: " + payload + "\n\n"
}

// anthropicTextStream 构造一段纯文本回复的完整 SSE 流（content 为内容块 JSON 数组）。
// anthropic 后端的非流式 generate 内部也走流式通道，假服务器统一以 SSE 应答。
func anthropicTextStream(content string, usage string) string {
	var sb strings.Builder
	sb.WriteString(anthropicStreamEvent("message_start",
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"usage":`+usage+`}}`))
	sb.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
	sb.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+content+`}}`))
	sb.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
	sb.WriteString(anthropicStreamEvent("message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":`+usageOutputTokens(usage)+`}}`))
	sb.WriteString(anthropicStreamEvent("message_stop", `{"type":"message_stop"}`))
	return sb.String()
}

// usageOutputTokens 从 usage JSON 中取出 output_tokens 数字（测试拼装用）。
func usageOutputTokens(usage string) string {
	var u struct {
		OutputTokens int `json:"output_tokens"`
	}
	_ = json.Unmarshal([]byte(usage), &u)
	return fmt.Sprintf("%d", u.OutputTokens)
}

// anthropicSSEReply 返回以 SSE 应答的 handler（body 为完整 SSE 流），并捕获请求体。
func anthropicSSEReply(sseBody string, reqBody *map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reqBody != nil {
			_ = json.NewDecoder(r.Body).Decode(reqBody)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseBody)
	}
}

// TestAnthropicGenerate 基本生成：system 走独立参数、文本解析与用量转换（含缓存命中）。
func TestAnthropicGenerate(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(anthropicSSEReply(
		anthropicTextStream(`"hi"`, anthropicUsageJSON(5, 3, 2)), &reqBody))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "claude-test", WithAPIFormat(APIFormatAnthropic))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}
	resp, usage, err := c.Generate(context.Background(), []Message{
		TextMessage(RoleSystem, "你是助手"),
		TextMessage(RoleUser, "hello"),
	}, ChatOptions{})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.Content != "hi" {
		t.Fatalf("content = %q", resp.Content)
	}
	if usage.PromptTokens != 5 || usage.CompletionTokens != 3 || usage.TotalTokens != 8 || usage.CachedTokens != 2 {
		t.Fatalf("usage 不符: %+v", usage)
	}
	// system 应在独立参数而非 messages 中
	sys, ok := reqBody["system"].([]any)
	if !ok || len(sys) != 1 {
		t.Fatalf("system 参数缺失或形态不对: %v", reqBody["system"])
	}
	msgs := reqBody["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("messages 应只有 user 一条, got %d", len(msgs))
	}
	if reqBody["max_tokens"].(float64) != anthropicDefaultMaxTokens {
		t.Fatalf("max_tokens 默认应为 %d, got %v", anthropicDefaultMaxTokens, reqBody["max_tokens"])
	}
}

// TestAnthropicGenerateToolCallWithThinking 带 thinking 与 tool_use 的响应解析：
// 思考文本进 ReasoningContent，带签名的思考块进 ThinkingBlocks，tool_use 转 ToolCall。
func TestAnthropicGenerateToolCallWithThinking(t *testing.T) {
	var sse strings.Builder
	sse.WriteString(anthropicStreamEvent("message_start",
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"usage":`+anthropicUsageJSON(10, 1, 0)+`}}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"先想想"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_abc"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":1,"content_block":{"type":"redacted_thinking","data":"enc_xyz"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":1}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_1","name":"time","input":{}}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"tz\":\"utc\"}"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":2}`))
	sse.WriteString(anthropicStreamEvent("message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":6}}`))
	sse.WriteString(anthropicStreamEvent("message_stop", `{"type":"message_stop"}`))

	srv := httptest.NewServer(anthropicSSEReply(sse.String(), nil))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "claude-test", WithAPIFormat(APIFormatAnthropic))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}
	resp, _, err := c.Generate(context.Background(), []Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if resp.ReasoningContent != "先想想" {
		t.Fatalf("ReasoningContent = %q", resp.ReasoningContent)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "toolu_1" ||
		resp.ToolCalls[0].Name != "time" || resp.ToolCalls[0].Arguments != `{"tz":"utc"}` {
		t.Fatalf("tool call 解析不符: %+v", resp.ToolCalls)
	}
	var tbs []thinkingBlock
	if err := json.Unmarshal(resp.ThinkingBlocks, &tbs); err != nil {
		t.Fatalf("ThinkingBlocks 反序列化失败: %v", err)
	}
	if len(tbs) != 2 || tbs[0].Type != "thinking" || tbs[0].Signature != "sig_abc" ||
		tbs[1].Type != "redacted_thinking" || tbs[1].Data != "enc_xyz" {
		t.Fatalf("ThinkingBlocks 内容不符: %+v", tbs)
	}
}

// TestAnthropicThinkingReplayAndToolRoundtrip 多轮回放：assistant 的思考块必须
// 原样回传且位于 tool_use 之前；tool 结果归属 user 角色。
func TestAnthropicThinkingReplayAndToolRoundtrip(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(anthropicSSEReply(
		anthropicTextStream(`"done"`, anthropicUsageJSON(1, 1, 0)), &reqBody))
	defer srv.Close()

	thinkingRaw := json.RawMessage(`[{"type":"thinking","thinking":"先想想","signature":"sig_abc"}]`)
	history := []Message{
		TextMessage(RoleUser, "现在几点"),
		{
			Role:           RoleAssistant,
			ToolCalls:      []llmtool.ToolCall{{ID: "toolu_1", Name: "time", Arguments: `{"tz":"utc"}`}},
			ThinkingBlocks: thinkingRaw,
		},
		{Role: RoleTool, ToolCallID: "toolu_1", Parts: []ContentPart{TextPart("12:00")}},
	}

	c, err := NewLLMClient(srv.URL, "test-key", "claude-test", WithAPIFormat(APIFormatAnthropic))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}
	if _, _, err := c.Generate(context.Background(), history, ChatOptions{}); err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	msgs := reqBody["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("应为 user/assistant/user 三条, got %d: %v", len(msgs), msgs)
	}
	asst := msgs[1].(map[string]any)
	blocks := asst["content"].([]any)
	if len(blocks) != 2 {
		t.Fatalf("assistant 应为 thinking+tool_use 两块, got %d: %v", len(blocks), blocks)
	}
	b0 := blocks[0].(map[string]any)
	if b0["type"] != "thinking" || b0["signature"] != "sig_abc" {
		t.Fatalf("思考块未原样回传: %v", b0)
	}
	b1 := blocks[1].(map[string]any)
	if b1["type"] != "tool_use" || b1["id"] != "toolu_1" {
		t.Fatalf("tool_use 块不符: %v", b1)
	}
	// tool 结果在 user 消息中
	toolMsg := msgs[2].(map[string]any)
	if toolMsg["role"] != "user" {
		t.Fatalf("tool 结果应归属 user 角色, got %v", toolMsg["role"])
	}
	tr := toolMsg["content"].([]any)[0].(map[string]any)
	if tr["type"] != "tool_result" || tr["tool_use_id"] != "toolu_1" {
		t.Fatalf("tool_result 块不符: %v", tr)
	}
	// content 为文本块数组形态
	trContent := tr["content"].([]any)[0].(map[string]any)
	if trContent["text"] != "12:00" {
		t.Fatalf("tool_result 内容不符: %v", tr)
	}
}

// TestAnthropicThinkingParams 开启深度思考：budget_tokens 下发且 max_tokens 抬高，
// temperature/top_p/top_k 不下发（API 不允许共存）。
func TestAnthropicThinkingParams(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(anthropicSSEReply(
		anthropicTextStream(`"ok"`, anthropicUsageJSON(1, 1, 0)), &reqBody))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "claude-test", WithAPIFormat(APIFormatAnthropic))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}
	effort := "high"
	temp, topP, topK := 0.5, 0.9, 20
	maxToken := 100 // 故意小于 budget，验证 max_tokens 被抬高
	_, _, err = c.Generate(context.Background(), []Message{TextMessage(RoleUser, "hi")}, ChatOptions{
		ReasoningEffort: &effort, Temperature: &temp, TopP: &topP, TopK: &topK, MaxToken: &maxToken,
	})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}

	thinking, ok := reqBody["thinking"].(map[string]any)
	if !ok || thinking["type"] != "enabled" {
		t.Fatalf("thinking 参数未下发: %v", reqBody["thinking"])
	}
	budget := thinking["budget_tokens"].(float64)
	if budget != 32768 {
		t.Fatalf("high 档 budget 应为 32768, got %v", budget)
	}
	if reqBody["max_tokens"].(float64) <= budget {
		t.Fatalf("max_tokens 应大于 budget, got %v", reqBody["max_tokens"])
	}
	for _, k := range []string{"temperature", "top_p", "top_k"} {
		if _, exists := reqBody[k]; exists {
			t.Fatalf("开启 thinking 时不应下发 %s", k)
		}
	}
}

// TestAnthropicStream 流式：文本/思考/签名/tool_use 参数增量累积与用量提取。
func TestAnthropicStream(t *testing.T) {
	var sse strings.Builder
	sse.WriteString(anthropicStreamEvent("message_start",
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-test","content":[],"usage":`+anthropicUsageJSON(7, 1, 3)+`}}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig_s"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"你好"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"！"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":1}`))
	sse.WriteString(anthropicStreamEvent("content_block_start",
		`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"toolu_9","name":"time","input":{}}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"tz\":"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_delta",
		`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"\"utc\"}"}}`))
	sse.WriteString(anthropicStreamEvent("content_block_stop", `{"type":"content_block_stop","index":2}`))
	sse.WriteString(anthropicStreamEvent("message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":9}}`))
	sse.WriteString(anthropicStreamEvent("message_stop", `{"type":"message_stop"}`))

	srv := httptest.NewServer(anthropicSSEReply(sse.String(), nil))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "claude-test", WithAPIFormat(APIFormatAnthropic))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}
	var deltas []string
	resp, usage, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")},
		ChatOptions{OnStreamDelta: func(d string) { deltas = append(deltas, d) }})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if resp.Content != "你好！" {
		t.Fatalf("内容累积不符: %q", resp.Content)
	}
	if got := strings.Join(deltas, ""); got != "你好！" {
		t.Fatalf("增量回调序列不符: %q", got)
	}
	if resp.ReasoningContent != "想" {
		t.Fatalf("ReasoningContent = %q", resp.ReasoningContent)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "toolu_9" ||
		resp.ToolCalls[0].Arguments != `{"tz":"utc"}` {
		t.Fatalf("tool call 组装不符: %+v", resp.ToolCalls)
	}
	var tbs []thinkingBlock
	if err := json.Unmarshal(resp.ThinkingBlocks, &tbs); err != nil || len(tbs) != 1 ||
		tbs[0].Thinking != "想" || tbs[0].Signature != "sig_s" {
		t.Fatalf("ThinkingBlocks 不符: %v %+v", err, tbs)
	}
	if usage.PromptTokens != 7 || usage.CompletionTokens != 9 || usage.TotalTokens != 16 || usage.CachedTokens != 3 {
		t.Fatalf("usage 不符: %+v", usage)
	}
}
