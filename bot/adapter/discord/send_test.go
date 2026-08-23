package discord

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// fakeDiscordAPI 记录调用的 discordAPI 假实现（无网络）。
type fakeDiscordAPI struct {
	sends       []*discordgo.MessageSend // ChannelMessageSendComplex 收到的载荷
	sendChannel []string
	edits       []string // ChannelMessageEdit 内容
	nextMsgID   int

	dmChannelID string // UserChannelCreate 返回的 DM 频道
	err         error  // 非 nil 时所有调用失败

	channelMessages []*discordgo.Message // ChannelMessages 返回
	channel         *discordgo.Channel
	guild           *discordgo.Guild
	auditLog        *discordgo.GuildAuditLog // GuildAuditLog 返回
	auditLogCalls   int
}

func (f *fakeDiscordAPI) send(channelID string, data *discordgo.MessageSend) (*discordgo.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.nextMsgID++
	f.sends = append(f.sends, data)
	f.sendChannel = append(f.sendChannel, channelID)
	return &discordgo.Message{ID: itoa(f.nextMsgID), ChannelID: channelID}, nil
}

func (f *fakeDiscordAPI) ChannelMessageSend(channelID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	return f.send(channelID, &discordgo.MessageSend{Content: content})
}

func (f *fakeDiscordAPI) ChannelMessageSendComplex(channelID string, data *discordgo.MessageSend, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	return f.send(channelID, data)
}

func (f *fakeDiscordAPI) ChannelMessageEdit(channelID, messageID, content string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.edits = append(f.edits, content)
	return &discordgo.Message{ID: messageID, ChannelID: channelID}, nil
}

func (f *fakeDiscordAPI) UserChannelCreate(recipientID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.dmChannelID == "" {
		f.dmChannelID = "dm-" + recipientID
	}
	return &discordgo.Channel{ID: f.dmChannelID, Type: discordgo.ChannelTypeDM}, nil
}

func (f *fakeDiscordAPI) ChannelMessage(channelID, messageID string, options ...discordgo.RequestOption) (*discordgo.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, m := range f.channelMessages {
		if m.ID == messageID {
			return m, nil
		}
	}
	return nil, errors.New("unknown message")
}

func (f *fakeDiscordAPI) ChannelMessages(channelID string, limit int, beforeID, afterID, aroundID string, options ...discordgo.RequestOption) ([]*discordgo.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.channelMessages, nil
}

func (f *fakeDiscordAPI) Channel(channelID string, options ...discordgo.RequestOption) (*discordgo.Channel, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.channel != nil {
		return f.channel, nil
	}
	return &discordgo.Channel{ID: channelID, GuildID: "g1", Name: "general", Type: discordgo.ChannelTypeGuildText}, nil
}

func (f *fakeDiscordAPI) GuildWithCounts(guildID string, options ...discordgo.RequestOption) (*discordgo.Guild, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.guild != nil {
		return f.guild, nil
	}
	return &discordgo.Guild{ID: guildID, Name: "guild", ApproximateMemberCount: 42}, nil
}

func (f *fakeDiscordAPI) GuildAuditLog(guildID, userID, beforeID string, actionType, limit int, options ...discordgo.RequestOption) (*discordgo.GuildAuditLog, error) {
	f.auditLogCalls++
	if f.err != nil {
		return nil, f.err
	}
	if f.auditLog != nil {
		return f.auditLog, nil
	}
	return &discordgo.GuildAuditLog{}, nil
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [8]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	return string(b[p:])
}

// segsChain 同时满足 GroupChain/FriendChain 的测试消息链。
type segsChain []message.OB11Segment

func (c segsChain) GetGroupMsg() []message.OB11Segment  { return c }
func (c segsChain) GetFriendMsg() []message.OB11Segment { return c }

func textSeg(t string) message.OB11Segment {
	return message.OB11Segment{Type: message.SegmentText, Data: message.TextMessage{Text: t}.Marshal()}
}

func atSeg(qq string) message.OB11Segment {
	return message.OB11Segment{Type: message.SegmentMention, Data: message.MentionMessage{QQ: message.QID(qq)}.Marshal()}
}

func replySeg(id string) message.OB11Segment {
	return message.OB11Segment{Type: message.SegmentReply, Data: message.ReplyMessage{Id: message.QID(id)}.Marshal()}
}

func newSendAdapter(api discordAPI) *discordAdapter {
	a := NewAdapter(nil)
	a.api = api
	a.selfID = "999"
	return a
}

func TestSendTextOnly(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	id, ok := a.SendGroupMsg("dc:c1", segsChain{textSeg("你好")})
	if !ok || id != "dc:c1:1" {
		t.Fatalf("id = %q ok = %v", id, ok)
	}
	if len(fake.sends) != 1 || fake.sends[0].Content != "你好" {
		t.Fatalf("sends = %+v", fake.sends)
	}
	if fake.sendChannel[0] != "c1" {
		t.Fatalf("channel = %q", fake.sendChannel[0])
	}
}

func TestSendRejectsForeignPrefix(t *testing.T) {
	a := newSendAdapter(&fakeDiscordAPI{})
	if _, ok := a.SendGroupMsg("123456", segsChain{textSeg("x")}); ok {
		t.Fatal("QQ 裸数字 ID 应拒绝")
	}
	if _, ok := a.SendGroupMsg("tg:c1", segsChain{textSeg("x")}); ok {
		t.Fatal("其他平台前缀应拒绝")
	}
}

func TestSendTextSplit(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	long := strings.Repeat("测", 2500) // 2500 rune > 1990
	_, ok := a.SendGroupMsg("dc:c1", segsChain{textSeg(long)})
	if !ok {
		t.Fatal("发送失败")
	}
	if len(fake.sends) != 2 {
		t.Fatalf("应分 2 包，实际 %d", len(fake.sends))
	}
	if len([]rune(fake.sends[0].Content)) != maxTextLen {
		t.Fatalf("首包长度 = %d", len([]rune(fake.sends[0].Content)))
	}
	if got := len([]rune(fake.sends[1].Content)); got != 2500-maxTextLen {
		t.Fatalf("次包长度 = %d", got)
	}
}

func TestSendMentionUsersOnly(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	_, ok := a.SendGroupMsg("dc:c1", segsChain{atSeg("dc:123"), textSeg("@everyone 别慌")})
	if !ok {
		t.Fatal("发送失败")
	}
	send := fake.sends[0]
	if !strings.Contains(send.Content, "<@123> ") {
		t.Fatalf("提及未渲染: %q", send.Content)
	}
	// AI 文本中的字面 @everyone 不得触发全服通知
	for _, p := range send.AllowedMentions.Parse {
		if p == discordgo.AllowedMentionTypeEveryone {
			t.Fatal("字面 @everyone 不应放开 everyone 权限")
		}
	}
}

func TestSendAtAll(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	_, ok := a.SendGroupMsg("dc:c1", segsChain{atSeg("all"), textSeg("注意")})
	if !ok {
		t.Fatal("发送失败")
	}
	send := fake.sends[0]
	if !strings.Contains(send.Content, "@everyone ") {
		t.Fatalf("at-all 未渲染: %q", send.Content)
	}
	found := false
	for _, p := range send.AllowedMentions.Parse {
		if p == discordgo.AllowedMentionTypeEveryone {
			found = true
		}
	}
	if !found {
		t.Fatal("显式 at-all 应放开 everyone 权限")
	}
}

func TestSendReplyFirstMessageOnly(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	long := strings.Repeat("x", maxTextLen+10)
	_, ok := a.SendGroupMsg("dc:c1", segsChain{replySeg("dc:c1:77"), textSeg(long)})
	if !ok {
		t.Fatal("发送失败")
	}
	if len(fake.sends) != 2 {
		t.Fatalf("应分 2 包，实际 %d", len(fake.sends))
	}
	if fake.sends[0].Reference == nil || fake.sends[0].Reference.MessageID != "77" {
		t.Fatalf("首包应携带引用: %+v", fake.sends[0].Reference)
	}
	if fake.sends[1].Reference != nil {
		t.Fatal("次包不应携带引用")
	}
}

func TestSendImageBase64(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	raw := []byte("png-bytes")
	seg := message.OB11Segment{Type: message.SegmentImage, Data: message.ImageMessage{
		File: "base64://" + base64.StdEncoding.EncodeToString(raw),
		Url:  "base64://" + base64.StdEncoding.EncodeToString(raw),
	}.Marshal()}
	_, ok := a.SendGroupMsg("dc:c1", segsChain{textSeg("看图"), seg})
	if !ok {
		t.Fatal("发送失败")
	}
	if len(fake.sends) != 1 {
		t.Fatalf("文本应与附件同条发送，实际 %d 条", len(fake.sends))
	}
	send := fake.sends[0]
	if send.Content != "看图" || len(send.Files) != 1 {
		t.Fatalf("send = %+v", send)
	}
	got, _ := io.ReadAll(send.Files[0].Reader)
	if string(got) != string(raw) {
		t.Fatalf("文件内容 = %q", got)
	}
}

func TestSendImageURLReupload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("remote-png"))
	}))
	defer srv.Close()
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake) // session 为 nil：下载走 http.DefaultClient
	seg := message.OB11Segment{Type: message.SegmentImage, Data: message.ImageMessage{
		File: srv.URL + "/x.png", Url: srv.URL + "/x.png",
	}.Marshal()}
	_, ok := a.SendGroupMsg("dc:c1", segsChain{seg})
	if !ok {
		t.Fatal("发送失败（http URL 应下载后重传）")
	}
	if len(fake.sends) != 1 || len(fake.sends[0].Files) != 1 {
		t.Fatalf("sends = %+v", fake.sends)
	}
	got, _ := io.ReadAll(fake.sends[0].Files[0].Reader)
	if string(got) != "remote-png" {
		t.Fatalf("重传内容 = %q", got)
	}
}

func TestSendFileBatching(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	segs := segsChain{}
	for i := range 12 {
		segs = append(segs, message.OB11Segment{Type: message.SegmentImage, Data: message.ImageMessage{
			File: "base64://" + base64.StdEncoding.EncodeToString([]byte{byte(i)}),
			Url:  "base64://" + base64.StdEncoding.EncodeToString([]byte{byte(i)}),
		}.Marshal()})
	}
	_, ok := a.SendGroupMsg("dc:c1", segs)
	if !ok {
		t.Fatal("发送失败")
	}
	if len(fake.sends) != 2 {
		t.Fatalf("12 张图应分 2 批（10+2），实际 %d", len(fake.sends))
	}
	if len(fake.sends[0].Files) != maxFilesPerMessage || len(fake.sends[1].Files) != 2 {
		t.Fatalf("分批 = %d/%d", len(fake.sends[0].Files), len(fake.sends[1].Files))
	}
}

func TestSendFaceDegradesToText(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	face := message.OB11Segment{Type: message.SegmentFace, Data: map[string]any{"text": "[表情]"}}
	_, ok := a.SendGroupMsg("dc:c1", segsChain{face})
	if !ok || fake.sends[0].Content != "[表情]" {
		t.Fatalf("face 段应退化为文本: %+v", fake.sends)
	}
}

func TestSendFriendOpensDM(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	id, ok := a.SendFriendMsg("dc:u1", segsChain{textSeg("私聊")})
	if !ok {
		t.Fatal("发送失败")
	}
	if id != "dc:dm-u1:1" {
		t.Fatalf("消息 ID 应挂在 DM 频道下: %q", id)
	}
	if fake.sendChannel[0] != "dm-u1" {
		t.Fatalf("发送频道 = %q", fake.sendChannel[0])
	}
}

func TestSendOversizeFileSkipped(t *testing.T) {
	fake := &fakeDiscordAPI{}
	a := newSendAdapter(fake)
	big := make([]byte, maxUploadSize+1)
	seg := message.OB11Segment{Type: message.SegmentFile, Data: message.FileMessage{
		File: "base64://" + base64.StdEncoding.EncodeToString(big), Name: "big.bin",
	}.Marshal()}
	_, ok := a.SendGroupMsg("dc:c1", segsChain{textSeg("文字"), seg})
	if !ok {
		t.Fatal("超限附件应跳过但整链成功")
	}
	if len(fake.sends) != 1 || len(fake.sends[0].Files) != 0 {
		t.Fatalf("sends = %+v", fake.sends)
	}
}

func TestSendAPIError(t *testing.T) {
	fake := &fakeDiscordAPI{err: errors.New("50001: Missing Access")}
	a := newSendAdapter(fake)
	if _, ok := a.SendGroupMsg("dc:c1", segsChain{textSeg("x")}); ok {
		t.Fatal("API 失败应返回 false")
	}
}

func TestSplitText(t *testing.T) {
	if parts := splitText("", maxTextLen); len(parts) != 0 {
		t.Fatalf("空文本 = %v", parts)
	}
	parts := splitText(strings.Repeat("a", maxTextLen*2+1), maxTextLen)
	if len(parts) != 3 {
		t.Fatalf("段数 = %d", len(parts))
	}
}
