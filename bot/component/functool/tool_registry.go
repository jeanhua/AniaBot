package functool

import (
	"context"
	"errors"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

var (
	ErrToolNotFound = errors.New("tool not found")
	ErrToolExecute  = errors.New("tool execution error")
)

type ToolExecutor interface {
	Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error)
}

type ToolRegistry struct {
	tools     []llms.Tool
	executors map[string]ToolExecutor
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		executors: make(map[string]ToolExecutor),
	}
}

func (r *ToolRegistry) RegisterTool(tool llms.Tool, executor ToolExecutor) {
	r.tools = append(r.tools, tool)
	if executor != nil {
		r.executors[tool.Function.Name] = executor
	}
}

func (r *ToolRegistry) RegisterTools(tools []llms.Tool, executor ToolExecutor) {
	for _, tool := range tools {
		r.RegisterTool(tool, executor)
	}
}

func (r *ToolRegistry) GetTools() []llms.Tool {
	return r.tools
}

func (r *ToolRegistry) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	executor, ok := r.executors[call.FunctionCall.Name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrToolNotFound, call.FunctionCall.Name)
	}
	return executor.Execute(ctx, call, msgFunc)
}

type defaultExecutor struct {
	handlers map[string]func(call llms.ToolCall, msgFunc OptionFuncs) (string, error)
}

func NewDefaultExecutor() *defaultExecutor {
	return &defaultExecutor{
		handlers: make(map[string]func(call llms.ToolCall, msgFunc OptionFuncs) (string, error)),
	}
}

func (e *defaultExecutor) Register(name string, handler func(call llms.ToolCall, msgFunc OptionFuncs) (string, error)) {
	e.handlers[name] = handler
}

func (e *defaultExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	handler, ok := e.handlers[call.FunctionCall.Name]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrToolNotFound, call.FunctionCall.Name)
	}
	return handler(call, msgFunc)
}
