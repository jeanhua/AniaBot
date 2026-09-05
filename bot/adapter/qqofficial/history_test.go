package qqofficial

import (
	"fmt"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// TestMsgCachePushFind 消息缓存：按 ID 反查与会话历史。
func TestMsgCachePushFind(t *testing.T) {
	c := newMsgCache(3, 10)
	m := message.Message{MessageId: "qo:M1", RawMessage: "hi"}
	c.Push("G1", m)

	if _, ok := c.Find("M1"); !ok {
		t.Fatal("Find 应命中")
	}
	if _, ok := c.Find("M2"); ok {
		t.Fatal("未缓存的 ID 不应命中")
	}
	hist := c.History("G1", 10)
	if len(hist) != 1 || hist[0].MessageId != "qo:M1" {
		t.Fatalf("History = %+v", hist)
	}
}

// TestMsgCacheEvictPerChat 单会话超上限淘汰最旧，ID 索引同步清理。
func TestMsgCacheEvictPerChat(t *testing.T) {
	c := newMsgCache(2, 10)
	for i := 1; i <= 3; i++ {
		c.Push("G1", message.Message{MessageId: message.QID(fmt.Sprintf("qo:M%d", i))})
	}
	hist := c.History("G1", 10)
	if len(hist) != 2 || hist[0].MessageId != "qo:M3" || hist[1].MessageId != "qo:M2" {
		t.Fatalf("History = %+v", hist)
	}
	if _, ok := c.Find("M1"); ok {
		t.Fatal("被淘汰的消息应从 ID 索引清除")
	}
}

// TestMsgCacheEvictChats 会话数超上限淘汰最久未更新的会话。
func TestMsgCacheEvictChats(t *testing.T) {
	c := newMsgCache(5, 2)
	base := time.Now()
	c.now = func() time.Time { return base }
	c.Push("G1", message.Message{MessageId: "qo:A"})
	c.now = func() time.Time { return base.Add(time.Second) }
	c.Push("G2", message.Message{MessageId: "qo:B"})
	c.now = func() time.Time { return base.Add(2 * time.Second) }
	c.Push("G3", message.Message{MessageId: "qo:C"})
	if len(c.msgs) != 2 {
		t.Fatalf("会话数 = %d, want 2", len(c.msgs))
	}
	if _, ok := c.Find("A"); ok {
		t.Fatal("最久未更新的会话应被淘汰")
	}
}

// TestGetMsgDetail 适配器查询走缓存，非本平台 ID 拒绝。
func TestGetMsgDetail(t *testing.T) {
	a := NewAdapter(nil)
	a.msgCache.Push("G1", message.Message{MessageId: "qo:M1"})
	if _, ok := a.GetMsgDetail("qo:M1"); !ok {
		t.Fatal("应命中缓存")
	}
	if _, ok := a.GetMsgDetail("123456"); ok {
		t.Fatal("非 qo: 前缀 ID 应拒绝")
	}
	if _, ok := a.GetMsgDetail("qo:M9"); ok {
		t.Fatal("未命中应返回 false")
	}
}

// TestHistoryFromCache 历史查询：默认 20 条、空缓存返回 false。
func TestHistoryFromCache(t *testing.T) {
	a := NewAdapter(nil)
	if _, ok := a.GetGroupMsgHistory("qo:G1", 10, 0); ok {
		t.Fatal("空缓存应返回 false")
	}
	for i := range 5 {
		a.msgCache.Push("G1", message.Message{MessageId: message.QID(fmt.Sprintf("qo:M%d", i))})
	}
	msgs, ok := a.GetGroupMsgHistory("qo:G1", 3, 0)
	if !ok || len(*msgs) != 3 {
		t.Fatalf("History = %d/%v", len(*msgs), ok)
	}
	if _, ok := a.GetGroupMsgHistory("123456", 3, 0); ok {
		t.Fatal("非 qo: 前缀群 ID 应拒绝")
	}
	if _, ok := a.GetGroupDetail("qo:G1"); ok {
		t.Fatal("官方无群资料接口，GetGroupDetail 应返回 false")
	}
}

// TestCacheSentStripsBase64 出站 base64 图片入缓存前剔除负载，GetMsgDetail 不再持有大图。
func TestCacheSentStripsBase64(t *testing.T) {
	a := NewAdapter(nil)
	segs := []message.OB11Segment{{
		Type: message.SegmentImage,
		Data: message.ImageMessage{File: "base64://AAAA", Url: "base64://AAAA", Summary: "[图片]"}.Marshal(),
	}}
	a.cacheSent("G1", true, "M1", segs)

	got, ok := a.GetMsgDetail("qo:M1")
	if !ok || len(got.Message) != 1 {
		t.Fatalf("GetMsgDetail = (%v, %v), want 1 条缓存消息", got, ok)
	}
	if _, has := got.Message[0].Data["file"]; has {
		t.Fatal("缓存中的 base64 file 未被剔除")
	}
	if _, has := got.Message[0].Data["url"]; has {
		t.Fatal("缓存中的 base64 url 未被剔除")
	}
	if _, has := segs[0].Data["file"]; !has {
		t.Fatal("原出站段被修改，base64 file 不应丢失")
	}
}
