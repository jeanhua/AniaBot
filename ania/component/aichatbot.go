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
	memory *memory.ConversationBuffer
}

func NewChatBot(baseURL, apiKey, model, prompt string) (*ChatBot, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, err
	}

	mem := memory.NewConversationBuffer(memory.WithReturnMessages(true))

	return &ChatBot{
		prompt: prompt,
		llm:    llm,
		memory: mem,
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string) (string, error) {
	variables, err := b.memory.LoadMemoryVariables(ctx, map[string]any{})
	if err != nil {
		return "", err
	}

	var messages []llms.MessageContent

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, b.prompt))

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		const MaxHistoryLen = 50
		if len(historyList) > MaxHistoryLen {
			historyList = historyList[len(historyList)-MaxHistoryLen:]
		}

		for _, msg := range historyList {
			messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
		}
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))

	completion, err := b.llm.GenerateContent(ctx, messages)
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
