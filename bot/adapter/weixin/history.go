package weixin

import (
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
)

const (
	// msgCachePerUser 每个用户缓存的最近消息数上限（超出淘汰最旧）。
	msgCachePerUser = 200
	// msgCacheMaxUsers 缓存用户数上限（超出淘汰最久未更新的用户）。
	msgCacheMaxUsers = 500
)

// msgCache 入站/出站消息内存缓存。iLink Bot API 无消息查询与历史端点，
// GetMsgDetail / GetFriendMsgHistory 由它兜底（仅覆盖适配器存活期间的消息；
// 重启后 AI 会话历史仍由 PersistentStorage 承载）。
type msgCache struct {
	mu       sync.Mutex
	msgs     map[string]msgCacheEntry // 用户原始 ID -> 消息列表（最新在前）
	perUser  int
	maxUsers int
	now      func() time.Time // 时钟（测试注入；nil 时用 time.Now）
}

type msgCacheEntry struct {
	msgs     []message.Message
	lastPush time.Time
}

func newMsgCache(perUser, maxUsers int) *msgCache {
	return &msgCache{msgs: map[string]msgCacheEntry{}, perUser: perUser, maxUsers: maxUsers, now: time.Now}
}

// Push 记录一条消息；用户列表超上限时淘汰最旧，用户数超上限时淘汰最久未更新的用户。
func (c *msgCache) Push(userID string, m message.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.msgs[userID]
	e.msgs = append([]message.Message{m}, e.msgs...)
	if len(e.msgs) > c.perUser {
		e.msgs = e.msgs[:c.perUser]
	}
	e.lastPush = c.now()
	c.msgs[userID] = e
	if len(c.msgs) > c.maxUsers {
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

// Find 按用户与消息 ID 查找。
func (c *msgCache) Find(userID, mid string) (*message.Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.msgs[userID].msgs {
		m := &c.msgs[userID].msgs[i]
		if _, id, ok := parseFrameMsgID(m.MessageId.String()); ok && id == mid {
			return m, true
		}
	}
	return nil, false
}

// History 返回用户最近 count 条消息（最新在前）。
func (c *msgCache) History(userID string, count int) []message.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.msgs[userID].msgs
	if count <= 0 || count > len(list) {
		count = len(list)
	}
	out := make([]message.Message, 0, count)
	out = append(out, list[:count]...)
	return out
}

// ---------- 查询（GetMsg 接口） ----------

// GetMsgDetail 获取消息详情：解析 "wx:<user>:<mid>" 后查内存缓存。
func (a *weixinAdapter) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	userID, mid, ok := parseFrameMsgID(msgId.String())
	if !ok {
		return nil, false
	}
	return a.msgCache.Find(userID, mid)
}

// GetGroupDetail 微信无群聊会话概念，返回 false。
func (a *weixinAdapter) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	return nil, false
}

// GetGroupMsgHistory 微信无群聊会话概念，返回 false。
func (a *weixinAdapter) GetGroupMsgHistory(groupId message.QID, count int, messageSeq int) (*[]message.Message, bool) {
	return nil, false
}

// GetFriendMsgHistory 获取私聊消息历史：返回内存缓存中该用户的最近消息。
func (a *weixinAdapter) GetFriendMsgHistory(userId message.QID, count int, messageSeq int) (*[]message.Message, bool) {
	raw := userId.TrimPrefix(idPrefix)
	if raw == userId.String() {
		return nil, false
	}
	if count <= 0 {
		count = 20
	}
	msgs := a.msgCache.History(raw, count)
	if len(msgs) == 0 {
		return nil, false
	}
	return &msgs, true
}
