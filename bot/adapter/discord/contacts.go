package discord

import (
	"sort"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// stateOf 返回当前会话的网关 State（未连接时为 nil）。
func (a *discordAdapter) stateOf() *discordgo.State {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.session == nil {
		return nil
	}
	return a.session.State
}

// GetGroupList 实现 adapter.ContactsExt：从网关 State 缓存列出机器人所在
// 服务器的文字/公告频道（Discord 的「群」寻址单元是频道而非服务器，
// SendGroupMsg 的目标即频道 ID）。成员数取所属服务器的 MemberCount。
// 网关未就绪（State 尚无服务器）时返回 false。
func (a *discordAdapter) GetGroupList() (*[]message.GroupInfo, bool) {
	st := a.stateOf()
	if st == nil {
		return nil, false
	}
	st.RLock()
	defer st.RUnlock()
	if len(st.Guilds) == 0 {
		return nil, false
	}
	out := make([]message.GroupInfo, 0, 64)
	for _, g := range st.Guilds {
		if g == nil {
			continue
		}
		for _, ch := range g.Channels {
			if ch == nil || (ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews) {
				continue
			}
			out = append(out, message.GroupInfo{
				GroupID:     message.QID(idPrefix + ch.ID),
				GroupName:   g.Name + " / #" + ch.Name,
				MemberCount: g.MemberCount,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GroupName < out[j].GroupName })
	return &out, true
}

// GetFriendList 实现 adapter.ContactsExt：best-effort——Discord 无枚举私聊
// 对端的 API，仅列出网关 State 中已知 DM 频道的对端用户（一般覆盖本次运行
// 以来有过来往的会话，不完整属正常现象）。
func (a *discordAdapter) GetFriendList() (*[]message.Friend, bool) {
	st := a.stateOf()
	if st == nil {
		return nil, false
	}
	a.mu.Lock()
	selfID := a.selfID
	a.mu.Unlock()
	st.RLock()
	defer st.RUnlock()
	out := make([]message.Friend, 0, len(st.PrivateChannels))
	for _, ch := range st.PrivateChannels {
		if ch == nil || ch.Type != discordgo.ChannelTypeDM {
			continue
		}
		for _, u := range ch.Recipients {
			if u == nil || u.ID == selfID {
				continue
			}
			nick := u.GlobalName
			if nick == "" {
				nick = u.Username
			}
			out = append(out, message.Friend{UserID: userQID(u.ID), Nickname: nick})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Nickname < out[j].Nickname })
	return &out, true
}
