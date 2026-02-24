package interceptor

import (
	"context"
	"log"
	"strconv"

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
			Order:     1,
			AdminOnly: true,
		},
	}
}

func (p *InterceptorPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.interceptGroup = cfg.GetStringSlice("plugin.interceptor.blacklist.groups")
	p.interceptUser = cfg.GetStringSlice("plugin.interceptor.blacklist.users")

	p.permitGroup = cfg.GetStringSlice("plugin.interceptor.whitelist.groups")
	p.permitUser = cfg.GetStringSlice("plugin.interceptor.whitelist.users")

	log.Println("拦截器插件初始化完成, 配置信息如下:")
	log.Println("拦截群聊:")
	for _, id := range p.interceptGroup {
		log.Println(id)
	}
	log.Println("拦截好友:")
	for _, id := range p.interceptUser {
		log.Println(id)
	}

	log.Println("放行群聊:")
	for _, id := range p.permitGroup {
		log.Println(id)
	}
	log.Println("放行好友:")
	for _, id := range p.permitUser {
		log.Println(id)
	}

	return nil
}

func (p *InterceptorPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	permit := p.check(TargetGroup, msg)
	if !permit {
		log.Println("拦截器插件拦截: [群]", msg.GroupId)
	}
	return permit, nil
}

func (p *InterceptorPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	permit := p.check(TargetFriend, msg)
	if !permit {
		log.Println("拦截器插件拦截: [好友]", msg.Sender.UserId)
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
			if id == "all" || ieqs(msg.GroupId, id) {
				return false
			}
		}
		for _, id := range p.interceptUser {
			if id == "all" || ieqs(msg.Sender.UserId, id) {
				return false
			}
		}
		for _, id := range p.permitGroup {
			if id == "all" || ieqs(msg.GroupId, id) {
				return true
			}
		}
	case TargetFriend:
		for _, id := range p.interceptUser {
			if id == "all" || ieqs(msg.Sender.UserId, id) {
				return false
			}
		}
		for _, id := range p.permitUser {
			if id == "all" || ieqs(msg.Sender.UserId, id) {
				return true
			}
		}
	}
	return false
}

func ieqs(numInt uint, numStr string) bool {
	return strconv.Itoa(int(numInt)) == numStr
}
