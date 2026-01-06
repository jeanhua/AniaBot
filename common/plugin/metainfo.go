package plugin

import (
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/spf13/viper"
)

type Meta struct {
	Name      string // 插件名字
	HelpWords string // 插件帮助字段，发送 /help 指令显示
	AdminOnly bool   // 插件是否为管理员触发(对其他人隐藏)
	Order     int    // 插件执行顺序，从小到大
}

func (p *Meta) GetMeta() *Meta {
	return p
}

func (p *Meta) OnGroupMsg(bot.Bot, *command.Command, message.Message) bool {
	return true
}

func (p *Meta) OnFriendMsg(bot.Bot, *command.Command, message.Message) bool {
	return true
}

func (p *Meta) Start(cfg *viper.Viper) {}
