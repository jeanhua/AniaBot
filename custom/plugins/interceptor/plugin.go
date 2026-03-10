package interceptor

import (
	"context"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type InterceptorPlugin struct {
	plugin.Meta

	interceptUser  []string
	interceptGroup []string

	permitUser  []string
	permitGroup []string
}

func NewInterceptorPlugin() *InterceptorPlugin {
	return &InterceptorPlugin{
		Meta: plugin.Meta{
			Name:      "消息拦截器插件",
			HelpWords: "拦截和放行配置信息中指定的群聊和用户，减少消息干扰",
			Order:     plugin.LevelLog + 1,
			AdminOnly: true,
			ShowFor:   plugin.ShowForNone,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
	}
}

func (p *InterceptorPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.interceptGroup = cfg.GetStringSlice("plugin.interceptor.blacklist.groups")
	p.interceptUser = cfg.GetStringSlice("plugin.interceptor.blacklist.users")

	p.permitGroup = cfg.GetStringSlice("plugin.interceptor.whitelist.groups")
	p.permitUser = cfg.GetStringSlice("plugin.interceptor.whitelist.users")

	p.Logger.Info("拦截器插件初始化完成, 配置信息如下:")
	p.Logger.Info("拦截群聊:")
	for _, id := range p.interceptGroup {
		p.Logger.Info("群聊:", "groupId", id)
	}
	p.Logger.Info("拦截好友:")
	for _, id := range p.interceptUser {
		p.Logger.Info("好友:", "userId", id)
	}

	p.Logger.Info("放行群聊:")
	for _, id := range p.permitGroup {
		p.Logger.Info("群聊:", "groupId", id)
	}
	p.Logger.Info("放行好友:")
	for _, id := range p.permitUser {
		p.Logger.Info("好友:", "userId", id)
	}

	return nil
}

func (p *InterceptorPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	permit := p.check(TargetGroup, msg)
	if !permit {
		p.Logger.Info("拦截: [群]", "groupId", msg.GroupId)
	}
	return permit, nil
}

func (p *InterceptorPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	permit := p.check(TargetFriend, msg)
	if !permit {
		p.Logger.Info("拦截: [好友]", "userId", msg.Sender.UserId)
	}
	return permit, nil
}

const (
	TargetGroup = iota
	TargetFriend
)

func (p *InterceptorPlugin) check(target int, msg message.Message) bool {
	switch target {
	case TargetGroup:
		for _, id := range p.interceptGroup {
			if id == "all" || msg.GroupId.String() == id {
				p.Logger.Info("触发全部拦截")
				return false
			}
		}
		for _, id := range p.interceptUser {
			if id == "all" || msg.Sender.UserId.String() == id {
				p.Logger.Info("触发好友拦截:", "userId", msg.Sender.UserId)
				return false
			}
		}
		for _, id := range p.permitGroup {
			if id == "all" || msg.GroupId.String() == id {
				return true
			}
		}
	case TargetFriend:
		for _, id := range p.interceptUser {
			if id == "all" || msg.Sender.UserId.String() == id {
				return false
			}
		}
		for _, id := range p.permitUser {
			if id == "all" || msg.Sender.UserId.String() == id {
				return true
			}
		}
	}
	return false
}
