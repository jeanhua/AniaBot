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
	store            HistoryStore
}

func newMessageWindow(maxContextTokens int, llmClient *LLMClient, compressor CompressorFunc, store HistoryStore) *messageWindow {
	return &messageWindow{
		maxContextTokens: maxContextTokens,
		llmClient:        llmClient,
		compressor:       compressor,
		store:            store,
	}
}

// load 从持久化存储回放历史；存储为空或未注入时保持空窗口。
func (w *messageWindow) load(ctx context.Context) {
	if w.store == nil {
		return
	}
	msgs, err := w.store.Load(ctx)
	if err != nil {
		// 加载失败不应阻断对话，按空历史继续，后续 Save 会覆盖
		return
	}
	// 回放的历史中的图片 URL（多为 QQ 临时签名链接）重启后大概率失效，
	// 若原样发给 LLM 会因拉取失败导致整轮对话报错。这里把图片片段降级为
	// 文本标记：仅作用于内存中回放后的副本，落盘的原始 URL 不变（无损），
	// 当前会话本轮新加载的图片在 persist 之前仍是 ImageURL 片段，不受影响。
	w.messages = degradeImagesToText(msgs)
}

// degradeImagesToText 将消息中基于 http(s) URL 的图片片段替换为文本标记。
// 用于回放持久化历史时规避失效的图片 URL（如 QQ 临时签名链接）。
// data URI（base64 内联，如本地图片）不依赖外部链接、重启不失效，故保留原样。
// 文本片段与工具调用不变。
func degradeImagesToText(msgs []Message) []Message {
	for i := range msgs {
		msg := &msgs[i]
		if len(msg.Parts) == 0 {
			continue
		}
		changed := false
		newParts := make([]ContentPart, 0, len(msg.Parts))
		for _, p := range msg.Parts {
			if p.Type == ContentPartImageURL && isRemoteImageURL(p.ImageURL) {
				newParts = append(newParts, TextPart("[图片，链接已失效]"))
				changed = true
				continue
			}
			newParts = append(newParts, p)
		}
		if changed {
			msg.Parts = newParts
		}
	}
	return msgs
}

// isRemoteImageURL 判断图片引用是否为可能失效的远程 http(s) 链接。
// data:、本地路径等非 http 形式返回 false（视为不失效，保留原样）。
func isRemoteImageURL(s string) bool {
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}

// persist 将当前历史落盘；store 未注入时为空操作。
// 使用独立的后台 context，避免请求被 /stop 取消时丢失刚写入的历史。
func (w *messageWindow) persist() {
	if w.store == nil {
		return
	}
	ctx := context.Background()
	if err := w.store.Save(ctx, w.messages); err != nil {
		// 落盘失败仅记录，不影响内存中的对话
		_ = err
	}
}

func (w *messageWindow) append(msgs ...Message) {
	w.messages = append(w.messages, msgs...)
	w.persist()
}

func (w *messageWindow) history() []Message {
	return w.messages
}

func (w *messageWindow) clear() {
	w.messages = nil
	w.lastPromptTokens = 0
	if w.store != nil {
		// 删除持久化历史，重启后也不再恢复
		_ = w.store.Clear(context.Background())
	}
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
	// 压缩后历史发生改变，需落盘覆盖旧记录
	w.persist()
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
