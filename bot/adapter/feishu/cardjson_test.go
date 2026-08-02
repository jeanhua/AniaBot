package feishu

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildCardJSON 卡片结构：schema 2.0 + markdown 元素。
func TestBuildCardJSON(t *testing.T) {
	raw := buildCardJSON("你好 **世界**")
	var card struct {
		Schema   string `json:"schema"`
		Elements []struct {
			Tag     string `json:"tag"`
			Content string `json:"content"`
		} `json:"elements"`
	}
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatalf("卡片 JSON 解析失败: %v (%s)", err, raw)
	}
	if card.Schema != "2.0" {
		t.Fatalf("schema = %q, want 2.0", card.Schema)
	}
	if len(card.Elements) != 1 || card.Elements[0].Tag != "markdown" || card.Elements[0].Content != "你好 **世界**" {
		t.Fatalf("卡片元素不符: %+v", card.Elements)
	}
}

// TestTruncateMarkdown 超限截断：字节安全（不截断多字节字符）、追加省略号。
func TestTruncateMarkdown(t *testing.T) {
	short := "短内容"
	if got := truncateMarkdown(short, 100); got != short {
		t.Fatalf("未超限不应截断, got %q", got)
	}

	// 中文 3 字节/字符，限 5 字节 → 截到 1 个汉字 + 省略号
	got := truncateMarkdown("你好世界", 5)
	if got != "你…" {
		t.Fatalf("字节安全截断不符, got %q", got)
	}

	// 长内容截断后总长不超过上限 + 省略号
	big := strings.Repeat("很长的内容", 5000) // 6 万字节
	truncated := truncateMarkdown(big, cardContentLimit)
	if len(truncated) > cardContentLimit+3 {
		t.Fatalf("截断后仍超限: %d > %d", len(truncated), cardContentLimit)
	}
	if !strings.HasSuffix(truncated, "…") {
		t.Fatalf("截断后应追加省略号")
	}
}
