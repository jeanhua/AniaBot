package telegram

import (
	"fmt"
	"testing"

	"github.com/jeanhua/AniaBot/common/model/message"
)

func cachedMsg(chatID int64, mid int, text string) message.Message {
	return message.Message{
		MessageId:  msgID(chatID, mid),
		Message:    []message.OB11Segment{{Type: message.SegmentText, Data: map[string]any{"text": text}}},
		RawMessage: text,
	}
}

// TestMsgCachePushFind 入站+出站缓存、Find 命中/未命中。
func TestMsgCachePushFind(t *testing.T) {
	c := newMsgCache(msgCachePerChat, msgCacheMaxChats)
	c.Push("-100", cachedMsg(-100, 1, "a"))
	c.Push("-100", cachedMsg(-100, 2, "b"))

	if m, ok := c.Find("-100", 2); !ok || m.RawMessage != "b" {
		t.Fatalf("Find(2) = (%v,%v), want b", m, ok)
	}
	if _, ok := c.Find("-100", 99); ok {
		t.Fatal("未命中不应返回")
	}
	if _, ok := c.Find("111", 1); ok {
		t.Fatal("其他会话不应命中")
	}
	// History 最新在前
	msgs := c.History("-100", 0)
	if len(msgs) != 2 || msgs[0].RawMessage != "b" || msgs[1].RawMessage != "a" {
		t.Fatalf("History = %+v, want [b a]", msgs)
	}
}

// TestMsgCacheCap 每会话消息数上限淘汰最旧；会话数上限淘汰最久未更新。
func TestMsgCacheCap(t *testing.T) {
	c := newMsgCache(3, 2)
	for i := 0; i < 5; i++ {
		c.Push("-100", cachedMsg(-100, i+1, fmt.Sprintf("m%d", i+1)))
	}
	if msgs := c.History("-100", 0); len(msgs) != 3 || msgs[0].RawMessage != "m5" {
		t.Fatalf("每会话上限淘汰失败: %+v", msgs)
	}
	// 会话数上限：-100 与 111 先入，222 入后淘汰最旧的 -100
	c.Push("111", cachedMsg(111, 1, "x"))
	c.Push("222", cachedMsg(222, 1, "y"))
	if _, ok := c.Find("-100", 5); ok {
		t.Fatal("超出会话数上限应淘汰最久未更新的会话")
	}
	if _, ok := c.Find("111", 1); !ok {
		t.Fatal("较新的会话应保留")
	}
}

// TestGetMsgDetail 从缓存取消息详情（QID 解析路径）。
func TestGetMsgDetail(t *testing.T) {
	a := testAdapter()
	a.msgCache.Push("-100", cachedMsg(-100, 7, "hello"))

	if m, ok := a.GetMsgDetail(message.QID("tg:-100:7")); !ok || m.RawMessage != "hello" {
		t.Fatalf("GetMsgDetail(tg:-100:7) = (%v,%v), want hello", m, ok)
	}
	if _, ok := a.GetMsgDetail(message.QID("tg:-100:8")); ok {
		t.Fatal("未命中应返回 false")
	}
	if _, ok := a.GetMsgDetail(message.QID("qq:123")); ok {
		t.Fatal("非 tg: 前缀应返回 false")
	}
}

// TestGetGroupDetailNilClient client 为 nil 时不 panic 返回 false。
func TestGetGroupDetailNilClient(t *testing.T) {
	a := NewAdapter(nil)
	if _, ok := a.GetGroupDetail(message.QID("tg:-100")); ok {
		t.Fatal("client 为 nil 应返回 false")
	}
}

// TestHistoryFromCache 历史查询走缓存；空缓存返回 false。
func TestHistoryFromCache(t *testing.T) {
	a := testAdapter()
	if _, ok := a.GetGroupMsgHistory(message.QID("tg:-100"), 10, 0); ok {
		t.Fatal("空缓存应返回 false")
	}
	a.msgCache.Push("-100", cachedMsg(-100, 1, "a"))
	a.msgCache.Push("-100", cachedMsg(-100, 2, "b"))
	msgs, ok := a.GetGroupMsgHistory(message.QID("tg:-100"), 1, 0)
	if !ok || len(*msgs) != 1 || (*msgs)[0].RawMessage != "b" {
		t.Fatalf("GetGroupMsgHistory = (%v,%v), want 1 条且最新", msgs, ok)
	}
	// 私聊历史：chat_id 即 user_id
	msgs, ok = a.GetFriendMsgHistory(message.QID("tg:111"), 0, 0)
	if ok || msgs != nil {
		t.Fatal("无该会话缓存应返回 false")
	}
}
