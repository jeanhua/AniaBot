package weixin

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// stubServer 测试桩：按路径分发到处理函数，返回 JSON 与可选 HTTP 状态码。
type stubServer struct {
	Server *httptest.Server
	URL    string
}

// startStub 启动测试桩服务，handler 返回响应体与 HTTP 状态码（0 视为 200）。
func startStub(t *testing.T, handle func(path, query string) (body string, status int)) *stubServer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, status := handle(r.URL.Path, r.URL.RawQuery)
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &stubServer{Server: srv, URL: srv.URL}
}

// newTestAdapter 构造用于测试的适配器：状态目录指向临时目录，API 指向测试桩。
func newTestAdapter(t *testing.T, apiBase string) *weixinAdapter {
	t.Helper()
	dir := t.TempDir()
	a := NewAdapter(nil)
	a.cfg = weixinConfig{
		apiBase:  apiBase,
		cdnBase:  DefaultCDNBase,
		botType:  DefaultBotType,
		stateDir: dir,
	}
	a.state = newStateStore(dir)
	a.ctxTokens = newContextTokenStore(filepath.Join(dir, "context_tokens.json"))
	return a
}
