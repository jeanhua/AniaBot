package acgwallpaper

import (
	"log"

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
		},
		pendding: pendding,
	}
}

func (p *AcgWallpaperPlugin) StartCron(bot bot.Bot, c plugin.CronManager) {
	go p.workFunc(bot)
}

func (p *AcgWallpaperPlugin) OnGroupMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
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
		return false
	}
	return true
}

func (p *AcgWallpaperPlugin) OnFriendMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
	if cmd.Name == "acg" {
		select {
		case p.pendding <- work{target: TargetFriend, userId: msg.Sender.UserId, groupId: 0}:
		default:
			builder := msgchain.Builder().Friend()
			builder.Text("任务队列满出来了，等待会再来问我要壁纸哦")
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
		}
		return false
	}
	return true
}

func (p *AcgWallpaperPlugin) workFunc(bot bot.Bot) {
	for {
		w := <-p.pendding
		switch w.target {
		case TargetFriend:
			builder := msgchain.Builder().Friend()
			builder.ImageUrl("https://api.yppp.net/api.php")
			if _, ok := bot.SendFriendMsg(w.userId, builder.Build()); ok {
				log.Printf("[发->好友:%d]:[二次元壁纸]\n", w.userId)
			} else {
				log.Printf("[发->好友:%d]:[二次元壁纸] 发送失败!!!\n", w.userId)
			}
		case TargetGroup:
			builder := msgchain.Builder().Group()
			builder.Mention(w.userId)
			builder.ImageUrl("https://api.yppp.net/api.php")
			if _, ok := bot.SendGroupMsg(w.groupId, builder.Build()); ok {
				log.Printf("[发->群聊:%d]:[二次元壁纸]\n", w.groupId)
			} else {
				log.Printf("[发->群聊:%d]:[二次元壁纸] 发送失败!!!\n", w.groupId)
			}
		}
	}
}
