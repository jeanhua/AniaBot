package functool

import (
	"context"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/pluginconfig"
)

// fakeConfigStore 进程内 ConfigStore 实现，仅用于单测。
type fakeConfigStore struct {
	data map[string]any
}

func newFakeConfigStore() *fakeConfigStore { return &fakeConfigStore{data: map[string]any{}} }

func (s *fakeConfigStore) Get(key string) (any, bool) {
	v, ok := s.data[key]
	return v, ok
}
func (s *fakeConfigStore) Set(key string, val any) error {
	s.data[key] = val
	return nil
}
func (s *fakeConfigStore) Delete(key string) bool {
	delete(s.data, key)
	return true
}
func (s *fakeConfigStore) All() map[string]any { return s.data }

// registerTestConfigFields 注册测试用配置字段（注册表进程级，重复注册原位覆盖）。
func registerTestConfigFields() {
	pluginconfig.Register(
		pluginconfig.Field{Key: "plugin.test.model", Label: "模型"},
		pluginconfig.Field{Key: "plugin.test.api_key", Label: "API Key", Sensitive: true},
		pluginconfig.Field{Key: "plugin.test.ratio", Label: "比率"},
	)
}

func TestConfigGet(t *testing.T) {
	registerTestConfigFields()
	store := newFakeConfigStore()
	store.data["plugin.test.model"] = "deepseek-chat"
	store.data["plugin.test.api_key"] = "sk-secret"
	tool := NewConfigGetTool(store)

	// 指定键：普通字段返回值
	out, err := tool.Execute(context.Background(), &ConfigGetParams{Key: "plugin.test.model"}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(out, `"deepseek-chat"`) {
		t.Fatalf("未返回配置值: %s", out)
	}

	// 指定键：敏感字段掩码，不泄露真实值
	out, err = tool.Execute(context.Background(), &ConfigGetParams{Key: "plugin.test.api_key"}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if strings.Contains(out, "sk-secret") || !strings.Contains(out, configMask) {
		t.Fatalf("敏感字段未掩码: %s", out)
	}

	// 指定键：不存在的键报错
	if _, err = tool.Execute(context.Background(), &ConfigGetParams{Key: "plugin.test.missing"}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("不存在的键应报错")
	}

	// 全量列表：包含已注册键，敏感字段掩码
	out, err = tool.Execute(context.Background(), &ConfigGetParams{}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(out, "plugin.test.model") {
		t.Fatalf("全量列表缺少键: %s", out)
	}
	if strings.Contains(out, "sk-secret") {
		t.Fatalf("全量列表泄露敏感值: %s", out)
	}
}

func TestConfigSet(t *testing.T) {
	registerTestConfigFields()
	pluginconfig.Register(pluginconfig.Field{Key: "bot.admin_panel.listen", Label: "监听地址"})
	store := newFakeConfigStore()
	tool := NewConfigSetTool(store)

	// 纯字符串值
	if _, err := tool.Execute(context.Background(), &ConfigSetParams{Key: "plugin.test.model", Value: "gpt-4o"}, llmtool.CallBackFuncs{}); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if v, _ := store.Get("plugin.test.model"); v != "gpt-4o" {
		t.Fatalf("字符串写入异常: %v", v)
	}

	// JSON 值保留类型
	if _, err := tool.Execute(context.Background(), &ConfigSetParams{Key: "plugin.test.ratio", Value: "0.6"}, llmtool.CallBackFuncs{}); err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if v, _ := store.Get("plugin.test.ratio"); v != 0.6 {
		t.Fatalf("数字写入异常（类型应保留）: %#v", v)
	}

	// 键大小写归一
	if _, err := tool.Execute(context.Background(), &ConfigSetParams{Key: "Plugin.Test.Model", Value: "x"}, llmtool.CallBackFuncs{}); err != nil {
		t.Fatalf("大小写归一失败: %v", err)
	}

	// 未注册的键拒绝
	if _, err := tool.Execute(context.Background(), &ConfigSetParams{Key: "meta.initialized", Value: "0"}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("未注册的键应拒绝写入")
	}

	// 面板监听地址不允许置空
	if _, err := tool.Execute(context.Background(), &ConfigSetParams{Key: "bot.admin_panel.listen", Value: "  "}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("listen 置空应拒绝")
	}
}
