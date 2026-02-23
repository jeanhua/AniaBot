package pluginlog

import (
	"context"
	"log"

	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
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

func (p *LogPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	lastStartTime, ok := p.Storage.GetString(context.Background(), "last_start_time")
	if !ok {
		lastStartTime = "未保存"
	}
	p.Storage.Set(context.Background(), "last_start_time", utils.GetFormattedTime())
	log.Println("日志打印插件初始化完成, 上次重启时间: ", lastStartTime)
	return nil
}

func (p *LogPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	str := utils.ExtraMessage(bot, msg)
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("[收<-群:%d 昵称:%s]: %s", msg.GroupId, name, str)
	return true, nil
}

func (p *LogPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	str := utils.ExtraMessage(bot, msg)
	name := msg.Sender.Card
	if name == "" {
		name = msg.Sender.Nickname
	}
	log.Printf("[收<-好友:%d 昵称:%s]: %s", msg.Sender.UserId, name, str)
	return true, nil
}
