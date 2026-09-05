package message

import (
	"reflect"
	"testing"
)

// TestStripInlinePayloadSegments 内联 base64/data 负载被剔除，http(s) URL 与
// file:// 等轻量引用保留，且不修改原段。
func TestStripInlinePayloadSegments(t *testing.T) {
	base64Img := ImageMessage{File: "base64://" + dummyB64(), Url: "base64://" + dummyB64(), Summary: "[图片]"}.Marshal()
	httpImg := ImageMessage{File: "https://example.com/a.png", Url: "https://example.com/a.png", Summary: "[图片]"}.Marshal()
	localImg := ImageMessage{File: "file:///tmp/a.png", Url: "file:///tmp/a.png", Summary: "[图片]"}.Marshal()
	base64File := FileMessage{File: "base64://" + dummyB64(), FileId: "file_id_1", Name: "a.png", dataMessage: dataMessage{URL: "https://example.com/a.png"}}.Marshal()
	dataVideo := VideoMessage{URL: "data:video/mp4;base64,AAAA"}.Marshal()
	dataRecord := RecordMessage{URL: "data:audio/ogg;base64,BBBB"}.Marshal()
	text := TextMessage{Text: "正文"}.Marshal()

	segs := []OB11Segment{
		{Type: SegmentText, Data: text},
		{Type: SegmentImage, Data: base64Img},
		{Type: SegmentImage, Data: httpImg},
		{Type: SegmentImage, Data: localImg},
		{Type: SegmentFile, Data: base64File},
		{Type: SegmentVideo, Data: dataVideo},
		{Type: SegmentRecord, Data: dataRecord},
	}
	got := StripInlinePayloadSegments(segs)
	if len(got) != len(segs) {
		t.Fatalf("长度 = %d, want %d", len(got), len(segs))
	}

	// 原段必须不受影响
	if _, ok := segs[1].Data["file"]; !ok {
		t.Fatal("原 image 段被修改了")
	}

	// 文本段原样保留
	if got[0].Data["text"] != "正文" {
		t.Fatal("文本段被意外修改")
	}

	// base64 图片：file/url 都被删除，summary 保留
	img := got[1]
	if _, ok := img.Data["file"]; ok {
		t.Fatal("base64 图片段仍保留 file")
	}
	if _, ok := img.Data["url"]; ok {
		t.Fatal("base64 图片段仍保留 url")
	}
	if img.Data["summary"] != "[图片]" {
		t.Fatalf("summary = %v", img.Data["summary"])
	}

	// http(s) URL 与 file:// 路径保留
	if got[2].Data["url"] != "https://example.com/a.png" {
		t.Fatal("http 图片 url 被误删")
	}
	if got[3].Data["url"] != "file:///tmp/a.png" {
		t.Fatal("file:// 图片 url 被误删")
	}

	// 文件段：只删内联 file，保留 file_id/name/url
	f := got[4]
	if _, ok := f.Data["file"]; ok {
		t.Fatal("base64 文件段仍保留 file")
	}
	if f.Data["file_id"] != "file_id_1" || f.Data["name"] != "a.png" || f.Data["url"] != "https://example.com/a.png" {
		t.Fatalf("文件段轻量字段被误删: %+v", f.Data)
	}

	// data: 视频/录音负载被删除
	if _, ok := got[5].Data["url"]; ok {
		t.Fatal("data: 视频 url 未删除")
	}
	if _, ok := got[6].Data["url"]; ok {
		t.Fatal("data: 录音 url 未删除")
	}
}

// TestStripInlinePayloadSegmentsNoChangeSharesMap 无需剔除的媒体段与原段共享 Data，
// 不产生无谓的大内存复制。
func TestStripInlinePayloadSegmentsNoChangeSharesMap(t *testing.T) {
	segs := []OB11Segment{
		{Type: SegmentImage, Data: ImageMessage{Url: "https://example.com/a.png"}.Marshal()},
	}
	got := StripInlinePayloadSegments(segs)
	if got[0].Data == nil || reflect.ValueOf(got[0].Data).Pointer() != reflect.ValueOf(segs[0].Data).Pointer() {
		t.Fatal("无负载的段应共享原 Data，避免多余复制")
	}
}

func dummyB64() string {
	// 24 字节 padding 后的内容，仅用于验证前缀识别
	return "AAAA"
}
