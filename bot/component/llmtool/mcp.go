package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPConfig MCP服务器配置
type MCPConfig struct {
	Name        string            `json:"name"`        // MCP服务器名称
	Command     string            `json:"command"`     // 启动命令
	Args        []string          `json:"args"`        // 命令参数
	Env         map[string]string `json:"env"`         // 环境变量
	Timeout     time.Duration     `json:"timeout"`     // 请求超时时间
	Description string            `json:"description"` // MCP服务器描述
}

// MCPClient MCP客户端（基于官方SDK）
type MCPClient struct {
	config  *MCPConfig
	client  *mcp.Client
	session *mcp.ClientSession
	tools   []*mcp.Tool
}

// NewMCPClient 创建新的MCP客户端
func NewMCPClient(config *MCPConfig) *MCPClient {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}

	client := mcp.NewClient(
		&mcp.Implementation{
			Name:    "ania-bot-mcp-client",
			Version: "1.0.0",
		},
		nil,
	)

	return &MCPClient{
		config: config,
		client: client,
	}
}

// Connect 连接到MCP服务器并获取工具列表
func (c *MCPClient) Connect(ctx context.Context) error {
	connectCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	log.Printf("[MCP:%s] 启动进程: %s %v", c.config.Name, c.config.Command, c.config.Args)

	// 创建命令传输
	cmd := exec.Command(c.config.Command, c.config.Args...)

	// 设置环境变量
	if len(c.config.Env) > 0 {
		env := cmd.Environ()
		for k, v := range c.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	transport := &mcp.CommandTransport{Command: cmd}

	// 连接到服务器
	session, err := c.client.Connect(connectCtx, transport, nil)
	if err != nil {
		return fmt.Errorf("连接MCP服务器失败: %w", err)
	}
	c.session = session

	log.Printf("[MCP:%s] 连接成功，获取工具列表...", c.config.Name)

	// 获取工具列表
	toolsResp, err := session.ListTools(connectCtx, &mcp.ListToolsParams{})
	if err != nil {
		c.Close()
		return fmt.Errorf("获取工具列表失败: %w", err)
	}

	c.tools = toolsResp.Tools
	log.Printf("[MCP:%s] 获取到 %d 个工具", c.config.Name, len(c.tools))

	return nil
}

// GetTools 获取MCP服务器提供的所有工具定义
func (c *MCPClient) GetTools() []*mcp.Tool {
	return c.tools
}

// CallTool 调用MCP工具
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	if c.session == nil {
		return "", fmt.Errorf("MCP客户端未连接")
	}

	log.Printf("[MCP:%s] 调用工具: %s", c.config.Name, toolName)

	// 解析参数
	var args map[string]any
	if len(arguments) > 0 {
		if err := json.Unmarshal(arguments, &args); err != nil {
			return "", fmt.Errorf("解析参数失败: %w", err)
		}
	}

	// 调用工具
	result, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("调用工具失败: %w", err)
	}

	if result.IsError {
		return "", fmt.Errorf("工具执行错误")
	}

	// 提取文本内容
	var textBuilder strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			textBuilder.WriteString(textContent.Text)
		}
	}

	return textBuilder.String(), nil
}

// Close 关闭MCP客户端
func (c *MCPClient) Close() error {
	if c.session != nil {
		return c.session.Close()
	}
	return nil
}

// MCPTool 包装MCP工具为本地Tool接口
type MCPTool struct {
	client     *MCPClient
	definition *mcp.Tool
}

// NewMCPTool 创建新的MCP工具包装器
func NewMCPTool(client *MCPClient, definition *mcp.Tool) *MCPTool {
	return &MCPTool{
		client:     client,
		definition: definition,
	}
}

// Name 返回工具名称
func (t *MCPTool) Name() string {
	return t.definition.Name
}

// Description 返回工具描述
func (t *MCPTool) Description() string {
	return t.definition.Description
}

// Params 返回工具参数定义
func (t *MCPTool) Params() any {
	return &struct{}{}
}

// GetInputSchema 返回 MCP 工具的输入 schema
func (t *MCPTool) GetInputSchema() map[string]any {
	if t.definition.InputSchema == nil {
		return nil
	}
	if schema, ok := t.definition.InputSchema.(map[string]any); ok {
		return schema
	}
	return nil
}

// Execute 执行MCP工具（标准接口）
func (t *MCPTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	args, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %w", err)
	}
	return t.ExecuteWithArgs(ctx, args, callbacks)
}

// ExecuteWithArgs 使用原始 JSON 参数执行 MCP 工具
func (t *MCPTool) ExecuteWithArgs(ctx context.Context, args json.RawMessage, callbacks CallBackFuncs) (string, error) {
	result, err := t.client.CallTool(ctx, t.definition.Name, args)
	if err != nil {
		return "", fmt.Errorf("MCP工具调用失败: %w", err)
	}
	return result, nil
}

// IsMCPTool 检查工具是否是MCP工具
func (t *MCPTool) IsMCPTool() bool {
	return true
}

// GetMCPToolDefinition 获取MCP工具原始定义
func (t *MCPTool) GetMCPToolDefinition() *mcp.Tool {
	return t.definition
}

// RegisterMCP 注册MCP服务器中的所有工具到执行器
func (e *ToolExecuter) RegisterMCP(client *MCPClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到MCP服务器
	if err := client.Connect(ctx); err != nil {
		return err
	}

	tools := client.GetTools()
	log.Printf("[MCP] 共发现 %d 个工具，开始注册...", len(tools))

	// 注册所有工具
	for _, toolDef := range tools {
		tool := NewMCPTool(client, toolDef)
		e.Register(tool)
	}

	log.Printf("[MCP] 所有工具注册完成")
	return nil
}

// RegisterMCPWithConfig 使用配置直接注册MCP工具
func (e *ToolExecuter) RegisterMCPWithConfig(config *MCPConfig) error {
	client := NewMCPClient(config)
	return e.RegisterMCP(client)
}
