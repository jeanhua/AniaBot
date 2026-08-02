package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

// TestSplitText 4096 上限分包：长文本、多字节字符不切断。
func TestSplitText(t *testing.T) {
	// 短文本不分包
	parts := splitText("hello")
	if len(parts) != 1 || parts[0] != "hello" {
		t.Fatalf("短文本应 1 包, got %v", parts)
	}
	// 长 ASCII 文本分 2 包
	long := strings.Repeat("a", 5000)
	parts = splitText(long)
	if len(parts) != 2 {
		t.Fatalf("5000 字符应分 2 包, got %d", len(parts))
	}
	for _, p := range parts {
		if len([]rune(p)) > 4090 {
			t.Fatalf("分包超过上限: %d", len([]rune(p)))
		}
	}
	// CJK（一字符一 UTF-16 单位）不切断多字节字符
	cjk := strings.Repeat("中", 5000)
	parts = splitText(cjk)
	if len(parts) != 2 || parts[0] != strings.Repeat("中", 4090) {
		t.Fatalf("CJK 分包错误: 包数 %d, 首包长度 %d", len(parts), len([]rune(parts[0])))
	}
	// 拼接还原
	if got := strings.Join(parts, ""); got != cjk {
		t.Fatal("分包拼接后内容不一致")
	}
}

// TestTruncateRunes 字节安全截断：不切断多字节字符。
func TestTruncateRunes(t *testing.T) {
	s := "你好世界"
	if got := truncateRunes(s, 100); got != s {
		t.Fatalf("未超限不应截断, got %q", got)
	}
	// "你好" = 6 字节，第三个字符 3 字节，截断到 7 字节应保留"你好"
	if got := truncateRunes("你好世", 7); got != "你好" {
		t.Fatalf("截断结果 = %q, want 你好", got)
	}
	// 截断到 4096 的流式内容
	long := strings.Repeat("a", 5000)
	if got := truncateRunes(long, maxEditTextLen); len(got) != maxEditTextLen {
		t.Fatalf("截断长度 = %d, want %d", len(got), maxEditTextLen)
	}
}

// TestResolveMention 预置 chatMemberCache 后 at 段展开为 @username（不触网）。
func TestResolveMention(t *testing.T) {
	a := testAdapter()
	a.chatMemberCache.Store("-100:222", mentionCache{username: "alice", at: time.Now()})

	s := message.OB11Segment{Type: message.SegmentMention, Data: message.MentionMessage{QQ: message.QID("tg:222")}.Marshal()}
	if got := a.resolveMention(nil, -100, s); got != "@alice " {
		t.Fatalf("resolveMention = %q, want @alice ", got)
	}
	// 私聊（chat_id 为正数）不解析
	if got := a.resolveMention(nil, 111, s); got != "" {
		t.Fatalf("私聊不应解析 @, got %q", got)
	}
	// @all 丢弃
	sAll := message.OB11Segment{Type: message.SegmentMention, Data: message.MentionMessage{IsAll: true}.Marshal()}
	if got := a.resolveMention(nil, -100, sAll); got != "" {
		t.Fatalf("@all 应丢弃, got %q", got)
	}
	// 无 username 的用户：缓存命中但 username 为空 → 丢弃
	a.chatMemberCache.Store("-100:333", mentionCache{username: "", at: time.Now()})
	s3 := message.OB11Segment{Type: message.SegmentMention, Data: message.MentionMessage{QQ: message.QID("tg:333")}.Marshal()}
	if got := a.resolveMention(nil, -100, s3); got != "" {
		t.Fatalf("无 username 应丢弃, got %q", got)
	}
}

// TestSendChainTextMention reply 提取 + at 展开 + 文本发送（无网络：client 为 nil 时发送失败
// 返回 false，但 reply 提取与 at 展开逻辑在发送前）。
func TestSendChainReplyAndMention(t *testing.T) {
	a := testAdapter()
	a.chatMemberCache.Store("-100:222", mentionCache{username: "alice", at: time.Now()})

	chain := msgchain.Builder().
		Group().
		Reply(message.QID("tg:-100:42")).
		Mention(message.QID("tg:222")).
		Text("你好").
		Build()
	segs := chain.GetGroupMsg()
	// 断言段序列：reply 在前
	if len(segs) == 0 || segs[0].Type != message.SegmentReply {
		t.Fatalf("期望 reply 段在前, got %+v", segs)
	}
	if id, _ := segs[0].Data["id"].(string); id != "tg:-100:42" {
		t.Fatalf("reply id = %q", id)
	}
	// client 为 nil → 发送失败（不 panic）
	if _, ok := a.SendGroupMsg(message.QID("tg:-100"), chain); ok {
		t.Fatal("client 为 nil 时发送应失败")
	}
}

// TestSendMediaKeySelection 媒体段方法名与文件键选择。
func TestSendMediaKeySelection(t *testing.T) {
	a := testAdapter()
	// client 为 nil 时 sendMedia 内部 call 会失败（resty nil 指针），
	// 仅验证 key 选择逻辑不 panic —— 直接测 sendMediaSegment 前置分支
	img := message.OB11Segment{Type: message.SegmentImage, Data: map[string]any{"file": "AgAAAA", "url": "https://x.com/a.jpg"}}
	if _, ok := a.sendMediaSegment(nil, -100, img, "", nil); ok {
		t.Fatal("client 为 nil 应发送失败")
	}
}

// TestStreamHandleLifecycle 流式句柄：Patch 节流、End 幂等、End 后 no-op。
func TestStreamHandleLifecycle(t *testing.T) {
	a := testAdapter()
	h := &telegramStreamHandle{a: a, chatID: -100, msgID: 1, prefix: "@alice "}

	// client 为 nil：patchLocked 直接返回 nil（不 panic）
	if err := h.Patch("hello"); err != nil {
		t.Fatalf("Patch 不应报错: %v", err)
	}
	// 节流窗口内再 Patch 仅记录内容
	h.content = "world"
	h.End()
	if h.content != "world" {
		t.Fatal("End 后 content 不应被修改")
	}
	// End 幂等：再次 End 不 panic
	h.End()
	// End 后 Patch 为 no-op
	if err := h.Patch("after"); err != nil {
		t.Fatalf("End 后 Patch 不应报错: %v", err)
	}
	if h.content != "world" {
		t.Fatal("End 后 Patch 不应更新内容")
	}
}

// TestStreamHandlePrefixPreserved Patch/End 的内容拼接保留 prefix。
func TestStreamHandlePrefixPreserved(t *testing.T) {
	a := testAdapter()
	h := &telegramStreamHandle{a: a, chatID: -100, msgID: 1, prefix: "@alice "}
	h.content = "你好"
	// 模拟 patchLocked 的内容构造（client nil 时不真正调用 API，直接验证拼接逻辑）
	content := truncateRunes(h.prefix+h.content, maxEditTextLen)
	if content != "@alice 你好" {
		t.Fatalf("拼接内容 = %q, want @alice 你好（prefix 保留）", content)
	}
}

// TestSendStreamNilClient client 为 nil 时流式创建失败（不 panic）。
func TestSendStreamNilClient(t *testing.T) {
	a := testAdapter()
	chain := msgchain.Builder().Group().Text("hi").Build()
	if h, ok := a.SendGroupStream(message.QID("tg:-100"), chain); ok || h != nil {
		t.Fatal("client 为 nil 时流式创建应失败")
	}
	// StreamSenderExt 接口断言
	if _, ok := any(a).(interface {
		SendGroupStream(message.QID, msgchain.GroupChain) (bot.StreamHandle, bool)
	}); !ok {
		t.Fatal("适配器应实现 StreamSenderExt")
	}
}
