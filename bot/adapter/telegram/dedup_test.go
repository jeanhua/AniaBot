package telegram

import (
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// TestUpdateDedupClaim 首次 claim 成功、重复失败、TTL 过期后可再 claim。
func TestUpdateDedupClaim(t *testing.T) {
	d := newUpdateDedup(50 * time.Millisecond)
	if !d.Claim(1) {
		t.Fatal("首次 claim 应成功")
	}
	if d.Claim(1) {
		t.Fatal("重复 claim 应失败")
	}
	if !d.Claim(2) {
		t.Fatal("不同 update_id 应互不影响")
	}
	time.Sleep(80 * time.Millisecond)
	if !d.Claim(1) {
		t.Fatal("TTL 过期后应可重新 claim")
	}
}

// TestHandleUpdateDedup 同一 update_id 的重复投递只触发一次回调。
func TestHandleUpdateDedup(t *testing.T) {
	a := testAdapter()
	delivered := make(chan struct{}, 4)
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(message.Message) { delivered <- struct{}{} },
	})

	u := &Update{UpdateID: 7, Message: textMsg(111, "private", 222, "hi")}
	a.processUpdates([]Update{*u}, 0)
	a.processUpdates([]Update{*u}, 0)

	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第一条消息应被分发")
	}
	if waitDeliver(t, delivered, 500*time.Millisecond) {
		t.Fatal("重复投递不应再触发插件链")
	}
}

// TestProcessUpdatesOffset 重复投递的 update 也要推进 offset（否则死循环重推）。
func TestProcessUpdatesOffset(t *testing.T) {
	a := NewAdapter(nil)
	offset := a.processUpdates([]Update{{UpdateID: 1}, {UpdateID: 2}, {UpdateID: 2}, {UpdateID: 3}}, 0)
	if offset != 4 {
		t.Fatalf("offset = %d, want 4（重复的 update_id=2 也要推进）", offset)
	}
	// 再次投递同一批：全部被去重，offset 保持
	if offset = a.processUpdates([]Update{{UpdateID: 1}, {UpdateID: 3}}, offset); offset != 4 {
		t.Fatalf("重推后 offset = %d, want 4", offset)
	}
}

// TestUpdateDedupDifferentUpdates 不同 update_id 不互相去重。
func TestUpdateDedupDifferentUpdates(t *testing.T) {
	a := testAdapter()
	delivered := make(chan struct{}, 4)
	a.SetTrigger(adapter.TriggerWrapper{
		OnFriendMsg: func(message.Message) { delivered <- struct{}{} },
	})
	a.processUpdates([]Update{{UpdateID: 1, Message: textMsg(111, "private", 222, "a")}, {UpdateID: 2, Message: textMsg(111, "private", 222, "b")}}, 0)
	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第一条消息应被分发")
	}
	if !waitDeliver(t, delivered, 2*time.Second) {
		t.Fatal("第二条消息应被分发")
	}
}
