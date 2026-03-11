package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// MCPConfig MCP服务器配置
type MCPConfig struct {
	Name        string            `json:"name"`        // MCP服务器名称
	Endpoint    string            `json:"endpoint"`    // MCP服务器端点URL
	Headers     map[string]string `json:"headers"`     // 自定义请求头
	Timeout     time.Duration     `json:"timeout"`     // 请求超时时间
	Description string            `json:"description"` // MCP服务器描述
}

// MCPToolDefinition MCP工具定义
type MCPToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// MCPListToolsResponse MCP列出工具响应
type MCPListToolsResponse struct {
	Tools []MCPToolDefinition `json:"tools"`
}

// MCPCallToolRequest MCP调用工具请求
type MCPCallToolRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// MCPCallToolResponse MCP调用工具响应
type MCPCallToolResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// MCPClient MCP客户端
type MCPClient struct {
	config     *MCPConfig
	httpClient *resty.Client
	tools      []MCPToolDefinition
}

// NewMCPClient 创建新的MCP客户端
func NewMCPClient(config *MCPConfig) *MCPClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	client := resty.New().
		SetTimeout(config.Timeout).
		SetHeaders(config.Headers)

	return &MCPClient{
		config:     config,
		httpClient: client,
	}
}

// Connect 连接到MCP服务器并获取工具列表
func (c *MCPClient) Connect(ctx context.Context) error {
	// 尝试从服务器获取工具列表
	// MCP协议支持通过 /tools 或 /list-tools 端点获取工具列表
	endpoints := []string{"/tools", "/list-tools", "/mcp/tools"}

	for _, endpoint := range endpoints {
		url := c.config.Endpoint + endpoint
		resp, err := c.httpClient.R().
			SetContext(ctx).
			Get(url)

		if err != nil {
			continue
		}

		if resp.StatusCode() == http.StatusOK {
			var listResp MCPListToolsResponse
			if err := json.Unmarshal(resp.Body(), &listResp); err == nil {
				c.tools = listResp.Tools
				return nil
			}
		}
	}

	// 如果无法自动发现，尝试通过配置文件或返回空列表
	return fmt.Errorf("无法从MCP服务器 %s 获取工具列表", c.config.Name)
}

// GetTools 获取MCP服务器提供的所有工具定义
func (c *MCPClient) GetTools() []MCPToolDefinition {
	return c.tools
}

// CallTool 调用MCP工具
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	req := MCPCallToolRequest{
		Name:      toolName,
		Arguments: arguments,
	}

	// 尝试不同的调用端点
	endpoints := []string{"/call", "/invoke", "/mcp/call", "/tools/" + toolName}

	for _, endpoint := range endpoints {
		url := c.config.Endpoint + endpoint
		resp, err := c.httpClient.R().
			SetContext(ctx).
			SetHeader("Content-Type", "application/json").
			SetBody(req).
			Post(url)

		if err != nil {
			continue
		}

		if resp.StatusCode() == http.StatusOK {
			var callResp MCPCallToolResponse
			if err := json.Unmarshal(resp.Body(), &callResp); err == nil {
				if callResp.Error != "" {
					return "", fmt.Errorf("MCP工具调用错误: %s", callResp.Error)
				}
				return callResp.Result, nil
			}
			// 如果不是标准响应格式，直接返回body
			return string(resp.Body()), nil
		}
	}

	return "", fmt.Errorf("调用MCP工具 %s 失败", toolName)
}

// MCPTool 包装MCP工具为本地Tool接口
type MCPTool struct {
	client     *MCPClient
	definition MCPToolDefinition
}

// NewMCPTool 创建新的MCP工具包装器
func NewMCPTool(client *MCPClient, definition MCPToolDefinition) *MCPTool {
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
	return &map[string]any{}
}

// Execute 执行MCP工具
func (t *MCPTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	// 将参数序列化为JSON
	args, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %w", err)
	}

	// 调用远程MCP工具
	result, err := t.client.CallTool(ctx, t.definition.Name, args)
	if err != nil {
		return "", err
	}

	return result, nil
}

// RegisterMCP 注册MCP服务器中的所有工具到执行器
func (e *ToolExecuter) RegisterMCP(client *MCPClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 连接到MCP服务器
	if err := client.Connect(ctx); err != nil {
		return err
	}

	// 注册所有工具
	for _, toolDef := range client.GetTools() {
		tool := NewMCPTool(client, toolDef)
		e.Register(tool)
	}

	return nil
}

// RegisterMCPWithConfig 使用配置直接注册MCP工具
func (e *ToolExecuter) RegisterMCPWithConfig(config *MCPConfig) error {
	client := NewMCPClient(config)
	return e.RegisterMCP(client)
}

// IsMCPTool 检查工具是否是MCP工具
func (t *MCPTool) IsMCPTool() bool {
	return true
}

// GetMCPToolDefinition 获取MCP工具原始定义
func (t *MCPTool) GetMCPToolDefinition() MCPToolDefinition {
	return t.definition
}

// GetMCPClient 获取MCP客户端
func (t *MCPTool) GetMCPClient() *MCPClient {
	return t.client
}

// NormalizeMCPEndpoint 规范化MCP端点URL
func NormalizeMCPEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = "http://" + endpoint
	}
	return strings.TrimSuffix(endpoint, "/")
}
