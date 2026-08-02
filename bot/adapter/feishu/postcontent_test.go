package feishu

import (
	"encoding/json"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// 回归：接收事件的 post（图文）消息 content 是顶层 title/content 结构
// （无 zh_cn 包装），必须翻译为非空段，否则事件被静默丢弃。
func TestParsePostContentReceiveEventTopLevel(t *testing.T) {
	a := &feishuAdapter{}
	// 官方「接收消息内容」文档结构：title/content 在顶层
	content := `{"title":"","content":[[{"tag":"text","text":"看这个图片"}],[{"tag":"img","image_key":"img_v2_abcdef"}]]}`
	segs := a.parsePostContent(content, nil)
	if len(segs) == 0 {
		t.Fatal("接收事件格式 post 消息被翻译为空段，事件会被静默丢弃")
	}
	if segs[0].Type != message.SegmentText || segs[0].Data["text"] != "看这个图片\n" {
		t.Fatalf("首段应为文本，got %+v", segs[0])
	}
	foundImg := false
	for _, s := range segs {
		if s.Type == message.SegmentImage && s.Data["file"] == "img_v2_abcdef" {
			foundImg = true
		}
	}
	if !foundImg {
		t.Fatalf("应包含图片段 img_v2_abcdef，got %+v", segs)
	}
}

// 回归：接收事件格式的 at 元素 user_id 是占位符（@_user_N），
// 应经 mentions 反查为 open_id，而不是输出 fs:@_user_N。
func TestParsePostContentReceiveEventAtPlaceholder(t *testing.T) {
	a := &feishuAdapter{}
	content := `{"title":"标题","content":[[{"tag":"text","text":"你好"}],[{"tag":"at","user_id":"@_user_1","user_name":""}]]}`
	mentions := []feishuMention{{key: "@_user_1", openID: "ou_abc"}}
	segs := a.parsePostContent(content, mentions)
	if len(segs) == 0 {
		t.Fatal("post 消息被翻译为空段")
	}
	at := segs[0]
	if segs[0].Type == message.SegmentText {
		at = segs[1]
	}
	if at.Type != message.SegmentMention || at.Data["qq"] != "fs:ou_abc" {
		t.Fatalf("at 占位符应反查为 fs:ou_abc，got %+v", at)
	}
}

// 兼容：发送/消息 API 侧的 zh_cn 包装结构仍可解析。
func TestParsePostContentZhCnWrapper(t *testing.T) {
	a := &feishuAdapter{}
	content := `{"zh_cn":{"title":"标题","content":[[{"tag":"text","text":"你好"}],[{"tag":"img","image_key":"img_v2_xyz"}]]}}`
	segs := a.parsePostContent(content, nil)
	if len(segs) == 0 {
		t.Fatal("zh_cn 包装结构被翻译为空段")
	}
	if segs[0].Data["text"] != "标题\n你好\n" {
		t.Fatalf("zh_cn 结构首段应为标题+文本，got %+v", segs[0])
	}
	b, _ := json.Marshal(segs)
	t.Logf("segs: %s", b)
}

// 兼容：接收事件侧 content 为空时回退 content_v2（markdown 场景）。
func TestParsePostContentContentV2Fallback(t *testing.T) {
	a := &feishuAdapter{}
	content := `{"title":"","content":[],"content_v2":[[{"tag":"md","text":"**粗体** ![图](img_v2_m)"}]]}`
	segs := a.parsePostContent(content, nil)
	if len(segs) == 0 {
		t.Fatal("content_v2 回退后仍为空段")
	}
	if segs[0].Data["text"] != "**粗体** ![图](img_v2_m)\n" {
		t.Fatalf("content_v2 应输出 md 文本，got %+v", segs[0])
	}
}

// 完整链路：接收事件 图文 post 消息经 eventMessageToOB11 产出非空段。
func TestEventMessageToOB11ReceivePostImage(t *testing.T) {
	a := &feishuAdapter{}
	content := `{"title":"","content":[[{"tag":"text","text":"看这个图片"}],[{"tag":"img","image_key":"img_v2_abcdef"}]]}`
	em := &larkim.EventMessage{
		MessageType: strPtr("post"),
		Content:     &content,
		MessageId:   strPtr("om_xxx"),
	}
	ob := a.eventMessageToOB11(em)
	if len(ob) == 0 {
		t.Fatal("完整链路：图文消息被翻译为空段，事件被静默丢弃")
	}
}
