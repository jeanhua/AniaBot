package utils

import (
	"strconv"
	"strings"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func ExtraMessageStr(msg message.Message) (text string, mention bool) {
	var builder strings.Builder
	mention = false
	for _, m := range msg.Message {
		switch m.Type {
		case "text":
			builder.WriteString(m.Data["text"].(string))
		case "at":
			if m.Type == "at" && m.Data["qq"].(string) == strconv.Itoa(int(msg.SelfId)) {
				mention = true
			}
		}
	}
	text = strings.TrimSpace(builder.String())
	return
}
