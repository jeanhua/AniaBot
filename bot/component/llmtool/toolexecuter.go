package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
)

type ToolExecuter struct {
	tools       map[string]Tool
	mcpManagers []*MCPToolManager
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

func (e *ToolExecuter) Tools() []ToolDef {
	return e.toolsWithSession(nil)
}

func (e *ToolExecuter) toolsWithSession(sessionTools map[string]Tool) []ToolDef {
	tools := make([]ToolDef, 0, len(e.tools)+len(sessionTools))
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

func (e *ToolExecuter) Execute(ctx context.Context, call ToolCall, callbacks CallBackFuncs) (string, error) {
	return e.executeWithSession(ctx, call, callbacks, nil)
}

func (e *ToolExecuter) executeWithSession(ctx context.Context, call ToolCall, callbacks CallBackFuncs, sessionTools map[string]Tool) (string, error) {
	log.Printf("[ToolExecuter] 尝试执行工具: name=%s, available=%v", call.Name, e.getToolNames())
	tool, ok := e.resolveTool(call.Name, sessionTools)
	if !ok {
		return "", fmt.Errorf("tool '%s' not found. Available tools: %v",
			call.Name, e.getToolNames())
	}

	if mcpTool, ok := tool.(*MCPTool); ok {
		result, err := mcpTool.ExecuteWithArgs(ctx, []byte(call.Arguments), callbacks)
		if err != nil {
			return "", fmt.Errorf("MCP tool '%s' execution failed: %w\nArguments: %s",
				call.Name, err, call.Arguments)
		}
		return result, nil
	}

	params := reflect.New(reflect.TypeOf(tool.Params()).Elem()).Interface()
	if err := json.Unmarshal([]byte(call.Arguments), params); err != nil {
		return "", fmt.Errorf("failed to parse arguments for tool '%s': %w\nArguments: %s\nExpected schema: %+v",
			call.Name, err, call.Arguments, tool.Params())
	}

	result, err := tool.Execute(ctx, params, callbacks)
	if err != nil {
		return "", fmt.Errorf("tool '%s' execution failed: %w", call.Name, err)
	}
	return result, nil
}

func (e *ToolExecuter) getToolNames() []string {
	names := make([]string, 0, len(e.tools))
	for name := range e.tools {
		names = append(names, name)
	}
	return names
}

func (e *ToolExecuter) NewSessionExecutor() *SessionToolExecutor {
	session := &SessionToolExecutor{
		shared:       e,
		sessionTools: make(map[string]Tool),
	}
	for _, manager := range e.mcpManagers {
		loaderTool := NewMCPLoaderTool(manager, session)
		session.sessionTools[loaderTool.Name()] = loaderTool
	}
	return session
}

type SessionToolExecutor struct {
	shared       *ToolExecuter
	sessionTools map[string]Tool
}

func (s *SessionToolExecutor) Tools() []ToolDef {
	return s.shared.toolsWithSession(s.sessionTools)
}

func (s *SessionToolExecutor) Execute(ctx context.Context, call ToolCall, callbacks CallBackFuncs) (string, error) {
	return s.shared.executeWithSession(ctx, call, callbacks, s.sessionTools)
}

func (s *SessionToolExecutor) RegisterSession(tool Tool) {
	s.sessionTools[tool.Name()] = tool
}

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
