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

// TestFriendlyTextSelfMention 艾特机器人自己时用 [at我] 占位（不暴露机器人自身 ID），
// 使 AI 明确知道本条消息 @ 了自己；其他 @ 目标仍显示 [at:id:…]，发送者自 @ 跳过。
func TestFriendlyTextSelfMention(t *testing.T) {
	msg := Message{
		SelfId: FromUint64(999),
		Sender: MessageSender{UserId: FromUint64(123), Nickname: "小明"},
		Message: []OB11Segment{
			{Type: SegmentMention, Data: map[string]any{"qq": "999"}},
			{Type: SegmentText, Data: map[string]any{"text": "你好"}},
			{Type: SegmentMention, Data: map[string]any{"qq": "888"}},
			{Type: SegmentMention, Data: map[string]any{"qq": "123"}},
			{Type: SegmentMention, Data: map[string]any{"qq": "all"}},
		},
	}
	text := msg.FriendlyText(true)
	want := "[nickname:小明 id:qq:123]: [at我]你好[at:id:qq:888][at:全体成员]"
	if text != want {
		t.Fatalf("FriendlyText = %q, want %q", text, want)
	}
}

// TestFriendlyTextNestedForward 合并转发内再嵌套合并转发时应逐层透传回调并递归展开，
// 内层转发不再退化为 [转发消息] 占位（GitHub issue #11）。
func TestFriendlyTextNestedForward(t *testing.T) {
	innerTextMsg := Message{
		Sender: MessageSender{UserId: FromUint64(303), Nickname: "内层发送者"},
		Message: []OB11Segment{
			{Type: SegmentText, Data: map[string]any{"text": "内层实际内容"}},
			{Type: SegmentImage, Data: ImageMessage{Url: "https://example.com/inner.png"}.Marshal()},
		},
	}
	outerTextMsg := Message{
		Sender: MessageSender{UserId: FromUint64(202), Nickname: "外层发送者"},
		Message: []OB11Segment{
			{Type: SegmentForward, Data: map[string]any{"id": "inner_fwd_1"}},
		},
	}
	root := Message{
		Sender: MessageSender{UserId: FromUint64(101), Nickname: "根发送者"},
		Message: []OB11Segment{
			{Type: SegmentForward, Data: map[string]any{"id": "outer_fwd_1"}},
		},
	}
	getForward := func(id QID) (*[]Message, bool) {
		switch id {
		case QID("outer_fwd_1"):
			return &[]Message{outerTextMsg}, true
		case QID("inner_fwd_1"):
			return &[]Message{innerTextMsg}, true
		}
		return nil, false
	}
	text := root.FriendlyText(true,
		WithGetForwardMsgFunc(getForward),
		WithGetImageOCRFunc(func(url string) string { return "图片OCR识别内容" }))
	for _, want := range []string{"<合并转发消息>", "外层发送者", "内层实际内容", "图片OCR识别内容"} {
		if !strings.Contains(text, want) {
			t.Fatalf("嵌套转发应递归展开, 缺少 %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "[转发消息]") {
		t.Fatalf("内层转发不应退化为占位符, got %q", text)
	}
}

// TestFriendlyTextForwardInlineContent 合并转发段内联携带 content（NapCat 展开
// 转发内容时的格式，嵌套层 id 仅供查看、无法再拉取）时应直接展开内联内容，
// 不再尝试按 id 拉取而显示「无法获取详情」（GitHub issue #11）。
func TestFriendlyTextForwardInlineContent(t *testing.T) {
	inner := Message{
		Sender: MessageSender{UserId: FromUint64(303), Nickname: "最内层"},
		Message: []OB11Segment{
			{Type: SegmentText, Data: map[string]any{"text": "内层实际内容"}},
			{Type: SegmentImage, Data: ImageMessage{Url: "https://example.com/nested.png"}.Marshal()},
		},
	}
	middle := Message{
		Sender: MessageSender{UserId: FromUint64(202), Nickname: "中间层"},
		Message: []OB11Segment{{
			Type: SegmentForward,
			Data: map[string]any{"id": "view-only-id", "content": []Message{inner}},
		}},
	}
	root := Message{
		Sender: MessageSender{UserId: FromUint64(101), Nickname: "外层"},
		Message: []OB11Segment{{
			Type: SegmentForward,
			Data: map[string]any{"id": "outer-view-id", "content": []Message{middle}},
		}},
	}
	// 内层 id 无法拉取时也应正常展开内联内容（NapCat 嵌套转发的真实形态）
	text := root.FriendlyText(true,
		WithGetForwardMsgFunc(func(QID) (*[]Message, bool) { return nil, false }),
		WithGetImageOCRFunc(func(url string) string { return "图片OCR识别内容" }))
	for _, want := range []string{"外层", "中间层", "内层实际内容", "图片OCR识别内容"} {
		if !strings.Contains(text, want) {
			t.Fatalf("内联 content 应被展开, 缺少 %q, got %q", want, text)
		}
	}
	if strings.Contains(text, "[转发消息") {
		t.Fatalf("内联 content 存在时不应出现转发占位, got %q", text)
	}
}
