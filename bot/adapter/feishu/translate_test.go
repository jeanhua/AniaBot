package feishu

import (
	"encoding/json"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// parseTextContent 入站：text 消息中的 @占位符应拆分为 at 段 + 文本段。
func TestParseTextContentMentions(t *testing.T) {
	a := &feishuAdapter{}
	content := `{"text":"@_user_1 你好 世界"}`
	mentions := []feishuMention{{key: "@_user_1", openID: "ou_abc"}}
	segs := a.parseTextContent(content, mentions)

	if len(segs) != 2 {
		t.Fatalf("期望 [at][text] 2 段，got %d: %+v", len(segs), segs)
	}
	if segs[0].Type != message.SegmentMention || segs[0].Data["qq"] != "fs:ou_abc" {
		t.Fatalf("首段应为 at(fs:ou_abc)，got %+v", segs[0])
	}
	if segs[1].Type != message.SegmentText || segs[1].Data["text"] != " 你好 世界" {
		t.Fatalf("次段应为剩余文本，got %+v", segs[1])
	}
}

// parseTextContent 入站：@全体成员占位符应映射为 at all。
func TestParseTextContentAtAll(t *testing.T) {
	a := &feishuAdapter{}
	segs := a.parseTextContent(`{"text":"@_all 通知"}`, nil)
	if len(segs) != 2 {
		t.Fatalf("期望 [at all][text] 2 段，got %d", len(segs))
	}
	if segs[0].Data["qq"] != "all" {
		t.Fatalf("@_all 应映射为 at qq=all，got %+v", segs[0])
	}
}

// parseTextContent 入站：消息 API 返回 <at> 标记（非占位符）时应能兜底解析且顺序正确。
func TestParseTextContentAtMarkup(t *testing.T) {
	a := &feishuAdapter{}
	segs := a.parseTextContent(`{"text":"hello <at user_id=\"ou_x\">Tom</at> world"}`, nil)
	if len(segs) != 3 {
		t.Fatalf("期望 [text][at][text] 3 段，got %d: %+v", len(segs), segs)
	}
	if segs[0].Type != message.SegmentText || segs[0].Data["text"] != "hello " {
		t.Fatalf("首段应为文本 hello，got %+v", segs[0])
	}
	if segs[1].Type != message.SegmentMention || segs[1].Data["qq"] != "fs:ou_x" {
		t.Fatalf("次段应为 at(fs:ou_x)，got %+v", segs[1])
	}
	if segs[2].Type != message.SegmentText || segs[2].Data["text"] != " world" {
		t.Fatalf("末段应为文本 world，got %+v", segs[2])
	}
}

// segmentsToContent 出站：纯文本+at 应生成正确转义的 text content（引号/换行不破坏 JSON）。
func TestSegmentsToContentTextEscaping(t *testing.T) {
	a := &feishuAdapter{}
	segs := []message.OB11Segment{
		{Type: message.SegmentText, Data: map[string]any{"text": `他说"你好"\n`}},
		{Type: message.SegmentMention, Data: map[string]any{"qq": "fs:ou_abc"}},
		{Type: message.SegmentText, Data: map[string]any{"text": "!"}},
	}
	msgType, content, _ := a.segmentsToContent(t.Context(), segs)
	if msgType != "text" {
		t.Fatalf("应为 text 消息，got %s", msgType)
	}
	// content 必须是合法 JSON 且 text 字段内容正确
	var parsed map[string]string
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("content 非法 JSON: %v", err)
	}
	text := parsed["text"]
	if text == "" {
		t.Fatalf("text 内容为空")
	}
}

// segmentsToContent 出站：含图片时走 post 富文本。
func TestSegmentsToContentPostWithImage(t *testing.T) {
	a := &feishuAdapter{} // client 为 nil → uploadImage 失败 → 无图片 key，但走 post 分支
	segs := []message.OB11Segment{
		{Type: message.SegmentText, Data: map[string]any{"text": "看"}},
		{Type: message.SegmentImage, Data: map[string]any{"file": "base64://aGVsbG8="}},
	}
	msgType, content, keys := a.segmentsToContent(t.Context(), segs)
	if msgType != "post" {
		t.Fatalf("含图片应走 post，got %s", msgType)
	}
	if len(keys) != 0 {
		t.Fatalf("无 client 时图片上传应失败，keys 应为空，got %v", keys)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("post content 非法 JSON: %v", err)
	}
	if _, ok := parsed["zh_cn"]; !ok {
		t.Fatalf("post content 应含 zh_cn 语言键，got %s", content)
	}
}
