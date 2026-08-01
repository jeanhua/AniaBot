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
	sdkClient := func(u, k string) openai.Client {
		return openai.NewClient(
			option.WithAPIKey(k),
			option.WithBaseURL(u),
			option.WithMaxRetries(0),
		)
	}
	c := &LLMClient{client: sdkClient(baseURL, "test-key"), model: "main-model"}
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
		c.fallback = &LLMClient{client: sdkClient(fbBase, fbKey), model: cfg.fallbackModel}
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

	c := &LLMClient{client: client, model: "main-model",
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
		WithFallback(fbSrv.URL, "fb-key", "fb-model"))

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
		WithFallback(fbSrv.URL, "fb-key", "fb-model"))

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
