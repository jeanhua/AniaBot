package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

type PluginWrapper struct {
	Plugin Plugin
	Event  BasicEvent
}

type Plugin interface {
	GetMeta() *Meta
}

type BasicEvent interface {
	OnGroupMsg(bot.Bot, message.Message) bool
	OnFriendMsg(bot.Bot, message.Message) bool
}
