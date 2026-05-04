package aichat

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

type CompressorFunc func(ctx context.Context, client *LLMClient, oldMsgs []Message) ([]Message, error)

type messageWindow struct {
	messages         []Message
	maxContextTokens int
	llmClient        *LLMClient
	compressor       CompressorFunc
	lastPromptTokens int
}

func newMessageWindow(maxContextTokens int, llmClient *LLMClient, compressor CompressorFunc) *messageWindow {
	return &messageWindow{
		maxContextTokens: maxContextTokens,
		llmClient:        llmClient,
		compressor:       compressor,
	}
}

func (w *messageWindow) append(msgs ...Message) {
	w.messages = append(w.messages, msgs...)
}

func (w *messageWindow) history() []Message {
	return w.messages
}

func (w *messageWindow) clear() {
	w.messages = nil
	w.lastPromptTokens = 0
}

func (w *messageWindow) RecordUsage(usage TokenUsage) {
	if usage.PromptTokens > 0 {
		w.lastPromptTokens = usage.PromptTokens
	}
}

func (w *messageWindow) needsCompression() bool {
	if w.maxContextTokens <= 0 || w.lastPromptTokens <= 0 {
		return false
	}
	return w.lastPromptTokens > int(float64(w.maxContextTokens)*0.8)
}

func (w *messageWindow) MaybeCompress(ctx context.Context) error {
	if !w.needsCompression() || w.compressor == nil || w.llmClient == nil {
		return nil
	}

	compressed, err := w.compressor(ctx, w.llmClient, w.messages)
	if err != nil {
		return fmt.Errorf("上下文压缩失败: %w", err)
	}

	w.messages = compressed
	w.lastPromptTokens = 0
	return nil
}

// EstimateTokens 基于字符数估算 token 数（中文约 1.5 token/字，英文约 0.25 token/字）
func EstimateTokens(msgs []Message) int {
	chineseCount := 0
	nonChineseCount := 0
	for _, m := range msgs {
		for _, p := range m.Parts {
			c, nc := countRunes(p.Text)
			chineseCount += c
			nonChineseCount += nc
		}
		for _, tc := range m.ToolCalls {
			c, nc := countRunes(tc.Name + tc.Arguments)
			chineseCount += c
			nonChineseCount += nc
		}
	}
	return int(float64(chineseCount)*1.5 + float64(nonChineseCount)*0.25)
}

// ExtractMessageText 提取消息中的纯文本内容
func ExtractMessageText(msg Message) string {
	var parts []string
	for _, p := range msg.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// FormatMessagesForSummary 将消息格式化为摘要用的文本
func FormatMessagesForSummary(msgs []Message) string {
	var buf strings.Builder
	for _, m := range msgs {
		if m.Role == RoleTool {
			continue
		}
		if len(m.ToolCalls) > 0 {
			continue
		}
		text := ExtractMessageText(m)
		if text == "" {
			continue
		}
		roleName := string(m.Role)
		switch m.Role {
		case RoleUser:
			roleName = "用户"
		case RoleAssistant:
			roleName = "助手"
		case RoleSystem:
			roleName = "系统"
		}
		buf.WriteString(fmt.Sprintf("[%s]: %s\n", roleName, text))
	}
	return buf.String()
}

// NewContextCompressor 创建上下文压缩函数
func NewContextCompressor(basePrompt string) CompressorFunc {
	return func(ctx context.Context, client *LLMClient, oldMsgs []Message) ([]Message, error) {
		text := FormatMessagesForSummary(oldMsgs)
		if text == "" {
			return []Message{TextMessage(RoleSystem, basePrompt)}, nil
		}

		compressPrompt := "你是一个对话摘要助手。请对以下历史对话进行简洁的摘要，保留关键信息、用户意图、讨论结论和重要上下文。工具调用细节和中间推理过程可以省略。"

		tempBuilder := NewMessageBuilder(compressPrompt)
		summaryMessages := tempBuilder.BuildChatMessages(text, nil)

		summary, err := client.GenerateSingle(ctx, summaryMessages, ChatOptions{})
		if err != nil {
			return nil, err
		}

		summary = removeThinkContent(summary)

		combinedPrompt := basePrompt + "\n\n[对话摘要]\n" + summary
		return []Message{TextMessage(RoleSystem, combinedPrompt)}, nil
	}
}

// countRunes 统计文本中的 rune 数量（中文字符计数用）
func countRunes(text string) (chinese int, nonChinese int) {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			chinese++
		} else {
			nonChinese++
		}
	}
	return
}
