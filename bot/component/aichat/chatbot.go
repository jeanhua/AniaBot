package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type ChatBot struct {
	llmClient        *LLMClient
	msgBuilder       *MessageBuilder
	toolOrchestrator *ToolOrchestrator
	window           *messageWindow
}

func NewChatBot(baseURL, apiKey, model, prompt string, maxContextTokens int, toolExecutor ToolExecutor, historyStore HistoryStore) (*ChatBot, error) {
	llmClient, err := NewLLMClient(baseURL, apiKey, model)
	if err != nil {
		return nil, err
	}

	msgBuilder := NewMessageBuilder(prompt)
	toolOrchestrator := NewToolOrchestrator(toolExecutor, msgBuilder)

	compressor := NewContextCompressor(prompt)
	window := newMessageWindow(maxContextTokens, llmClient, compressor, historyStore)

	return &ChatBot{
		llmClient:        llmClient,
		msgBuilder:       msgBuilder,
		toolOrchestrator: toolOrchestrator,
		window:           window,
	}, nil
}

// LoadHistory 从持久化存储回放历史对话，重启后调用以恢复上下文。
// historyStore 未注入时为空操作。
func (b *ChatBot) LoadHistory(ctx context.Context) {
	b.window.load(ctx)
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, callbacks llmtool.CallBackFuncs, opts ChatOptions) (string, TokenUsage, error) {
	// 压缩检查：在构建消息之前，确保上下文不超限
	if err := b.window.MaybeCompress(ctx); err != nil {
		return "", TokenUsage{}, err
	}

	messages := b.msgBuilder.BuildChatMessages(userInput, b.window.history())

	response, updatedMessages, usage, err := b.toolOrchestrator.ExecuteWithTools(ctx, b.llmClient, messages, callbacks, opts)
	if err != nil {
		return "", usage, fmt.Errorf("chat execution failed: %w", err)
	}

	b.window.RecordUsage(usage)

	historyLen := len(b.window.history())
	newMessagesStart := 1 + historyLen
	if newMessagesStart < len(updatedMessages) {
		b.window.append(updatedMessages[newMessagesStart:]...)
	}

	response = removeThinkContent(response)

	return response, usage, nil
}

func (b *ChatBot) GetSingleImageDesc(ctx context.Context, userInput string, imageURL string, opts ChatOptions) (string, error) {
	messages := b.msgBuilder.BuildVisionMessages(userInput, imageURL)
	return b.llmClient.GenerateSingle(ctx, messages, opts)
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	b.window.clear()
	return nil
}

func (b *ChatBot) ClearDynamicTools() int {
	if b.toolOrchestrator != nil && b.toolOrchestrator.executor != nil {
		if session, ok := b.toolOrchestrator.executor.(*llmtool.SessionToolExecutor); ok {
			return session.ClearDynamicMCPTools()
		}
	}
	return 0
}

func (b *ChatBot) SetToolOrchestrator(orchestrator *ToolOrchestrator) {
	b.toolOrchestrator = orchestrator
}

// SetToolObserver 设置工具调用观察者（每次工具执行完成后回调），传 nil 取消。
// 由调用方保证同一 ChatBot 的 Chat 调用串行（插件层按会话加锁）。
func (b *ChatBot) SetToolObserver(fn func(ToolCallInfo)) {
	b.toolOrchestrator.SetToolObserver(fn)
}

func (b *ChatBot) SetSkillManager(manager *llmtool.SkillManager) {
	b.msgBuilder.WithSkillManager(manager)
}

// SetMaxIterations 设置工具调用循环的最大轮数（子代理等场景使用比主对话更小的上限）。
func (b *ChatBot) SetMaxIterations(max int) {
	b.toolOrchestrator.SetMaxIterations(max)
}
