package llmtool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// MCPStdioConfig MCP stdio 服务器配置
type MCPStdioConfig struct {
	Name        string            `json:"name"`        // MCP服务器名称
	Command     string            `json:"command"`     // 启动命令
	Args        []string          `json:"args"`        // 命令参数
	Env         map[string]string `json:"env"`         // 环境变量
	Timeout     time.Duration     `json:"timeout"`     // 请求超时时间
	Description string            `json:"description"` // MCP服务器描述
}

// MCPStdioClient MCP stdio 客户端
type MCPStdioClient struct {
	config  *MCPStdioConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	scanner *bufio.Scanner
	mu      sync.Mutex
	tools   []MCPToolDefinition
}

// MCPJSONRPCRequest JSON-RPC 请求
type MCPJSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPJSONRPCResponse JSON-RPC 响应
type MCPJSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *MCPJSONRPCError `json:"error,omitempty"`
}

// MCPJSONRPCError JSON-RPC 错误
type MCPJSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewMCPStdioClient 创建新的 MCP stdio 客户端
func NewMCPStdioClient(config *MCPStdioConfig) *MCPStdioClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	return &MCPStdioClient{
		config: config,
	}
}

// Connect 启动 MCP stdio 服务器并获取工具列表
func (c *MCPStdioClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 创建命令
	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// 设置环境变量
	if len(c.config.Env) > 0 {
		env := c.cmd.Environ()
		for k, v := range c.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		c.cmd.Env = env
	}

	// 获取管道
	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("无法获取 stdin 管道: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("无法获取 stdout 管道: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("无法获取 stderr 管道: %w", err)
	}

	// 启动进程
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("无法启动 MCP 服务器: %w", err)
	}

	// 创建 scanner 读取 stdout
	c.scanner = bufio.NewScanner(c.stdout)

	// 获取工具列表
	tools, err := c.listTools(ctx)
	if err != nil {
		c.Close()
		return fmt.Errorf("无法获取工具列表: %w", err)
	}
	c.tools = tools

	return nil
}

// listTools 向 MCP 服务器请求工具列表
func (c *MCPStdioClient) listTools(ctx context.Context) ([]MCPToolDefinition, error) {
	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP 错误: %s", resp.Error.Message)
	}

	var result struct {
		Tools []MCPToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析工具列表失败: %w", err)
	}

	return result.Tools, nil
}

// sendRequest 发送 JSON-RPC 请求并等待响应
func (c *MCPStdioClient) sendRequest(ctx context.Context, req MCPJSONRPCRequest) (*MCPJSONRPCResponse, error) {
	// 序列化请求
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 发送请求（添加换行符）
	c.mu.Lock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	c.mu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 等待响应
	done := make(chan *MCPJSONRPCResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		if !c.scanner.Scan() {
			errChan <- fmt.Errorf("读取响应失败: %v", c.scanner.Err())
			return
		}

		line := c.scanner.Text()
		var resp MCPJSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			errChan <- fmt.Errorf("解析响应失败: %w", err)
			return
		}
		done <- &resp
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errChan:
		return nil, err
	case resp := <-done:
		return resp, nil
	}
}

// GetTools 获取 MCP 服务器提供的所有工具定义
func (c *MCPStdioClient) GetTools() []MCPToolDefinition {
	return c.tools
}

// CallTool 调用 MCP 工具
func (c *MCPStdioClient) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	params := map[string]any{
		"name":      toolName,
		"arguments": arguments,
	}
	paramsData, err := json.Marshal(params)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %w", err)
	}

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  paramsData,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("MCP 工具调用错误: %s", resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		// 如果不是标准格式，直接返回原始结果
		return string(resp.Result), nil
	}

	if result.IsError {
		return "", fmt.Errorf("工具执行错误: %s", result.Content)
	}

	// 合并所有文本内容
	var text string
	for _, content := range result.Content {
		if content.Type == "text" {
			text += content.Text
		}
	}

	return text, nil
}

// Close 关闭 MCP stdio 客户端
func (c *MCPStdioClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin != nil {
		c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}

	return nil
}
