package component

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/functool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

type ChatBotOption func(*ChatBot)

func WithMessageStore(store MessageStore) ChatBotOption {
	return func(b *ChatBot) {
		b.messageStore = store
	}
}

func WithToolExecutor(exec ToolExecutor) ChatBotOption {
	return func(b *ChatBot) {
		b.toolExecutor = exec
	}
}

func WithMaxIterations(iterations int) ChatBotOption {
	return func(b *ChatBot) {
		b.maxIterations = iterations
	}
}

func WithConversationId(id string) ChatBotOption {
	return func(b *ChatBot) {
		b.conversationId = id
	}
}

type ChatBot struct {
	prompt         string
	llm            LLM
	memory         *memory.ConversationWindowBuffer
	messageStore   MessageStore
	toolExecutor   ToolExecutor
	conversationId string
	maxIterations  int
}

func NewChatBot(llm LLM, prompt string, windowSize int, opts ...ChatBotOption) *ChatBot {
	mem := memory.NewConversationWindowBuffer(
		windowSize,
		memory.WithReturnMessages(true),
	)

	bot := &ChatBot{
		prompt:        prompt,
		llm:           llm,
		memory:        mem,
		maxIterations: 5,
	}

	for _, opt := range opts {
		opt(bot)
	}

	return bot
}

func (b *ChatBot) buildMessages(ctx context.Context, userInput string) ([]llms.MessageContent, error) {
	variables, err := b.memory.LoadMemoryVariables(ctx, map[string]any{})
	if err != nil {
		return nil, err
	}

	var messages []llms.MessageContent
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, b.prompt))

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		for _, msg := range historyList {
			messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
		}
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))
	return messages, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, msgFunc functool.OptionFuncs, opt ...llms.CallOption) (string, error) {
	messages, err := b.buildMessages(ctx, userInput)
	if err != nil {
		return "", err
	}

	var tools []llms.Tool
	if b.toolExecutor != nil {
		tools = b.toolExecutor.GetTools()
	}

	callopt := append(opt, llms.WithTools(tools))

	for i := 0; i < b.maxIterations; i++ {
		completion, err := b.llm.GenerateContent(ctx, messages, callopt...)
		if err != nil {
			return "", err
		}

		if len(completion.Choices) == 0 {
			return "", fmt.Errorf("no choices returned from LLM")
		}

		choice := completion.Choices[0]

		if len(choice.ToolCalls) == 0 {
			respText := choice.Content
			err = b.memory.SaveContext(ctx,
				map[string]any{"prompt": userInput},
				map[string]any{"response": respText},
			)
			if err != nil {
				return "", err
			}
			if b.messageStore != nil && b.conversationId != "" {
				err = b.messageStore.SaveMessage(ctx, b.conversationId, Message{
					Role:    "human",
					Content: userInput,
					Time:    time.Now(),
				})
				if err != nil {
					return "", err
				}
				err = b.messageStore.SaveMessage(ctx, b.conversationId, Message{
					Role:    "assistant",
					Content: respText,
					Time:    time.Now(),
				})
				if err != nil {
					return "", err
				}
			}
			return respText, nil
		}

		aiMsg := llms.MessageContent{
			Role: llms.ChatMessageTypeAI,
		}
		if choice.Content != "" {
			msgFunc.SendText(choice.Content)
			aiMsg.Parts = append(aiMsg.Parts, llms.TextPart(choice.Content))
		}
		for _, call := range choice.ToolCalls {
			aiMsg.Parts = append(aiMsg.Parts, call)
		}
		messages = append(messages, aiMsg)

		for _, call := range choice.ToolCalls {
			var callResult string
			if b.toolExecutor != nil {
				callResult, err = b.toolExecutor.Execute(ctx, call, msgFunc)
			} else {
				err = errors.New("tool executor not configured")
			}
			if err != nil {
				callResult = fmt.Sprintf("Error executing tool: %v", err)
			}
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: call.ID,
						Name:       call.FunctionCall.Name,
						Content:    callResult,
					},
				},
			})
		}

		if i == b.maxIterations-1 {
			messages = append(messages, llms.TextParts(
				llms.ChatMessageTypeSystem,
				"你的Tool Call连续调用已经达到限制，请先基于当前获取结果回答用户问题，如果需要更多Tool Call，请先向用户发送请求，得到用户允许后重新刷新限额",
			))

			finalCompletion, err := b.llm.GenerateContent(ctx, messages)
			if err != nil {
				return "", err
			}

			if len(finalCompletion.Choices) == 0 {
				return "", fmt.Errorf("no choices returned from final LLM call")
			}

			respText := finalCompletion.Choices[0].Content
			err = b.memory.SaveContext(ctx,
				map[string]any{"prompt": userInput},
				map[string]any{"response": respText},
			)
			if err != nil {
				return "", err
			}
			if b.messageStore != nil && b.conversationId != "" {
				_ = b.messageStore.SaveMessage(ctx, b.conversationId, Message{
					Role:    "human",
					Content: userInput,
					Time:    time.Now(),
				})
				_ = b.messageStore.SaveMessage(ctx, b.conversationId, Message{
					Role:    "assistant",
					Content: respText,
					Time:    time.Now(),
				})
			}
			return respText, err
		}
	}

	return "", fmt.Errorf("unexpected error: exceeded maximum iterations")
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
	err := b.memory.Clear(ctx)
	if err != nil {
		return err
	}
	if b.messageStore != nil && b.conversationId != "" {
		return b.messageStore.ClearHistory(ctx, b.conversationId)
	}
	return nil
}

func (b *ChatBot) LoadHistory(ctx context.Context, limit int) error {
	if b.messageStore == nil || b.conversationId == "" {
		return nil
	}
	messages, err := b.messageStore.GetMessages(ctx, b.conversationId, limit)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		switch msg.Role {
		case "human":
			b.memory.SaveContext(ctx, map[string]any{"prompt": msg.Content}, map[string]any{})
		case "assistant":
			b.memory.SaveContext(ctx, map[string]any{}, map[string]any{"response": msg.Content})
		}
	}
	return nil
}

func (b *ChatBot) SetConversationId(id string) {
	b.conversationId = id
}

func (b *ChatBot) GetConversationId() string {
	return b.conversationId
}

type LegacyChatBot struct {
	*ChatBot
	searchToken string
}

func NewLegacyChatBot(baseURL, apiKey, model, prompt string, windowSize int, searchToken string) (*LegacyChatBot, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, err
	}

	openAILLM := NewOpenAILLM(llm)
	toolExecutor := NewToolExecutor(searchToken)

	bot := NewChatBot(openAILLM, prompt, windowSize, WithToolExecutor(toolExecutor))

	return &LegacyChatBot{
		ChatBot:     bot,
		searchToken: searchToken,
	}, nil
}

func (b *ChatBot) parseImageUrls(text string) []string {
	var urls []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			urls = append(urls, line)
		}
	}
	return urls
}
