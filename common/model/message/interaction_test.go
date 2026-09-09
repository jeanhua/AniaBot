package message

import (
	"encoding/json"
	"testing"
)

// TestKeyboardMarshalParse keyboard 段 Marshal/ParseKeyboard 往返：
// 进程内 typed 值与 JSON 往返后的 map 形态都能解析。
func TestKeyboardMarshalParse(t *testing.T) {
	kb := KeyboardMessage{Rows: [][]InlineButton{
		{InlineButton{Text: "◀️ 上一页", Data: "music:pg:1"}, InlineButton{Text: "▶️ 下一页", Data: "music:pg:3"}},
		{InlineButton{Text: "打开网页", URL: "https://example.com"}},
	}}
	seg := OB11Segment{Type: SegmentKeyboard, Data: kb.Marshal()}

	// 进程内 typed 值
	var got KeyboardMessage
	if !ParseKeyboard(seg, &got) {
		t.Fatal("typed 形态解析失败")
	}
	if len(got.Rows) != 2 || len(got.Rows[0]) != 2 || got.Rows[1][0].URL != "https://example.com" {
		t.Fatalf("typed 解析结果不符: %+v", got)
	}

	// JSON 往返（消息缓存/日志等序列化场景）
	bs, err := json.Marshal(&seg)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back OB11Segment
	if err := json.Unmarshal(bs, &back); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	var got2 KeyboardMessage
	if !ParseKeyboard(back, &got2) {
		t.Fatal("JSON 往返形态解析失败")
	}
	if len(got2.Rows) != 2 || got2.Rows[0][1].Data != "music:pg:3" {
		t.Fatalf("JSON 往返解析结果不符: %+v", got2)
	}

	// 非法段：类型不符 / 空 rows / 无效按钮被跳过
	if ParseKeyboard(OB11Segment{Type: SegmentText}, &got) {
		t.Fatal("非 keyboard 段不应解析成功")
	}
	if ParseKeyboard(OB11Segment{Type: SegmentKeyboard, Data: map[string]any{"rows": [][]InlineButton{}}}, &got) {
		t.Fatal("空 rows 不应解析成功")
	}
	invalid := OB11Segment{Type: SegmentKeyboard, Data: map[string]any{
		"rows": []any{[]any{map[string]any{"text": "无目标"}, map[string]any{"data": "x"}}},
	}}
	if ParseKeyboard(invalid, &got) {
		t.Fatal("全无效按钮的行不应解析成功")
	}
}

// TestExtractKeyboard 提取首个 keyboard 段：剩余段保持顺序、不改入参；
// 无键盘段时返回 nil。
func TestExtractKeyboard(t *testing.T) {
	segs := []OB11Segment{
		{Type: SegmentText, Data: TextMessage{Text: "列表"}.Marshal()},
		{Type: SegmentKeyboard, Data: KeyboardMessage{Rows: [][]InlineButton{{{Text: "下一页", Data: "a:pg:2"}}}}.Marshal()},
		{Type: SegmentFace, Data: FaceMessage{Id: 1}.Marshal()},
		{Type: SegmentKeyboard, Data: KeyboardMessage{Rows: [][]InlineButton{{{Text: "第二个", Data: "a:x"}}}}.Marshal()},
	}
	rest, kb := ExtractKeyboard(segs)
	if kb == nil {
		t.Fatal("应提取到键盘")
	}
	if kb.Rows[0][0].Data != "a:pg:2" {
		t.Fatalf("应取首个键盘段: %+v", kb)
	}
	if len(rest) != 3 || rest[0].Type != SegmentText || rest[2].Type != SegmentKeyboard {
		t.Fatalf("剩余段不符（第二个键盘段保留）: %+v", rest)
	}
	if len(segs) != 4 {
		t.Fatal("入参切片不应被修改")
	}

	if rest2, kb2 := ExtractKeyboard(segs[:1]); kb2 != nil || len(rest2) != 1 {
		t.Fatalf("无键盘段时应返回 nil: %+v", kb2)
	}
}

// TestStripKeyboardSegments 剥离全部 keyboard 段；无键盘时返回原切片。
func TestStripKeyboardSegments(t *testing.T) {
	segs := []OB11Segment{
		{Type: SegmentText, Data: TextMessage{Text: "a"}.Marshal()},
		{Type: SegmentKeyboard, Data: KeyboardMessage{Rows: [][]InlineButton{{{Text: "x", Data: "a:x"}}}}.Marshal()},
		{Type: SegmentText, Data: TextMessage{Text: "b"}.Marshal()},
	}
	rest := StripKeyboardSegments(segs)
	if len(rest) != 2 || rest[1].Type != SegmentText {
		t.Fatalf("剥离结果不符: %+v", rest)
	}
	same := StripKeyboardSegments(segs[:1])
	if len(same) != 1 || &same[0] != &segs[0] {
		t.Fatal("无键盘段时应返回原切片")
	}
}

// TestInteractionEventFields 归一化交互事件的基本字段（编译期契约）。
func TestInteractionEventFields(t *testing.T) {
	ev := InteractionEvent{
		Platform: "telegram", MessageType: "group",
		GroupId: "tg:-100123", UserId: "tg:42", MessageId: "tg:-100123:7",
		CallbackId: "cb1", Data: "pg:2", AnswerText: "已翻页",
	}
	if ev.GroupId.String() != "tg:-100123" || ev.Data != "pg:2" {
		t.Fatalf("字段不符: %+v", ev)
	}
}
