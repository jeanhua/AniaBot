package aichat

import "github.com/jeanhua/AniaBot/bot/component/llmtool"

// MessageRole 消息角色
type MessageRole string

const (
	RoleSystem    MessageRole = "system"
	RoleDeveloper MessageRole = "developer"
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleTool      MessageRole = "tool"
)

// ContentPartType 内容片段类型
type ContentPartType int

const (
	ContentPartText ContentPartType = iota
	ContentPartImageURL
)

// ContentPart 消息内容片段（文本或图片）
type ContentPart struct {
	Type     ContentPartType `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL string          `json:"image_url,omitempty"`
}

// Message 对话消息
type Message struct {
	Role       MessageRole        `json:"role"`
	Parts      []ContentPart      `json:"parts,omitempty"`
	ToolCallID string             `json:"tool_call_id,omitempty"` // tool 结果消息使用
	ToolCalls  []llmtool.ToolCall `json:"tool_calls,omitempty"`   // assistant 消息使用
	// ReasoningContent 保存 API 返回的推理过程内容（如 DeepSeek 的 reasoning_content），
	// 在多轮对话（特别是 tool calling）中需要原样传回，否则 API 会报错。
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// MarshalJSON / UnmarshalJSON 让 ContentPartType 以字符串编码，
// 便于持久化存储时可读、可迁移。
func (t ContentPartType) MarshalJSON() ([]byte, error) {
	switch t {
	case ContentPartImageURL:
		return []byte(`"image_url"`), nil
	default:
		return []byte(`"text"`), nil
	}
}

func (t *ContentPartType) UnmarshalJSON(data []byte) error {
	switch string(data) {
	case `"image_url"`:
		*t = ContentPartImageURL
	default:
		*t = ContentPartText
	}
	return nil
}

// ChatOptions LLM 调用参数
type ChatOptions struct {
	MaxToken           *int
	MaxCompletionToken *int
	Temperature        *float64
	TopP               *float64
	TopK               *int // 非标准参数，部分兼容 API 使用
	Tools              []llmtool.ToolDef
	ReasoningEffort    *string // "low", "medium", "high"

	// OnStreamDelta 流式文本增量回调（nil = 一次性生成）。
	// 注意：在流读取的同一 goroutine 串行调用，回调内勿做重 IO；
	// 回调收到的 delta 尚未去除 <think> 块（由调用方在缓冲上统一处理）。
	OnStreamDelta func(string)
	// OnStreamRoundEnd 工具调用轮结束回调（toolexecutor 在工具边界调用）：
	// 调用方应 End 当前流式消息；下一轮首个增量创建新消息。
	OnStreamRoundEnd func()
}

func TextPart(text string) ContentPart {
	return ContentPart{Type: ContentPartText, Text: text}
}

func ImageURLPart(url string) ContentPart {
	return ContentPart{Type: ContentPartImageURL, ImageURL: url}
}

func TextMessage(role MessageRole, text string) Message {
	return Message{
		Role:  role,
		Parts: []ContentPart{TextPart(text)},
	}
}
