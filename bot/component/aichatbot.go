package component

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	prompt       string
	llm          llms.Model
	memory       *memory.ConversationWindowBuffer
	searchToken  string
	toolExecuter *llmtool.ToolExecuter
}

func NewChatBot(baseURL, apiKey, model, prompt string, windowSize int, searchToken string) (*ChatBot, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, err
	}

	mem := memory.NewConversationWindowBuffer(
		windowSize,
		memory.WithReturnMessages(true),
	)

	toolExecuter := functool.CreateDefaultTools(searchToken)

	return &ChatBot{
		prompt:       prompt,
		llm:          llm,
		memory:       mem,
		searchToken:  searchToken,
		toolExecuter: toolExecuter,
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, msgFunc llmtool.CallBackFuncs, opt ...llms.CallOption) (string, error) {
	messages, err := b.buildMessages(ctx, userInput)
	if err != nil {
		return "", err
	}

	callopt := append(opt, llms.WithTools(b.toolExecuter.Tools()))

	respText, err := b.executeWithTools(ctx, messages, callopt, msgFunc)
	if err != nil {
		return "", err
	}

	err = b.memory.SaveContext(ctx,
		map[string]any{"prompt": userInput},
		map[string]any{"response": respText},
	)
	return respText, err
}

func (b *ChatBot) buildMessages(ctx context.Context, userInput string) ([]llms.MessageContent, error) {
	variables, err := b.memory.LoadMemoryVariables(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}

	messages := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, b.prompt),
	}

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		for _, msg := range historyList {
			messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
		}
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))
	return messages, nil
}

func (b *ChatBot) executeWithTools(ctx context.Context, messages []llms.MessageContent, callopt []llms.CallOption, msgFunc llmtool.CallBackFuncs) (string, error) {
	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		completion, err := b.llm.GenerateContent(ctx, messages, callopt...)
		if err != nil {
			return "", err
		}

		if len(completion.Choices) == 0 {
			return "", fmt.Errorf("no choices returned from LLM")
		}

		choice := completion.Choices[0]

		if len(choice.ToolCalls) == 0 {
			return choice.Content, nil
		}

		messages = append(messages, b.buildAIMessage(choice.ToolCalls, choice.Content, msgFunc))

		toolResults, err := b.executeToolCalls(ctx, choice.ToolCalls, msgFunc)
		if err != nil {
			return "", err
		}
		messages = append(messages, toolResults...)

		if i == maxIterations-1 {
			messages = append(messages, b.buildToolLimitMessage())
			finalCompletion, err := b.llm.GenerateContent(ctx, messages)
			if err != nil {
				return "", err
			}
			if len(finalCompletion.Choices) == 0 {
				return "", fmt.Errorf("no choices returned from final LLM call")
			}
			return finalCompletion.Choices[0].Content, nil
		}
	}

	return "", fmt.Errorf("unexpected error: exceeded maximum iterations")
}

func (b *ChatBot) executeToolCalls(ctx context.Context, toolCalls []llms.ToolCall, msgFunc llmtool.CallBackFuncs) ([]llms.MessageContent, error) {
	var results []llms.MessageContent
	for _, call := range toolCalls {
		result, err := b.toolExecuter.Execute(ctx, call, msgFunc)
		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
		}
		results = append(results, b.buildToolMessage(call, result))
	}
	return results, nil
}

func (b *ChatBot) GetSingleImageDesc(ctx context.Context, userInput string, imageUrl string, opt ...llms.CallOption) (string, error) {
	parts := []llms.ContentPart{
		llms.TextPart(userInput),
		llms.ImageURLPart(imageUrl),
	}
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextPart(b.prompt)},
		},
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: parts,
		},
	}

	completion, err := b.llm.GenerateContent(ctx, messages, opt...)
	if err != nil {
		return "", err
	}
	return completion.Choices[0].Content, nil
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	return b.memory.Clear(ctx)
}

func (b *ChatBot) SetToolExecuter(executer *llmtool.ToolExecuter) {
	b.toolExecuter = executer
}

// buildToolMessage 构建工具响应消息
func (b *ChatBot) buildToolMessage(call llms.ToolCall, result string) llms.MessageContent {
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

// buildAIMessage 构建AI消息（包含工具调用）
func (b *ChatBot) buildAIMessage(toolCalls []llms.ToolCall, content string, msgFunc llmtool.CallBackFuncs) llms.MessageContent {
	aiMsg := llms.MessageContent{
		Role: llms.ChatMessageTypeAI,
	}
	if content != "" {
		if msgFunc.SendText != nil {
			msgFunc.SendText(content)
		}
		aiMsg.Parts = append(aiMsg.Parts, llms.TextPart(content))
	}
	for _, call := range toolCalls {
		aiMsg.Parts = append(aiMsg.Parts, call)
	}
	return aiMsg
}

// buildToolLimitMessage 构建工具调用限制消息
func (b *ChatBot) buildToolLimitMessage() llms.MessageContent {
	return llms.TextParts(
		llms.ChatMessageTypeSystem,
		"你的Tool Call连续调用已经达到限制，请先基于当前获取结果回答用户问题，如果需要更多Tool Call，请先向用户发送请求，得到用户允许后重新刷新限额",
	)
}
