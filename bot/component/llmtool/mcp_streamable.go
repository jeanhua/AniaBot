package llmtool

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MCPStreamableClient MCP Streamable HTTP 客户端
type MCPStreamableClient struct {
	config      *MCPConfig
	httpClient  *http.Client
	sessionID   string
	mu          sync.RWMutex
	tools       []MCPToolDefinition
	requestID   atomic.Int64

	// SSE 相关
	sseConn     io.ReadCloser
	sseScanner  *bufio.Scanner
	sseCancel   context.CancelFunc
	responses   map[int64]chan *MCPJSONRPCResponse
	respMu      sync.RWMutex
}

// NewMCPStreamableClient 创建新的 MCP Streamable HTTP 客户端
func NewMCPStreamableClient(config *MCPConfig) *MCPStreamableClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &MCPStreamableClient{
		config:     config,
		httpClient: &http.Client{Timeout: config.Timeout},
		responses:  make(map[int64]chan *MCPJSONRPCResponse),
	}
}

// Connect 连接到 MCP Streamable HTTP 服务器
func (c *MCPStreamableClient) Connect(ctx context.Context) error {
	// 1. 首先发送初始化请求获取 session ID
	if err := c.initialize(ctx); err != nil {
		return fmt.Errorf("初始化失败: %w", err)
	}

	// 2. 建立 SSE 连接接收服务器消息
	if err := c.connectSSE(ctx); err != nil {
		return fmt.Errorf("建立 SSE 连接失败: %w", err)
	}

	// 3. 获取工具列表
	tools, err := c.listTools(ctx)
	if err != nil {
		return fmt.Errorf("获取工具列表失败: %w", err)
	}
	c.tools = tools

	return nil
}

// initialize 发送初始化请求，获取或确认 session ID
func (c *MCPStreamableClient) initialize(ctx context.Context) error {
	params := map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{},
		"clientInfo": map[string]string{
			"name":    "ania-bot-mcp-client",
			"version": "1.0.0",
		},
	}

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "initialize",
	}

	paramsData, _ := json.Marshal(params)
	req.Params = paramsData

	resp, err := c.sendHTTPRequest(ctx, req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("初始化错误: %s", resp.Error.Message)
	}

	return nil
}

// connectSSE 建立 SSE 连接
func (c *MCPStreamableClient) connectSSE(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 关闭之前的连接
	if c.sseCancel != nil {
		c.sseCancel()
	}
	if c.sseConn != nil {
		c.sseConn.Close()
	}

	// 创建新的 SSE 连接
	url := c.config.Endpoint + "/message"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// 设置 Accept 头部为 text/event-stream
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// 如果有 session ID，添加到请求头
	if c.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", c.sessionID)
	}

	// 添加自定义头部
	for k, v := range c.config.Headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE 连接失败，状态码: %d", resp.StatusCode)
	}

	// 保存 session ID
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.sessionID = sessionID
	}

	c.sseConn = resp.Body
	c.sseScanner = bufio.NewScanner(resp.Body)

	// 启动后台 goroutine 处理 SSE 事件
	sseCtx, cancel := context.WithCancel(context.Background())
	c.sseCancel = cancel
	go c.handleSSEEvents(sseCtx)

	return nil
}

// handleSSEEvents 处理 SSE 事件流
func (c *MCPStreamableClient) handleSSEEvents(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			// 记录 panic 但保持运行
		}
	}()

	var currentData strings.Builder

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !c.sseScanner.Scan() {
			// 连接断开，尝试重连
			time.Sleep(2 * time.Second)
			c.mu.RLock()
			sessionID := c.sessionID
			c.mu.RUnlock()

			if sessionID != "" {
				if err := c.connectSSE(context.Background()); err != nil {
					// 重连失败，继续等待
					continue
				}
			}
			continue
		}

		line := c.sseScanner.Text()

		// SSE 格式: data: {...}
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			currentData.WriteString(data)
		} else if line == "" {
			// 空行表示事件结束
			if currentData.Len() > 0 {
				c.handleSSEMessage(currentData.String())
				currentData.Reset()
			}
		}
	}
}

// handleSSEMessage 处理 SSE 消息
func (c *MCPStreamableClient) handleSSEMessage(data string) {
	var resp MCPJSONRPCResponse
	if err := json.Unmarshal([]byte(data), &resp); err != nil {
		return
	}

	// 查找等待的响应通道
	c.respMu.RLock()
	ch, ok := c.responses[int64(resp.ID)]
	c.respMu.RUnlock()

	if ok {
		select {
		case ch <- &resp:
		default:
		}
	}
}

// sendHTTPRequest 发送 HTTP POST 请求
func (c *MCPStreamableClient) sendHTTPRequest(ctx context.Context, req MCPJSONRPCRequest) (*MCPJSONRPCResponse, error) {
	c.mu.RLock()
	sessionID := c.sessionID
	c.mu.RUnlock()

	url := c.config.Endpoint + "/message"

	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// 添加 session ID
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	// 添加自定义头部
	for k, v := range c.config.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 更新 session ID
	if newSessionID := resp.Header.Get("Mcp-Session-Id"); newSessionID != "" {
		c.mu.Lock()
		c.sessionID = newSessionID
		c.mu.Unlock()
	}

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 如果是 SSE 响应，等待通过 SSE 通道接收
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return c.waitForResponse(ctx, req.ID)
	}

	// 直接解析 JSON 响应
	var jsonResp MCPJSONRPCResponse
	if err := json.Unmarshal(body, &jsonResp); err != nil {
		return nil, err
	}

	return &jsonResp, nil
}

// waitForResponse 等待响应
func (c *MCPStreamableClient) waitForResponse(ctx context.Context, id int) (*MCPJSONRPCResponse, error) {
	ch := make(chan *MCPJSONRPCResponse, 1)

	c.respMu.Lock()
	c.responses[int64(id)] = ch
	c.respMu.Unlock()

	defer func() {
		c.respMu.Lock()
		delete(c.responses, int64(id))
		c.respMu.Unlock()
		close(ch)
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-ch:
		if resp == nil {
			return nil, fmt.Errorf("连接已关闭")
		}
		return resp, nil
	case <-time.After(c.config.Timeout):
		return nil, fmt.Errorf("请求超时")
	}
}

// listTools 获取工具列表
func (c *MCPStreamableClient) listTools(ctx context.Context) ([]MCPToolDefinition, error) {
	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/list",
	}

	resp, err := c.sendHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("获取工具列表错误: %s", resp.Error.Message)
	}

	var result struct {
		Tools []MCPToolDefinition `json:"tools"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	return result.Tools, nil
}

// GetTools 获取工具列表
func (c *MCPStreamableClient) GetTools() []MCPToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.tools
}

// CallTool 调用工具
func (c *MCPStreamableClient) CallTool(ctx context.Context, toolName string, arguments json.RawMessage) (string, error) {
	params := map[string]any{
		"name":      toolName,
		"arguments": arguments,
	}
	paramsData, _ := json.Marshal(params)

	req := MCPJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      c.nextRequestID(),
		Method:  "tools/call",
		Params:  paramsData,
	}

	resp, err := c.sendHTTPRequest(ctx, req)
	if err != nil {
		return "", err
	}

	if resp.Error != nil {
		return "", fmt.Errorf("工具调用错误: %s", resp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError,omitempty"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
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

	return text, nil
}

// nextRequestID 生成下一个请求 ID
func (c *MCPStreamableClient) nextRequestID() int {
	return int(c.requestID.Add(1))
}

// Close 关闭连接
func (c *MCPStreamableClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sseCancel != nil {
		c.sseCancel()
	}
	if c.sseConn != nil {
		c.sseConn.Close()
	}

	return nil
}
