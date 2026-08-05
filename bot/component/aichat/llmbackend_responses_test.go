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

// responsesJSON 构造一条 OpenAI Responses 响应 JSON。
func responsesJSON(output string, usage string) string {
	return `{"id":"resp_test","object":"response","created_at":1,"model":"gpt-test",` +
		`"status":"completed","output":` + output + `,"usage":` + usage + `}`
}

func responsesUsageJSON(input, output, cached int) string {
	return fmt.Sprintf(`{"input_tokens":%d,"output_tokens":%d,"total_tokens":%d,`+
		`"input_tokens_details":{"cached_tokens":%d},"output_tokens_details":{"reasoning_tokens":0}}`,
		input, output, input+output, cached)
}

// TestResponsesGenerate 基本生成：system 走 instructions、OutputText 聚合与用量转换。
func TestResponsesGenerate(t *testing.T) {
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, responsesJSON(
			`[{"type":"message","id":"m1","role":"assistant","status":"completed",`+
				`"content":[{"type":"output_text","text":"hi","annotations":[]}]}]`,
			responsesUsageJSON(5, 3, 2)))
	}))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "gpt-test", WithAPIFormat(APIFormatResponses))
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
	if reqBody["instructions"] != "你是助手" {
		t.Fatalf("instructions 不符: %v", reqBody["instructions"])
	}
	input := reqBody["input"].([]any)
	if len(input) != 1 {
		t.Fatalf("input 应只有 user 一项, got %d: %v", len(input), input)
	}
}

// TestResponsesToolCallRoundtrip function_call 解析与多轮回放：assistant 拆为
// message + function_call 项，tool 结果转 function_call_output 项。
func TestResponsesToolCallRoundtrip(t *testing.T) {
	var calls int
	var reqBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewDecoder(r.Body).Decode(&reqBody)
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			fmt.Fprint(w, responsesJSON(
				`[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"time","arguments":"{\"tz\":\"utc\"}","status":"completed"}]`,
				responsesUsageJSON(10, 4, 0)))
			return
		}
		fmt.Fprint(w, responsesJSON(
			`[{"type":"message","id":"m2","role":"assistant","status":"completed",`+
				`"content":[{"type":"output_text","text":"12 点","annotations":[]}]}]`,
			responsesUsageJSON(12, 2, 0)))
	}))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "gpt-test", WithAPIFormat(APIFormatResponses))
	if err != nil {
		t.Fatalf("NewLLMClient 失败: %v", err)
	}

	// 第一轮：解析 function_call
	resp, _, err := c.Generate(context.Background(), []Message{TextMessage(RoleUser, "几点了")}, ChatOptions{})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call_1" ||
		resp.ToolCalls[0].Name != "time" || resp.ToolCalls[0].Arguments != `{"tz":"utc"}` {
		t.Fatalf("function_call 解析不符: %+v", resp.ToolCalls)
	}

	// 第二轮：回放 assistant + tool 结果，验证 input 项形态
	history := []Message{
		TextMessage(RoleUser, "几点了"),
		{Role: RoleAssistant, ToolCalls: []llmtool.ToolCall{resp.ToolCalls[0]}},
		{Role: RoleTool, ToolCallID: "call_1", Parts: []ContentPart{TextPart("12:00")}},
	}
	if _, _, err := c.Generate(context.Background(), history, ChatOptions{}); err != nil {
		t.Fatalf("第二轮 Generate 失败: %v", err)
	}
	input := reqBody["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("应为 user/function_call/function_call_output 三项, got %d: %v", len(input), input)
	}
	fc := input[1].(map[string]any)
	if fc["type"] != "function_call" || fc["call_id"] != "call_1" || fc["name"] != "time" {
		t.Fatalf("function_call 回放不符: %v", fc)
	}
	fco := input[2].(map[string]any)
	if fco["type"] != "function_call_output" || fco["call_id"] != "call_1" || fco["output"] != "12:00" {
		t.Fatalf("function_call_output 不符: %v", fco)
	}
}

// responsesStreamEvent 构造一条 Responses SSE 帧。
func responsesStreamEvent(payload string) string {
	return "data: " + payload + "\n\n"
}

// TestResponsesStream 流式：文本增量、function_call 参数增量与 completed 事件用量。
func TestResponsesStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"time","arguments":""}}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"tz\":"}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"\"utc\"}"}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"fc_1","call_id":"call_1","name":"time","arguments":"{\"tz\":\"utc\"}"}}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.output_text.delta","item_id":"m1","output_index":1,"delta":"你好"}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.output_text.delta","item_id":"m1","output_index":1,"delta":"！"}`))
		fmt.Fprint(w, responsesStreamEvent(`{"type":"response.completed","response":`+responsesJSON(
			`[{"type":"function_call","id":"fc_1","call_id":"call_1","name":"time","arguments":"{\"tz\":\"utc\"}","status":"completed"},`+
				`{"type":"message","id":"m1","role":"assistant","status":"completed","content":[{"type":"output_text","text":"你好！","annotations":[]}]}]`,
			responsesUsageJSON(7, 5, 1))+`}`))
	}))
	defer srv.Close()

	c, err := NewLLMClient(srv.URL, "test-key", "gpt-test", WithAPIFormat(APIFormatResponses))
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
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "call_1" ||
		resp.ToolCalls[0].Arguments != `{"tz":"utc"}` {
		t.Fatalf("function_call 组装不符: %+v", resp.ToolCalls)
	}
	if usage.PromptTokens != 7 || usage.CompletionTokens != 5 || usage.TotalTokens != 12 || usage.CachedTokens != 1 {
		t.Fatalf("usage 不符: %+v", usage)
	}
}

// TestNewLLMClientUnknownFormat 未知 API 格式应在构造期报错。
func TestNewLLMClientUnknownFormat(t *testing.T) {
	if _, err := NewLLMClient("https://x", "k", "m", WithAPIFormat("weird")); err == nil {
		t.Fatal("未知格式应返回错误")
	}
}
