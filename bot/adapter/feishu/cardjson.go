package feishu

import (
	"encoding/json"
	"unicode/utf8"
)

// cardContentLimit 卡片内容保守上限：飞书交互卡片（interactive）JSON 总限制 30KB，
// 流式更新时按此截断 markdown 内容，为 JSON 结构留出余量。
const cardContentLimit = 28 * 1024

// buildCardJSON 构造 schema 2.0 交互式卡片 JSON（markdown 元素）。
// 飞书 im.message.patch 仅支持更新 interactive 卡片，流式回复以此承载内容。
func buildCardJSON(markdown string) string {
	card := map[string]any{
		"schema": "2.0",
		"elements": []map[string]any{
			{"tag": "markdown", "content": truncateMarkdown(markdown, cardContentLimit)},
		},
	}
	b, err := json.Marshal(card)
	if err != nil {
		// 纯 map 序列化理论不可达
		return `{"schema":"2.0","elements":[{"tag":"markdown","content":""}]}`
	}
	return string(b)
}

// truncateMarkdown 按字节截断（UTF-8 安全，避免截断在多字节字符中间），
// 超限时追加省略号。
func truncateMarkdown(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	truncated := s[:maxBytes]
	// 逐字节回退直到末尾是完整 UTF-8 序列（最长回退 3 字节）
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "…"
}
