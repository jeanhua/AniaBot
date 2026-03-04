package pluginsys

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type PluginSys struct {
	plugin.Meta
	adminId    uint
	pluginInfo []bot.PluginInfo
}

func NewPluginSys() *PluginSys {
	return &PluginSys{
		Meta: plugin.Meta{
			Name:      "系统插件",
			HelpWords: "AniaBot系统插件",
			Order:     plugin.LevelLog,
		},
	}
}

func (p *PluginSys) Start(ctx context.Context, cfg *viper.Viper) error {
	p.adminId = cfg.GetUint("bot.admin_id")
	return nil
}

func (p *PluginSys) Awake(ctx context.Context, bot bot.Bot) error {
	p.pluginInfo = bot.GetPluginList()
	return nil
}

func (p *PluginSys) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "help" {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, info := range p.pluginInfo {
			if info.AdminOnly && msg.Sender.UserId != p.adminId {
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
			p.Logger.Println("Bot消息发送失败，无法响应 /help")
		}
		return false, nil
	}
	return true, nil
}

func (p *PluginSys) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "help" && cmd.Mention {
		var pluginInfo strings.Builder
		pluginInfo.WriteString("\n欢迎使用AniaBot，已加载插件:")
		idx := 1
		for _, info := range p.pluginInfo {
			if info.AdminOnly && msg.Sender.UserId != p.adminId {
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
			p.Logger.Println("Bot消息发送失败，无法响应 /help")
		}
		return false, nil
	}
	return true, nil
}
