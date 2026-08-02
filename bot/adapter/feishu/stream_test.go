package feishu

import (
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

// TestSendStreamNoClient client==nil（测试/离线）时流式创建返回 false，不 panic。
func TestSendStreamNoClient(t *testing.T) {
	a := NewAdapter(nil)
	if h, ok := a.SendGroupStream(message.QID("fs:oc_chat"), msgchain.Builder().Group().Text("hi").Build()); ok || h != nil {
		t.Fatalf("client==nil 时应返回 (nil,false), got (%v,%v)", h, ok)
	}
	// 私聊（openID 为空）直接 false
	if h, ok := a.SendFriendStream(message.QID(""), msgchain.Builder().Friend().Text("hi").Build()); ok || h != nil {
		t.Fatalf("空 openID 时应返回 (nil,false), got (%v,%v)", h, ok)
	}
}

// TestStreamHandleLifecycle 句柄状态机：End 幂等、End 后 Patch 为 no-op。
func TestStreamHandleLifecycle(t *testing.T) {
	a := NewAdapter(nil) // client==nil：patchLocked 提前返回，不 panic
	h := &feishuStreamHandle{a: a, msgID: "om_1", content: "初始"}

	if err := h.Patch("更新"); err != nil {
		t.Fatalf("Patch 不应报错（client==nil 时跳过发送）: %v", err)
	}
	h.End()
	h.End() // 幂等
	if err := h.Patch("结束后"); err != nil {
		t.Fatalf("End 后 Patch 应为 no-op 不报错: %v", err)
	}
	if !h.closed {
		t.Fatal("End 后 closed 应为 true")
	}
}

// TestStreamHandlePrefixPreserved 提及前缀在每次 Patch 时保留（卡片内容替换不丢 @）。
func TestStreamHandlePrefixPreserved(t *testing.T) {
	h := &feishuStreamHandle{prefix: `<at user_id="ou_1"></at>`}
	h.content = "正文"
	content := buildCardJSON(h.prefix + h.content)
	if !strings.Contains(content, `ou_1`) || !strings.Contains(content, "正文") {
		t.Fatalf("Patch 内容应同时含提及前缀与正文, got %s", content)
	}
}
