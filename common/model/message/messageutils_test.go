package message

import (
	"strings"
	"testing"
)

// TestFriendlyTextNicknameFallback 昵称不可得（飞书通讯录查询失败/权限缺失）时兜底显示「用户」，
// 避免出现 [nickname: id:fs:ou_xxx]: 的空昵称前缀。
func TestFriendlyTextNicknameFallback(t *testing.T) {
	msg := Message{
		Message: []OB11Segment{{Type: SegmentText, Data: map[string]any{"text": "你好"}}},
		Sender:  MessageSender{UserId: FromUint64(123456)},
	}
	text := msg.FriendlyText(true)
	want := "[nickname:用户 id:qq:123456]: 你好"
	if !strings.HasPrefix(text, want) {
		t.Fatalf("空昵称应兜底为「用户」, got %q", text)
	}

	// 有昵称时保持原样
	msg.Sender.Nickname = "小明"
	text = msg.FriendlyText(true)
	if !strings.HasPrefix(text, "[nickname:小明 id:qq:123456]: ") {
		t.Fatalf("有昵称时应显示昵称, got %q", text)
	}

	// 群名片优先于昵称
	msg.Sender.Card = "群名片"
	text = msg.FriendlyText(true)
	if !strings.HasPrefix(text, "[nickname:群名片 id:qq:123456]: ") {
		t.Fatalf("群名片应优先, got %q", text)
	}
}
