package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	llmClient         *LLMClient
	msgBuilder        *MessageBuilder
	toolOrchestrator  *ToolOrchestrator
	memory            *memory.ConversationWindowBuffer
}

func NewChatBot(baseURL, apiKey, model, prompt string, windowSize int, toolExecutor ToolExecutor) (*ChatBot, error) {
	llmClient, err := NewLLMClient(baseURL, apiKey, model)
	if err != nil {
		return nil, err
	}

	mem := memory.NewConversationWindowBuffer(
		windowSize,
		memory.WithReturnMessages(true),
	)

	msgBuilder := NewMessageBuilder(prompt, mem)
	toolOrchestrator := NewToolOrchestrator(toolExecutor, msgBuilder)

	return &ChatBot{
		llmClient:        llmClient,
		msgBuilder:       msgBuilder,
		toolOrchestrator: toolOrchestrator,
		memory:           mem,
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, callbacks llmtool.CallBackFuncs, opts ...llms.CallOption) (string, error) {
	history, err := b.loadHistory()
	if err != nil {
		return "", fmt.Errorf("failed to load history: %w", err)
	}

	messages := b.msgBuilder.BuildChatMessages(userInput, history)

	response, _, err := b.toolOrchestrator.ExecuteWithTools(ctx, b.llmClient, messages, callbacks, opts...)
	if err != nil {
		return "", fmt.Errorf("chat execution failed: %w", err)
	}

	if err := b.saveContext(userInput, response); err != nil {
		return "", fmt.Errorf("failed to save context: %w", err)
	}

	return response, nil
}

func (b *ChatBot) GetSingleImageDesc(ctx context.Context, userInput string, imageURL string, opts ...llms.CallOption) (string, error) {
	messages := b.msgBuilder.BuildVisionMessages(userInput, imageURL)
	return b.llmClient.GenerateSingle(ctx, messages, opts...)
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	return b.memory.Clear(ctx)
}

func (b *ChatBot) SetToolOrchestrator(orchestrator *ToolOrchestrator) {
	b.toolOrchestrator = orchestrator
}

func (b *ChatBot) loadHistory() ([]llms.ChatMessage, error) {
	variables, err := b.memory.LoadMemoryVariables(context.Background(), map[string]any{})
	if err != nil {
		return nil, err
	}

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		return historyList, nil
	}

	return nil, nil
}

func (b *ChatBot) saveContext(userInput, response string) error {
	return b.memory.SaveContext(
		context.Background(),
		map[string]any{"prompt": userInput},
		map[string]any{"response": response},
	)
}
