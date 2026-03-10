package acgwallpaper

import (
	"context"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type AcgWallpaperPlugin struct {
	plugin.Meta
	pendding chan work
}

func NewAcgWallpaperPlugin(maxWork int) *AcgWallpaperPlugin {
	pendding := make(chan work, maxWork)
	return &AcgWallpaperPlugin{
		Meta: plugin.Meta{
			Name:      "二次元壁纸插件",
			HelpWords: "给我发送 /acg 获取哦",
			Order:     plugin.LevelNormal,
			ShowFor:   plugin.ShowForGroup | plugin.ShowForFriend,
			Author:    "jeanhua",
			Version:   "1.0.0",
		},
		pendding: pendding,
	}
}

func (p *AcgWallpaperPlugin) Awake(ctx context.Context, bot bot.Bot) error {
	bot.Go("AcgWallPager插件工作线程", func() {
		p.workFunc(bot)
	})
	return nil
}

func (p *AcgWallpaperPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Mention && cmd.Name == "acg" {
		select {
		case p.pendding <- work{target: TargetGroup, userId: msg.Sender.UserId, groupId: msg.GroupId}:
		default:
			builder := msgchain.Builder().Group()
			builder.Reply(msg.MessageId)
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 任务队列满出来了，等待会再来问我要壁纸哦")
			bot.SendGroupMsg(msg.GroupId, builder.Build())
		}
		return false, nil
	}
	return true, nil
}

func (p *AcgWallpaperPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if cmd.Name == "acg" {
		select {
		case p.pendding <- work{target: TargetFriend, userId: msg.Sender.UserId, groupId: 0}:
		default:
			builder := msgchain.Builder().Friend()
			builder.Text("任务队列满出来了，等待会再来问我要壁纸哦")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false, nil
	}
	return true, nil
}

func (p *AcgWallpaperPlugin) workFunc(bot bot.Bot) {
	for {
		w := <-p.pendding
		switch w.target {
		case TargetFriend:
			builder := msgchain.Builder().Friend()
			builder.ImageUrl("https://api.yppp.net/api.php")
			if _, ok := bot.SendFriendMsg(w.userId, builder.Build()); ok {
				p.Logger.Info("发送二次元壁纸消息", "userId", w.userId)
			} else {
				p.Logger.Error("发送二次元壁纸消息失败", "userId", w.userId)
			}
		case TargetGroup:
			builder := msgchain.Builder().Group()
			builder.ImageUrl("https://api.yppp.net/api.php")
			if _, ok := bot.SendGroupMsg(w.groupId, builder.Build()); ok {
				p.Logger.Info("发送二次元壁纸消息", "groupId", w.groupId)
			} else {
				p.Logger.Error("发送二次元壁纸消息失败", "groupId", w.groupId)
			}
		}
	}
}
