package napcat

import (
	"encoding/base64"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

func TestStripQQSegments(t *testing.T) {
	chain := msgchain.Builder().Group().
		Mention(message.FromUint64(123)).
		Reply(message.FromUint64(456)).
		Build()
	got := stripQQSegments(chain.GetGroupMsg())
	if got[0].Data["qq"] != "123" {
		t.Fatalf("mention = %v, want 123", got[0].Data["qq"])
	}
	if got[1].Data["id"] != "456" {
		t.Fatalf("reply = %v, want 456", got[1].Data["id"])
	}
}

func TestStripQQForward(t *testing.T) {
	inner := msgchain.Builder().Group().
		Mention(message.FromUint64(789)).
		Build()
	forwardBuilder := msgchain.Builder().GroupForward()
	forwardBuilder.Message(message.FromUint64(321), "nick", inner)
	forward := forwardBuilder.Build()
	got := stripQQForward(forward.GetForwardMsg())
	if got.Messages[0].Data.UserId != "321" {
		t.Fatalf("node user = %q, want 321", got.Messages[0].Data.UserId)
	}
	if got.Messages[0].Data.Content[0].Data["qq"] != "789" {
		t.Fatalf("nested mention = %v, want 789", got.Messages[0].Data.Content[0].Data["qq"])
	}
}

// pngB64 一段最小合法 PNG 文件头的 base64（含 PNG 魔数）。
var pngB64 = base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\n0000000000000000"))

func TestStripQQSegmentsFileToImage(t *testing.T) {
	// base64 图片 + 图片扩展名 → 转 image 段
	got := stripQQSegments(msgchain.Builder().Group().
		FileBase64("photo.png", pngB64).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentImage {
		t.Fatalf("图片文件应转为 image 段, got %+v", got)
	}
	if got[0].Data["file"] != "base64://"+pngB64 || got[0].Data["url"] != "base64://"+pngB64 {
		t.Fatalf("image 段应保留 base64 源, got %+v", got[0].Data)
	}
	if got[0].Data["summary"] != "[图片]" {
		t.Fatalf("image 段 summary = %v, want [图片]", got[0].Data["summary"])
	}

	// 非图片扩展名 + 非图片内容 → 保持 file 段
	got = stripQQSegments(msgchain.Builder().Group().
		FileBase64("doc.pdf", base64.StdEncoding.EncodeToString([]byte("plain text"))).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentFile {
		t.Fatalf("非图片文件应保持 file 段, got %+v", got)
	}

	// 图片 URL（带查询参数）→ 转 image 段
	got = stripQQSegments(msgchain.Builder().Group().
		FileUrl("photo.png", "https://e.com/photo.png?token=1").
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentImage {
		t.Fatalf("图片 URL 应转为 image 段, got %+v", got)
	}
	if got[0].Data["file"] != "https://e.com/photo.png?token=1" {
		t.Fatalf("image 段应保留原始 URL, got %+v", got[0].Data["file"])
	}
}

func TestStripQQSegmentsFileSniff(t *testing.T) {
	// 文件名不带图片后缀但内容是 PNG → 按内容识别转 image
	got := stripQQSegments(msgchain.Builder().Group().
		FileBase64("data.bin", pngB64).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentImage {
		t.Fatalf("PNG 内容应转 image 段, got %+v", got)
	}

	// 文件名带图片后缀但内容不是图片 → 保持 file 段（避免图片接口发送失败）
	got = stripQQSegments(msgchain.Builder().Group().
		FileBase64("photo.png", base64.StdEncoding.EncodeToString([]byte("not-an-image"))).
		Build().GetGroupMsg())
	if len(got) != 1 || got[0].Type != message.SegmentFile {
		t.Fatalf("非图片内容不应转 image 段, got %+v", got)
	}

	// data:image/ 前缀直接判定为图片
	got = stripQQSegments([]message.OB11Segment{{
		Type: message.SegmentFile,
		Data: map[string]any{"file": "data:image/png;base64,AAAA", "name": "x.bin"},
	}})
	if len(got) != 1 || got[0].Type != message.SegmentImage {
		t.Fatalf("data:image 源应转 image 段, got %+v", got)
	}

	// 仅有 file_id 没有 file 源 → 保持 file 段（无法作为图片发送）
	got = stripQQSegments([]message.OB11Segment{{
		Type: message.SegmentFile,
		Data: message.FileMessage{FileId: "f1", Name: "a.png"}.Marshal(),
	}})
	if len(got) != 1 || got[0].Type != message.SegmentFile {
		t.Fatalf("仅 file_id 的段不应转 image, got %+v", got)
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
