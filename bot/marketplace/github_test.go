package marketplace

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testGitHubClient 构造指向本地测试服务器的 GitHub 客户端。
func testGitHubClient(t *testing.T, h http.HandlerFunc) *githubClient {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := newGitHubClient(defaultRepo, "")
	c.client.SetBaseURL(srv.URL)
	return c
}

func TestFetchReadmeMissing(t *testing.T) {
	c := testGitHubClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	readme, err := c.fetchReadme(context.Background(), "example", "README.md", "")
	if err != nil || readme != "" {
		t.Fatalf("404 应视为未提供 README（空串、无错误），got readme=%q err=%v", readme, err)
	}
}

func TestFetchReadmeUnauthorized(t *testing.T) {
	c := testGitHubClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "5000")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
	})
	if _, err := c.fetchReadme(context.Background(), "example", "README.md", ""); err == nil {
		t.Fatal("401 应返回错误，而不是被静默当作未提供 README")
	}
}

func TestFetchReadmeOK(t *testing.T) {
	content := "# 示例插件\n\n你好"
	c := testGitHubClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ghContent{
			Type:     "file",
			Encoding: "base64",
			Content:  base64.StdEncoding.EncodeToString([]byte(content)),
		})
	})
	readme, err := c.fetchReadme(context.Background(), "example", "README.md", "")
	if err != nil || readme != content {
		t.Fatalf("200 应返回 README 内容，got readme=%q err=%v", readme, err)
	}
}

func TestVerifyToken(t *testing.T) {
	c := testGitHubClient(t, func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || auth == "Bearer ghp_bad" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"login":"jeanhua"}`))
	})
	user, ok, invalid := c.verifyToken(context.Background())
	if ok || invalid || user != "" {
		t.Fatalf("无 Token 应 ok=false invalid=false，got user=%q ok=%v invalid=%v", user, ok, invalid)
	}
	// 失效 Token（401）：应判为 invalid
	c2 := newGitHubClient(defaultRepo, "ghp_bad")
	c2.client.SetBaseURL(c.client.BaseURL)
	if _, ok, invalid := c2.verifyToken(context.Background()); ok || !invalid {
		t.Fatalf("失效 Token 应 invalid=true，got ok=%v invalid=%v", ok, invalid)
	}
	// 有效 Token：应返回用户名
	c3 := newGitHubClient(defaultRepo, "ghp_test")
	c3.client.SetBaseURL(c.client.BaseURL)
	user, ok, invalid = c3.verifyToken(context.Background())
	if !ok || invalid || user != "jeanhua" {
		t.Fatalf("有效 Token 应返回用户名，got user=%q ok=%v invalid=%v", user, ok, invalid)
	}
}
