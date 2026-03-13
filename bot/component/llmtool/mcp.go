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
	ToolFilter  ToolFilterFunc    `json:"-"`           // 工具过滤器（可选）
}

// ToolFilterFunc 工具过滤函数，返回 true 表示保留该工具
type ToolFilterFunc func(tool *mcp.Tool) bool

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

	// 应用工具过滤器
	if c.config.ToolFilter != nil {
		filtered := make([]*mcp.Tool, 0)
		for _, tool := range toolsResp.Tools {
			if c.config.ToolFilter(tool) {
				filtered = append(filtered, tool)
			}
		}
		c.tools = filtered
		log.Printf("[MCP:%s] 过滤后获取到 %d 个工具（原始 %d 个）",
			c.config.Name, len(c.tools), len(toolsResp.Tools))
	} else {
		c.tools = toolsResp.Tools
		log.Printf("[MCP:%s] 获取到 %d 个工具", c.config.Name, len(c.tools))
	}

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

// MCPToolManager 管理 MCP 工具的延迟加载
type MCPToolManager struct {
	client          *MCPClient
	toolCache       map[string]*MCPTool
	toolDefinitions []*mcp.Tool
}

// NewMCPToolManager 创建 MCP 工具管理器
func NewMCPToolManager(client *MCPClient) *MCPToolManager {
	return &MCPToolManager{
		client:    client,
		toolCache: make(map[string]*MCPTool),
	}
}

// Initialize 初始化管理器，连接并获取工具列表
func (m *MCPToolManager) Initialize(ctx context.Context) error {
	if err := m.client.Connect(ctx); err != nil {
		return err
	}
	m.toolDefinitions = m.client.GetTools()
	log.Printf("[MCP:%s] 发现 %d 个工具（延迟加载模式）", m.client.config.Name, len(m.toolDefinitions))
	return nil
}

// GetToolNames 获取所有工具名称和简短描述
func (m *MCPToolManager) GetToolNames() []map[string]string {
	result := make([]map[string]string, 0, len(m.toolDefinitions))
	for _, tool := range m.toolDefinitions {
		result = append(result, map[string]string{
			"name":        tool.Name,
			"description": tool.Description,
		})
	}
	return result
}

// LoadTool 按需加载具体工具
func (m *MCPToolManager) LoadTool(toolName string) (*MCPTool, error) {
	// 检查缓存
	if tool, ok := m.toolCache[toolName]; ok {
		return tool, nil
	}

	// 查找工具定义
	for _, toolDef := range m.toolDefinitions {
		if toolDef.Name == toolName {
			tool := NewMCPTool(m.client, toolDef)
			m.toolCache[toolName] = tool
			log.Printf("[MCP:%s] 加载工具: %s", m.client.config.Name, toolName)
			return tool, nil
		}
	}

	return nil, fmt.Errorf("工具 '%s' 不存在", toolName)
}

// RegisterMCP 注册MCP服务器中的所有工具到执行器（传统方式，会导致上下文爆炸）
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

// RegisterMCPWithDiscovery 使用工具发现模式注册 MCP（推荐方式，避免上下文爆炸）
func (e *ToolExecuter) RegisterMCPWithDiscovery(client *MCPClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := NewMCPToolManager(client)
	if err := manager.Initialize(ctx); err != nil {
		return err
	}

	// 注册工具发现工具
	discoveryTool := NewMCPDiscoveryTool(manager)
	e.Register(discoveryTool)

	// 注册工具加载器
	loaderTool := NewMCPLoaderTool(manager, e)
	e.Register(loaderTool)

	log.Printf("[MCP:%s] 工具发现模式注册完成", client.config.Name)
	return nil
}

// RegisterMCPWithConfig 使用配置直接注册MCP工具
func (e *ToolExecuter) RegisterMCPWithConfig(config *MCPConfig) error {
	client := NewMCPClient(config)
	return e.RegisterMCP(client)
}

// RegisterMCPWithConfigDiscovery 使用配置和工具发现模式注册 MCP（推荐）
func (e *ToolExecuter) RegisterMCPWithConfigDiscovery(config *MCPConfig) error {
	client := NewMCPClient(config)
	return e.RegisterMCPWithDiscovery(client)
}

// MCPDiscoveryTool 工具发现工具
type MCPDiscoveryTool struct {
	manager *MCPToolManager
}

func NewMCPDiscoveryTool(manager *MCPToolManager) *MCPDiscoveryTool {
	return &MCPDiscoveryTool{manager: manager}
}

func (t *MCPDiscoveryTool) Name() string {
	return fmt.Sprintf("mcp_discover_%s", t.manager.client.config.Name)
}

func (t *MCPDiscoveryTool) Description() string {
	return fmt.Sprintf("发现 %s MCP 服务器提供的所有可用工具。返回工具名称和简短描述列表。使用此工具了解有哪些工具可用，然后使用 mcp_load 工具加载具体工具。", t.manager.client.config.Description)
}

func (t *MCPDiscoveryTool) Params() any {
	return &struct{}{}
}

func (t *MCPDiscoveryTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	tools := t.manager.GetToolNames()
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// MCPLoaderTool 工具加载工具
type MCPLoaderTool struct {
	manager  *MCPToolManager
	executer *ToolExecuter
}

type MCPLoaderParams struct {
	ToolName string `json:"tool_name" desc:"要加载的工具名称"`
}

func NewMCPLoaderTool(manager *MCPToolManager, executer *ToolExecuter) *MCPLoaderTool {
	return &MCPLoaderTool{
		manager:  manager,
		executer: executer,
	}
}

func (t *MCPLoaderTool) Name() string {
	return fmt.Sprintf("mcp_load_%s", t.manager.client.config.Name)
}

func (t *MCPLoaderTool) Description() string {
	return fmt.Sprintf("加载 %s MCP 服务器的指定工具。加载后该工具将可用于后续调用。参数: tool_name - 要加载的工具名称（从 mcp_discover 获取）", t.manager.client.config.Description)
}

func (t *MCPLoaderTool) Params() any {
	return &MCPLoaderParams{}
}

func (t *MCPLoaderTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	p := params.(*MCPLoaderParams)

	tool, err := t.manager.LoadTool(p.ToolName)
	if err != nil {
		return "", err
	}

	// 动态注册工具到执行器
	t.executer.Register(tool)

	schema := tool.GetInputSchema()
	schemaJSON, _ := json.MarshalIndent(schema, "", "  ")

	return fmt.Sprintf("工具 '%s' 已加载成功。\n描述: %s\n参数定义:\n%s",
		tool.Name(), tool.Description(), string(schemaJSON)), nil
}

// 常用工具过滤器

// FilterByPrefix 创建按名称前缀过滤的过滤器
func FilterByPrefix(prefix string) ToolFilterFunc {
	return func(tool *mcp.Tool) bool {
		return strings.HasPrefix(tool.Name, prefix)
	}
}

// FilterByNames 创建按名称列表过滤的过滤器
func FilterByNames(names ...string) ToolFilterFunc {
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}
	return func(tool *mcp.Tool) bool {
		return nameSet[tool.Name]
	}
}

// FilterByKeywords 创建按描述关键词过滤的过滤器
func FilterByKeywords(keywords ...string) ToolFilterFunc {
	return func(tool *mcp.Tool) bool {
		desc := strings.ToLower(tool.Description)
		for _, keyword := range keywords {
			if strings.Contains(desc, strings.ToLower(keyword)) {
				return true
			}
		}
		return false
	}
}

// CombineFilters 组合多个过滤器（AND 逻辑）
func CombineFilters(filters ...ToolFilterFunc) ToolFilterFunc {
	return func(tool *mcp.Tool) bool {
		for _, filter := range filters {
			if !filter(tool) {
				return false
			}
		}
		return true
	}
}

// AnyFilter 组合多个过滤器（OR 逻辑）
func AnyFilter(filters ...ToolFilterFunc) ToolFilterFunc {
	return func(tool *mcp.Tool) bool {
		for _, filter := range filters {
			if filter(tool) {
				return true
			}
		}
		return false
	}
}
