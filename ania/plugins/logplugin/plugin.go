package logplugin

import (
	"log"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
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

func (p *LogPlugin) OnGroupMsg(bot bot.Bot, msg message.Message) bool {
	var rawStrMsg strings.Builder
	for _, m := range msg.Message {
		if m.Type == "text" {
			rawStrMsg.WriteString(m.Data["text"].(string))
		}
	}
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到群聊消息[%d %s]: %s", msg.GroupId, name, rawStrMsg.String())
	return true
}

func (p *LogPlugin) OnFriendMsg(bot bot.Bot, msg message.Message) bool {
	var rawStrMsg strings.Builder
	for _, m := range msg.Message {
		if m.Type == "text" {
			rawStrMsg.WriteString(m.Data["text"].(string))
		}
	}
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("收到好友消息[%d %s]: %s", msg.Sender.UserId, name, rawStrMsg.String())
	return true
}
