package aichat

import "github.com/tmc/langchaingo/llms"

// messageWindow 维护完整的对话消息历史（含工具调用链），支持窗口裁剪。
// langchain 的 ConversationWindowBuffer 只能存 Human/AI 纯文本消息，
// 无法保存 ToolCall / ToolResult 消息，因此改用自维护的消息列表。
type messageWindow struct {
	messages   []llms.MessageContent
	windowSize int // 保留最近 N 轮完整对话（每轮 = 1 条 Human + 若干 AI/Tool + 1 条最终 AI）
}

func newMessageWindow(windowSize int) *messageWindow {
	return &messageWindow{windowSize: windowSize}
}

// append 追加若干条消息
func (w *messageWindow) append(msgs ...llms.MessageContent) {
	w.messages = append(w.messages, msgs...)
	w.trim()
}

// trim 按 Human 消息数量裁剪，保留最近 windowSize 轮
func (w *messageWindow) trim() {
	if w.windowSize <= 0 {
		return
	}

	// 统计 Human 消息数量
	humanCount := 0
	for _, m := range w.messages {
		if m.Role == llms.ChatMessageTypeHuman {
			humanCount++
		}
	}

	// 超出窗口则从头删除最早的完整一轮（从第一条 Human 到下一条 Human 之前）
	for humanCount > w.windowSize {
		// 找到第一条 Human 消息的索引
		firstHuman := -1
		for i, m := range w.messages {
			if m.Role == llms.ChatMessageTypeHuman {
				firstHuman = i
				break
			}
		}
		if firstHuman < 0 {
			break
		}
		// 找到下一条 Human 消息（即下一轮的起始位置）
		nextHuman := -1
		for i := firstHuman + 1; i < len(w.messages); i++ {
			if w.messages[i].Role == llms.ChatMessageTypeHuman {
				nextHuman = i
				break
			}
		}
		if nextHuman < 0 {
			// 只剩一轮，不再裁剪
			break
		}
		w.messages = w.messages[nextHuman:]
		humanCount--
	}
}

// history 返回当前窗口内的所有消息（不含 system prompt）
func (w *messageWindow) history() []llms.MessageContent {
	return w.messages
}

// clear 清空历史
func (w *messageWindow) clear() {
	w.messages = nil
}
