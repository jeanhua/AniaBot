package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	llmClient        *LLMClient
	msgBuilder       *MessageBuilder
	toolOrchestrator *ToolOrchestrator
	memory           *memory.ConversationWindowBuffer
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

	// ExecuteWithTools 返回完整的消息历史，包括工具调用和结果
	response, updatedMessages, err := b.toolOrchestrator.ExecuteWithTools(ctx, b.llmClient, messages, callbacks, opts...)
	if err != nil {
		return "", fmt.Errorf("chat execution failed: %w", err)
	}

	// 保存完整的对话上下文，包括工具调用
	if err := b.saveContext(userInput, response, updatedMessages); err != nil {
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

// saveContext 保存完整的对话上下文，包括工具调用和结果
func (b *ChatBot) saveContext(userInput, response string, messages []llms.MessageContent) error {
	// 检查是否有工具调用，并提取工具调用信息
	var toolCallsSummary string
	foundUserInput := false

	for _, msg := range messages {
		// 找到当前用户输入后开始记录
		if msg.Role == llms.ChatMessageTypeHuman && !foundUserInput {
			for _, part := range msg.Parts {
				if textPart, ok := part.(llms.TextContent); ok {
					if textPart.Text == userInput {
						foundUserInput = true
						break
					}
				}
			}
			continue
		}

		if !foundUserInput {
			continue
		}

		// 提取工具调用信息
		if msg.Role == llms.ChatMessageTypeAI {
			for _, part := range msg.Parts {
				if toolCall, ok := part.(llms.ToolCall); ok {
					toolCallsSummary += fmt.Sprintf("\n[Called tool: %s with args: %s]",
						toolCall.FunctionCall.Name,
						toolCall.FunctionCall.Arguments)
				}
			}
		}

		// 提取工具结果
		if msg.Role == llms.ChatMessageTypeTool {
			for _, part := range msg.Parts {
				if toolResp, ok := part.(llms.ToolCallResponse); ok {
					// 截断过长的结果
					result := []rune(toolResp.Content)
					if len(result) > 8000 {
						result = append(result[:8000], []rune("... (truncated)")...)
					}
					toolCallsSummary += fmt.Sprintf("\n[Tool %s returned: %s]",
						toolResp.Name,
						string(result))
				}
			}
		}
	}

	// 如果没有工具调用，使用简单的保存方式
	if toolCallsSummary == "" {
		return b.memory.SaveContext(
			context.Background(),
			map[string]any{"prompt": userInput},
			map[string]any{"response": response},
		)
	}

	// 将工具调用信息附加到响应中保存
	// 这样模型在下次对话时能看到之前的工具调用历史
	enrichedResponse := response + toolCallsSummary

	return b.memory.SaveContext(
		context.Background(),
		map[string]any{"prompt": userInput},
		map[string]any{"response": enrichedResponse},
	)
}
