package utils

import (
	"strings"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func ExtraMessageStr(msg message.Message) (string, bool) {
	var builder strings.Builder
	mention := false
	for _, m := range msg.Message {
		switch m.Type {
		case "text":
			if text, ok := m.Data["text"].(string); ok {
				builder.WriteString(text)
			}
		case "at":
			if qq, ok := m.Data["qq"].(string); ok {
				if qq == msg.SelfId.String() {
					mention = true
				}
			}
		}
	}
	text := strings.TrimSpace(builder.String())
	return text, mention
}

func HasMention(msg message.Message) bool {
	for _, m := range msg.Message {
		if m.Type == "at" {
			// Data 来自外部 JSON，qq 字段可能缺失或非字符串，必须用 comma-ok 断言
			if qq, ok := m.Data["qq"].(string); ok && qq == msg.SelfId.String() {
				return true
			}
		}
	}
	return false
}
