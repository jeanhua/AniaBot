package message

import (
	"encoding/base64"
	"testing"
)

// pngB64 一段最小合法 PNG 文件头的 base64（含 PNG 魔数）。
var pngB64 = base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n0000000000000000"))

func TestFileSegmentAsImage(t *testing.T) {
	// base64 图片 + 图片扩展名 → 转 image 段数据
	img, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{File: "base64://" + pngB64, Name: "photo.png"}.Marshal(),
	})
	if !ok {
		t.Fatal("图片文件应可转 image 段")
	}
	if img["file"] != "base64://"+pngB64 || img["url"] != "base64://"+pngB64 {
		t.Fatalf("image 段应保留 base64 源, got %+v", img)
	}
	if img["summary"] != "[图片]" {
		t.Fatalf("image 段 summary = %v, want [图片]", img["summary"])
	}

	// 非图片扩展名 + 非图片内容 → 保持 file 段
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{File: "base64://" + base64.StdEncoding.EncodeToString([]byte("plain text")), Name: "doc.pdf"}.Marshal(),
	}); ok {
		t.Fatal("非图片文件不应转 image 段")
	}

	// 图片 URL（带查询参数）→ 转 image 段数据，保留原始 URL
	img, ok = FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{File: "https://e.com/photo.png?token=1", Name: "photo.png"}.Marshal(),
	})
	if !ok {
		t.Fatal("图片 URL 应可转 image 段")
	}
	if img["file"] != "https://e.com/photo.png?token=1" {
		t.Fatalf("image 段应保留原始 URL, got %v", img["file"])
	}

	// 文件名不带图片后缀但内容是 PNG → 按内容识别转 image
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{File: "base64://" + pngB64, Name: "data.bin"}.Marshal(),
	}); !ok {
		t.Fatal("PNG 内容应转 image 段")
	}

	// 文件名带图片后缀但内容不是图片 → 保持 file 段（避免图片接口发送失败）
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{File: "base64://" + base64.StdEncoding.EncodeToString([]byte("not-an-image")), Name: "photo.png"}.Marshal(),
	}); ok {
		t.Fatal("非图片内容不应转 image 段")
	}

	// data:image/ 前缀直接判定为图片
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: map[string]any{"file": "data:image/png;base64,AAAA", "name": "x.bin"},
	}); !ok {
		t.Fatal("data:image 源应转 image 段")
	}

	// 仅有 file_id 没有 file 源 → 保持 file 段（无法作为图片发送）
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentFile,
		Data: FileMessage{FileId: "f1", Name: "a.png"}.Marshal(),
	}); ok {
		t.Fatal("仅 file_id 的段不应转 image")
	}

	// 非 file 段不转换
	if _, ok := FileSegmentAsImage(OB11Segment{
		Type: SegmentImage,
		Data: ImageMessage{File: "base64://" + pngB64}.Marshal(),
	}); ok {
		t.Fatal("非 file 段不应转换")
	}

	// Data 为 nil 不转换
	if _, ok := FileSegmentAsImage(OB11Segment{Type: SegmentFile}); ok {
		t.Fatal("无数据的 file 段不应转换")
	}
}

func TestSniffImageMagic(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nxxx"), true},
		{"jpeg", []byte{0xff, 0xd8, 0xff, 0xe0}, true},
		{"gif", []byte("GIF89a"), true},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), true},
		{"bmp", []byte("BM\x00\x00"), true},
		{"text", []byte("hello world"), false},
		{"short", []byte{0xff, 0xd8}, false},
	}
	for _, c := range cases {
		if got := sniffImageMagic(c.data); got != c.want {
			t.Errorf("%s: sniffImageMagic = %v, want %v", c.name, got, c.want)
		}
	}
}
