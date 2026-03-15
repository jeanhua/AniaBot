package aichat

import (
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
)

type MessageBuilder struct {
	prompt       string
	skillManager *llmtool.SkillManager // 可选，注入后会在 system prompt 中附加 <available_skills>
}

func NewMessageBuilder(prompt string) *MessageBuilder {
	return &MessageBuilder{prompt: prompt}
}

// NewMessageBuilderWithSkill 创建带 skill 支持的 MessageBuilder
func NewMessageBuilderWithSkill(prompt string, manager *llmtool.SkillManager) *MessageBuilder {
	return &MessageBuilder{
		prompt:       prompt,
		skillManager: manager,
	}
}

// WithSkillManager 为已有的 MessageBuilder 注入 SkillManager
func (b *MessageBuilder) WithSkillManager(manager *llmtool.SkillManager) {
	b.skillManager = manager
}

// buildSystemPrompt 构建最终的 system prompt（基础 prompt + skill 列表块）
func (b *MessageBuilder) buildSystemPrompt() string {
	if b.skillManager == nil {
		return b.prompt
	}
	skillBlock := b.skillManager.BuildAvailableSkillsPrompt()
	if skillBlock == "" {
		return b.prompt
	}
	return b.prompt + "\n\n" + skillBlock
}

// BuildChatMessages 构建本轮请求的完整消息列表。
// history 是 messageWindow 中保存的历史消息（已包含工具调用链，不含 system prompt）
func (b *MessageBuilder) BuildChatMessages(userInput string, history []llms.MessageContent) []llms.MessageContent {
	messages := make([]llms.MessageContent, 0, 1+len(history)+1)
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, b.buildSystemPrompt()))
	messages = append(messages, history...)
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))
	return messages
}

func (b *MessageBuilder) BuildVisionMessages(userInput, imageURL string) []llms.MessageContent {
	parts := []llms.ContentPart{
		llms.TextPart(userInput),
		llms.ImageURLPart(imageURL),
	}

	return []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextPart(b.buildSystemPrompt())},
		},
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: parts,
		},
	}
}

func (b *MessageBuilder) BuildToolMessage(toolCallID, name, result string) llms.MessageContent {
	return llms.MessageContent{
		Role: llms.ChatMessageTypeTool,
		Parts: []llms.ContentPart{
			llms.ToolCallResponse{
				ToolCallID: toolCallID,
				Name:       name,
				Content:    result,
			},
		},
	}
}

func (b *MessageBuilder) BuildAIMessage(content string, toolCalls []llms.ToolCall) llms.MessageContent {
	msg := llms.MessageContent{
		Role: llms.ChatMessageTypeAI,
	}

	if content != "" {
		msg.Parts = append(msg.Parts, llms.TextPart(content))
	}

	for _, call := range toolCalls {
		msg.Parts = append(msg.Parts, call)
	}

	return msg
}

func (b *MessageBuilder) BuildToolLimitMessage() llms.MessageContent {
	return llms.TextParts(
		llms.ChatMessageTypeSystem,
		"你的Tool Call连续调用已经达到限制，请先基于当前获取结果回答用户问题，如果需要更多Tool Call，请先向用户发送请求，得到用户允许后重新刷新限额",
	)
}
