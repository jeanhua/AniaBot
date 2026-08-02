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
