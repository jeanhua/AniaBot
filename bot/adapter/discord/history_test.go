package discord

import (
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

func TestMsgCachePushFind(t *testing.T) {
	c := newMsgCache(3, 10)
	c.Push("ch1", message.Message{MessageId: msgID("ch1", "m1"), RawMessage: "一"})
	c.Push("ch1", message.Message{MessageId: msgID("ch1", "m2"), RawMessage: "二"})
	m, ok := c.Find("ch1", "m1")
	if !ok || m.RawMessage != "一" {
		t.Fatalf("Find = %v %v", m, ok)
	}
	if _, ok := c.Find("ch1", "mX"); ok {
		t.Fatal("不存在的消息应未命中")
	}
	if _, ok := c.Find("chX", "m1"); ok {
		t.Fatal("其他频道应未命中")
	}
}

func TestMsgCacheEvictPerChat(t *testing.T) {
	c := newMsgCache(2, 10)
	for _, id := range []string{"1", "2", "3"} {
		c.Push("ch1", message.Message{MessageId: msgID("ch1", id)})
	}
	h := c.History("ch1", 10)
	if len(h) != 2 || h[0].MessageId != msgID("ch1", "3") {
		t.Fatalf("应淘汰最旧且最新在前: %+v", h)
	}
}

func TestMsgCacheEvictOldestChat(t *testing.T) {
	c := newMsgCache(5, 2)
	at := time.Now()
	c.now = func() time.Time { return at }
	c.Push("ch1", message.Message{MessageId: msgID("ch1", "1")})
	at = at.Add(time.Second)
	c.Push("ch2", message.Message{MessageId: msgID("ch2", "1")})
	at = at.Add(time.Second)
	c.Push("ch3", message.Message{MessageId: msgID("ch3", "1")})
	if len(c.msgs) != 2 {
		t.Fatalf("会话数 = %d", len(c.msgs))
	}
	if _, ok := c.msgs["ch1"]; ok {
		t.Fatal("最久未更新的 ch1 应被淘汰")
	}
}

func TestGetMsgDetailCacheHit(t *testing.T) {
	fake := &fakeDiscordAPI{err: errors.New("不应调用 API")}
	a := newSendAdapter(fake)
	a.msgCache.Push("c1", message.Message{MessageId: msgID("c1", "m1"), RawMessage: "缓存"})
	m, ok := a.GetMsgDetail("dc:c1:m1")
	if !ok || m.RawMessage != "缓存" {
		t.Fatalf("缓存命中失败: %v %v", m, ok)
	}
}

func TestGetMsgDetailAPIFallback(t *testing.T) {
	fake := &fakeDiscordAPI{channelMessages: []*discordgo.Message{
		{ID: "m2", ChannelID: "c1", GuildID: "g1", Content: "来自API", Timestamp: time.Now(), Author: &discordgo.User{ID: "u1"}},
	}}
	a := newSendAdapter(fake)
	m, ok := a.GetMsgDetail("dc:c1:m2")
	if !ok || m.RawMessage != "来自API" {
		t.Fatalf("API 回退失败: %v %v", m, ok)
	}
	// 应已回写缓存
	if _, ok := a.msgCache.Find("c1", "m2"); !ok {
		t.Fatal("API 结果应回写缓存")
	}
	if _, ok := a.GetMsgDetail("dc:c1:unknown"); ok {
		t.Fatal("未知消息应返回 false")
	}
	if _, ok := a.GetMsgDetail("123456"); ok {
		t.Fatal("非法 ID 应返回 false")
	}
}

func TestChannelHistoryAPIOrder(t *testing.T) {
	fake := &fakeDiscordAPI{channelMessages: []*discordgo.Message{
		{ID: "3", ChannelID: "c1", GuildID: "g1", Content: "新", Timestamp: time.Now(), Author: &discordgo.User{ID: "u"}},
		{ID: "2", ChannelID: "c1", GuildID: "g1", Content: "旧", Timestamp: time.Now(), Author: &discordgo.User{ID: "u"}},
	}}
	a := newSendAdapter(fake)
	msgs, ok := a.GetGroupMsgHistory("dc:c1", 10, 0)
	if !ok || len(*msgs) != 2 {
		t.Fatalf("历史 = %v %v", msgs, ok)
	}
	if (*msgs)[0].RawMessage != "新" || (*msgs)[1].RawMessage != "旧" {
		t.Fatal("应保持最新在前")
	}
}

func TestChannelHistoryCacheFallback(t *testing.T) {
	fake := &fakeDiscordAPI{err: errors.New("API 不可用")}
	a := newSendAdapter(fake)
	a.msgCache.Push("c1", message.Message{MessageId: msgID("c1", "m1"), RawMessage: "缓存历史"})
	msgs, ok := a.GetGroupMsgHistory("dc:c1", 10, 0)
	if !ok || len(*msgs) != 1 || (*msgs)[0].RawMessage != "缓存历史" {
		t.Fatalf("缓存兜底失败: %v %v", msgs, ok)
	}
}

func TestChannelHistoryEmpty(t *testing.T) {
	fake := &fakeDiscordAPI{err: errors.New("API 不可用")}
	a := newSendAdapter(fake)
	if _, ok := a.GetGroupMsgHistory("dc:c1", 10, 0); ok {
		t.Fatal("无 API 无缓存应返回 false")
	}
}

func TestGetFriendHistoryOpensDM(t *testing.T) {
	fake := &fakeDiscordAPI{channelMessages: []*discordgo.Message{
		{ID: "1", ChannelID: "dm-u1", Content: "dm历史", Timestamp: time.Now(), Author: &discordgo.User{ID: "u1"}},
	}}
	a := newSendAdapter(fake)
	msgs, ok := a.GetFriendMsgHistory("dc:u1", 10, 0)
	if !ok || len(*msgs) != 1 {
		t.Fatalf("私聊历史 = %v %v", msgs, ok)
	}
}

func TestGetGroupDetail(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	info, ok := a.GetGroupDetail("dc:c1")
	if !ok || info.GroupName != "general" || info.MemberCount != 42 {
		t.Fatalf("群详情 = %+v %v", info, ok)
	}
}

func TestGetGroupDetailRejectsDM(t *testing.T) {
	fake := &fakeDiscordAPI{channel: &discordgo.Channel{ID: "dm1", Type: discordgo.ChannelTypeDM}}
	a := newSendAdapter(fake)
	if _, ok := a.GetGroupDetail("dc:dm1"); ok {
		t.Fatal("DM 频道不是群，应返回 false")
	}
}

func TestHistoryCountClamp(t *testing.T) {
	fake := &fakeDiscordAPI{err: errors.New("x")}
	a := newSendAdapter(fake)
	for i := range 5 {
		a.msgCache.Push("c1", message.Message{MessageId: msgID("c1", itoa(i+1))})
	}
	// count<=0 默认 20（缓存只有 5 条，全量返回）
	msgs, ok := a.GetGroupMsgHistory("dc:c1", 0, 0)
	if !ok || len(*msgs) != 5 {
		t.Fatalf("默认条数 = %v", msgs)
	}
}
