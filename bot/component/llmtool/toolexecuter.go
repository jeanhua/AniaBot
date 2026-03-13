package llmtool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/tmc/langchaingo/llms"
)

type ToolExecuter struct {
	tools map[string]Tool
}

func NewToolExecuter() *ToolExecuter {
	return &ToolExecuter{
		tools: make(map[string]Tool),
	}
}

func (e *ToolExecuter) Register(tool Tool) {
	if _, ok := e.tools[tool.Name()]; ok {
		panic("tool already registered")
	}
	e.tools[tool.Name()] = tool
}

func (e *ToolExecuter) Tools() []llms.Tool {
	tools := make([]llms.Tool, 0, len(e.tools))
	for _, tool := range e.tools {
		tools = append(tools, structToOpenAITool(tool))
	}
	return tools
}

func (e *ToolExecuter) Execute(ctx context.Context, call llms.ToolCall, callbacks CallBackFuncs) (string, error) {
	tool, ok := e.tools[call.FunctionCall.Name]
	if !ok {
		return "", fmt.Errorf("tool '%s' not found. Available tools: %v",
			call.FunctionCall.Name, e.getToolNames())
	}

	// 检查是否是 MCP 工具
	if mcpTool, ok := tool.(*MCPTool); ok {
		// MCP 工具直接使用原始 JSON 参数
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

// UnregisterTool 注销指定名称的工具
func (e *ToolExecuter) UnregisterTool(toolName string) bool {
	if _, ok := e.tools[toolName]; ok {
		delete(e.tools, toolName)
		return true
	}
	return false
}

// ClearDynamicMCPTools 清理所有动态加载的 MCP 工具（保留发现和加载工具）
func (e *ToolExecuter) ClearDynamicMCPTools() int {
	cleared := 0
	toDelete := make([]string, 0)

	for name, tool := range e.tools {
		// 检查是否是 MCP 工具
		if _, ok := tool.(*MCPTool); ok {
			// 保留发现和加载工具，删除其他动态加载的工具
			if _, isDiscovery := tool.(*MCPDiscoveryTool); !isDiscovery {
				if _, isLoader := tool.(*MCPLoaderTool); !isLoader {
					toDelete = append(toDelete, name)
				}
			}
		}
	}

	for _, name := range toDelete {
		delete(e.tools, name)
		cleared++
	}

	return cleared
}
