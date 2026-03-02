package component

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	prompt      string
	llm         llms.Model
	memory      *memory.ConversationWindowBuffer
	searchToken string
	tools       []llms.Tool
	registry    *functool.ToolRegistry
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

	return &ChatBot{
		prompt:      prompt,
		llm:         llm,
		memory:      mem,
		searchToken: searchToken,
		tools:       functool.GetDefaultTools(),
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, msgFunc functool.OptionFuncs, opt ...llms.CallOption) (string, error) {
	messages, err := b.buildMessages(ctx, userInput)
	if err != nil {
		return "", err
	}

	callopt := append(opt, llms.WithTools(b.tools))

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

func (b *ChatBot) executeWithTools(ctx context.Context, messages []llms.MessageContent, callopt []llms.CallOption, msgFunc functool.OptionFuncs) (string, error) {
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

		messages = append(messages, functool.BuildAIMessage(choice.ToolCalls, choice.Content, msgFunc))

		toolResults, err := b.executeToolCalls(ctx, choice.ToolCalls, msgFunc)
		if err != nil {
			return "", err
		}
		messages = append(messages, toolResults...)

		if i == maxIterations-1 {
			messages = append(messages, functool.BuildToolLimitMessage()...)
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

	return "", functool.BuildToolLimitError()
}

func (b *ChatBot) executeToolCalls(ctx context.Context, toolCalls []llms.ToolCall, msgFunc functool.OptionFuncs) ([]llms.MessageContent, error) {
	var results []llms.MessageContent
	for _, call := range toolCalls {
		result, err := b.executeSingleTool(ctx, call, msgFunc)
		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
		}
		results = append(results, functool.BuildToolMessage(call, result))
	}
	return results, nil
}

func (b *ChatBot) executeSingleTool(ctx context.Context, call llms.ToolCall, msgFunc functool.OptionFuncs) (string, error) {
	if b.registry != nil {
		return b.registry.Execute(ctx, call, msgFunc)
	}
	return functool.ToolExecutorAdapter(b.searchToken, msgFunc).Execute(ctx, call, msgFunc)
}

func (b *ChatBot) ChatWithImage(ctx context.Context, userInput string, imageUrl string, opt ...llms.CallOption) (string, error) {
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

func (b *ChatBot) SetToolRegistry(registry *functool.ToolRegistry) {
	b.registry = registry
}

func (b *ChatBot) SetTools(tools []llms.Tool) {
	b.tools = tools
}
