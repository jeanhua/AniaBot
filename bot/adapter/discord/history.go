package discord

import (
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

const (
	// msgCachePerChat 每个会话缓存的最近消息数上限（超出淘汰最旧）。
	msgCachePerChat = 200
	// msgCacheMaxChats 缓存会话数上限（超出淘汰最久未更新的会话）。
	msgCacheMaxChats = 500
	// historyMaxCount Discord 历史端点单次最大拉取条数。
	historyMaxCount = 100
)

// msgCache 入站/出站消息内存缓存：GetMsgDetail 快路径、撤回通知作者反查、
// 历史 API 失败时的兜底（仅覆盖适配器存活期间的消息；重启后 AI 会话历史
// 仍由 PersistentStorage 承载）。
type msgCache struct {
	mu       sync.Mutex
	msgs     map[string]msgCacheEntry // channelID(原始) -> 消息列表（最新在前）
	perChat  int
	maxChats int
	now      func() time.Time // 时钟（测试注入）
}

type msgCacheEntry struct {
	msgs     []message.Message
	lastPush time.Time
}

func newMsgCache(perChat, maxChats int) *msgCache {
	return &msgCache{msgs: map[string]msgCacheEntry{}, perChat: perChat, maxChats: maxChats, now: time.Now}
}

// Push 记录一条消息；会话列表超上限时淘汰最旧，会话数超上限时淘汰最久未更新的会话。
func (c *msgCache) Push(channelID string, m message.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.msgs[channelID]
	e.msgs = append([]message.Message{m}, e.msgs...)
	if len(e.msgs) > c.perChat {
		e.msgs = e.msgs[:c.perChat]
	}
	e.lastPush = c.now()
	c.msgs[channelID] = e
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

// Find 按频道与消息 ID 查找。
func (c *msgCache) Find(channelID, messageID string) (*message.Message, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, m := range c.msgs[channelID].msgs {
		if _, mid, ok := parseMsgID(m.MessageId.String()); ok && mid == messageID {
			mm := m
			return &mm, true
		}
	}
	return nil, false
}

// History 返回会话最近 count 条消息（最新在前）。
func (c *msgCache) History(channelID string, count int) []message.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.msgs[channelID].msgs
	if count <= 0 || count > len(list) {
		count = len(list)
	}
	out := make([]message.Message, 0, count)
	out = append(out, list[:count]...)
	return out
}

// ---------- 查询 ----------

// GetMsgDetail 获取消息详情：解析 "dc:<channel>:<msgid>" 后先查内存缓存，
// 未命中走 ChannelMessage API（Discord 有单条消息查询端点，与 Telegram 不同）。
func (a *discordAdapter) GetMsgDetail(msgId message.QID) (*message.Message, bool) {
	channelID, messageID, ok := parseMsgID(msgId.String())
	if !ok {
		return nil, false
	}
	if m, ok := a.msgCache.Find(channelID, messageID); ok {
		return m, true
	}
	if a.api == nil {
		return nil, false
	}
	dm, err := a.api.ChannelMessage(channelID, messageID)
	if err != nil || dm.Author == nil {
		a.logger.Debug("Discord ChannelMessage 失败", "channelId", channelID, "messageId", messageID, "error", err)
		return nil, false
	}
	m := a.translateMessage(dm)
	if m == nil {
		return nil, false
	}
	a.msgCache.Push(channelID, *m)
	return m, true
}

// GetGroupDetail 获取群聊详情：Channel + GuildWithCounts（with_counts=true 为
// REST 参数，无需 Server Members 特权意图）。DM 频道不是"群"，返回 false。
func (a *discordAdapter) GetGroupDetail(groupId message.QID) (*message.GroupInfo, bool) {
	if a.api == nil {
		return nil, false
	}
	channelID, ok := parseChannelID(groupId)
	if !ok {
		return nil, false
	}
	ch, err := a.api.Channel(channelID)
	if err != nil {
		a.logger.Debug("Discord Channel 失败", "channelId", channelID, "error", err)
		return nil, false
	}
	if ch.Type == discordgo.ChannelTypeDM || ch.Type == discordgo.ChannelTypeGroupDM || ch.GuildID == "" {
		return nil, false
	}
	info := &message.GroupInfo{GroupID: groupId, GroupName: ch.Name}
	g, err := a.api.GuildWithCounts(ch.GuildID)
	if err != nil {
		a.logger.Debug("Discord GuildWithCounts 失败", "guildId", ch.GuildID, "error", err)
		return info, true // 频道信息可用，成员数缺失不致命
	}
	if info.GroupName == "" {
		info.GroupName = g.Name
	}
	info.MemberCount = g.ApproximateMemberCount
	return info, true
}

// GetGroupMsgHistory 获取群聊消息历史：ChannelMessages API（最新在前），
// 失败时回退内存缓存。message_seq 为 OneBot 数字序号语义，无法寻址 snowflake，忽略。
func (a *discordAdapter) GetGroupMsgHistory(groupId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	channelID, ok := parseChannelID(groupId)
	if !ok {
		return nil, false
	}
	return a.channelHistory(channelID, count)
}

// GetFriendMsgHistory 获取私聊消息历史：先经 UserChannelCreate 解析 DM 频道再拉取。
func (a *discordAdapter) GetFriendMsgHistory(userId message.QID, count int, message_seq int) (*[]message.Message, bool) {
	if a.api == nil {
		return nil, false
	}
	rawUserID, ok := parseChannelID(userId)
	if !ok {
		return nil, false
	}
	dm, err := a.api.UserChannelCreate(rawUserID)
	if err != nil {
		a.logger.Debug("Discord UserChannelCreate 失败", "userId", rawUserID, "error", err)
		return nil, false
	}
	return a.channelHistory(dm.ID, count)
}

// channelHistory 频道历史：API 优先（count 收敛到 1-100，默认 20，最新在前），
// 翻译为空的消息跳过；API 失败回退内存缓存。
func (a *discordAdapter) channelHistory(channelID string, count int) (*[]message.Message, bool) {
	if count <= 0 {
		count = 20
	}
	if count > historyMaxCount {
		count = historyMaxCount
	}
	if a.api != nil {
		list, err := a.api.ChannelMessages(channelID, count, "", "", "")
		if err == nil {
			msgs := make([]message.Message, 0, len(list))
			for _, dm := range list {
				if dm.Author == nil {
					continue
				}
				if m := a.translateMessage(dm); m != nil {
					msgs = append(msgs, *m)
				}
			}
			if len(msgs) > 0 {
				return &msgs, true
			}
		} else {
			a.logger.Debug("Discord ChannelMessages 失败，回退内存缓存", "channelId", channelID, "error", err)
		}
	}
	msgs := a.msgCache.History(channelID, count)
	if len(msgs) == 0 {
		return nil, false
	}
	return &msgs, true
}
