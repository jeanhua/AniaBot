package pluginrepeat

import (
	"sync"
	"sync/atomic"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

type RepeatPlugin struct {
	plugin.Meta
	admin      uint
	repeatGMap sync.Map
	enable     atomic.Bool
}

type repeatCount struct {
	lock     sync.Mutex
	msg      string
	count    int
	repeated bool
}

func NewPlugin() *RepeatPlugin {
	p := &RepeatPlugin{}
	p.Name = "复读机插件"
	p.HelpWords = "只会复读..., at我发送 /close(enable) repeat 可关闭(开启)复读机"
	p.AdminOnly = false
	return p
}

func (p *RepeatPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	if cmd != nil {
		if cmd.Mention && cmd.Name == "close" && len(cmd.Args) >= 1 && cmd.Args[0] == "repeat" {
			if msg.Sender.UserId == p.admin {
				p.enable.Store(false)
				builder := msgchain.Builder.Group()
				builder.Text("已关闭复读机")
				bot.SendGroupMsg(msg.GroupId, builder.Build())
				return false
			} else {
				builder := msgchain.Builder.Group()
				builder.Text("你没有权限哦")
				bot.SendGroupMsg(msg.GroupId, builder.Build())
				return false
			}
		} else if cmd.Mention && cmd.Name == "enable" && len(cmd.Args) >= 1 && cmd.Args[0] == "repeat" {
			if msg.Sender.UserId == p.admin {
				p.enable.Store(true)
				builder := msgchain.Builder.Group()
				builder.Text("已开启复读机")
				bot.SendGroupMsg(msg.GroupId, builder.Build())
				return false
			} else {
				builder := msgchain.Builder.Group()
				builder.Text("你没有权限哦")
				bot.SendGroupMsg(msg.GroupId, builder.Build())
				return false
			}
		}
	}

	if p.enable.Load() == false {
		return true
	}
	val, ok := p.repeatGMap.Load(msg.GroupId)
	if !ok {
		p.repeatGMap.Store(msg.GroupId, &repeatCount{
			msg:   msg.RawMessage,
			count: 1,
		})
		return true
	}

	rc := val.(*repeatCount)
	needRepeat := false

	rc.lock.Lock()
	if rc.msg == msg.RawMessage {
		rc.count++
		if rc.count >= 3 && !rc.repeated {
			needRepeat = true
			rc.repeated = true
		}
	} else {
		rc.msg = msg.RawMessage
		rc.count = 1
		rc.repeated = false
	}
	rc.lock.Unlock()
	if needRepeat {
		builder := msgchain.Builder.Group()
		builder.Raw(msg.Message...)
		bot.SendGroupMsg(msg.GroupId, builder.Build())
	}
	return true
}

func (p *RepeatPlugin) OnFriendMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	return true
}

func (p *RepeatPlugin) Start(cfg *viper.Viper) {
	p.admin = cfg.GetUint("bot.admin_id")
	p.enable.Store(true)
}
