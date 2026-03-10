package chatrecordsmaker

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/spf13/viper"
)

func getTextFromMessage(msg []message.OB11Segment) string {
	for _, seg := range msg {
		if seg.Type == "text" {
			if text, ok := seg.Data["text"].(string); ok {
				return text
			}
		}
	}
	return ""
}

type ChatRecordsMakerPlugin struct {
	plugin.Meta

	sessions map[message.QID]*session
	mu       sync.RWMutex
}

type session struct {
	step        int
	messages    []messageItem
	tempNick    string
	tempUserId  string
	tempContent string
}

type messageItem struct {
	nickname string
	userId   string
	content  string
	time     time.Time
}

func NewChatRecordsMakerPlugin() *ChatRecordsMakerPlugin {
	return &ChatRecordsMakerPlugin{
		Meta: plugin.Meta{
			Name:      "聊天记录伪造插件",
			HelpWords: "伪造合并转发聊天记录，发送「伪造记录」开始创建",
			Order:     plugin.LevelNormal,
		},
		sessions: make(map[message.QID]*session),
	}
}

func (p *ChatRecordsMakerPlugin) Start(_ context.Context, cfg *viper.Viper) error {
	p.Logger.Info("聊天记录伪造插件已启动")
	return nil
}

func (p *ChatRecordsMakerPlugin) Stop() {}

func (p *ChatRecordsMakerPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	userId := msg.Sender.UserId
	text := getTextFromMessage(msg.Message)

	p.mu.Lock()
	defer p.mu.Unlock()

	s, exists := p.sessions[userId]
	if !exists && !strings.Contains(text, "伪造记录") && !strings.Contains(text, "开始创建") {
		return true, nil
	}

	if strings.Contains(text, "伪造记录") || strings.Contains(text, "开始创建") || strings.Contains(text, "创建聊天记录") {
		p.sessions[userId] = &session{step: 1}
		p.sendHelp(b, userId)
		return false, nil
	}

	if !exists {
		return true, nil
	}

	switch s.step {
	case 1:
		s.tempNick = text
		p.askForUserId(b, userId)
		s.step = 2

	case 2:
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			p.sendMsg(b, userId, "请输入有效的QQ号码：")
			return false, nil
		}
		s.tempUserId = text
		p.askForContent(b, userId, s.tempNick)
		s.step = 3

	case 3:
		s.tempContent = text
		s.messages = append(s.messages, messageItem{
			nickname: s.tempNick,
			userId:   s.tempUserId,
			content:  s.tempContent,
			time:     time.Now(),
		})
		s.tempNick = ""
		s.tempUserId = ""
		s.tempContent = ""
		p.askContinue(b, userId, len(s.messages))
		s.step = 4

	case 4:
		textLower := strings.ToLower(text)
		if strings.Contains(textLower, "是") || strings.Contains(textLower, "继续") || strings.Contains(textLower, "再") || text == "1" {
			p.askForNick(b, userId)
			s.step = 1
		} else if strings.Contains(textLower, "否") || strings.Contains(textLower, "不") || strings.Contains(textLower, "完成") || text == "2" {
			p.sendPreview(b, userId, s.messages)
			p.askConfirmSend(b, userId)
			s.step = 5
		} else {
			p.sendMsg(b, userId, "请回复「1」继续添加消息，或「2」完成并发送")
		}

	case 5:
		textLower := strings.ToLower(text)
		if strings.Contains(textLower, "是") || strings.Contains(textLower, "确定") || strings.Contains(textLower, "发送") || text == "1" {
			p.sendForwardMsg(b, userId, s.messages)
			delete(p.sessions, userId)
		} else if strings.Contains(textLower, "否") || strings.Contains(textLower, "取消") || text == "2" {
			p.sendMsg(b, userId, "已取消发送")
			delete(p.sessions, userId)
		} else {
			p.sendMsg(b, userId, "请回复「1」确认发送，或「2」取消")
		}
	}

	return false, nil
}

func (p *ChatRecordsMakerPlugin) sendHelp(b bot.Bot, userId message.QID) {
	builder := msgchain.Builder().Friend()
	builder.Text("━ ━ 聊天记录伪造助手 ━ ━\n\n")
	builder.Text("我将帮你创建一个伪造的合并转发聊天记录\n\n")
	builder.Text("请按以下步骤操作：\n")
	builder.Text("1️⃣ 输入第1条消息的「昵称」\n\n")
	builder.Text("当前进度：请输入昵称")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) askForUserId(b bot.Bot, userId message.QID) {
	builder := msgchain.Builder().Friend()
	builder.Text(fmt.Sprintf("昵称已设置为：%s\n\n", p.sessions[userId].tempNick))
	builder.Text("请输入该用户的「QQ号码」：")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) askForContent(b bot.Bot, userId message.QID, nick string) {
	builder := msgchain.Builder().Friend()
	builder.Text(fmt.Sprintf("QQ号已设置：%s\n\n", p.sessions[userId].tempUserId))
	builder.Text(fmt.Sprintf("请输入「%s」发送的消息内容：", nick))
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) askContinue(b bot.Bot, userId message.QID, count int) {
	builder := msgchain.Builder().Friend()
	builder.Text(fmt.Sprintf("已添加第%d条消息\n\n", count))
	builder.Text("请选择下一步操作：\n")
	builder.Text("「1」继续添加下一条消息\n")
	builder.Text("「2」完成并发送合并转发")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) askForNick(b bot.Bot, userId message.QID) {
	builder := msgchain.Builder().Friend()
	builder.Text("━ ━ 添加新消息 ━ ━\n\n")
	builder.Text("请输入下一条消息的「昵称」：")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) sendPreview(b bot.Bot, userId message.QID, messages []messageItem) {
	builder := msgchain.Builder().Friend()
	builder.Text("━ ━ 聊天记录预览 ━ ━\n\n")
	for i, m := range messages {
		builder.Text(fmt.Sprintf("%d. [%s]\n%s\n\n", i+1, m.nickname, m.content))
	}
	builder.Text("━ ━ ━ ━ ━ ━ ━ ━ ━ ━\n\n")
	builder.Text("确认发送这条合并转发吗？\n")
	builder.Text("「1」确认发送\n")
	builder.Text("「2」取消")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) askConfirmSend(b bot.Bot, userId message.QID) {
	builder := msgchain.Builder().Friend()
	builder.Text("请回复「1」确认发送，或「2」取消：")
	b.SendFriendMsg(userId, builder.Build())
}

func (p *ChatRecordsMakerPlugin) sendForwardMsg(b bot.Bot, userId message.QID, messages []messageItem) {
	chain := msgchain.Builder().FriendForward()
	for _, m := range messages {
		uid, _ := strconv.ParseUint(m.userId, 10, 64)
		friendChain := msgchain.Builder().Friend()
		friendChain.Text(m.content)
		chain.Message(message.QID(uid), m.nickname, friendChain.Build())
	}
	b.SendFriendForwardMsg(userId, chain.Build())
}

func (p *ChatRecordsMakerPlugin) sendMsg(b bot.Bot, userId message.QID, text string) {
	builder := msgchain.Builder().Friend()
	builder.Text(text)
	b.SendFriendMsg(userId, builder.Build())
}
