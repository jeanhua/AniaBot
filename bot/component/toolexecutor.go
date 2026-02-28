package component

import (
	"context"
	"errors"

	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/tmc/langchaingo/llms"
)

type ToolExecutor interface {
	Execute(ctx context.Context, call llms.ToolCall, msgFunc functool.OptionFuncs) (string, error)
	GetTools() []llms.Tool
}

type DefaultToolExecutor struct {
	searchToken string
	tools       []llms.Tool
}

func NewToolExecutor(searchToken string) *DefaultToolExecutor {
	exec := &DefaultToolExecutor{
		searchToken: searchToken,
	}
	exec.initTools()
	return exec
}

func (e *DefaultToolExecutor) initTools() {
	e.tools = append(e.tools, functool.MakeJinaTool()...)
	e.tools = append(e.tools, functool.MakeTimeTool()...)
	e.tools = append(e.tools, functool.MakeMemeTool()...)
	e.tools = append(e.tools, functool.MakeFileTool())
}

func (e *DefaultToolExecutor) GetTools() []llms.Tool {
	return e.tools
}

func (e *DefaultToolExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc functool.OptionFuncs) (string, error) {
	switch call.FunctionCall.Name {
	case functool.JINA_TOOL_SEARCH_NAME, functool.JINA_TOOL_EXPLORE_NAME:
		return functool.TryHanleJina(ctx, e.searchToken, call)
	case functool.TIME_TOOL_NAME:
		return functool.TryHandleTimeCall(call)
	case functool.MEME_TOOL_NAME:
		return functool.TryHandleMemeFunc(call, msgFunc)
	case functool.FILE_TOOL_NAME:
		return functool.TryHandleFileTool(call, msgFunc)
	default:
		return "", errors.New("tool not exist")
	}
}
