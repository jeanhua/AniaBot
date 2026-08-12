package discord

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

// newStateAdapter 构造注入网关 State 的适配器（通讯录列表纯 State 映射，无需连接）。
func newStateAdapter(t *testing.T) *discordAdapter {
	t.Helper()
	s, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("discordgo.New: %v", err)
	}
	a := NewAdapter(nil)
	a.session = s
	return a
}

func TestGetGroupListFromState(t *testing.T) {
	a := newStateAdapter(t)
	a.session.State.Guilds = []*discordgo.Guild{
		{
			ID:          "g2",
			Name:        "Beta 服",
			MemberCount: 7,
			Channels: []*discordgo.Channel{
				{ID: "c3", Name: "general", Type: discordgo.ChannelTypeGuildText},
				{ID: "c4", Name: "secret", Type: discordgo.ChannelTypeGuildNews},
			},
		},
		{
			ID:          "g1",
			Name:        "Alpha 服",
			MemberCount: 42,
			Channels: []*discordgo.Channel{
				{ID: "c1", Name: "chat", Type: discordgo.ChannelTypeGuildText},
				{ID: "c2", Name: "voice", Type: discordgo.ChannelTypeGuildVoice}, // 语音频道应被过滤
				nil, // 防御 nil 条目
			},
		},
	}
	groups, ok := a.GetGroupList()
	if !ok || groups == nil {
		t.Fatalf("GetGroupList ok = %v", ok)
	}
	if len(*groups) != 3 {
		t.Fatalf("应列出 3 个文字/公告频道，实际 %d", len(*groups))
	}
	// 按名称排序：Alpha 服 / #chat 在最前
	first := (*groups)[0]
	if first.GroupID != "dc:c1" || first.GroupName != "Alpha 服 / #chat" || first.MemberCount != 42 {
		t.Fatalf("首条 = %+v", first)
	}
	for _, g := range *groups {
		if g.GroupID == "dc:c2" {
			t.Fatal("语音频道不应出现")
		}
	}
}

func TestGetGroupListNotReady(t *testing.T) {
	a := newStateAdapter(t)
	if _, ok := a.GetGroupList(); ok {
		t.Fatal("State 无服务器（网关未就绪）应返回 false")
	}
	a2 := NewAdapter(nil) // session 为 nil
	if _, ok := a2.GetGroupList(); ok {
		t.Fatal("未连接应返回 false")
	}
}

func TestGetFriendListFromDMChannels(t *testing.T) {
	a := newStateAdapter(t)
	a.selfID = "bot1"
	a.session.State.PrivateChannels = []*discordgo.Channel{
		{
			ID:   "dm1",
			Type: discordgo.ChannelTypeDM,
			Recipients: []*discordgo.User{
				{ID: "bot1", Username: "机器人"}, // 自身应被排除
				{ID: "u1", Username: "alice", GlobalName: "爱丽丝"},
			},
		},
		{
			ID:   "dm2",
			Type: discordgo.ChannelTypeDM,
			Recipients: []*discordgo.User{
				{ID: "u2", Username: "bob"}, // 无 GlobalName 时回退 Username
			},
		},
		{ID: "dm3", Type: discordgo.ChannelTypeGroupDM}, // 群 DM 应被过滤
	}
	friends, ok := a.GetFriendList()
	if !ok || friends == nil {
		t.Fatalf("GetFriendList ok = %v", ok)
	}
	if len(*friends) != 2 {
		t.Fatalf("应列出 2 个私聊对端，实际 %d", len(*friends))
	}
	byID := map[string]string{}
	for _, f := range *friends {
		byID[f.UserID.String()] = f.Nickname
	}
	if byID["dc:u1"] != "爱丽丝" || byID["dc:u2"] != "bob" {
		t.Fatalf("friends = %+v", *friends)
	}
}

func TestGetFriendListEmptyWhenNoDM(t *testing.T) {
	a := newStateAdapter(t)
	friends, ok := a.GetFriendList()
	if !ok || friends == nil || len(*friends) != 0 {
		t.Fatalf("无私聊时应返回空列表：ok=%v friends=%v", ok, friends)
	}
	if _, ok := NewAdapter(nil).GetFriendList(); ok {
		t.Fatal("未连接应返回 false")
	}
}
