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
