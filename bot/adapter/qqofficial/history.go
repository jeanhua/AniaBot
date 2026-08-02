package qqofficial

import (
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

// msgCache 入站/出站消息内存缓存。QQ 官方 v2 群聊/单聊场景无单条消息查询与
// 历史端点，GetMsgDetail / GetGroupMsgHistory / GetFriendMsgHistory 由它兜底
// （仅覆盖适配器存活期间的消息；重启后 AI 会话历史仍由 PersistentStorage 承载）。
type msgCache struct {
	mu       sync.Mutex
	msgs     map[string]msgCacheEntry // conversation(原始 openid) -> 消息列表（最新在前）
	idIndex  map[string]string        // 原始消息 ID -> conversation（GetMsgDetail 反查）
	perChat  int
	maxChats int
	now      func() time.Time // 时钟（测试注入；nil 时用 time.Now）
}

type msgCacheEntry struct {
	msgs     []message.Message
	lastPush time.Time
}

func newMsgCache(perChat, maxChats int) *msgCache {
	return &msgCache{
		msgs:    map[string]msgCacheEntry{},
		idIndex: map[string]string{},
		perChat: perChat, maxChats: maxChats, now: time.Now,
	}
}

// Push 记录一条消息；会话列表超上限时淘汰最旧，会话数超上限时淘汰最久未更新的会话。
func (c *msgCache) Push(conversation string, m message.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.msgs[conversation]
	e.msgs = append([]message.Message{m}, e.msgs...)
	if len(e.msgs) > c.perChat {
		// 淘汰的消息同步清掉 ID 索引
		for _, old := range e.msgs[c.perChat:] {
			delete(c.idIndex, old.MessageId.TrimPrefix(idPrefix))
		}
		e.msgs = e.msgs[:c.perChat]
	}
	e.lastPush = c.now()
	c.msgs[conversation] = e
	if raw := m.MessageId.TrimPrefix(idPrefix); raw != "" {
		c.idIndex[raw] = conversation
	}
	if len(c.msgs) > c.maxChats {
		var oldest string
		var oldestAt time.Time
		for k, v := range c.msgs {
			if oldest == "" || v.lastPush.Before(oldestAt) {
				oldest, oldestAt = k, v.lastPush
			}
		}
		for _, old := range c.msgs[oldest].msgs {
			delete(c.idIndex, old.MessageId.TrimPrefix(idPrefix))
		}
		delete(c.msgs, oldest)
	}
}

// Find 按原始消息 ID 查找（官方消息 ID 全局唯一，经 idIndex 反查会话）。
func (c *msgCache) Find(rawMsgID string) (*message.Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	conversation, ok := c.idIndex[rawMsgID]
	if !ok {
		return nil, false
	}
	for _, m := range c.msgs[conversation].msgs {
		if m.MessageId.TrimPrefix(idPrefix) == rawMsgID {
			mm := m
			return &mm, true
		}
	}
	return nil, false
}

// History 返回会话最近 count 条消息（最新在前）。
func (c *msgCache) History(conversation string, count int) []message.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.msgs[conversation].msgs
	if count <= 0 || count > len(list) {
		count = len(list)
	}
	out := make([]message.Message, 0, count)
	out = append(out, list[:count]...)
	return out
}

// ---------- 查询 ----------

// GetMsgDetail 获取消息详情：官方无单条消息查询端点，查内存缓存，未命中返回 false。
func (a *qqOfficialAdapter) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	raw, ok := parseOpenID(msgId)
	if !ok {
		return nil, false
	}
	return a.msgCache.Find(raw)
}

// GetGroupDetail 获取群详情：官方 v2 群聊场景无群资料查询接口，不可用。
func (a *qqOfficialAdapter) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	return nil, false
}

// GetGroupMsgHistory 获取群消息历史：官方无历史端点，返回内存缓存中
// 该会话的最近消息；缓存未命中返回 false。
func (a *qqOfficialAdapter) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	openid, ok := parseOpenID(groupId)
	if !ok {
		return nil, false
	}
	return a.historyFromCache(openid, count)
}

// GetFriendMsgHistory 获取私聊消息历史：同 GetGroupMsgHistory，走内存缓存。
func (a *qqOfficialAdapter) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	openid, ok := parseOpenID(userId)
	if !ok {
		return nil, false
	}
	return a.historyFromCache(openid, count)
}

func (a *qqOfficialAdapter) historyFromCache(openid string, count int) (*[]message.Message, bool) {
	if count <= 0 {
		count = 20
	}
	msgs := a.msgCache.History(openid, count)
	if len(msgs) == 0 {
		return nil, false
	}
	return &msgs, true
}
