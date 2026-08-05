package adminpanel

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/core/configstore"
)

// TestConfigExport 导出接口应返回完整配置：敏感字段不掩码、带下载文件名。
func TestConfigExport(t *testing.T) {
	store := configstore.New(newFakePersistent(), slog.Default())
	s := &Server{opt: Options{Config: store}}
	if err := store.Set("plugin.ai_chat_bot.api_key", "sk-test-secret"); err != nil {
		t.Fatalf("设置敏感配置失败: %v", err)
	}
	if err := store.Set("bot.admin_panel.listen", "127.0.0.1:7700"); err != nil {
		t.Fatalf("设置普通配置失败: %v", err)
	}

	rec := httptest.NewRecorder()
	s.handleConfigExport(rec, httptest.NewRequest("GET", "/api/config/export", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, "attachment; filename=\"aniabot-config-") || !strings.HasSuffix(cd, ".json\"") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("返回的不是合法 JSON: %v", err)
	}
	if v, _ := got["plugin.ai_chat_bot.api_key"].(string); v != "sk-test-secret" {
		t.Fatalf("敏感字段未导出真实值: %q", v)
	}
	if v, _ := got["bot.admin_panel.listen"].(string); v != "127.0.0.1:7700" {
		t.Fatalf("普通字段缺失或错误: %q", v)
	}
}
