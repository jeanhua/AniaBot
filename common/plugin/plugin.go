package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

type PluginWrapper struct {
	StartFunc StartupEvent
	Plugin    Plugin
	Event     BasicEvent
}

type Plugin interface {
	GetMeta() *Meta
}

type BasicEvent interface {
	OnGroupMsg(bot.Bot, *command.Command, message.Message) bool
	OnFriendMsg(bot.Bot, *command.Command, message.Message) bool
}

type StartupEvent interface {
	Start(cfg *viper.Viper)
}
