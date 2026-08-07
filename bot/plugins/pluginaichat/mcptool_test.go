package pluginaichat

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// fakeConfigEditor 实现 plugin.ConfigEditor 的内存版（供 MCP 工具测试）
type fakeConfigEditor struct {
	data map[string]any
}

func newFakeConfigEditor() *fakeConfigEditor { return &fakeConfigEditor{data: map[string]any{}} }

func (s *fakeConfigEditor) Get(key string) (any, bool) {
	v, ok := s.data[key]
	return v, ok
}
func (s *fakeConfigEditor) Set(key string, val any) error {
	s.data[key] = val
	return nil
}
func (s *fakeConfigEditor) Delete(key string) bool {
	delete(s.data, key)
	return true
}
func (s *fakeConfigEditor) All() map[string]any { return s.data }

// newMCPToolPlugin 构造带空工具执行器与内存配置中心的测试插件
func newMCPToolPlugin(t *testing.T) *AIChatPlugin {
	t.Helper()
	p := &AIChatPlugin{
		toolExecutor: llmtool.NewToolExecuter(),
	}
	p.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	p.ConfigEditor = newFakeConfigEditor()
	return p
}

// mcpTools 按注册顺序返回 MCP 管理工具（list / add / remove / reconnect）。
func mcpTools(p *AIChatPlugin) (llmtool.Tool, llmtool.Tool, llmtool.Tool, llmtool.Tool) {
	tools := newMCPTools(p)
	if len(tools) != 4 {
		panic("MCP 工具数量不符")
	}
	return tools[0], tools[1], tools[2], tools[3]
}

func TestMCPAddParamsValidation(t *testing.T) {
	cases := []struct {
		name    string
		params  mcpAddParams
		wantErr string
	}{
		{"非法名称", mcpAddParams{Name: "bad name!"}, "不合法"},
		{"stdio 缺 command", mcpAddParams{Name: "fs"}, "必须提供 command"},
		{"streamable 缺 endpoint", mcpAddParams{Name: "remote", Transport: "streamable"}, "必须提供 endpoint"},
		{"未知传输类型", mcpAddParams{Name: "x", Transport: "grpc"}, "未知传输类型"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, _, err := c.params.toServerEntry(); err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("期望错误包含 %q，实际: %v", c.wantErr, err)
			}
		})
	}

	// 合法 stdio 参数
	entry, cfg, err := (&mcpAddParams{
		Name:       "fs",
		Command:    "npx",
		Args:       []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
		Env:        []string{"API_KEY=xxx", "bad-item-no-equals"},
		TimeoutSec: 30,
	}).toServerEntry()
	if err != nil {
		t.Fatalf("合法参数校验失败: %v", err)
	}
	if entry.Transport != "stdio" || cfg.Command != "npx" || len(cfg.Args) != 3 {
		t.Fatalf("stdio 转换不符: %+v", cfg)
	}
	if len(cfg.Env) != 1 || cfg.Env["API_KEY"] != "xxx" {
		t.Fatalf("env 解析不符: %v", cfg.Env)
	}
	if cfg.Timeout.String() != "30s" || entry.TimeoutSecs != 30 {
		t.Fatalf("超时转换不符: %v / %d", cfg.Timeout, entry.TimeoutSecs)
	}

	// 合法 streamable 参数
	_, cfg2, err := (&mcpAddParams{
		Name:      "remote",
		Transport: "streamable",
		Endpoint:  "https://example.com/mcp",
		Headers:   []string{"Authorization=Bearer token"},
	}).toServerEntry()
	if err != nil {
		t.Fatalf("合法 streamable 参数校验失败: %v", err)
	}
	if cfg2.Endpoint != "https://example.com/mcp" || cfg2.Headers["Authorization"] != "Bearer token" {
		t.Fatalf("streamable 转换不符: %+v", cfg2)
	}
}

func TestMCPAddTool_NameConflict(t *testing.T) {
	p := newMCPToolPlugin(t)
	editor := p.ConfigEditor.(*fakeConfigEditor)
	editor.data[mcpConfigKey] = `{"servers":[{"name":"fs","command":"npx"}]}`

	_, addTool, _, _ := mcpTools(p)
	// 与配置中已有服务器重名：应在连接前直接报错
	_, err := addTool.Execute(context.Background(), &mcpAddParams{Name: "fs", Command: "npx"}, llmtool.CallBackFuncs{})
	if err == nil || !strings.Contains(err.Error(), "已存在") {
		t.Fatalf("重名应报错，实际: %v", err)
	}
}

func TestMCPRemoveTool(t *testing.T) {
	p := newMCPToolPlugin(t)
	editor := p.ConfigEditor.(*fakeConfigEditor)
	editor.data[mcpConfigKey] = `{"servers":[{"name":"fs","command":"npx"},{"name":"web","endpoint":"https://example.com/mcp"}]}`

	_, _, removeTool, _ := mcpTools(p)

	// 删除不存在的服务器
	if _, err := removeTool.Execute(context.Background(), &mcpRemoveParams{Name: "nope"}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("删除不存在的服务器应报错")
	}

	// 删除存在的服务器（运行时未注册，注销错误被容忍，配置仍应删除）
	out, err := removeTool.Execute(context.Background(), &mcpRemoveParams{Name: "fs"}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if !strings.Contains(out, "已删除") {
		t.Fatalf("删除提示不符: %s", out)
	}
	raw, _ := editor.Get(mcpConfigKey)
	if strings.Contains(raw.(string), `"fs"`) {
		t.Fatalf("配置中仍残留被删服务器: %s", raw)
	}
	if !strings.Contains(raw.(string), `"web"`) {
		t.Fatalf("其他服务器配置不应受影响: %s", raw)
	}
}

func TestMCPListTool(t *testing.T) {
	p := newMCPToolPlugin(t)
	listTool, _, _, _ := mcpTools(p)

	// 空配置
	out, err := listTool.Execute(context.Background(), &mcpListParams{}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(out, "没有配置任何 MCP 服务器") {
		t.Fatalf("空列表提示不符: %s", out)
	}

	// 有配置但未连接（运行时未注册）
	p.ConfigEditor.(*fakeConfigEditor).data[mcpConfigKey] =
		`{"servers":[{"name":"fs","command":"npx","args":["-y","server-fs"],"description":"文件系统"}]}`
	out, err = listTool.Execute(context.Background(), &mcpListParams{}, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !strings.Contains(out, "fs") || !strings.Contains(out, "文件系统") || !strings.Contains(out, "未连接") {
		t.Fatalf("列表内容不符: %s", out)
	}
}

func TestMCPReconnectTool_NotRegistered(t *testing.T) {
	p := newMCPToolPlugin(t)
	_, _, _, reconnectTool := mcpTools(p)
	// 运行时与配置中均不存在
	if _, err := reconnectTool.Execute(context.Background(), &mcpReconnectParams{Name: "ghost"}, llmtool.CallBackFuncs{}); err == nil {
		t.Fatal("重连不存在的服务器应报错")
	}
}
