package msgchain

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// TestBuilderSegmentRoundtrip msgchain 构造的段可被 ParseXxx 正确解析
// （类型化段数据的单一事实来源：Marshal 键与 Parse 读取键一致）。
func TestBuilderSegmentRoundtrip(t *testing.T) {
	// 图片：同时写 file 与 url（ParseImage 依赖 url）
	segs := Builder().Group().ImageUrl("https://example.com/a.png").Build().GetGroupMsg()
	if len(segs) != 1 || segs[0].Type != message.SegmentImage {
		t.Fatalf("期望一个 image 段, got %+v", segs)
	}
	var im message.ImageMessage
	if !message.ParseImage(segs[0], &im) {
		t.Fatal("ParseImage 解析失败")
	}
	if im.Url != "https://example.com/a.png" || im.File != "https://example.com/a.png" {
		t.Fatalf("ImageUrl 应同时写 file 与 url, got %+v", im)
	}

	// 提及：QQ 数字与带前缀 ID
	msegs := Builder().Group().Mention(message.QID("fs:ou_abc")).Build().GetGroupMsg()
	var mm message.MentionMessage
	if !message.ParseMention(msegs[0], &mm) {
		t.Fatal("ParseMention 解析失败")
	}
	if mm.QQ != "fs:ou_abc" || mm.IsAll {
		t.Fatalf("Mention 解析不符, got %+v", mm)
	}

	// 视频：同时写 file 与 url（ParseVideo 依赖 url）
	vsegs := Builder().Group().VideoUrl("https://example.com/v.mp4").Build().GetGroupMsg()
	var vm message.VideoMessage
	if !message.ParseVideo(vsegs[0], &vm) {
		t.Fatal("ParseVideo 解析失败")
	}
	if vm.URL != "https://example.com/v.mp4" {
		t.Fatalf("VideoUrl 应写 url, got %+v", vm)
	}

	// 文件：file/name 键
	fsegs := Builder().Group().FileUrl("doc.pdf", "https://example.com/doc.pdf").Build().GetGroupMsg()
	var fm message.FileMessage
	if !message.ParseFile(fsegs[0], &fm) {
		t.Fatal("ParseFile 解析失败")
	}
	if fm.File != "https://example.com/doc.pdf" || fm.Name != "doc.pdf" {
		t.Fatalf("FileUrl 解析不符, got %+v", fm)
	}
}

// TestBuilderKeyboard Keyboard 构造键盘段：Button/ButtonURL/Row 辅助、
// 重复调用覆盖前次、ParseKeyboard 往返一致。
func TestBuilderKeyboard(t *testing.T) {
	rows := [][]message.InlineButton{
		Row(Button("◀️ 上一页", "music:pg:1"), Button("▶️ 下一页", "music:pg:3")),
		Row(ButtonURL("网页版", "https://example.com")),
	}
	segs := Builder().Group().Text("列表").Keyboard(rows...).Build().GetGroupMsg()
	if len(segs) != 2 || segs[1].Type != message.SegmentKeyboard {
		t.Fatalf("应追加一个 keyboard 段: %+v", segs)
	}
	var kb message.KeyboardMessage
	if !message.ParseKeyboard(segs[1], &kb) {
		t.Fatal("ParseKeyboard 解析失败")
	}
	if kb.Rows[0][0].Text != "◀️ 上一页" || kb.Rows[0][1].Data != "music:pg:3" || kb.Rows[1][0].URL != "https://example.com" {
		t.Fatalf("按钮数据不符: %+v", kb)
	}

	// 后加覆盖前加：只剩一个 keyboard 段
	segs2 := Builder().Group().Keyboard(rows...).Keyboard(Row(Button("仅一页", "music:pg:1"))).Build().GetGroupMsg()
	if len(segs2) != 1 {
		t.Fatalf("重复 Keyboard 应覆盖而非追加: %+v", segs2)
	}
	var kb2 message.KeyboardMessage
	if !message.ParseKeyboard(segs2[0], &kb2) || len(kb2.Rows) != 1 || kb2.Rows[0][0].Data != "music:pg:1" {
		t.Fatalf("覆盖后的键盘不符: %+v", kb2)
	}

	// Keyboard() 空参移除已有键盘段
	segs3 := Builder().Group().Keyboard(rows...).Keyboard().Build().GetGroupMsg()
	if len(segs3) != 0 {
		t.Fatalf("空参 Keyboard 应移除键盘段: %+v", segs3)
	}

	// 私聊链同样支持
	fsegs := Builder().Friend().Text("列表").Keyboard(Row(Button("下一页", "m:pg:2"))).Build().GetFriendMsg()
	if len(fsegs) != 2 || fsegs[1].Type != message.SegmentKeyboard {
		t.Fatalf("私聊链 Keyboard 不符: %+v", fsegs)
	}
}
