package feishu

import (
	"context"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func strPtr(s string) *string { return &s }

func receiveEvent(id string) *larkim.P2MessageReceiveV1 {
	return &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Sender: &larkim.EventSender{
				SenderType: strPtr("user"),
				SenderId:   &larkim.UserId{OpenId: strPtr("ou_sender")},
			},
			Message: &larkim.EventMessage{
				MessageId:   strPtr(id),
				ChatId:      strPtr("oc_chat"),
				ChatType:    strPtr("p2p"),
				MessageType: strPtr("text"),
				Content:     strPtr(`{"text":"hi"}`),
			},
		},
	}
}

// waitDeliver 等待一次异步分发，超时返回 false。
func waitDeliver(t *testing.T, ch <-chan struct{}, timeout time.Duration) bool {
	t.Helper()
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
}

// TestOnReceiveDedup 同一 message_id 的重复投递（飞书 at-least-once 重推）只触发一次插件链。
func TestOnReceiveDedup(t *testing.T) {
	a := NewAdapter(nil)
	delivered := make(chan struct{}, 4)
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(message.Message) { delivered <- struct{}{} },
	})

	ctx := context.Background()
	ev := receiveEvent("om_test_dedup")
	if err := a.onReceive(ctx, ev); err != nil {
		t.Fatalf("首次投递处理失败: %v", err)
	}
	if err := a.onReceive(ctx, ev); err != nil {
		t.Fatalf("重复投递处理失败: %v", err)
	}

	// 第一条应被异步分发
	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第一条消息应被分发")
	}
	// 重复投递不应再分发（去重窗口内）
	if waitDeliver(t, delivered, 500*time.Millisecond) {
		t.Fatal("重复投递不应再触发插件链")
	}
}

// TestMessageKey 验证 core 层去重键（adapter.EventKeyer.MessageKey）：去掉 fs: 前缀、以 msg: 为键。
func TestMessageKey(t *testing.T) {
	a := NewAdapter(nil)

	if key, ok := a.MessageKey(message.Message{MessageId: message.QID("fs:om_abc")}); !ok || key != "msg:om_abc" {
		t.Fatalf("MessageKey(fs:om_abc) = (%q,%v), want (msg:om_abc,true)", key, ok)
	}
	if key, ok := a.MessageKey(message.Message{MessageId: message.QID("")}); ok {
		t.Fatalf("空 MessageId 应返回 false, got key=%q", key)
	}
	// NoticeKey：撤回通知按 message_id 提供稳定键；其他通知类型返回 false
	if key, ok := a.NoticeKey("group_recall", message.GroupRecallNotice{MessageId: message.QID("fs:om_r1")}); !ok || key != "recall:om_r1" {
		t.Fatalf("NoticeKey(group_recall) = (%q,%v), want (recall:om_r1,true)", key, ok)
	}
	if key, ok := a.NoticeKey("group_recall", message.GroupRecallNotice{MessageId: message.QID("")}); ok {
		t.Fatalf("空 MessageId 的通知应返回 false, got key=%q", key)
	}
	if _, ok := a.NoticeKey("group_msg_emoji_like", message.GroupMsgEmojiLikeNotice{}); ok {
		t.Fatal("表情回应通知应返回 false（core 无可靠键，维持适配器级去重）")
	}
}

// TestResolveName 昵称解析：client==nil（测试/离线）不 panic 返回空串；缓存命中直接返回。
func TestResolveName(t *testing.T) {
	a := NewAdapter(nil)
	if name := a.resolveName("ou_xxx"); name != "" {
		t.Fatalf("client==nil 时应返回空串, got %q", name)
	}

	a.nameCache.Store("ou_cached", "小明")
	if name := a.resolveName("ou_cached"); name != "小明" {
		t.Fatalf("缓存命中应返回昵称, got %q", name)
	}
	if name := a.resolveName(""); name != "" {
		t.Fatalf("空 openID 应返回空串, got %q", name)
	}
}

// TestOnReceiveDifferentMessage 不同 message_id 不应互相去重。
func TestOnReceiveDifferentMessage(t *testing.T) {
	a := NewAdapter(nil)
	delivered := make(chan struct{}, 4)
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(message.Message) { delivered <- struct{}{} },
	})

	ctx := context.Background()
	_ = a.onReceive(ctx, receiveEvent("om_a"))
	_ = a.onReceive(ctx, receiveEvent("om_b"))

	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第一条消息应被分发")
	}
	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第二条消息应被分发")
	}
}
