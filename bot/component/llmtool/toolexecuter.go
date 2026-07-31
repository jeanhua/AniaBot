package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
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

// Register 注册工具；同名工具已存在时跳过并记录日志。
// 不能 panic：重名可能来自用户配置（如 files.mcp_json 中重复的服务器名），
// panic 会越过注册循环的容错逻辑，中断插件的整个初始化流程。
func (e *ToolExecuter) Register(tool Tool) {
	if _, ok := e.tools[tool.Name()]; ok {
		log.Printf("[ToolExecuter] 工具 '%s' 已注册，跳过重复注册", tool.Name())
		return
	}
	e.tools[tool.Name()] = tool
}

func (e *ToolExecuter) Tools() []ToolDef {
	return e.toolsWithSession(nil)
}

// toolsWithSession 合并共享工具与会话工具的定义列表。
// 输出按工具名排序：Go map 遍历顺序随机，若直接序列化会导致每次请求的
// tools 字段排列不同，把上游 prompt 前缀缓存（如 DeepSeek context caching）
// 全部打失，必须保证完全确定的输出顺序。
func (e *ToolExecuter) toolsWithSession(sessionTools map[string]Tool) []ToolDef {
	names := make([]string, 0, len(e.tools)+len(sessionTools))
	for name := range e.tools {
		names = append(names, name)
	}
	for name := range sessionTools {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]ToolDef, 0, len(names))
	for _, name := range names {
		if tool, ok := e.tools[name]; ok {
			tools = append(tools, structToOpenAITool(tool))
		} else if tool, ok := sessionTools[name]; ok {
			tools = append(tools, structToOpenAITool(tool))
		}
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
	// 排序保证输出确定：该列表会作为"工具未找到"错误文本回填给 LLM
	sort.Strings(names)
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
