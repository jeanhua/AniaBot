package pluginnews

import (
	"context"

	"github.com/jeanhua/AniaBot/common/aniaerror"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type NewsPlugin struct {
	plugin.Meta
	cronExpress string
	api         string
	groups      []uint

	adminId message.QID
}

func NewNewsPlugin() *NewsPlugin {
	return &NewsPlugin{
		Meta: plugin.Meta{
			Name:      "每日新闻插件",
			HelpWords: "每日准点在指定群里新闻播报，发送 /news 立即获取，管理员发送 /news force 强制执行发送任务",
		},
	}
}

func (p *NewsPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.adminId = message.QID(cfg.GetInt64("bot.admin_id"))
	p.cronExpress = cfg.GetString("plugin.dailyNews.cron")
	if p.cronExpress == "" {
		p.Logger.Error("读取daily news cron表达式错误")
		return aniaerror.ParameterInitializeError
	}
	p.api = cfg.GetString("plugin.dailyNews.api")
	if p.api == "" {
		p.Logger.Error("读取daily news api错误")
		return aniaerror.ParameterInitializeError
	}
	groups := cfg.GetIntSlice("plugin.dailyNews.groups")
	for _, g := range groups {
		p.Logger.Info("播报群聊注册", "groupId", g)
		p.groups = append(p.groups, uint(g))
	}
	return nil
}

func (p *NewsPlugin) StartCron(ctx context.Context, bot bot.Bot, c plugin.CronManager) error {
	c.AddFunc(p.cronExpress, func() {
		p.sendNews(bot)
	})
	return nil
}

func (p *NewsPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention && cmd.Name == "news" {
		builder := msgchain.Builder().Group()
		builder.ImageUrl(p.api)
		_, ok := bot.SendGroupMsg(msg.GroupId, builder.Build())
		if ok {
			p.Logger.Info("发送消息", "groupId", msg.GroupId, "message", "[每日新闻]")
		} else {
			p.Logger.Error("发送消息失败", "groupId", msg.GroupId, "message", "[每日新闻] 发送失败...")
		}
		return false, nil
	}
	return true, nil
}

func (p *NewsPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if msg.Sender.UserId == p.adminId && cmd.Name == "news" && len(cmd.Args) > 0 && cmd.Args[0] == "force" {
		p.sendNews(bot)
		return false, nil
	}
	return true, nil
}

func (p *NewsPlugin) sendNews(bot bot.Bot) {
	for _, group := range p.groups {
		builder := msgchain.Builder().Group()
		builder.ImageUrl(p.api)
		_, ok := bot.SendGroupMsg(message.QID(group), builder.Build())
		if ok {
			p.Logger.Info("发送消息", "groupId", group, "message", "[每日新闻]")
		} else {
			p.Logger.Error("发送消息失败", "groupId", group, "message", "[每日新闻] 发送失败...")
		}
	}
}
