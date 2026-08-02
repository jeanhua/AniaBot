package telegram

import (
	"context"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
)

const (
	// msgCachePerChat 每个会话缓存的最近消息数上限（超出淘汰最旧）。
	msgCachePerChat = 200
	// msgCacheMaxChats 缓存会话数上限（超出淘汰最久未更新的会话）。
	msgCacheMaxChats = 500
)

// msgCache 入站/出站消息内存缓存。Telegram Bot API 无单条消息查询与历史端点，
// GetMsgDetail / GetGroupMsgHistory / GetFriendMsgHistory 由它兜底（仅覆盖
// 适配器存活期间的消息；重启后 AI 会话历史仍由 PersistentStorage 承载）。
type msgCache struct {
	mu       sync.Mutex
	msgs     map[string]msgCacheEntry // chatID(原始) -> 消息列表（最新在前）
	perChat  int                      // 每会话消息数上限
	maxChats int                      // 会话数上限
}

type msgCacheEntry struct {
	msgs     []message.Message
	lastPush time.Time
}

func newMsgCache(perChat, maxChats int) *msgCache {
	return &msgCache{msgs: map[string]msgCacheEntry{}, perChat: perChat, maxChats: maxChats}
}

// Push 记录一条消息；会话列表超上限时淘汰最旧，会话数超上限时淘汰最久未更新的会话。
func (c *msgCache) Push(chatID string, m message.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.msgs[chatID]
	e.msgs = append([]message.Message{m}, e.msgs...)
	if len(e.msgs) > c.perChat {
		e.msgs = e.msgs[:c.perChat]
	}
	e.lastPush = time.Now()
	c.msgs[chatID] = e
	if len(c.msgs) > c.maxChats {
		var oldest string
		var oldestAt time.Time
		for k, v := range c.msgs {
			if oldest == "" || v.lastPush.Before(oldestAt) {
				oldest, oldestAt = k, v.lastPush
			}
		}
		delete(c.msgs, oldest)
	}
}

// Find 按会话与消息 ID 查找（消息 ID 仅同一会话内唯一）。
func (c *msgCache) Find(chatID string, messageID int) (*message.Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs[chatID].msgs {
		if _, mid, ok := parseMsgID(m.MessageId.String()); ok && mid == messageID {
			mm := m
			return &mm, true
		}
	}
	return nil, false
}

// History 返回会话最近 count 条消息（最新在前）。
func (c *msgCache) History(chatID string, count int) []message.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.msgs[chatID].msgs
	if count <= 0 || count > len(list) {
		count = len(list)
	}
	out := make([]message.Message, 0, count)
	out = append(out, list[:count]...)
	return out
}

// ---------- 查询 ----------

// GetMsgDetail 获取消息详情：解析 "tg:<chat>:<msgid>" 后查内存缓存。
// Bot API 无单条消息查询端点，缓存未命中返回 false。
func (a *telegramAdapter) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	chatID, messageID, ok := parseMsgID(msgId.String())
	if !ok {
		return nil, false
	}
	return a.msgCache.Find(chatIDRaw(chatID), messageID)
}

// GetGroupDetail 获取群聊详情：getChat → title/member_count。
func (a *telegramAdapter) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	if a.client == nil {
		return nil, false
	}
	chatID, ok := parseChatID(groupId)
	if !ok {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var res Chat
	if err := a.client.call(ctx, "getChat", map[string]any{"chat_id": chatID}, &res); err != nil {
		a.logger.Debug("Telegram getChat 失败", "chatId", chatID, "error", err)
		return nil, false
	}
	return &message.GroupInfo{GroupID: groupId, GroupName: res.Title, MemberCount: res.MemberCount}, true
}

// GetGroupMsgHistory 获取群聊消息历史：Bot API 无历史端点，返回内存缓存中
// 该会话的最近消息；缓存未命中返回 false。
func (a *telegramAdapter) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	chatID, ok := parseChatID(groupId)
	if !ok {
		return nil, false
	}
	return a.historyFromCache(chatIDRaw(chatID), count)
}

// GetFriendMsgHistory 获取私聊消息历史：同 GetGroupMsgHistory，走内存缓存。
func (a *telegramAdapter) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	chatID, ok := parseChatID(userId)
	if !ok {
		return nil, false
	}
	return a.historyFromCache(chatIDRaw(chatID), count)
}

func (a *telegramAdapter) historyFromCache(chatID string, count int) (*[]message.Message, bool) {
	if count <= 0 {
		count = 20
	}
	msgs := a.msgCache.History(chatID, count)
	if len(msgs) == 0 {
		return nil, false
	}
	return &msgs, true
}
