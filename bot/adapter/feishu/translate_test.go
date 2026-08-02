package feishu

import (
	"encoding/json"
	"strings"
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

// segmentsToContent 出站：文本+at 生成 post + md 元素，markdown 原样保留、@ 拆为 at 元素。
func TestSegmentsToContentMarkdown(t *testing.T) {
	a := &feishuAdapter{}
	segs := []message.OB11Segment{
		{Type: message.SegmentMention, Data: map[string]any{"qq": "fs:ou_abc"}},
		{Type: message.SegmentText, Data: map[string]any{"text": "# 标题\n**粗体**\n- 列表"}},
	}
	msgType, content, _ := a.segmentsToContent(t.Context(), segs)
	if msgType != "post" {
		t.Fatalf("应生成 post 消息，got %s", msgType)
	}
	var parsed struct {
		ZhCN struct {
			Content [][]map[string]any `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("content 非法 JSON: %v\n%s", err, content)
	}
	if len(parsed.ZhCN.Content) < 2 {
		t.Fatalf("应有 [@行][md行] 两段，got %d", len(parsed.ZhCN.Content))
	}
	// 首行应含 at 元素（@ou_abc）
	first := parsed.ZhCN.Content[0]
	if first[0]["tag"] != "at" || first[0]["user_id"] != "ou_abc" {
		t.Fatalf("首段应为 at(ou_abc)，got %+v", first)
	}
	// 末行应为 md 元素，markdown 原样保留（引号/换行不被破坏）
	last := parsed.ZhCN.Content[len(parsed.ZhCN.Content)-1]
	if last[len(last)-1]["tag"] != "md" {
		t.Fatalf("末段应为 md 元素，got %+v", last)
	}
	mdText, _ := last[len(last)-1]["text"].(string)
	if !strings.Contains(mdText, "# 标题") || !strings.Contains(mdText, "**粗体**") {
		t.Fatalf("markdown 应原样保留，got %q", mdText)
	}
}

// segmentsToContent 出站：含图片时 post 末尾追加 img 元素。
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

// segmentsToContent 出站：@全体成员在 post 中尽力而为（md 文本前置 <at user_id="all">）。
func TestSegmentsToContentAtAll(t *testing.T) {
	a := &feishuAdapter{}
	segs := []message.OB11Segment{
		{Type: message.SegmentMention, Data: map[string]any{"qq": "all"}},
		{Type: message.SegmentText, Data: map[string]any{"text": "通知"}},
	}
	msgType, content, _ := a.segmentsToContent(t.Context(), segs)
	if msgType != "post" {
		t.Fatalf("应生成 post 消息，got %s", msgType)
	}
	var parsed struct {
		ZhCN struct {
			Content [][]map[string]any `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("content 非法 JSON: %v", err)
	}
	last := parsed.ZhCN.Content[len(parsed.ZhCN.Content)-1]
	mdText, _ := last[len(last)-1]["text"].(string)
	if !strings.HasPrefix(mdText, `<at user_id="all">`) {
		t.Fatalf("@all 标记应前置注入到 md 文本，got %q", mdText)
	}
}

// appendImagesToPost 图片 key 追加到 post 最后一个段落。
func TestAppendImagesToPost(t *testing.T) {
	content := `{"zh_cn":{"title":"","content":[[{"tag":"md","text":"hi"}]]}}`
	out := appendImagesToPost(content, []string{"img_v2_x", "img_v2_y"})
	var parsed struct {
		ZhCN struct {
			Content [][]map[string]any `json:"content"`
		} `json:"zh_cn"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("追加图片后 content 非法 JSON: %v", err)
	}
	last := parsed.ZhCN.Content[len(parsed.ZhCN.Content)-1]
	if len(last) != 3 {
		t.Fatalf("md 后应追加 2 个 img 元素，got %d", len(last))
	}
	if last[1]["tag"] != "img" || last[1]["image_key"] != "img_v2_x" {
		t.Fatalf("img 元素错误，got %+v", last[1])
	}
}
