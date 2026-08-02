package qqofficial

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-resty/resty/v2"
)

// TestParseExpiresIn expires_in 官方文档为 number、调用示例为字符串，两种形态都兼容。
func TestParseExpiresIn(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{float64(7200), 7200},
		{"7200", 7200},
		{"bad", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := parseExpiresIn(c.in); got != c.want {
			t.Errorf("parseExpiresIn(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// TestTokenManagerRefresh 首次获取 token 后缓存复用；临近过期时自动刷新。
func TestTokenManagerRefresh(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"tok-` + string(rune('0'+n)) + `","expires_in":"7200"}`))
	}))
	defer srv.Close()

	tm := newTokenManager("appid", "secret", resty.New())
	tm.url = srv.URL

	tok1, err := tm.Token(context.Background())
	if err != nil {
		t.Fatalf("首次 Token() 出错: %v", err)
	}
	if tok1 != "tok-1" {
		t.Fatalf("首次 Token() = %q, want tok-1", tok1)
	}
	// 有效期内复用缓存，不再请求
	tok2, err := tm.Token(context.Background())
	if err != nil || tok2 != tok1 {
		t.Fatalf("缓存复用失败: tok=%q err=%v", tok2, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("有效期内不应重复请求，calls=%d", calls.Load())
	}
	// 临近过期（余量内）强制刷新
	tm.expiresAt = time.Now().Add(time.Minute)
	if _, err := tm.Token(context.Background()); err != nil {
		t.Fatalf("刷新 Token() 出错: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("临近过期应重新请求，calls=%d", calls.Load())
	}
}

// TestTokenManagerInvalidate Invalidate 后下次 Token() 强制刷新。
func TestTokenManagerInvalidate(t *testing.T) {
	tm := newTokenManager("appid", "secret", resty.New())
	tm.token = "cached"
	tm.expiresAt = time.Now().Add(time.Hour)
	tm.Invalidate()
	if !tm.expiresAt.IsZero() {
		t.Fatal("Invalidate 应清空过期时间")
	}
}

// TestTokenManagerError 凭据错误（err_code 100016）如实返回错误。
func TestTokenManagerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"err_code":100016,"message":"invalid appid or secret","trace_id":"t1"}`))
	}))
	defer srv.Close()
	tm := newTokenManager("bad", "bad", resty.New())
	tm.url = srv.URL
	if _, err := tm.Token(context.Background()); err == nil {
		t.Fatal("凭据错误应返回 error")
	}
}
