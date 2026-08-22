package message

import (
	"strings"
	"testing"
)

func TestImageHashStableAndDistinct(t *testing.T) {
	const url = "https://gchat.qpic.cn/download?fileid=abc123"
	if ImageHash(url) != ImageHash(url) {
		t.Fatal("同一引用的哈希应稳定")
	}
	if len(ImageHash(url)) != 8 {
		t.Fatalf("哈希长度 = %d, want 8", len(ImageHash(url)))
	}
	if ImageHash(url) == ImageHash("https://gchat.qpic.cn/download?fileid=xyz789") {
		t.Fatal("不同图片的哈希应不同")
	}
}

func TestImageMessageHashFallbackToFile(t *testing.T) {
	img := ImageMessage{File: "a.png"}
	if img.Hash() != ImageHash("a.png") {
		t.Fatal("URL 为空时应退化为文件名哈希")
	}
	img.Url = "https://example.com/a.png"
	if img.Hash() != ImageHash("https://example.com/a.png") {
		t.Fatal("应优先使用 URL 哈希")
	}
}

func TestFriendlyTextImageHashMark(t *testing.T) {
	const url = "https://gchat.qpic.cn/download?fileid=abc123"
	raw := Message{
		Message: []OB11Segment{
			{Type: SegmentImage, Data: map[string]any{"url": url, "file": "a.png"}},
		},
	}

	text := raw.FriendlyText(true, WithNoSenderPrefix())
	want := "[图片 " + ImageHash(url) + " url:" + url + "]"
	if !strings.Contains(text, want) {
		t.Fatalf("FriendlyText 应同时包含哈希与 URL, got %q", text)
	}

	// showUrl=false 时保持无哈希的简式标记
	raw.Message = []OB11Segment{
		{Type: SegmentImage, Data: map[string]any{"url": url, "file": "a.png"}},
	}
	text = raw.FriendlyText(false, WithNoSenderPrefix())
	if !strings.Contains(text, "[图片]") {
		t.Fatalf("showUrl=false 应为 [图片], got %q", text)
	}
}
