package napcat

import "testing"

// TestNewNapcatHttpAdapterClientReady 回归：httpClient 必须在构造时就绪，
// Serve 尚未运行（如首次启动等待设置向导）时插件调用发送接口应优雅失败而非 nil panic。
func TestNewNapcatHttpAdapterClientReady(t *testing.T) {
	a, ok := NewNapcatHttpAdapter().(*napcatHttpAdapter)
	if !ok {
		t.Fatal("NewNapcatHttpAdapter 应返回 *napcatHttpAdapter")
	}
	if a.httpClient == nil {
		t.Fatal("httpClient 应在构造时就绪，否则 Serve 前的发送调用会 nil panic")
	}
	// Serve 前调用 postAndCheck：请求会失败（无 baseUrl），但不应 panic
	if a.postAndCheck("http://127.0.0.1:0/ping", nil, nil) {
		t.Fatal("对不可达地址的请求应返回 false")
	}
}
