package pluginantiwithdrawal

import (
	"log"
	"strconv"
	"sync"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
)

type AntiWithdrawalPlugin struct {
	plugin.Meta
	msg sync.Map
}

func NewPlugin() *AntiWithdrawalPlugin {
	p := &AntiWithdrawalPlugin{}
	p.Name = "群防撤回插件"
	p.HelpWords = "群聊回顾最近的n条消息，发送 /explore [n] 获取，n<=100，默认50"
	p.AdminOnly = false
	return p
}

func (p *AntiWithdrawalPlugin) OnGroupMsg(bot bot.Bot, cmd *command.Command, msg message.Message) bool {
	queueI, _ := p.msg.LoadOrStore(msg.GroupId, NewMessageQueue[*message.Message](100))
	queue := queueI.(*MessageQueue[*message.Message])
	if cmd != nil && cmd.Mention && cmd.Name == "explore" {
		n := 50
		if len(cmd.Args) >= 1 {
			num, err := strconv.Atoi(cmd.Args[0])
			if err == nil && num > 0 && num <= 100 {
				n = num
			}
		}
		cachemsg := queue.Get(n)
		fbuilder := msgchain.Builder.Forward()
		for _, m := range cachemsg {
			_builder := msgchain.Builder.Group()
			_builder.Raw(m.Message)
			fbuilder.Message(m.Sender.UserId, m.Sender.Nickname, _builder.Build())
		}
		success, _ := bot.SendGroupForwardMsg(msg.GroupId, fbuilder.Build())
		if !success {
			log.Println("[群聊防撤回插件]: 无法转发消息")
		}
		return false
	}
	queue.Add(&msg)
	return true
}
