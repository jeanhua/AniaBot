package functool

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/llms"
)

type ToolInitializer func(registry *ToolRegistry, searchToken string, msgFunc OptionFuncs)

var initializers []ToolInitializer

func RegisterToolInitializer(initFunc ToolInitializer) {
	initializers = append(initializers, initFunc)
}

func InitToolRegistry(searchToken string, msgFunc OptionFuncs) *ToolRegistry {
	registry := NewToolRegistry()

	for _, initFunc := range initializers {
		initFunc(registry, searchToken, msgFunc)
	}

	return registry
}

func GetDefaultTools() []llms.Tool {
	tools := []llms.Tool{}
	tools = append(tools, MakeJinaTool()...)
	tools = append(tools, MakeTimeTool()...)
	tools = append(tools, MakeMemeTool()...)
	tools = append(tools, MakeFileTool())
	return tools
}

func init() {
	RegisterToolInitializer(func(registry *ToolRegistry, searchToken string, msgFunc OptionFuncs) {
		jinaTools := MakeJinaTool()
		registry.RegisterTools(jinaTools, &jinaExecutor{searchToken: searchToken})
	})

	RegisterToolInitializer(func(registry *ToolRegistry, searchToken string, msgFunc OptionFuncs) {
		timeTools := MakeTimeTool()
		registry.RegisterTools(timeTools, &timeExecutor{})
	})

	RegisterToolInitializer(func(registry *ToolRegistry, searchToken string, msgFunc OptionFuncs) {
		memeTools := MakeMemeTool()
		registry.RegisterTools(memeTools, &memeExecutor{msgFunc: msgFunc})
	})

	RegisterToolInitializer(func(registry *ToolRegistry, searchToken string, msgFunc OptionFuncs) {
		fileTools := []llms.Tool{MakeFileTool()}
		registry.RegisterTools(fileTools, &fileExecutor{msgFunc: msgFunc})
	})
}

type jinaExecutor struct {
	searchToken string
}

func (e *jinaExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	return TryHanleJina(ctx, e.searchToken, call)
}

type timeExecutor struct{}

func (e *timeExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	return TryHandleTimeCall(call)
}

type memeExecutor struct {
	msgFunc OptionFuncs
}

func (e *memeExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	return TryHandleMemeFunc(call, msgFunc)
}

type fileExecutor struct {
	msgFunc OptionFuncs
}

func (e *fileExecutor) Execute(ctx context.Context, call llms.ToolCall, msgFunc OptionFuncs) (string, error) {
	return TryHandleFileTool(call, msgFunc)
}

func ToolExecutorAdapter(searchToken string, msgFunc OptionFuncs) ToolExecutor {
	de := NewDefaultExecutor()
	de.Register(JINA_TOOL_SEARCH_NAME, func(call llms.ToolCall, _ OptionFuncs) (string, error) {
		return TryHanleJina(context.Background(), searchToken, call)
	})
	de.Register(JINA_TOOL_EXPLORE_NAME, func(call llms.ToolCall, _ OptionFuncs) (string, error) {
		return TryHanleJina(context.Background(), searchToken, call)
	})
	de.Register(TIME_TOOL_NAME, func(call llms.ToolCall, _ OptionFuncs) (string, error) {
		return TryHandleTimeCall(call)
	})
	de.Register(MEME_TOOL_NAME, func(call llms.ToolCall, mFunc OptionFuncs) (string, error) {
		return TryHandleMemeFunc(call, mFunc)
	})
	de.Register(FILE_TOOL_NAME, func(call llms.ToolCall, mFunc OptionFuncs) (string, error) {
		return TryHandleFileTool(call, mFunc)
	})
	return de
}

func BuildToolMessage(call llms.ToolCall, result string) llms.MessageContent {
	return llms.MessageContent{
		Role: llms.ChatMessageTypeTool,
		Parts: []llms.ContentPart{
			llms.ToolCallResponse{
				ToolCallID: call.ID,
				Name:       call.FunctionCall.Name,
				Content:    result,
			},
		},
	}
}

func BuildAIMessage(toolCalls []llms.ToolCall, content string, msgFunc OptionFuncs) llms.MessageContent {
	aiMsg := llms.MessageContent{
		Role: llms.ChatMessageTypeAI,
	}
	if content != "" {
		msgFunc.SendText(content)
		aiMsg.Parts = append(aiMsg.Parts, llms.TextPart(content))
	}
	for _, call := range toolCalls {
		aiMsg.Parts = append(aiMsg.Parts, call)
	}
	return aiMsg
}

func BuildToolLimitMessage() []llms.MessageContent {
	return []llms.MessageContent{
		llms.TextParts(
			llms.ChatMessageTypeSystem,
			"你的Tool Call连续调用已经达到限制，请先基于当前获取结果回答用户问题，如果需要更多Tool Call，请先向用户发送请求，得到用户允许后重新刷新限额",
		),
	}
}

func BuildToolLimitError() error {
	return fmt.Errorf("unexpected error: exceeded maximum iterations")
}
