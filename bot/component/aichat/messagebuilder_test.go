package aichat

import "testing"

func TestBuildImageContextMessage(t *testing.T) {
	builder := NewMessageBuilder("system prompt")
	urls := []string{"https://example.com/1.png", "https://example.com/2.png"}

	message := builder.BuildImageContextMessage(urls)
	if message.Role != RoleUser {
		t.Fatalf("Role = %q, want %q", message.Role, RoleUser)
	}
	if len(message.Parts) != 3 {
		t.Fatalf("len(Parts) = %d, want 3", len(message.Parts))
	}
	if message.Parts[0].Type != ContentPartText {
		t.Fatalf("first part type = %v, want text", message.Parts[0].Type)
	}
	for i, url := range urls {
		part := message.Parts[i+1]
		if part.Type != ContentPartImageURL {
			t.Fatalf("part %d type = %v, want image URL", i+1, part.Type)
		}
		if part.ImageURL != url {
			t.Fatalf("part %d URL = %q, want %q", i+1, part.ImageURL, url)
		}
	}
}
