package discord

import (
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// ---------- 通知事件映射 ----------

// onMessageDelete 消息删除 → 撤回通知。
// Discord 不告知删除者（OperatorId 恒空），消息作者从内存缓存反查（无缓存则空）。
func (a *discordAdapter) onMessageDelete(s *discordgo.Session, e *discordgo.MessageDelete) {
	if e.Message == nil {
		return
	}
	trig := a.triggerOf()
	now := uint(time.Now().Unix())
	mid := msgID(e.ChannelID, e.ID)
	var authorID message.QID
	if cached, ok := a.msgCache.Find(e.ChannelID, e.ID); ok {
		authorID = cached.UserId
	}
	if e.GuildID == "" {
		if trig.OnFriendRecall == nil {
			return
		}
		n := message.FriendRecallNotice{UserId: authorID, MessageId: mid}
		n.Time = now
		n.PostType = "notice"
		n.NoticeType = "friend_recall"
		n.SelfId = a.SelfID()
		n.SetPlatform(Platform)
		trig.OnFriendRecall(n)
		return
	}
	if trig.OnGroupRecall == nil {
		return
	}
	n := message.GroupRecallNotice{
		GroupId:   message.QID(idPrefix + e.ChannelID),
		UserId:    authorID,
		MessageId: mid,
	}
	n.Time = now
	n.PostType = "notice"
	n.NoticeType = "group_recall"
	n.SelfId = a.SelfID()
	n.SetPlatform(Platform)
	trig.OnGroupRecall(n)
}

// onReactionAdd 表情回应 → 群消息表情回应通知（仅 guild；DM 反应无对应公共通知，
// 与 Telegram 私聊回应不投递一致）。EmojiId 取 Unicode 字符或自定义表情 ID。
func (a *discordAdapter) onReactionAdd(s *discordgo.Session, e *discordgo.MessageReactionAdd) {
	if e.MessageReaction == nil || e.GuildID == "" || e.UserID == "" {
		return
	}
	trig := a.triggerOf()
	if trig.OnGroupMsgEmojiLike == nil {
		return
	}
	emojiID := e.Emoji.Name
	if emojiID == "" {
		emojiID = e.Emoji.ID // 自定义表情
	}
	if emojiID == "" {
		return
	}
	n := message.GroupMsgEmojiLikeNotice{
		GroupId:   message.QID(idPrefix + e.ChannelID),
		UserId:    userQID(e.UserID),
		MessageId: msgID(e.ChannelID, e.MessageID),
	}
	n.Likes = append(n.Likes, struct {
		EmojiId string `json:"emoji_id"`
		Count   int    `json:"count"`
	}{EmojiId: emojiID, Count: 1})
	n.Time = uint(time.Now().Unix())
	n.PostType = "notice"
	n.NoticeType = "group_msg_emoji_like"
	n.SelfId = a.SelfID()
	n.SetPlatform(Platform)
	trig.OnGroupMsgEmojiLike(n)
}

func (a *discordAdapter) emitPlatformEvent(eventType string, data any) {
	trig := a.triggerOf()
	if trig.OnPlatformEvent == nil {
		return
	}
	trig.OnPlatformEvent(message.PlatformEvent{
		Platform: Platform,
		Type:     eventType,
		Data:     data,
	})
}
