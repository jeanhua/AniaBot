package waifupics

import (
	"github.com/go-resty/resty/v2"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type WaifuPlugin struct {
	plugin.Meta
	client   *resty.Client
	pendding chan work
}

const categoryHelps = `
---- 人像 ----
waifu[默认] 老婆/女神角色
neko 猫娘
shinobu 忍（角色名）
megumin 惠惠（《为美好的世界献上祝福》角色）

---- 表情包 ----
bully 欺负
cuddle 拥抱/依偎
cry 哭泣
hug 拥抱
awoo 狼娘
kiss 亲吻
lick 舔
pat 摸头/轻拍
smug 得意/ smug脸
bonk 敲头
yeet 扔/丢
blush 脸红
smile 微笑
wave 挥手
highfive 击掌
handhold 牵手
nom 大口吃/嗷呜吃
bite 咬
glomp 猛扑拥抱
slap 扇巴掌
kill 杀
kick 踢
happy 开心
wink 眨眼
poke 戳
dance 跳舞
cringe 尴尬/抠脚趾`

var categories = []string{"waifu", "neko", "shinobu", "megumin", "bully", "cuddle", "cry", "hug", "awoo", "kiss", "lick", "pat", "smug", "bonk", "yeet", "blush", "smile", "wave", "highfive", "handhold", "nom", "bite", "glomp", "slap", "kill", "kick", "happy", "wink", "poke", "dance", "cringe"}

func validateCate(category string) bool {
	for _, c := range categories {
		if category == c {
			return true
		}
	}
	return false
}

func NewWaifuPlugin(maxWork int) *WaifuPlugin {
	c := make(chan work, maxWork)
	client := resty.New()
	return &WaifuPlugin{
		Meta: plugin.Meta{
			Name:      "waifu.pics插件",
			HelpWords: "发送 /waifu [类别] 获取，获取类别发送 /waifu help",
		},
		pendding: c,
		client:   client,
	}
}

// OnGroupMsg 收到群聊消息触发事件
func (p *WaifuPlugin) OnGroupMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
	if !cmd.Mention || cmd.Name != "waifu" {
		return true
	}
	category := "waifu"
	if len(cmd.Args) > 0 {
		if cmd.Args[0] == "help" {
			builder := msgchain.Builder().Group()
			builder.Mention(msg.Sender.UserId)
			builder.Text(" 发送 /waifu [类别] 获取哦, 类别如下").Face(12)
			builder.Text(categoryHelps)
			bot.SendGroupMsg(msg.GroupId, builder.Build())
			return false
		} else {
			category = cmd.Args[0]
			if !validateCate(category) {
				builder := msgchain.Builder().Group()
				builder.Mention(msg.Sender.UserId)
				builder.Text(" 不存在此分类哦")
				bot.SendGroupMsg(msg.GroupId, builder.Build())
				return false
			}
		}
	}

	select {
	case p.pendding <- work{
		category: category,
		target:   TargetGroup,
		userId:   msg.Sender.UserId,
		groupId:  msg.GroupId,
	}:
	default:
		builder := msgchain.Builder().Group()
		builder.Mention(msg.Sender.UserId)
		builder.Text(" 请求过于频繁，请稍后再试哦").Face(12)
		bot.SendGroupMsg(msg.GroupId, builder.Build())
	}
	return false
}

// OnFriendMsg 收到私聊消息触发事件
func (p *WaifuPlugin) OnFriendMsg(bot bot.Bot, cmd command.Command, msg message.Message) bool {
	if cmd.Name != "waifu" {
		return true
	}
	category := "waifu"
	if len(cmd.Args) > 0 {
		if cmd.Args[0] == "help" {
			builder := msgchain.Builder().Friend()
			builder.Text("发送 /waifu [类别] 获取哦, 类别如下").Face(12)
			builder.Text(categoryHelps)
			bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
			return false
		} else {
			category = cmd.Args[0]
			if !validateCate(category) {
				builder := msgchain.Builder().Friend()
				builder.Text("不存在此分类哦")
				bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
				return false
			}
		}
	}

	select {
	case p.pendding <- work{
		category: category,
		target:   TargetFriend,
		userId:   msg.Sender.UserId,
		groupId:  msg.GroupId,
	}:
	default:
		builder := msgchain.Builder().Friend()
		builder.Text("请求过于频繁，请稍后再试哦").Face(12)
		bot.SendFriendMsg(msg.Sender.UserId, builder.Build())
	}
	return false
}

func (p *WaifuPlugin) Awake(bot bot.Bot) {
	go p.workFunc(bot)
}

func (p *WaifuPlugin) workFunc(bot bot.Bot) {
	for {
		w := <-p.pendding
		resp := respTy{}
		if _, err := p.client.R().SetResult(&resp).Get("https://api.waifu.pics/sfw/" + w.category); err != nil {
			switch w.target {
			case TargetFriend:
				builder := msgchain.Builder().Friend()
				builder.Text("请求失败，请稍后再试哦")
				bot.SendFriendMsg(w.userId, builder.Build())
			case TargetGroup:
				builder := msgchain.Builder().Group()
				builder.Mention(w.userId)
				builder.Text(" 请求失败，请稍后再试哦")
				bot.SendGroupMsg(w.groupId, builder.Build())
			}
			continue
		}
		switch w.target {
		case TargetFriend:
			builder := msgchain.Builder().Friend()
			builder.ImageUrl(resp.URL)
			bot.SendFriendMsg(w.userId, builder.Build())
		case TargetGroup:
			builder := msgchain.Builder().Group()
			builder.ImageUrl(resp.URL)
			bot.SendGroupMsg(w.groupId, builder.Build())
		}
	}
}

type respTy struct {
	URL string `json:"url"`
}
