package groupnewsletter

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

type groupMessageBuffer struct {
	messages  []collectedMessage
	persisted int
	mu        sync.RWMutex
}

type collectedMessage struct {
	Time     int64       `json:"time"`
	UserId   message.QID `json:"user_id"`
	Nickname string      `json:"nickname"`
	Content  string      `json:"content"`
}

func (p *GroupNewsletter) collectMessage(_ context.Context, b bot.Bot, msg message.Message) {
	p.msgsMu.Lock()
	if _, ok := p.groupMsgs[msg.GroupId]; !ok {
		p.groupMsgs[msg.GroupId] = &groupMessageBuffer{
			messages:  make([]collectedMessage, 0),
			persisted: 0,
		}
	}
	buffer := p.groupMsgs[msg.GroupId]
	p.msgsMu.Unlock()

	content := buildContent(b, msg)
	nickname := msg.Sender.Card
	if nickname == "" {
		nickname = msg.Sender.Nickname
	}

	collected := collectedMessage{
		Time:     time.Now().Unix(),
		UserId:   msg.Sender.UserId,
		Nickname: nickname,
		Content:  content,
	}

	buffer.mu.Lock()
	buffer.messages = append(buffer.messages, collected)
	if len(buffer.messages) > p.config.maxMessages {
		buffer.messages = buffer.messages[len(buffer.messages)-p.config.maxMessages:]
		if buffer.persisted > len(buffer.messages) {
			buffer.persisted = len(buffer.messages)
		}
	}
	count := len(buffer.messages)
	buffer.mu.Unlock()

	// 异步持久化，避免阻塞消息事件处理
	select {
	case p.saveChan <- msg.GroupId:
	default:
		// channel 满时放弃本次持久化，下次消息到来时会再次尝试
	}

	// 达到阈值且未在生成中，触发生成通知
	if count >= p.config.msgThreshold && !p.isGenerating(msg.GroupId) {
		select {
		case p.notifyChan <- msg.GroupId:
		default:
		}
	}
}

func (p *GroupNewsletter) getMessageCount(groupId message.QID) int {
	p.msgsMu.RLock()
	buffer, ok := p.groupMsgs[groupId]
	p.msgsMu.RUnlock()

	if !ok {
		return 0
	}
	buffer.mu.RLock()
	defer buffer.mu.RUnlock()
	return len(buffer.messages)
}

// buildContent 将消息链拼接为可读文本
func buildContent(b bot.Bot, msg message.Message) string {
	var sb strings.Builder
	for _, m := range msg.Message {
		sb.WriteString(m.FriendlyText(
			message.WithGetGroupUserInfo(msg.GroupId, func(groupId, userId message.QID) (*message.GroupUserInfo, bool) {
				return b.GetGroupUserInfo(groupId, userId)
			}),
			message.WithGetForwardMsgFunc(b.GetForwardMsg),
			message.WithGetMsgFunc(b.GetMsgDetail),
		))
	}
	return sb.String()
}

// isGenerating 检查某个群是否正在生成群刊
func (p *GroupNewsletter) isGenerating(groupId message.QID) bool {
	p.generateMu.Lock()
	defer p.generateMu.Unlock()
	_, ok := p.generating[groupId]
	return ok
}

// trySetGenerating 尝试标记某个群为生成中，返回 false 表示已在生成
func (p *GroupNewsletter) trySetGenerating(groupId message.QID) bool {
	p.generateMu.Lock()
	defer p.generateMu.Unlock()
	if _, ok := p.generating[groupId]; ok {
		return false
	}
	p.generating[groupId] = struct{}{}
	return true
}

func (p *GroupNewsletter) clearGenerating(groupId message.QID) {
	p.generateMu.Lock()
	defer p.generateMu.Unlock()
	delete(p.generating, groupId)
}
