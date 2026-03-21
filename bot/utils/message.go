package utils

import (
	"strconv"
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
				if qq == strconv.Itoa(int(msg.SelfId)) {
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
			if m.Type == "at" && m.Data["qq"].(string) == strconv.Itoa(int(msg.SelfId)) {
				return true
			}
		}
	}
	return false
}
