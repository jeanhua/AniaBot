package telegram

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
)

// TestBuildReplyMarkup 框架键盘 → reply_markup JSON：回调按钮映射 callback_data，
// 链接按钮映射 url，超长回调数据的按钮被丢弃（避免 Bot API 400 拒绝整个键盘）。
func TestBuildReplyMarkup(t *testing.T) {
	a := NewAdapter(nil)
	long := strings.Repeat("x", 65)
	kb := &message.KeyboardMessage{Rows: [][]message.InlineButton{
		{msgchain.Button("◀️ 上一页", "music:pg:1"), msgchain.Button("▶️ 下一页", "music:pg:3")},
		{msgchain.ButtonURL("网页版", "https://example.com")},
		{msgchain.Button("超长丢弃", long)},
	}}
	got := a.buildReplyMarkup(kb)
	if got == "" {
		t.Fatal("有效键盘不应返回空")
	}
	var rm tgReplyMarkup
	if err := json.Unmarshal([]byte(got), &rm); err != nil {
		t.Fatalf("reply_markup 应为合法 JSON: %v", err)
	}
	if len(rm.InlineKeyboard) != 2 {
		t.Fatalf("应为 2 行（仅含超长按钮的行被跳过）: %+v", rm)
	}
	if rm.InlineKeyboard[0][0].CallbackData != "music:pg:1" || rm.InlineKeyboard[0][1].CallbackData != "music:pg:3" {
		t.Fatalf("回调按钮映射不符: %+v", rm.InlineKeyboard[0])
	}
	if rm.InlineKeyboard[1][0].URL != "https://example.com" || rm.InlineKeyboard[1][0].CallbackData != "" {
		t.Fatalf("链接按钮映射不符: %+v", rm.InlineKeyboard[1])
	}

	// 全部无效 → 空串（不携带 reply_markup 参数）
	empty := a.buildReplyMarkup(&message.KeyboardMessage{Rows: [][]message.InlineButton{
		{msgchain.Button("超长", long)},
	}})
	if empty != "" {
		t.Fatalf("全无效键盘应返回空串: %q", empty)
	}
	if a.buildReplyMarkup(nil) != "" {
		t.Fatal("nil 键盘应返回空串")
	}
}

// TestUpdateCallbackQueryUnmarshal 长轮询 Update 能解析 callback_query 更新。
func TestUpdateCallbackQueryUnmarshal(t *testing.T) {
	raw := []byte(`{"update_id":7,"callback_query":{
		"id":"cq1","from":{"id":42,"is_bot":false,"first_name":"U"},
		"message":{"message_id":9,"date":1,"chat":{"id":-100123,"type":"supergroup","title":"G"}},
		"data":"音乐点歌:pg:2"}}`)
	var u Update
	if err := json.Unmarshal(raw, &u); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	cq := u.CallbackQuery
	if cq == nil || cq.ID != "cq1" || cq.From.ID != 42 || cq.Data != "音乐点歌:pg:2" {
		t.Fatalf("callback_query 解析不符: %+v", cq)
	}
	if cq.Message == nil || cq.Message.Chat.ID != -100123 || cq.Message.MessageID != 9 {
		t.Fatalf("所在消息解析不符: %+v", cq.Message)
	}

	// 所在消息不可达（inline 模式等）时 message 缺省
	var u2 Update
	if err := json.Unmarshal([]byte(`{"update_id":8,"callback_query":{"id":"cq2","from":{"id":1},"data":"x"}}`), &u2); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if u2.CallbackQuery == nil || u2.CallbackQuery.Message != nil {
		t.Fatalf("无 message 的回调解析不符: %+v", u2.CallbackQuery)
	}
}

// TestInteractionEventFromCallbackQuery handleCallbackQuery 的归一化字段
// （群聊/私聊分支、消息与用户 ID）。通过注入 TriggerWrapper 捕获。
func TestInteractionEventFromCallbackQuery(t *testing.T) {
	a := NewAdapter(nil)
	var got *message.InteractionEvent
	a.SetTrigger(adapter.TriggerWrapper{OnInteraction: func(ev message.InteractionEvent) {
		got = &ev
	}})

	a.handleCallbackQuery(&CallbackQuery{
		ID:      "cq1",
		From:    User{ID: 42},
		Message: &Message{MessageID: 9, Chat: Chat{ID: -100123, Type: "supergroup"}},
		Data:    "音乐点歌:pg:2",
	})
	if got == nil {
		t.Fatal("群聊回调应上报 InteractionEvent")
	}
	if got.Platform != Platform || got.MessageType != "group" || got.GroupId != "tg:-100123" ||
		got.UserId != "tg:42" || got.MessageId != "tg:-100123:9" || got.CallbackId != "cq1" || got.Data != "音乐点歌:pg:2" {
		t.Fatalf("群聊归一化字段不符: %+v", got)
	}

	a.handleCallbackQuery(&CallbackQuery{
		ID:      "cq2",
		From:    User{ID: 42},
		Message: &Message{MessageID: 3, Chat: Chat{ID: 42, Type: "private"}},
		Data:    "m:x",
	})
	if got.MessageType != "private" || got.GroupId != "" || got.MessageId != "tg:42:3" {
		t.Fatalf("私聊归一化字段不符: %+v", got)
	}

	// 所在消息不可达：直接应答失效提示，不上报
	got = nil
	a.handleCallbackQuery(&CallbackQuery{ID: "cq3", From: User{ID: 42}, Data: "m:x"})
	if got != nil {
		t.Fatal("无所在消息的回调不应上报")
	}
}
