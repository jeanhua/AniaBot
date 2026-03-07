package groupnewsletter

import (
	"fmt"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

func (p *GroupNewsletter) handleCommand(b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	reply := func(text string) {
		b.SendGroupMsg(msg.GroupId, msgchain.Builder().Group().
			Reply(msg.MessageId).Text(text).Build())
	}

	if p.llm == nil {
		reply("群刊插件未正确配置，请检查 API 配置")
		return false, nil
	}

	if len(cmd.Args) > 0 && cmd.Args[0] == "gen" {
		return p.handleGenCommand(b, msg, reply)
	}

	// 默认：查看当前状态
	count := p.getMessageCount(msg.GroupId)
	reply(fmt.Sprintf("当前已收集 %d 条消息，需要 %d 条触发自动生成群刊",
		count, p.config.msgThreshold))
	return false, nil
}

func (p *GroupNewsletter) handleGenCommand(b bot.Bot, msg message.Message, reply func(string)) (bool, error) {
	count := p.getMessageCount(msg.GroupId)
	if count == 0 {
		reply("当前没有收集到消息，无法生成群刊")
		return false, nil
	}
	if p.isGenerating(msg.GroupId) {
		reply("群刊正在生成中，请稍候…")
		return false, nil
	}

	b.Go("GroupNewsletter插件生成群刊线程", func() {
		p.generateForGroup(p.pluginCtx, b, msg.GroupId, true)
	})
	reply(fmt.Sprintf("正在生成群刊（共 %d 条消息），请稍后…", count))
	return false, nil
}
