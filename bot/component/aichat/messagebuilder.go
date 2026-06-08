package aichat

import (
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type MessageBuilder struct {
	prompt       string
	skillManager *llmtool.SkillManager
}

func NewMessageBuilder(prompt string) *MessageBuilder {
	return &MessageBuilder{prompt: prompt}
}

func NewMessageBuilderWithSkill(prompt string, manager *llmtool.SkillManager) *MessageBuilder {
	return &MessageBuilder{
		prompt:       prompt,
		skillManager: manager,
	}
}

func (b *MessageBuilder) WithSkillManager(manager *llmtool.SkillManager) {
	b.skillManager = manager
}

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

func (b *MessageBuilder) BuildChatMessages(userInput string, history []Message) []Message {
	messages := make([]Message, 0, 1+len(history)+1)
	messages = append(messages, TextMessage(RoleSystem, b.buildSystemPrompt()))
	messages = append(messages, history...)
	messages = append(messages, TextMessage(RoleUser, userInput))
	return messages
}

func (b *MessageBuilder) BuildVisionMessages(userInput, imageURL string) []Message {
	parts := []ContentPart{
		TextPart(userInput),
		ImageURLPart(imageURL),
	}

	return []Message{
		{
			Role:  RoleSystem,
			Parts: []ContentPart{TextPart(b.buildSystemPrompt())},
		},
		{
			Role:  RoleUser,
			Parts: parts,
		},
	}
}

func (b *MessageBuilder) BuildToolMessage(toolCallID, name, result string) Message {
	return Message{
		Role:       RoleTool,
		ToolCallID: toolCallID,
		Parts:      []ContentPart{TextPart(result)},
	}
}

func (b *MessageBuilder) BuildAIMessage(content string, toolCalls []llmtool.ToolCall) Message {
	msg := Message{
		Role: RoleAssistant,
	}

	if content != "" {
		msg.Parts = append(msg.Parts, TextPart(content))
	}

	msg.ToolCalls = toolCalls

	return msg
}

func (b *MessageBuilder) BuildAIMessageWithReasoning(content string, toolCalls []llmtool.ToolCall, reasoningContent string) Message {
	msg := b.BuildAIMessage(content, toolCalls)
	msg.ReasoningContent = reasoningContent
	return msg
}

func (b *MessageBuilder) BuildToolLimitMessage() Message {
	return TextMessage(
		RoleUser,
		"<system>你的Tool Call连续调用已经达到限制，请先基于当前获取结果回答用户问题，如果需要更多Tool Call，请先向用户发送请求，得到用户允许后重新刷新限额</system>",
	)
}
