package luckylilia

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

func TestToLil(t *testing.T) {
	cases := []struct {
		in   message.QID
		want message.QID
	}{
		{"qq:12345", "lil:12345"},
		{"12345", "12345"}, // 未规范化的裸数字不处理（QID 反序列化后必带 qq:）
		{"all", "all"},     // @全体成员标记原样保留
		{"fs:oc_xxx", "fs:oc_xxx"},
		{"", ""},
	}
	for _, c := range cases {
		if got := toLil(c.in); got != c.want {
			t.Fatalf("toLil(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStripLilSegments(t *testing.T) {
	chain := msgchain.Builder().Group().
		Mention(message.QID("lil:123")).
		Reply(message.QID("lil:456")).
		Build()
	got := stripLilSegments(chain.GetGroupMsg())
	if got[0].Data["qq"] != "123" {
		t.Fatalf("mention = %v, want 123", got[0].Data["qq"])
	}
	if got[1].Data["id"] != "456" {
		t.Fatalf("reply = %v, want 456", got[1].Data["id"])
	}

	// @全体成员（段数据 qq=all）必须原样保留
	got = stripLilSegments([]message.OB11Segment{{
		Type: message.SegmentMention,
		Data: map[string]any{"qq": "all"},
	}})
	if got[0].Data["qq"] != "all" {
		t.Fatalf("mention all = %v, want all", got[0].Data["qq"])
	}
}

func TestStripLilForward(t *testing.T) {
	inner := msgchain.Builder().Group().
		Mention(message.QID("lil:789")).
		Build()
	forwardBuilder := msgchain.Builder().GroupForward()
	forwardBuilder.Message(message.QID("lil:321"), "nick", inner)
	forward := forwardBuilder.Build()
	got := stripLilForward(forward.GetForwardMsg())
	if got.Messages[0].Data.UserId != "321" {
		t.Fatalf("node user = %q, want 321", got.Messages[0].Data.UserId)
	}
	if got.Messages[0].Data.Content[0].Data["qq"] != "789" {
		t.Fatalf("nested mention = %v, want 789", got.Messages[0].Data.Content[0].Data["qq"])
	}
}

// TestRelilMessage 验证入站消息（先按 QQ 规范化为 qq: 前缀）整体改写为 lil: 前缀，
// 覆盖顶层 ID 字段与段内 at/reply ID。
func TestRelilMessage(t *testing.T) {
	raw := []byte(`{
		"post_type": "message",
		"message_type": "group",
		"sub_type": "normal",
		"message_id": 1001,
		"user_id": 111,
		"group_id": 222,
		"self_id": 333,
		"raw_message": "[CQ:at,qq=111] hi [CQ:reply,id=1001]",
		"sender": {"user_id": 111, "nickname": "n"},
		"message": [
			{"type": "at", "data": {"qq": "111"}},
			{"type": "text", "data": {"text": "hi"}},
			{"type": "reply", "data": {"id": "1001"}}
		]
	}`)
	var msg message.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	message.NormalizeQQMessage(&msg)
	relilMessage(&msg)

	if msg.MessageId != "lil:1001" {
		t.Fatalf("message_id = %q, want lil:1001", msg.MessageId)
	}
	if msg.UserId != "lil:111" || msg.Sender.UserId != "lil:111" {
		t.Fatalf("user_id = %q/%q, want lil:111", msg.UserId, msg.Sender.UserId)
	}
	if msg.GroupId != "lil:222" {
		t.Fatalf("group_id = %q, want lil:222", msg.GroupId)
	}
	if msg.SelfId != "lil:333" {
		t.Fatalf("self_id = %q, want lil:333", msg.SelfId)
	}
	if msg.Message[0].Data["qq"] != "lil:111" {
		t.Fatalf("at qq = %v, want lil:111", msg.Message[0].Data["qq"])
	}
	if msg.Message[2].Data["id"] != "lil:1001" {
		t.Fatalf("reply id = %v, want lil:1001", msg.Message[2].Data["id"])
	}
}

// pngB64 一段最小合法 PNG 文件头的 base64（含 PNG 魔数）。
var pngB64 = base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n0000000000000000"))

func TestStripLilSegmentsFileToImage(t *testing.T) {
	// base64 图片 + 图片扩展名 → 转 image 段
	got := stripLilSegments(msgchain.Builder().Group().
		FileBase64("photo.png", pngB64).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentImage {
		t.Fatalf("图片文件应转为 image 段, got %+v", got)
	}
	if got[0].Data["file"] != "base64://"+pngB64 {
		t.Fatalf("image 段应保留 base64 源, got %+v", got[0].Data)
	}

	// 非图片扩展名 + 非图片内容 → 保持 file 段
	got = stripLilSegments(msgchain.Builder().Group().
		FileBase64("doc.pdf", base64.StdEncoding.EncodeToString([]byte("plain text"))).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentFile {
		t.Fatalf("非图片文件应保持 file 段, got %+v", got)
	}
}
