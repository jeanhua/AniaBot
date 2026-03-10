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
		tools = append(tools, structToOpenAITool(tool.Name(), tool.Description(), tool.Params()))
	}
	return tools
}

func (e *ToolExecuter) Execute(ctx context.Context, call llms.ToolCall, callbacks CallBackFuncs) (string, error) {
	tool, ok := e.tools[call.FunctionCall.Name]
	if !ok {
		return "", errors.New("tool not found")
	}
	params := reflect.New(reflect.TypeOf(tool.Params()).Elem()).Interface()
	if err := json.Unmarshal([]byte(call.FunctionCall.Arguments), params); err != nil {
		return "", err
	}
	return tool.Execute(ctx, params, callbacks)
}
