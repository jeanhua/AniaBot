package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"sort"
	"sync"
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
// 同名工具共享层与会话层并存时只保留一份定义，与 resolveTool 一致由会话层
// 优先——否则 tools 字段出现两份同名 function 定义（部分提供方直接 400 拒绝），
// 且「下发的定义」与「实际执行的工具」不一致。
func (e *ToolExecuter) toolsWithSession(sessionTools map[string]Tool) []ToolDef {
	seen := make(map[string]struct{}, len(e.tools)+len(sessionTools))
	names := make([]string, 0, len(e.tools)+len(sessionTools))
	for name := range sessionTools {
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for name := range e.tools {
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]ToolDef, 0, len(names))
	for _, name := range names {
		if tool, ok := sessionTools[name]; ok {
			tools = append(tools, structToOpenAITool(tool))
		} else if tool, ok := e.tools[name]; ok {
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
	shared *ToolExecuter
	mu     sync.RWMutex // 保护 sessionTools：同一轮的多个工具调用并行执行，
	// mcp_load 等工具会并发 RegisterSession（写）而其他工具的 Execute/Tools 在读，
	// 并发 map 读写是不可恢复的 fatal error（见 aichat.ToolOrchestrator 并行调度）
	sessionTools map[string]Tool
}

// snapshotSessionTools 拷贝当前会话工具表：读写均在锁内完成快照，
// 后续遍历/解析在锁外进行，避免与并发 RegisterSession 产生 map 竞争。
func (s *SessionToolExecutor) snapshotSessionTools() map[string]Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot := make(map[string]Tool, len(s.sessionTools))
	for name, tool := range s.sessionTools {
		snapshot[name] = tool
	}
	return snapshot
}

func (s *SessionToolExecutor) Tools() []ToolDef {
	return s.shared.toolsWithSession(s.snapshotSessionTools())
}

func (s *SessionToolExecutor) Execute(ctx context.Context, call ToolCall, callbacks CallBackFuncs) (string, error) {
	return s.shared.executeWithSession(ctx, call, callbacks, s.snapshotSessionTools())
}

func (s *SessionToolExecutor) RegisterSession(tool Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessionTools[tool.Name()] = tool
}

func (s *SessionToolExecutor) ClearDynamicMCPTools() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cleared := 0
	for name, tool := range s.sessionTools {
		if _, ok := tool.(*MCPTool); ok {
			delete(s.sessionTools, name)
			cleared++
		}
	}
	return cleared
}
