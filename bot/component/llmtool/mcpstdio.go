package llmtool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

// MCPStdioConfig MCP stdio 服务器配置
type MCPStdioConfig struct {
	Name        string            `json:"name"`
	Command     string            `json:"command"`
	Args        []string          `json:"args"`
	Env         map[string]string `json:"env"`
	Timeout     time.Duration     `json:"timeout"`
	Description string            `json:"description"`
}

// MCPStdioClient MCP stdio 客户端
type MCPStdioClient struct {
	config    *MCPStdioConfig
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	writeMu   sync.Mutex // 只保护 stdin 写入
	tools     []MCPToolDefinition
	requestID atomic.Int64 // 原子操作，支持并发安全

	// 响应分发：后台 goroutine 读取所有响应，按 ID 分发
	pendingMu sync.Mutex
	pending   map[int]chan *MCPJSONRPCResponse
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
		config.Timeout = 120 * time.Second // npx 需要更长时间下载
	}

	return &MCPStdioClient{
		config:  config,
		pending: make(map[int]chan *MCPJSONRPCResponse),
	}
}

// Connect 启动 MCP stdio 服务器并获取工具列表
func (c *MCPStdioClient) Connect(ctx context.Context) error {
	log.Printf("[MCP:%s] 环境变量配置: %v", c.config.Name, c.config.Env)
	connectCtx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	log.Printf("[MCP:%s] 启动 stdio 进程: %s %v", c.config.Name, c.config.Command, c.config.Args)

	c.cmd = exec.CommandContext(connectCtx, c.config.Command, c.config.Args...)

	// 设置环境变量（继承系统环境变量并添加自定义环境变量）
	if len(c.config.Env) > 0 {
		env := os.Environ()
		for k, v := range c.config.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		c.cmd.Env = env
	}

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

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("无法启动 MCP 服务器: %w", err)
	}

	log.Printf("[MCP:%s] 进程已启动，PID: %d", c.config.Name, c.cmd.Process.Pid)

	// 后台读取 stderr（用于调试 npx 下载进度等）
	go c.readStderr()

	// 启动后台响应分发 goroutine（不持任何锁，独立读取 stdout）
	go c.readLoop()

	// 发送初始化请求
	log.Printf("[MCP:%s] 发送初始化请求...", c.config.Name)
	if err := c.initialize(connectCtx); err != nil {
		c.Close()
		return fmt.Errorf("初始化失败: %w", err)
	}
	log.Printf("[MCP:%s] 初始化成功", c.config.Name)

	// 获取工具列表
	log.Printf("[MCP:%s] 获取工具列表...", c.config.Name)
	tools, err := c.listTools(connectCtx)
	if err != nil {
		c.Close()
		return fmt.Errorf("无法获取工具列表: %w", err)
	}
	c.tools = tools
	log.Printf("[MCP:%s] 获取到 %d 个工具", c.config.Name, len(tools))

	return nil
}

// readLoop 是唯一读取 stdout 的 goroutine，负责将响应分发给等待的请求
func (c *MCPStdioClient) readLoop() {
	scanner := bufio.NewScanner(c.stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var resp MCPJSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			log.Printf("[MCP:%s] 解析响应失败: %v, raw: %s", c.config.Name, err, line)
			continue
		}

		// 通知等待该 ID 的请求
		c.pendingMu.Lock()
		ch, ok := c.pending[resp.ID]
		if ok {
			delete(c.pending, resp.ID)
		}
		c.pendingMu.Unlock()

		if ok {
			ch <- &resp
		} else {
			// 可能是通知（ID=0）或未匹配的响应，忽略
			log.Printf("[MCP:%s] 收到未匹配响应，ID=%d", c.config.Name, resp.ID)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[MCP:%s] stdout 读取错误: %v", c.config.Name, err)
	}

	// stdout 关闭，通知所有等待者
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
}

// readStderr 读取 stderr 输出用于调试
func (c *MCPStdioClient) readStderr() {
	scanner := bufio.NewScanner(c.stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		log.Printf("[MCP:%s:stderr] %s", c.config.Name, scanner.Text())
	}
}

// sendRequest 发送 JSON-RPC 请求并等待响应（不持任何锁等待）
func (c *MCPStdioClient) sendRequest(ctx context.Context, req MCPJSONRPCRequest) (*MCPJSONRPCResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 注册等待通道（在发送前注册，避免响应先到）
	ch := make(chan *MCPJSONRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[req.ID] = ch
	c.pendingMu.Unlock()

	// 发送请求
	c.writeMu.Lock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	c.writeMu.Unlock()
	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, req.ID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}

	// 等待响应（不持任何锁）
	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, req.ID)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("请求超时或取消: %w", ctx.Err())
	case resp, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("连接已关闭")
		}
		return resp, nil
	}
}

// sendNotification 发送通知（不需要响应，不持写锁以外的锁）
func (c *MCPStdioClient) sendNotification(method string) {
	notify := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	data, err := json.Marshal(notify)
	if err != nil {
		return
	}

	c.writeMu.Lock()
	fmt.Fprintf(c.stdin, "%s\n", data)
	c.writeMu.Unlock()
}

// nextRequestID 生成下一个请求 ID（原子操作，并发安全）
func (c *MCPStdioClient) nextRequestID() int {
	return int(c.requestID.Add(1))
}

// initialize 发送初始化请求
func (c *MCPStdioClient) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]any{
			"roots":    map[string]any{"listChanged": true},
			"sampling": map[string]any{},
		},
		"clientInfo": map[string]string{
			"name":    "ania-bot-mcp-client",
			"version": "1.0.0",
		},
	}
	paramsData, _ := json.Marshal(params)

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
		Params:  paramsData,
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("初始化请求失败: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("MCP 初始化错误: %s", resp.Error.Message)
	}

	log.Printf("[MCP:%s] 收到初始化响应，发送 initialized 通知...", c.config.Name)
	c.sendNotification("notifications/initialized")
	log.Printf("[MCP:%s] initialized 通知已发送", c.config.Name)

	return nil
}

// listTools 向 MCP 服务器请求工具列表
func (c *MCPStdioClient) listTools(ctx context.Context) ([]MCPToolDefinition, error) {
	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
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

// GetTools 获取 MCP 服务器提供的所有工具定义
func (c *MCPStdioClient) GetTools() []MCPToolDefinition {
	return c.tools
}

// CallTool 调用 MCP 工具
func (c *MCPStdioClient) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	// 直接使用原始 JSON arguments，避免 map 解析导致字段丢失或类型变化
	// 如果 arguments 为空或无效，使用空对象
	if len(arguments) == 0 || !json.Valid(arguments) {
		arguments = json.RawMessage("{}")
	}

	// 调试模式：打印工具输入
	if c.config.Env != nil {
		if v, ok := c.config.Env["DEBUG"]; ok {
			debug := v == "true" || v == "1" || v == "yes"
			if debug {
				log.Printf("[MCP:%s:DEBUG] 工具调用输入: name=%s, arguments=%s", c.config.Name, toolName, string(arguments))
			}
		}
	}

	// 构建标准 MCP tools/call 请求参数
	// arguments 直接嵌入，保留原始字段名和类型
	type callParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	paramsData, err := json.Marshal(callParams{
		Name:      toolName,
		Arguments: arguments,
	})
	if err != nil {
		return "", fmt.Errorf("序列化工具调用参数失败: %w", err)
	}

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/call",
		Params:  json.RawMessage(paramsData),
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
		// 调试模式：打印原始响应
		if c.config.Env != nil {
			if v, ok := c.config.Env["DEBUG"]; ok {
				debug := v == "true" || v == "1" || v == "yes"
				if debug {
					log.Printf("[MCP:%s:DEBUG] 工具调用输出 (原始): %s", c.config.Name, string(resp.Result))
				}
			}
		}
		return string(resp.Result), nil
	}

	if result.IsError {
		return "", fmt.Errorf("工具执行错误")
	}

	var text string
	for _, content := range result.Content {
		if content.Type == "text" {
			text += content.Text
		}
	}

	// 调试模式：打印工具输出
	if c.config.Env != nil {
		if v, ok := c.config.Env["DEBUG"]; ok {
			debug := v == "true" || v == "1" || v == "yes"
			if debug {
				log.Printf("[MCP:%s:DEBUG] 工具调用输出: %s", c.config.Name, text)
			}
		}
	}

	return text, nil
}

// Close 关闭 MCP stdio 客户端
func (c *MCPStdioClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}
