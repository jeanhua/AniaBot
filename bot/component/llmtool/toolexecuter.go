package llmtool

import (
	"context"
	"encoding/json"
	"errors"
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
		return "", errors.New("tool not found")
	}

	// 检查是否是 MCP 工具
	if mcpTool, ok := tool.(*MCPTool); ok {
		// MCP 工具直接使用原始 JSON 参数
		return mcpTool.ExecuteWithArgs(ctx, []byte(call.FunctionCall.Arguments), callbacks)
	}

	params := reflect.New(reflect.TypeOf(tool.Params()).Elem()).Interface()
	if err := json.Unmarshal([]byte(call.FunctionCall.Arguments), params); err != nil {
		return "", err
	}
	return tool.Execute(ctx, params, callbacks)
}
