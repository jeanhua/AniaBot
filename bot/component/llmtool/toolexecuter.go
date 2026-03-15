package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tmc/langchaingo/llms"
)

type ToolExecuter struct {
	tools       map[string]Tool
	mcpManagers []*MCPToolManager // 用于 session 创建时注入 LoaderTool
}

func NewToolExecuter() *ToolExecuter {
	return &ToolExecuter{
		tools:       make(map[string]Tool),
		mcpManagers: make([]*MCPToolManager, 0),
	}
}

func (e *ToolExecuter) Register(tool Tool) {
	if _, ok := e.tools[tool.Name()]; ok {
		panic("tool already registered")
	}
	e.tools[tool.Name()] = tool
}

func (e *ToolExecuter) Tools() []llms.Tool {
	return e.toolsWithSession(nil)
}

// toolsWithSession 合并共享工具和会话级动态工具，会话工具优先
func (e *ToolExecuter) toolsWithSession(sessionTools map[string]Tool) []llms.Tool {
	tools := make([]llms.Tool, 0, len(e.tools)+len(sessionTools))
	for _, tool := range e.tools {
		tools = append(tools, structToOpenAITool(tool))
	}
	for _, tool := range sessionTools {
		tools = append(tools, structToOpenAITool(tool))
	}
	return tools
}

func (e *ToolExecuter) resolveTool(name string, sessionTools map[string]Tool) (Tool, bool) {
	if sessionTools != nil {
		if t, ok := sessionTools[name]; ok {
			return t, true
		}
	}
	t, ok := e.tools[name]
	return t, ok
}

func (e *ToolExecuter) Execute(ctx context.Context, call llms.ToolCall, callbacks CallBackFuncs) (string, error) {
	return e.executeWithSession(ctx, call, callbacks, nil)
}

func (e *ToolExecuter) executeWithSession(ctx context.Context, call llms.ToolCall, callbacks CallBackFuncs, sessionTools map[string]Tool) (string, error) {
	tool, ok := e.resolveTool(call.FunctionCall.Name, sessionTools)
	if !ok {
		return "", fmt.Errorf("tool '%s' not found. Available tools: %v",
			call.FunctionCall.Name, e.getToolNames())
	}

	// 检查是否是 MCP 工具
	if mcpTool, ok := tool.(*MCPTool); ok {
		result, err := mcpTool.ExecuteWithArgs(ctx, []byte(call.FunctionCall.Arguments), callbacks)
		if err != nil {
			return "", fmt.Errorf("MCP tool '%s' execution failed: %w\nArguments: %s",
				call.FunctionCall.Name, err, call.FunctionCall.Arguments)
		}
		return result, nil
	}

	params := reflect.New(reflect.TypeOf(tool.Params()).Elem()).Interface()
	if err := json.Unmarshal([]byte(call.FunctionCall.Arguments), params); err != nil {
		return "", fmt.Errorf("failed to parse arguments for tool '%s': %w\nArguments: %s\nExpected schema: %+v",
			call.FunctionCall.Name, err, call.FunctionCall.Arguments, tool.Params())
	}

	result, err := tool.Execute(ctx, params, callbacks)
	if err != nil {
		return "", fmt.Errorf("tool '%s' execution failed: %w", call.FunctionCall.Name, err)
	}
	return result, nil
}

// getToolNames 获取所有已注册工具的名称列表
func (e *ToolExecuter) getToolNames() []string {
	names := make([]string, 0, len(e.tools))
	for name := range e.tools {
		names = append(names, name)
	}
	return names
}

// NewSessionExecutor 创建一个会话级执行器，动态工具隔离到会话，不污染共享层
func (e *ToolExecuter) NewSessionExecutor() *SessionToolExecutor {
	session := &SessionToolExecutor{
		shared:       e,
		sessionTools: make(map[string]Tool),
	}
	// 为每个 MCP manager 注入会话级 LoaderTool
	for _, manager := range e.mcpManagers {
		loaderTool := NewMCPLoaderTool(manager, session)
		session.sessionTools[loaderTool.Name()] = loaderTool
	}
	return session
}

// SessionToolExecutor 会话级工具执行器，动态加载的工具只在本会话可见
type SessionToolExecutor struct {
	shared       *ToolExecuter
	sessionTools map[string]Tool
}

func (s *SessionToolExecutor) Tools() []llms.Tool {
	return s.shared.toolsWithSession(s.sessionTools)
}

func (s *SessionToolExecutor) Execute(ctx context.Context, call llms.ToolCall, callbacks CallBackFuncs) (string, error) {
	return s.shared.executeWithSession(ctx, call, callbacks, s.sessionTools)
}

// RegisterSession 注册工具到会话级（不影响其他会话）
func (s *SessionToolExecutor) RegisterSession(tool Tool) {
	s.sessionTools[tool.Name()] = tool
}

// ClearDynamicMCPTools 清理会话级动态加载的 MCP 工具
func (s *SessionToolExecutor) ClearDynamicMCPTools() int {
	cleared := 0
	for name, tool := range s.sessionTools {
		if _, ok := tool.(*MCPTool); ok {
			delete(s.sessionTools, name)
			cleared++
		}
	}
	return cleared
}
