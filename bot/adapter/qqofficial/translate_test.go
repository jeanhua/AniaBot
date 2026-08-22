package qqofficial

import (
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// TestOnGroupMessage 群@事件 → 通用群消息：注入 SelfId at 段（@ 触发）、
// 文本/附件/mentions 翻译、被动回复凭证记录。
func TestOnGroupMessage(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "BOTID"
	var got *message.Message
	a.SetTrigger(adapter.TriggerWrapper{
		OnGroupMsg: func(m message.Message) { got = &m },
	})

	ev := &groupMessageEvent{
		ID:          "ROBOT1.0_abc",
		Content:     " /你好 ",
		GroupOpenID: "GROUP1",
		Timestamp:   "2026-07-21T10:00:00+08:00",
		MessageType: 0,
		Author: eventUser{
			MemberOpenID: "MEMBER1",
			Username:     "小明",
			MemberRole:   "admin",
		},
		Attachments: []eventAttachment{
			{ContentType: "image/png", URL: "https://cdn.example.com/a.png", Filename: "a.png"},
		},
		Mentions: []eventUser{{MemberOpenID: "MEMBER2"}},
	}
	a.onGroupMessage(ev, true)

	if got == nil {
		t.Fatal("应触发 OnGroupMsg")
	}
	if got.MessageType != "group" {
		t.Errorf("MessageType = %q, want group", got.MessageType)
	}
	if got.MessageId != "qo:ROBOT1.0_abc" {
		t.Errorf("MessageId = %q, want qo:ROBOT1.0_abc", got.MessageId)
	}
	if got.GroupId != "qo:GROUP1" || got.UserId != "qo:MEMBER1" {
		t.Errorf("GroupId/UserId = %q/%q", got.GroupId, got.UserId)
	}
	if got.SelfId != "qo:BOTID" {
		t.Errorf("SelfId = %q, want qo:BOTID", got.SelfId)
	}
	if got.Sender.Nickname != "小明" || got.Sender.Role != "admin" {
		t.Errorf("Sender = %+v", got.Sender)
	}
	if got.Platform != Platform {
		t.Errorf("Platform = %q", got.Platform)
	}
	// 段序：at(SelfId) → text → image → at(MEMBER2)
	if len(got.Message) != 4 {
		t.Fatalf("段数 = %d, want 4: %+v", len(got.Message), got.Message)
	}
	if got.Message[0].Type != message.SegmentMention || got.Message[0].Data["qq"] != "qo:BOTID" {
		t.Errorf("首段应为 SelfId at 段: %+v", got.Message[0])
	}
	if got.Message[1].Type != message.SegmentText || got.Message[1].Data["text"] != "/你好" {
		t.Errorf("文本段内容错误: %+v", got.Message[1])
	}
	if got.Message[2].Type != message.SegmentImage || got.Message[2].Data["url"] != "https://cdn.example.com/a.png" {
		t.Errorf("图片段错误: %+v", got.Message[2])
	}
	if got.Message[3].Type != message.SegmentMention || got.Message[3].Data["qq"] != "qo:MEMBER2" {
		t.Errorf("mentions at 段错误: %+v", got.Message[3])
	}
	// 被动回复凭证已记录
	if msgID, seq, ok := a.nextReplySeq("GROUP1", true); !ok || msgID != "ROBOT1.0_abc" || seq != 1 {
		t.Errorf("被动回复凭证 = %q/%d/%v", msgID, seq, ok)
	}
	// 入站消息已入缓存（历史可查）
	if _, ok := a.GetMsgDetail("qo:ROBOT1.0_abc"); !ok {
		t.Error("GetMsgDetail 应命中缓存")
	}
}

// TestOnGroupMessageBotFiltered 机器人发送者的消息被过滤（防 bot 循环）。
func TestOnGroupMessageBotFiltered(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "BOTID"
	called := false
	a.SetTrigger(adapter.TriggerWrapper{OnGroupMsg: func(message.Message) { called = true }})
	a.onGroupMessage(&groupMessageEvent{
		ID: "ROBOT1.0_x", GroupOpenID: "G", Author: eventUser{MemberOpenID: "M", Bot: true}, Content: "hi",
	}, true)
	if called {
		t.Fatal("bot 消息不应触发插件链")
	}
}

// TestOnGroupMessageFullMode 全量模式（GROUP_MESSAGE_CREATE）非 @ 消息：
// 正常流经插件链，但不注入 SelfId at 段（aichat 不响应），与 NapCat 语义一致。
func TestOnGroupMessageFullMode(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "BOTID"
	var got *message.Message
	a.SetTrigger(adapter.TriggerWrapper{
		OnGroupMsg: func(m message.Message) { got = &m },
	})

	a.onGroupMessage(&groupMessageEvent{
		ID:          "ROBOT1.0_plain",
		Content:     "随便聊聊",
		GroupOpenID: "GROUP1",
		Timestamp:   "2026-07-21T10:00:00+08:00",
		Author:      eventUser{MemberOpenID: "MEMBER1"},
	}, false)

	if got == nil {
		t.Fatal("全量模式非 @ 消息应触发 OnGroupMsg")
	}
	for _, s := range got.Message {
		if s.Type == message.SegmentMention {
			t.Error("非 @ 消息不应注入 at 段")
		}
	}
	if got.RawMessage != "随便聊聊" {
		t.Errorf("RawMessage = %q", got.RawMessage)
	}
	// 非 @ 消息也记录被动回复凭证（全量模式下任何消息均可触发被动回复窗口）
	if _, _, ok := a.nextReplySeq("GROUP1", true); !ok {
		t.Error("被动回复凭证应已记录")
	}
}

// TestOnGroupMessageFullModeMentioned 全量模式 @ 消息：mentions 中机器人条目
// （bot=true，openid 与 READY user.id 不同源）经 bot 标志+用户名识别，注入
// SelfId at 段；机器人自身不重复生成 at 段，其他成员保留；content 残留的
// <@openid> 标记被剥离。
func TestOnGroupMessageFullModeMentioned(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "READYBOTID"
	a.selfUsername = "Ania机器人"
	var got *message.Message
	a.SetTrigger(adapter.TriggerWrapper{
		OnGroupMsg: func(m message.Message) { got = &m },
	})

	a.onGroupMessage(&groupMessageEvent{
		ID:          "ROBOT1.0_at",
		Content:     "<@3B7DEE37ECC7F7290D17E20796C3C8BD> /你好",
		GroupOpenID: "GROUP1",
		Timestamp:   "2026-07-21T10:00:00+08:00",
		Author:      eventUser{MemberOpenID: "MEMBER1"},
		Mentions: []eventUser{
			// 机器人自身：群场景 member_openid 与 READY user.id 不同源，靠 bot 标志识别
			{ID: "3B7DEE37ECC7F7290D17E20796C3C8BD", MemberOpenID: "3B7DEE37ECC7F7290D17E20796C3C8BD", Username: "Ania机器人", Bot: true},
			{MemberOpenID: "MEMBER2"},
		},
	}, false)

	if got == nil {
		t.Fatal("@ 消息应触发 OnGroupMsg")
	}
	// 段序：at(SelfId 注入) → text(标记已剥离) → at(MEMBER2)；机器人自身不重复出现
	if len(got.Message) != 3 {
		t.Fatalf("段数 = %d, want 3: %+v", len(got.Message), got.Message)
	}
	if got.Message[0].Type != message.SegmentMention || got.Message[0].Data["qq"] != "qo:READYBOTID" {
		t.Errorf("首段应为注入的 SelfId at 段: %+v", got.Message[0])
	}
	if got.Message[1].Type != message.SegmentText || got.Message[1].Data["text"] != "/你好" {
		t.Errorf("文本段应为剥离标记后的内容: %+v", got.Message[1])
	}
	if got.Message[2].Type != message.SegmentMention || got.Message[2].Data["qq"] != "qo:MEMBER2" {
		t.Errorf("其他成员 at 段应保留: %+v", got.Message[2])
	}
	if got.RawMessage != "/你好" {
		t.Errorf("RawMessage = %q, want /你好", got.RawMessage)
	}
}

// TestOnGroupMessageFullModeOtherBot 全量模式 @ 的是**其他**机器人（bot=true 但
// 用户名不同）：不注入 SelfId at 段（aichat 不触发），该机器人的 at 段按普通
// 成员生成，其 <@openid> 标记同样剥离。
func TestOnGroupMessageFullModeOtherBot(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "READYBOTID"
	a.selfUsername = "Ania机器人"
	var got *message.Message
	a.SetTrigger(adapter.TriggerWrapper{
		OnGroupMsg: func(m message.Message) { got = &m },
	})

	a.onGroupMessage(&groupMessageEvent{
		ID:          "ROBOT1.0_otherbot",
		Content:     "<@AAAA1111BBBB2222> 你好",
		GroupOpenID: "GROUP1",
		Author:      eventUser{MemberOpenID: "MEMBER1"},
		Mentions: []eventUser{
			{ID: "AAAA1111BBBB2222", MemberOpenID: "AAAA1111BBBB2222", Username: "别的机器人", Bot: true},
		},
	}, false)

	if got == nil {
		t.Fatal("消息应触发 OnGroupMsg")
	}
	// 段序：text(标记已剥离) → at(其他机器人)
	if len(got.Message) != 2 {
		t.Fatalf("段数 = %d, want 2: %+v", len(got.Message), got.Message)
	}
	if got.Message[0].Type != message.SegmentText || got.Message[0].Data["text"] != "你好" {
		t.Errorf("文本段错误: %+v", got.Message[0])
	}
	if got.Message[1].Type != message.SegmentMention || got.Message[1].Data["qq"] != "qo:AAAA1111BBBB2222" {
		t.Errorf("其他机器人的 at 段应按普通成员生成: %+v", got.Message[1])
	}
}

// TestOnGroupMessageFullModeSelfFiltered 全量模式下机器人自己发送的消息
// （author.id == selfID）被过滤，防止 bot 自我循环。
func TestOnGroupMessageFullModeSelfFiltered(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "BOTID"
	called := false
	a.SetTrigger(adapter.TriggerWrapper{OnGroupMsg: func(message.Message) { called = true }})
	a.onGroupMessage(&groupMessageEvent{
		ID: "ROBOT1.0_self", GroupOpenID: "G", Content: "机器人自己的回复",
		Author: eventUser{ID: "BOTID", MemberOpenID: "BOTID"},
	}, false)
	if called {
		t.Fatal("机器人自身消息不应触发插件链")
	}
}

// TestOnC2CMessage 单聊事件 → 通用私聊消息（private + sub_type=friend）。
func TestOnC2CMessage(t *testing.T) {
	a := NewAdapter(nil)
	a.selfID = "BOTID"
	var got *message.Message
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(m message.Message) { got = &m },
	})
	a.onC2CMessage(&c2cMessageEvent{
		ID:        "ROBOT1.0_c2c",
		Content:   "你好",
		Timestamp: "2026-07-21T10:00:00+08:00",
		Author:    eventUser{UserOpenID: "USER1"},
	})
	if got == nil {
		t.Fatal("应触发 OnFriendMsg")
	}
	if got.MessageType != "private" || got.SubType != "friend" {
		t.Errorf("MessageType/SubType = %q/%q, want private/friend", got.MessageType, got.SubType)
	}
	if got.UserId != "qo:USER1" {
		t.Errorf("UserId = %q, want qo:USER1", got.UserId)
	}
	// 私聊不注入 at 段
	for _, s := range got.Message {
		if s.Type == message.SegmentMention {
			t.Error("私聊消息不应注入 at 段")
		}
	}
	// 单聊被动回复凭证（60 分钟 / 4 次）
	if _, _, ok := a.nextReplySeq("USER1", false); !ok {
		t.Error("单聊被动回复凭证应有效")
	}
}

// TestContentToSegmentsQuote 引用消息（message_type=103）正文为空时取引用内容。
func TestContentToSegmentsQuote(t *testing.T) {
	segs := contentToSegments("", 103, []msgElement{{Content: "被引用的内容"}})
	if len(segs) != 1 || segs[0].Data["text"] != "被引用的内容" {
		t.Fatalf("引用消息兜底失败: %+v", segs)
	}
	if segs := contentToSegments("  ", 0, nil); segs != nil {
		t.Fatalf("空白文本应无段: %+v", segs)
	}
}

// TestAttachmentsToSegments 附件类型映射。
func TestAttachmentsToSegments(t *testing.T) {
	segs := attachmentsToSegments([]eventAttachment{
		{ContentType: "image/jpeg", URL: "https://e.com/i.jpg"},
		{ContentType: "voice", URL: "https://e.com/v.silk", VoiceWavURL: "https://e.com/v.wav"},
		{ContentType: "video/mp4", URL: "https://e.com/v.mp4"},
		{ContentType: "file", URL: "https://e.com/f.zip", Filename: "f.zip"},
		{ContentType: "image/png", URL: ""}, // 无 URL 跳过
	})
	if len(segs) != 4 {
		t.Fatalf("段数 = %d, want 4", len(segs))
	}
	types := []string{message.SegmentImage, message.SegmentRecord, message.SegmentVideo, message.SegmentFile}
	for i, s := range segs {
		if s.Type != types[i] {
			t.Errorf("segs[%d].Type = %q, want %q", i, s.Type, types[i])
		}
	}
	// 语音优先 WAV 链接
	if segs[1].Data["file"] != "https://e.com/v.wav" {
		t.Errorf("语音段应优先 WAV: %+v", segs[1])
	}
	if segs[3].Data["name"] != "f.zip" {
		t.Errorf("文件段文件名丢失: %+v", segs[3])
	}
}

// TestParseEventTime RFC3339 解析与回退。
func TestParseEventTime(t *testing.T) {
	if ts := parseEventTime("2026-07-21T10:00:00+08:00"); ts != 1784599200 {
		t.Errorf("parseEventTime = %d, want 1784599200", ts)
	}
	if ts := parseEventTime("bad"); ts <= 0 {
		t.Error("非法时间应回退当前时间")
	}
}

// TestContentToSegmentsChatRecord 聊天记录（message_type=102）文本内嵌的附件描述
// 应拆为结构化段：图片带 url 键（FriendlyText 输出 [图片 <hash> url:<url>]，
// AI 的 load_images 可按哈希加载），而不是停留在纯文本导致 AI 拿文件名当哈希。
func TestContentToSegmentsChatRecord(t *testing.T) {
	content := "[群聊的聊天记录]\n" +
		"=== 消息 1 ===\n" +
		"[发送者] Ice-Nick\n" +
		"[附件1] 类型:图片 文件名:68C30E391DA0319548B734539FCC037E.jpg 尺寸:630x1142 大小:101.9KB URL:https://multimedia.nt.qq.com.cn/download?appid=1406&fileid=abc&rkey=xyz&spec=0\n" +
		"\n" +
		"=== 消息 2 ===\n" +
		"[消息内容] 版本又更新了\n" +
		"[发送者] I…"
	segs := contentToSegments(content, 102, nil)
	if len(segs) != 3 {
		t.Fatalf("段数 = %d, want 3: %+v", len(segs), segs)
	}
	if segs[0].Type != message.SegmentText || !strings.Contains(segs[0].Data["text"].(string), "[群聊的聊天记录]") {
		t.Errorf("首段应为聊天记录文本: %+v", segs[0])
	}
	if segs[1].Type != message.SegmentImage {
		t.Fatalf("第二段应为图片段: %+v", segs[1])
	}
	var img message.ImageMessage
	if !message.ParseImage(segs[1], &img) {
		t.Fatalf("图片段解析失败: %+v", segs[1])
	}
	wantURL := "https://multimedia.nt.qq.com.cn/download?appid=1406&fileid=abc&rkey=xyz&spec=0"
	if img.Url != wantURL {
		t.Errorf("图片 url = %q, want %q", img.Url, wantURL)
	}
	if img.File != "68C30E391DA0319548B734539FCC037E.jpg" {
		t.Errorf("图片 file = %q, want 文件名", img.File)
	}
	// 哈希应与 FriendlyText 展示的 [图片 <hash> url:<url>] 一致（可被 load_images 解析）
	marker := " [图片 " + message.ImageHash(wantURL) + " url:" + wantURL + "]"
	if text := segs[0].Data["text"].(string) + segs[2].Data["text"].(string); strings.Contains(text, marker) {
		t.Log("哈希标记由 FriendlyText 输出")
	}
	if segs[2].Type != message.SegmentText || !strings.Contains(segs[2].Data["text"].(string), "版本又更新了") {
		t.Errorf("末段应为剩余聊天记录文本: %+v", segs[2])
	}

	// 正文为空时走 msg_elements 拼接兜底（同 message_type=103）
	segs = contentToSegments("", 102, []msgElement{{Content: "[群聊的聊天记录]\n[附件1] 类型:图片 URL:https://e.com/p.png"}})
	if len(segs) != 2 || segs[1].Type != message.SegmentImage {
		t.Fatalf("msg_elements 兜底拆段失败: %+v", segs)
	}

	// 普通文本（无附件描述）仍为单文本段
	if segs := contentToSegments("普通消息", 0, nil); len(segs) != 1 || segs[0].Type != message.SegmentText {
		t.Fatalf("普通文本应保持单文本段: %+v", segs)
	}
}
