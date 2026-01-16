package component

import (
	"context"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	prompt string
	llm    llms.Model
	memory *memory.ConversationWindowBuffer
}

func NewChatBot(baseURL, apiKey, model, prompt string, windowSize int) (*ChatBot, error) {
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
		prompt: prompt,
		llm:    llm,
		memory: mem,
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, opt ...llms.CallOption) (string, error) {
	variables, err := b.memory.LoadMemoryVariables(ctx, map[string]any{})
	if err != nil {
		return "", err
	}

	var messages []llms.MessageContent

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, b.prompt))

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		for _, msg := range historyList {
			messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
		}
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))

	completion, err := b.llm.GenerateContent(ctx, messages, opt...)
	if err != nil {
		return "", err
	}

	respText := completion.Choices[0].Content

	err = b.memory.SaveContext(ctx,
		map[string]any{"prompt": userInput},
		map[string]any{"response": respText},
	)
	if err != nil {
		return "", err
	}

	return respText, nil
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
