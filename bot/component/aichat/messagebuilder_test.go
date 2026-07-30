package aichat

import (
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func TestBuildImageContextMessage(t *testing.T) {
	builder := NewMessageBuilder("system prompt")
	urls := []string{"https://example.com/1.png", "https://example.com/2.png"}

	msg := builder.BuildImageContextMessage(urls)
	if msg.Role != RoleUser {
		t.Fatalf("Role = %q, want %q", msg.Role, RoleUser)
	}
	// 1 个引导文本 + 每张图片各 1 个哈希标签与 1 个图片片段
	if len(msg.Parts) != 1+2*len(urls) {
		t.Fatalf("len(Parts) = %d, want %d", len(msg.Parts), 1+2*len(urls))
	}
	if msg.Parts[0].Type != ContentPartText {
		t.Fatalf("first part type = %v, want text", msg.Parts[0].Type)
	}
	for i, url := range urls {
		label := msg.Parts[1+2*i]
		wantLabel := "[图片 " + message.ImageHash(url) + "]"
		if label.Type != ContentPartText || label.Text != wantLabel {
			t.Fatalf("part %d 哈希标签 = %+v, want text %q", 1+2*i, label, wantLabel)
		}
		part := msg.Parts[2+2*i]
		if part.Type != ContentPartImageURL {
			t.Fatalf("part %d type = %v, want image URL", 2+2*i, part.Type)
		}
		if part.ImageURL != url {
			t.Fatalf("part %d URL = %q, want %q", 2+2*i, part.ImageURL, url)
		}
	}
}
