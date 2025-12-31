package utils

import (
	"strings"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func ExtraMessageStr(msg message.Message) string {
	var builder strings.Builder
	for _, msg := range msg.Message {
		switch msg.Type {
		case "text":
			builder.WriteString(msg.Data["text"].(string))
		}
	}
	return builder.String()
}
