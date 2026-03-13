package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
)

// ChatBot 聊天机器人
type ChatBot struct {
	llmClient        *LLMClient
	msgBuilder       *MessageBuilder
	toolOrchestrator *ToolOrchestrator
	window           *messageWindow
}

func NewChatBot(baseURL, apiKey, model, prompt string, windowSize int, toolExecutor ToolExecutor) (*ChatBot, error) {
	llmClient, err := NewLLMClient(baseURL, apiKey, model)
	if err != nil {
		return nil, err
	}

	msgBuilder := NewMessageBuilder(prompt)
	toolOrchestrator := NewToolOrchestrator(toolExecutor, msgBuilder)

	return &ChatBot{
		llmClient:        llmClient,
		msgBuilder:       msgBuilder,
		toolOrchestrator: toolOrchestrator,
		window:           newMessageWindow(windowSize),
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, callbacks llmtool.CallBackFuncs, opts ...llms.CallOption) (string, error) {
	// 构建本轮消息：system prompt + 历史消息 + 本次 human 消息
	messages := b.msgBuilder.BuildChatMessages(userInput, b.window.history())

	response, updatedMessages, err := b.toolOrchestrator.ExecuteWithTools(ctx, b.llmClient, messages, callbacks, opts...)
	if err != nil {
		return "", fmt.Errorf("chat execution failed: %w", err)
	}

	// 从 updatedMessages 中提取本轮新增的消息（去掉 system prompt 和历史部分）
	// updatedMessages 结构：[system, ...history, human, (ai+tool+tool_result)..., ai_final]
	// 保存本轮完整对话：human 消息 + AI 响应 + 工具调用链
	historyLen := len(b.window.history())
	newMessagesStart := 1 + historyLen // 跳过 system prompt (1) + 历史消息 (historyLen)
	if newMessagesStart < len(updatedMessages) {
		b.window.append(updatedMessages[newMessagesStart:]...)
	}

	return response, nil
}

func (b *ChatBot) GetSingleImageDesc(ctx context.Context, userInput string, imageURL string, opts ...llms.CallOption) (string, error) {
	messages := b.msgBuilder.BuildVisionMessages(userInput, imageURL)
	return b.llmClient.GenerateSingle(ctx, messages, opts...)
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	b.window.clear()
	return nil
}

// ClearDynamicTools 清理动态加载的 MCP 工具
func (b *ChatBot) ClearDynamicTools() int {
	if b.toolOrchestrator != nil && b.toolOrchestrator.executor != nil {
		if executer, ok := b.toolOrchestrator.executor.(*llmtool.ToolExecuter); ok {
			return executer.ClearDynamicMCPTools()
		}
	}
	return 0
}

func (b *ChatBot) SetToolOrchestrator(orchestrator *ToolOrchestrator) {
	b.toolOrchestrator = orchestrator
}

func (b *ChatBot) SetSkillManager(manager *llmtool.SkillManager) {
	b.msgBuilder.WithSkillManager(manager)
}
