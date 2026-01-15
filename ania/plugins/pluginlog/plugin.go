package pluginlog

import (
	"log"

	"github.com/jeanhua/AniaBot/ania/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type LogPlugin struct {
	plugin.Meta
}

func NewPlugin() *LogPlugin {
	p := &LogPlugin{}
	p.Name = "日志打印插件"
	p.HelpWords = "用于在控制台打印日志信息"
	p.AdminOnly = true
	return p
}

func (p *LogPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	str := utils.ExtraMessage(bot, msg)
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到群聊消息[%d %s]: %s", msg.GroupId, name, str)
	return true
}

func (p *LogPlugin) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	str := utils.ExtraMessage(bot, msg)
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到好友消息[%d %s]: %s", msg.Sender.UserId, name, str)
	return true
}
