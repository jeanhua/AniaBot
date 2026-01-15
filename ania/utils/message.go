package utils

import (
	"strconv"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
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

func ExtraMessage(bot bot.Bot, msg message.Message) string {
	var s strings.Builder
	for _, m := range msg.Message {
		s.WriteString(m.FriendlyText(bot.GetMsgDetail, true))
	}
	return s.String()
}
