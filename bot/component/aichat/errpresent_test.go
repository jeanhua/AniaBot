package aichat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/openai/openai-go/v3"
)

func TestUserErrorTextOpenAIError(t *testing.T) {
	oaiErr := &openai.Error{
		Code:       "invalid_api_key",
		Message:    "Incorrect API key provided: sk-proj-abcdefghijklmnop1234567890.",
		StatusCode: 401,
		Request: func() *http.Request {
			req, _ := http.NewRequest(http.MethodPost, "https://api.example.com/v1/chat/completions", nil)
			return req
		}(),
		Response: &http.Response{StatusCode: 401},
	}
	wrapped := fmt.Errorf("chat execution failed: %w", fmt.Errorf("LLM generation failed: %w", oaiErr))

	text := UserErrorText(wrapped)
	for _, want := range []string{"HTTP 401", "API 密钥无效", "Incorrect API key provided"} {
		if !strings.Contains(text, want) {
			t.Fatalf("文案缺少 %q: %s", want, text)
		}
	}
	if strings.Contains(text, "sk-proj-abcdefghijklmnop1234567890") {
		t.Fatalf("密钥未被脱敏: %s", text)
	}
	if !strings.Contains(text, "sk-proj****") {
		t.Fatalf("脱敏后应保留密钥前缀便于辨认: %s", text)
	}
	if strings.Contains(text, "api.example.com") {
		t.Fatalf("用户文案不应包含请求 URL: %s", text)
	}
}

func TestUserErrorTextAnthropicError(t *testing.T) {
	antErr := &anthropic.Error{StatusCode: 429, Response: &http.Response{StatusCode: 429}}
	if err := antErr.UnmarshalJSON([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"Number of requests has exceeded your rate limit"}}`)); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}
	wrapped := fmt.Errorf("LLM generation failed: %w", antErr)

	text := UserErrorText(wrapped)
	if !strings.Contains(text, "HTTP 429") || !strings.Contains(text, "触发限流") {
		t.Fatalf("文案缺少状态码含义: %s", text)
	}
	if !strings.Contains(text, "Number of requests has exceeded your rate limit") {
		t.Fatalf("文案缺少上游错误说明: %s", text)
	}
	if strings.Contains(text, "rate_limit_error") || strings.Contains(text, `"type"`) {
		t.Fatalf("用户文案不应包含原始 JSON: %s", text)
	}
}

func TestUserErrorTextStatusHints(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{401, "API 密钥无效"},
		{402, "余额不足"},
		{404, "模型或接口地址不存在"},
		{500, "服务端错误"},
		{529, "服务过载"},
	}
	for _, c := range cases {
		oaiErr := &openai.Error{StatusCode: c.status, Response: &http.Response{StatusCode: c.status}}
		if got := UserErrorText(oaiErr); !strings.Contains(got, c.want) {
			t.Fatalf("HTTP %d 文案缺少 %q: %s", c.status, c.want, got)
		}
	}
}

func TestUserErrorTextFallback(t *testing.T) {
	// 网络错误：剥离内部包装前缀，脱敏后展示
	err := fmt.Errorf("chat execution failed: %w",
		fmt.Errorf("LLM generation failed (fallback): %w",
			errors.New(`Post "https://api.example.com/v1": dial tcp: connection refused`)))
	text := UserErrorText(err)
	if !strings.Contains(text, "AI 请求失败") {
		t.Fatalf("兜底文案应有前缀: %s", text)
	}
	if strings.Contains(text, "chat execution failed") || strings.Contains(text, "LLM generation failed") {
		t.Fatalf("内部包装前缀应被剥离: %s", text)
	}
	if !strings.Contains(text, "connection refused") {
		t.Fatalf("兜底文案应保留具体原因: %s", text)
	}
}

func TestUserErrorTextContextErrors(t *testing.T) {
	if got := UserErrorText(context.Canceled); got != "AI 响应已被停止" {
		t.Fatalf("取消文案不符: %s", got)
	}
	if got := UserErrorText(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)); !strings.Contains(got, "请求超时") {
		t.Fatalf("超时文案不符: %s", got)
	}
}

func TestUserErrorTextKnownPatterns(t *testing.T) {
	// DeepSeek 真实报错样例：图片不受支持
	oaiErr := &openai.Error{
		StatusCode: 400,
		Message:    `.messages[5].image[0]: You have uploaded an unsupported image. Please make sure your image is valid and has one of the following formats: webp, png, jpeg, and gif.`,
		Code:       "invalid_request_error",
		Response:   &http.Response{StatusCode: 400},
	}
	text := UserErrorText(oaiErr)
	if !strings.Contains(text, "图片不受支持") {
		t.Fatalf("应命中图片不受支持的中文归纳: %s", text)
	}
	if strings.Contains(text, "messages[5]") || strings.Contains(text, "You have uploaded") {
		t.Fatalf("命中已知模式后不应展示原始英文长文: %s", text)
	}

	// 上下文超长
	lenErr := &openai.Error{
		StatusCode: 400,
		Message:    "This model's maximum context length is 65536 tokens.",
		Response:   &http.Response{StatusCode: 400},
	}
	if got := UserErrorText(lenErr); !strings.Contains(got, "上下文长度") {
		t.Fatalf("应命中上下文超长的中文归纳: %s", got)
	}
}

func TestSafeErrorTextMaskAndTruncate(t *testing.T) {
	masked := SafeErrorText(errors.New("key=abcdefghijklmnopqrstuvwxyz123456 failed"))
	if strings.Contains(masked, "abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("键值形态的密钥未被脱敏: %s", masked)
	}
	if !strings.Contains(masked, "key=****") {
		t.Fatalf("应保留键名: %s", masked)
	}

	bearer := SafeErrorText(errors.New("Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid"))
	if strings.Contains(bearer, "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9") {
		t.Fatalf("Bearer token 未被脱敏: %s", bearer)
	}

	long := SafeErrorText(errors.New(strings.Repeat("很长的错误信息", 200)))
	if runes := len([]rune(long)); runes > maxUserErrRunes+3 {
		t.Fatalf("超长文案应截断，实际 %d rune: %s", runes, long)
	}

	// 普通错误原样保留（含中文），不被误脱敏
	if got := SafeErrorText(errors.New("unknown api format: \"foo\"")); got != `unknown api format: "foo"` {
		t.Fatalf("普通错误不应被改写: %s", got)
	}
}
