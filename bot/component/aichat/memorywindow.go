package aichat

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/common/model/message"
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
	// 文本标记。新落盘的数据在 persist 时已剔除图片（见 degradeImagesForPersist），
	// 此处理主要兼容旧版落盘数据中仍保留的图片片段。
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
				// 保留图片哈希标记，AI 仍可将历史提及与具体图片对应
				newParts = append(newParts, TextPart("[图片 "+message.ImageHash(p.ImageURL)+"，链接已失效]"))
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
// 落盘副本中的图片片段一律降级为文本标记（degradeImagesForPersist）：
// 远程 http(s) 链接重启后失效无保留价值；data URI（base64 内联图片）体积
// 可达 MB 级，若随历史整体反复全量重写会撑大单 key 并造成严重写放大
// （MySQL MEDIUMTEXT 超限还会导致落盘静默失败、历史丢失）。
// 仅影响落盘数据，内存中当前会话的消息仍保留图片供本轮对话使用。
// 使用独立的后台 context，避免请求被 /stop 取消时丢失刚写入的历史。
func (w *messageWindow) persist() {
	if w.store == nil {
		return
	}
	ctx := context.Background()
	if err := w.store.Save(ctx, degradeImagesForPersist(w.messages)); err != nil {
		// 落盘失败仅记录，不影响内存中的对话
		_ = err
	}
}

// degradeImagesForPersist 将待落盘消息中的所有图片片段（http(s) 与 data URI）
// 替换为文本标记，返回新切片，不修改原消息。
func degradeImagesForPersist(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	for i := range msgs {
		msg := msgs[i]
		if len(msg.Parts) == 0 {
			out[i] = msg
			continue
		}
		changed := false
		newParts := make([]ContentPart, 0, len(msg.Parts))
		for _, p := range msg.Parts {
			if p.Type == ContentPartImageURL {
				// 降级为带哈希的文本标记，与消息文本中的图片标识一致
				newParts = append(newParts, TextPart("[图片 "+message.ImageHash(p.ImageURL)+"]"))
				changed = true
				continue
			}
			newParts = append(newParts, p)
		}
		if changed {
			msg.Parts = newParts
		}
		out[i] = msg
	}
	return out
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
	// 压缩判断需要的是当前上下文的真实大小（最后一次调用的 prompt token），
	// 而非跨工具轮次累加的 PromptTokens——累加值会虚高，导致过早触发有损压缩
	if usage.LastPromptTokens > 0 {
		w.lastPromptTokens = usage.LastPromptTokens
	} else if usage.PromptTokens > 0 {
		w.lastPromptTokens = usage.PromptTokens
	}
}

func (w *messageWindow) needsCompression() bool {
	if w.maxContextTokens <= 0 {
		return false
	}
	threshold := int(float64(w.maxContextTokens) * 0.8)
	if w.lastPromptTokens > 0 {
		return w.lastPromptTokens > threshold
	}
	// 上游未上报 usage 时用字符数粗估兜底：缺少真实 token 统计不代表上下文很小，
	// 若不兜底历史会无限增长，撑大落盘 key 并拖慢每轮的全量读写。
	return w.estimateTokens() > threshold
}

// estimateTokens 用字符数粗估当前历史的 token 量，仅用于 usage 缺失时的兜底判断。
// 约 2 个字符折 1 token（中英文混合的保守估计）；图片片段按固定值估算，
// 不统计 data URI 的 base64 长度（与真实多模态 token 计量无关）。
func (w *messageWindow) estimateTokens() int {
	total := 0
	for _, m := range w.messages {
		for _, p := range m.Parts {
			if p.Type == ContentPartImageURL {
				total += 1000 // 多模态图片的约值
				continue
			}
			total += utf8.RuneCountInString(p.Text)
		}
		total += utf8.RuneCountInString(m.ReasoningContent)
		for _, tc := range m.ToolCalls {
			total += utf8.RuneCountInString(tc.Name) + utf8.RuneCountInString(tc.Arguments)
		}
	}
	return total / 2
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
