package adminpanel

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeanhua/AniaBot/bot/core/configstore"
)

// TestConfigPutNullDeletesKey 配置更新接口收到 null 值时应删除该键
// （可选参数清空=恢复未配置语义），普通值正常写入。
func TestConfigPutNullDeletesKey(t *testing.T) {
	store := configstore.New(newFakePersistent(), slog.Default())
	s := &Server{opt: Options{Config: store, Logger: slog.Default()}}
	if err := store.Set("plugin.ai_chat_bot.temperature", 1.2); err != nil {
		t.Fatalf("预置配置失败: %v", err)
	}

	// 预置敏感字段，验证掩码占位符走「不修改」而非删除
	if err := store.Set("plugin.ai_chat_bot.api_key", "sk-secret"); err != nil {
		t.Fatalf("预置敏感配置失败: %v", err)
	}

	body := map[string]any{
		"plugin.ai_chat_bot.temperature": nil,
		"plugin.ai_chat_bot.top_p":       0.9,
		"plugin.ai_chat_bot.api_key":     maskPlaceholder,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("编码请求失败: %v", err)
	}
	rec := httptest.NewRecorder()
	s.handleConfigPut(rec, httptest.NewRequest("PUT", "/api/config", bytes.NewReader(raw)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, ok := store.Get("plugin.ai_chat_bot.temperature"); ok {
		t.Error("temperature 应被删除（null=清空）")
	}
	if v, ok := store.Get("plugin.ai_chat_bot.top_p"); !ok || v.(float64) != 0.9 {
		t.Errorf("top_p 应写入 0.9, got %v, ok=%v", v, ok)
	}
	// 掩码占位符=不修改：api_key 原值应保留，且不能被 null 逻辑误删
	if v, ok := store.Get("plugin.ai_chat_bot.api_key"); !ok || v.(string) != "sk-secret" {
		t.Errorf("api_key 应保持不变, got %v, ok=%v", v, ok)
	}
}
