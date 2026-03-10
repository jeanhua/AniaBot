package pluginsys

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
)

type PluginSys struct {
	plugin.Meta

	lastPanicTime *time.Time
}

func NewPluginSys() *PluginSys {
	return &PluginSys{
		Meta: plugin.Meta{
			Name:      "系统插件",
			HelpWords: "AniaBot系统插件",
			Order:     plugin.LevelLog,
			ShowFor:   plugininfo.ShowForGroup | plugininfo.ShowForFriend,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
	}
}

func (p *PluginSys) Awake(ctx context.Context, bot bot.Bot) error {
	builder := msgchain.Builder().Friend()
	builder.Text("AniaBot启动成功，发送 /help 查看插件加载信息")
	_, ok := bot.SendFriendMsg(p.SystemConfig.AdminId, builder.Build())
	if !ok {
		p.Logger.Error("Bot消息发送失败，无法发送启动成功消息")
	}
	return nil
}

func (p *PluginSys) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "help" {
		plugins := bot.GetPluginList()
		var pluginInfo strings.Builder
		pluginInfo.WriteString("欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, info := range plugins {
			if info.AdminOnly && msg.Sender.UserId != p.SystemConfig.AdminId {
				continue
			}
			if msg.Sender.UserId != p.SystemConfig.AdminId && info.ShowFor&plugininfo.ShowForFriend == 0 {
				continue
			}
			pName := info.Name
			pHelpWords := info.HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder().Friend()
		c.Text(pluginInfo.String())
		_, ok := bot.SendFriendMsg(msg.Sender.UserId, c.Build())
		if !ok {
			p.Logger.Error("Bot消息发送失败，无法响应 /help")
		}
		return false, nil
	}
	return true, nil
}

func (p *PluginSys) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "help" && cmd.Mention {
		plugins := bot.GetPluginList()
		var pluginInfo strings.Builder
		pluginInfo.WriteString("\n欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, info := range plugins {
			if info.AdminOnly && msg.Sender.UserId != p.SystemConfig.AdminId {
				continue
			}
			if info.ShowFor&plugininfo.ShowForGroup == 0 {
				continue
			}
			pName := info.Name
			pHelpWords := info.HelpWords
			pluginInfo.WriteString(fmt.Sprintf("\n%d. %s: %s", idx, pName, pHelpWords))
			idx += 1
		}
		c := msgchain.Builder().Group()
		c.Mention(msg.Sender.UserId)
		c.Text(pluginInfo.String())
		_, ok := bot.SendGroupMsg(msg.GroupId, c.Build())
		if !ok {
			p.Logger.Error("Bot消息发送失败，无法响应 /help")
		}
		return false, nil
	}
	return true, nil
}

func (p *PluginSys) OnPanic(ctx context.Context, bot bot.Bot, name string, err any) {
	p.Logger.Error("插件运行时panic", "name", name, "err", err)
	now := time.Now()
	if p.lastPanicTime == nil || now.Sub(*p.lastPanicTime) > time.Minute {
		p.lastPanicTime = &now
		builder := msgchain.Builder().Friend()
		builder.Text(fmt.Sprintf("线程 %s 运行时panic: %v", name, err))
		_, ok := bot.SendFriendMsg(p.SystemConfig.AdminId, builder.Build())
		if !ok {
			p.Logger.Error("Bot消息发送失败，无法通知管理员")
		}
	}
}
