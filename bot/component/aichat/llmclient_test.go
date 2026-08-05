package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// fakeLLMServer 返回 chat completion 成功响应的 handler 包装。
func fakeChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"id":"chatcmpl-test","object":"chat.completion","created":1,"model":"test",`+
		`"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],`+
		`"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`)
}

// newTestClient 构造应用层测试客户端：关闭 SDK 内置重试（option.WithMaxRetries(0)），
// 让测试精确验证应用层重试与备用切换逻辑；生产代码保留 SDK 默认重试。
func newTestClient(baseURL string, opts ...LLMClientOption) *LLMClient {
	cfg := llmClientConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}
	sdkBackend := func(u, k, m string) *chatCompletionsBackend {
		return &chatCompletionsBackend{
			client: openai.NewClient(
				option.WithAPIKey(k),
				option.WithBaseURL(u),
				option.WithMaxRetries(0),
			),
			model: m,
		}
	}
	c := &LLMClient{backend: sdkBackend(baseURL, "test-key", "main-model"), model: "main-model"}
	if cfg.maxAttempts > 1 {
		c.retry = &retryConfig{maxAttempts: cfg.maxAttempts, baseDelay: cfg.baseDelay}
	}
	if cfg.fallbackModel != "" {
		fbBase, fbKey := cfg.fallbackBaseURL, cfg.fallbackAPIKey
		if fbBase == "" {
			fbBase = baseURL
		}
		if fbKey == "" {
			fbKey = "test-key"
		}
		c.fallback = &LLMClient{backend: sdkBackend(fbBase, fbKey, cfg.fallbackModel), model: cfg.fallbackModel}
	}
	return c
}

func genReq(c *LLMClient) (GenerateResponse, TokenUsage, error) {
	return c.Generate(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
}

// TestGenerateRetrySucceeds 429→429→200：应用层重试两次后成功，请求总数 3。
func TestGenerateRetrySucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":{"message":"rate limited","type":"rate_limit_error"}}`)
			return
		}
		fakeChatHandler(w, r)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(3, time.Millisecond))
	resp, usage, err := genReq(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hi" {
		t.Fatalf("content = %q", resp.Content)
	}
	if usage.TotalTokens != 8 {
		t.Fatalf("total tokens = %d, want 8", usage.TotalTokens)
	}
	if calls.Load() != 3 {
		t.Fatalf("request count = %d, want 3", calls.Load())
	}
}

// TestGenerateNoRetryOnClientError 400 不可重试：只发一次请求且返回包装错误。
func TestGenerateNoRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":{"message":"bad request","type":"invalid_request_error"}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(3, time.Millisecond))
	_, _, err := genReq(c)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "LLM generation failed") {
		t.Fatalf("error = %v, want wrapped message", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("request count = %d, want 1", calls.Load())
	}
}

// flakyTransport 前 fail 次请求返回网络错误（SDK 不重试网络错误），之后转真实 server。
type flakyTransport struct {
	fail atomic.Int32
	base http.RoundTripper
}

func (f *flakyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.fail.Add(-1) >= 0 {
		return nil, errors.New("connection reset by peer")
	}
	return f.base.RoundTrip(req)
}

// TestGenerateRetryNetworkError 网络错误（SDK 不重试）由应用层重试兜底。
func TestGenerateRetryNetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(fakeChatHandler))
	defer srv.Close()

	tr := &flakyTransport{fail: atomic.Int32{}}
	tr.fail.Store(1)
	tr.base = srv.Client().Transport
	if tr.base == nil {
		tr.base = http.DefaultTransport
	}
	client := openaiClientWithTransport(srv.URL, tr)

	c := &LLMClient{backend: &chatCompletionsBackend{client: client, model: "main-model"}, model: "main-model",
		retry: &retryConfig{maxAttempts: 3, baseDelay: time.Millisecond}}
	if _, _, err := genReq(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// openaiClientWithTransport 用自定义 RoundTripper 构造 openai.Client（测试注入网络错误用）。
func openaiClientWithTransport(baseURL string, tr http.RoundTripper) openai.Client {
	return openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(baseURL),
		option.WithHTTPClient(&http.Client{Transport: tr}),
	)
}

// TestGenerateNoRetryWhenDisabled max_attempts=1 不重试：只发一次请求。
func TestGenerateNoRetryWhenDisabled(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"down","type":"server_error"}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(1, time.Millisecond))
	if _, _, err := genReq(c); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("request count = %d, want 1", calls.Load())
	}
}

// TestGenerateContextCancel 上下文取消立即返回 ctx.Err()，不重试。
func TestGenerateContextCancel(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"down","type":"server_error"}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(5, time.Millisecond))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := c.Generate(ctx, []Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("request count = %d, want 0（取消后不应发请求）", calls.Load())
	}
}

// TestGenerateFallbackSuccess 主模型重试耗尽后切换备用模型成功。
func TestGenerateFallbackSuccess(t *testing.T) {
	var mainCalls atomic.Int32
	mainSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mainCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":{"message":"down","type":"server_error"}}`)
	}))
	defer mainSrv.Close()

	var fbCalls atomic.Int32
	var fbModel atomic.Value
	fbSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fbCalls.Add(1)
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		fbModel.Store(body.Model)
		fakeChatHandler(w, r)
	}))
	defer fbSrv.Close()

	c := newTestClient(mainSrv.URL,
		WithRetry(2, time.Millisecond),
		WithFallback(fbSrv.URL, "fb-key", "fb-model", ""))

	resp, _, err := genReq(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hi" {
		t.Fatalf("content = %q", resp.Content)
	}
	if got := fbModel.Load().(string); got != "fb-model" {
		t.Fatalf("fallback model = %q, want fb-model", got)
	}
	if mainCalls.Load() != 2 || fbCalls.Load() != 1 {
		t.Fatalf("main=%d(2), fallback=%d(1)", mainCalls.Load(), fbCalls.Load())
	}
}

// TestGenerateFallbackAlsoFails 备用模型也失败时返回 fallback 错误。
func TestGenerateFallbackAlsoFails(t *testing.T) {
	errHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		fmt.Fprint(w, `{"error":{"message":"down","type":"server_error"}}`)
	})
	mainSrv := httptest.NewServer(errHandler)
	defer mainSrv.Close()
	fbSrv := httptest.NewServer(errHandler)
	defer fbSrv.Close()

	c := newTestClient(mainSrv.URL,
		WithRetry(2, time.Millisecond),
		WithFallback(fbSrv.URL, "fb-key", "fb-model", ""))

	_, _, err := genReq(c)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "(fallback)") {
		t.Fatalf("error = %v, want fallback marker", err)
	}
}

// TestRetryDelayBounds 退避时间有上限且为正。
func TestRetryDelayBounds(t *testing.T) {
	for i := 0; i < 10; i++ {
		d := retryDelay(time.Second, i)
		if d <= 0 || d > 60*time.Second {
			t.Fatalf("retryDelay(%d) = %v, out of bounds", i, d)
		}
	}
	if d := retryDelay(40*time.Second, 0); d > 60*time.Second {
		t.Fatalf("retryDelay cap exceeded: %v", d)
	}
}

// ---- 流式生成（GenerateStream）----

// sseChunk 构造一条 SSE data 帧。
func sseChunk(payload string) string {
	return "data: " + payload + "\n\n"
}

func streamEvent(choices string) string {
	return `{"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test","choices":` + choices + `}`
}

func usageEvent() string {
	return `{"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":5,"total_tokens":12}}`
}

func streamServer(events []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, ev := range events {
			fmt.Fprint(w, sseChunk(ev))
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}
}

// TestGenerateStreamContent 流式内容累积与增量回调、末块 usage。
func TestGenerateStreamContent(t *testing.T) {
	srv := httptest.NewServer(streamServer([]string{
		streamEvent(`[{"index":0,"delta":{"role":"assistant","content":"你好"},"finish_reason":null}]`),
		streamEvent(`[{"index":0,"delta":{"content":"，世界"},"finish_reason":null}]`),
		streamEvent(`[{"index":0,"delta":{"content":"！"},"finish_reason":"stop"}]`),
		usageEvent(),
	}))
	defer srv.Close()

	var deltas []string
	c := newTestClient(srv.URL)
	resp, usage, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")},
		ChatOptions{OnStreamDelta: func(d string) { deltas = append(deltas, d) }})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if resp.Content != "你好，世界！" {
		t.Fatalf("内容累积不符: %q", resp.Content)
	}
	if got := strings.Join(deltas, ""); got != "你好，世界！" {
		t.Fatalf("增量回调序列不符: %q", got)
	}
	if usage.PromptTokens != 7 || usage.CompletionTokens != 5 || usage.TotalTokens != 12 {
		t.Fatalf("流式 usage 不符: %+v", usage)
	}
}

// TestGenerateStreamUsageOnFinishChunk usage 附带在 finish_reason 块（Choices 非空）
// 的提供方形态（DeepSeek 部分响应、智谱等）：usage 同样必须被提取，
// 否则流式平台（Telegram/飞书）的 token 统计全为 0。
func TestGenerateStreamUsageOnFinishChunk(t *testing.T) {
	srv := httptest.NewServer(streamServer([]string{
		streamEvent(`[{"index":0,"delta":{"role":"assistant","content":"你好"},"finish_reason":null}]`),
		// usage 附带在最后一个内容块（Choices 非空 + finish_reason=stop），无独立末块
		`{"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"test",` +
			`"choices":[{"index":0,"delta":{},"finish_reason":"stop"}],` +
			`"usage":{"prompt_tokens":9,"completion_tokens":4,"total_tokens":13}}`,
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, usage, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if resp.Content != "你好" {
		t.Fatalf("内容累积不符: %q", resp.Content)
	}
	if usage.PromptTokens != 9 || usage.CompletionTokens != 4 || usage.TotalTokens != 13 {
		t.Fatalf("finish 块携带的 usage 未被提取: %+v", usage)
	}
}

// TestGenerateStreamToolCalls 工具调用增量按 Index 组装（乱序到达）。
func TestGenerateStreamToolCalls(t *testing.T) {
	srv := httptest.NewServer(streamServer([]string{
		streamEvent(`[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","function":{"name":"time","arguments":""}}]},"finish_reason":null}]`),
		streamEvent(`[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"web_search","arguments":""}}]},"finish_reason":null}]`),
		streamEvent(`[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"q\":\"x\"}"}}]},"finish_reason":null}]`),
		streamEvent(`[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{}"}}]},"finish_reason":"tool_calls"}]`),
		usageEvent(),
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, _, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if len(resp.ToolCalls) != 2 {
		t.Fatalf("应有 2 个工具调用, got %+v", resp.ToolCalls)
	}
	if resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Name != "web_search" || resp.ToolCalls[0].Arguments != "{}" {
		t.Fatalf("tool call 0 组装不符: %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].ID != "call_2" || resp.ToolCalls[1].Name != "time" || resp.ToolCalls[1].Arguments != `{"q":"x"}` {
		t.Fatalf("tool call 1 组装不符: %+v", resp.ToolCalls[1])
	}
}

// TestGenerateStreamRetryBeforeStart 首字节前失败可重试（请求总数 2）。
func TestGenerateStreamRetryBeforeStart(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			fmt.Fprint(w, `{"error":{"message":"boom"}}`)
			return
		}
		streamServer([]string{
			streamEvent(`[{"index":0,"delta":{"content":"ok"}}]`),
			usageEvent(),
		})(w, r)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(3, time.Millisecond))
	resp, _, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("内容不符: %q", resp.Content)
	}
	if calls.Load() != 2 {
		t.Fatalf("首字节前失败应重试一次（共 2 次请求）, got %d", calls.Load())
	}
}

// TestGenerateStreamNoRetryAfterStart 首字节后失败不重试（避免重复输出）。
func TestGenerateStreamNoRetryAfterStart(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		// 输出一段真实内容后跟一个 SSE error 事件，模拟流中途出错
		fmt.Fprint(w, sseChunk(streamEvent(`[{"index":0,"delta":{"content":"部分"}}]`)))
		fmt.Fprint(w, "data: {\"error\":{\"message\":\"stream boom\"}}\n\n")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, WithRetry(3, time.Millisecond))
	_, _, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err == nil {
		t.Fatal("流中出错应返回错误")
	}
	if calls.Load() != 1 {
		t.Fatalf("首字节后失败不应重试, got %d 次请求", calls.Load())
	}
}

// TestGenerateStreamReasoning 流式 reasoning_content 提取。
func TestGenerateStreamReasoning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, sseChunk(`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"","reasoning_content":"思考中"},"finish_reason":null}]}`))
		fmt.Fprint(w, sseChunk(`{"id":"c","object":"chat.completion.chunk","created":1,"model":"m","choices":[{"index":0,"delta":{"content":"答案"},"finish_reason":"stop"}]}`))
		fmt.Fprint(w, sseChunk(usageEvent()))
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	resp, _, err := c.GenerateStream(context.Background(),
		[]Message{TextMessage(RoleUser, "hello")}, ChatOptions{})
	if err != nil {
		t.Fatalf("GenerateStream 失败: %v", err)
	}
	if resp.ReasoningContent != "思考中" {
		t.Fatalf("reasoning_content 提取不符: %q", resp.ReasoningContent)
	}
	if resp.Content != "答案" {
		t.Fatalf("内容不符: %q", resp.Content)
	}
}
