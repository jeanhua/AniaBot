package telegram

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/bot"
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

// TestSendChainAttachesKeyboard 端到端（假 Bot API 服务器）：携带 keyboard 段的
// 消息先发正文（sendMessage 不带 reply_markup），随后经 editMessageReplyMarkup
// 把内联键盘附加到最后一条消息；纯文本消息不触发额外编辑。
func TestSendChainAttachesKeyboard(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	chain := msgchain.Builder().Group().
		Text("搜索结果（第 1 页）").
		Keyboard(msgchain.Row(msgchain.Button("▶️ 下一页", "音乐点歌:pg:2"))).
		Build()
	if _, ok := a.SendGroupMsg("tg:-100", chain); !ok {
		t.Fatal("发送失败")
	}
	if n := f.count("sendMessage"); n != 1 {
		t.Fatalf("sendMessage 调用 = %d, want 1", n)
	}
	if _, has := f.req(0).json["reply_markup"]; has {
		t.Fatalf("正文消息不应携带 reply_markup: %+v", f.req(0).json)
	}
	if n := f.count("editMessageReplyMarkup"); n != 1 {
		t.Fatalf("editMessageReplyMarkup 调用 = %d, want 1", n)
	}
	rec := f.req(1)
	if rec.json["chat_id"] != float64(-100) || rec.json["message_id"] != float64(42) {
		t.Fatalf("附加目标不符: %+v", rec.json)
	}
	var rm tgReplyMarkup
	if err := json.Unmarshal([]byte(rec.json["reply_markup"].(string)), &rm); err != nil {
		t.Fatalf("reply_markup 非法 JSON: %v", err)
	}
	if len(rm.InlineKeyboard) != 1 || rm.InlineKeyboard[0][0].CallbackData != "音乐点歌:pg:2" {
		t.Fatalf("内联键盘不符: %+v", rm)
	}

	// 无 keyboard 段的普通消息：不应触发额外编辑
	if _, ok := a.SendGroupMsg("tg:-100", msgchain.Builder().Group().Text("纯文本").Build()); !ok {
		t.Fatal("发送失败")
	}
	if n := f.count("editMessageReplyMarkup"); n != 1 {
		t.Fatalf("纯文本不应触发 editMessageReplyMarkup, got %d", n-1)
	}
}

// TestEditMsgUpdatesTextAndKeyboard 端到端：MsgEditor 编辑更新文本并更换按钮；
// 不携带 keyboard 段时只改文本、不带 reply_markup（保持原按钮）。
func TestEditMsgUpdatesTextAndKeyboard(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	if !a.EditGroupMsg("tg:-100:42", msgchain.Builder().Group().
		Text("搜索结果（第 2 页）").
		Keyboard(msgchain.Row(
			msgchain.Button("◀️ 上一页", "音乐点歌:pg:1"),
			msgchain.Button("▶️ 下一页", "音乐点歌:pg:3"),
		)).Build()) {
		t.Fatal("编辑失败")
	}
	if n := f.count("editMessageText"); n != 1 {
		t.Fatalf("editMessageText 调用 = %d, want 1", n)
	}
	rec := f.req(0)
	if rec.json["text"] != "搜索结果（第 2 页）" || rec.json["message_id"] != float64(42) {
		t.Fatalf("编辑参数不符: %+v", rec.json)
	}
	var rm tgReplyMarkup
	if err := json.Unmarshal([]byte(rec.json["reply_markup"].(string)), &rm); err != nil {
		t.Fatalf("reply_markup 非法 JSON: %v", err)
	}
	if len(rm.InlineKeyboard) != 1 || len(rm.InlineKeyboard[0]) != 2 {
		t.Fatalf("按钮应为 1 行 2 个: %+v", rm)
	}

	if !a.EditGroupMsg("tg:-100:42", msgchain.Builder().Group().Text("内容更新").Build()) {
		t.Fatal("编辑失败")
	}
	rec = f.req(1)
	if _, has := rec.json["reply_markup"]; has {
		t.Fatalf("未携带 keyboard 段不应更换按钮: %+v", rec.json)
	}
}

// TestAnswerInteractionCallbackQuery 端到端：应答点击经 answerCallbackQuery
// 送达，text 非空时携带。
func TestAnswerInteractionCallbackQuery(t *testing.T) {
	f := newFakeAPI()
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	if !a.AnswerInteraction("cq1", "已翻到第 2 页") {
		t.Fatal("应答失败")
	}
	if n := f.count("answerCallbackQuery"); n != 1 {
		t.Fatalf("answerCallbackQuery 调用 = %d, want 1", n)
	}
	rec := f.req(0)
	if rec.json["callback_query_id"] != "cq1" || rec.json["text"] != "已翻到第 2 页" {
		t.Fatalf("应答参数不符: %+v", rec.json)
	}

	if !a.AnswerInteraction("cq2", "") {
		t.Fatal("应答失败")
	}
	if _, has := f.req(1).json["text"]; has {
		t.Fatalf("空文本不应携带 text: %+v", f.req(1).json)
	}
}

// TestGetUpdatesRequestsCallbackQuery getUpdates 显式订阅 callback_query，
// 不依赖 Telegram 侧持久化的 allowed_updates 设置。
func TestGetUpdatesRequestsCallbackQuery(t *testing.T) {
	f := newFakeAPI()
	f.updates = []Update{{UpdateID: 1}}
	a, srv := testAdapterWithServer(f)
	defer srv.Close()

	if _, err := a.getUpdates(t.Context(), 0, 1); err != nil {
		t.Fatalf("getUpdates 失败: %v", err)
	}
	rec := f.req(0)
	au, ok := rec.json["allowed_updates"].([]any)
	if !ok {
		t.Fatalf("getUpdates 应显式携带 allowed_updates: %+v", rec.json)
	}
	found := false
	for _, v := range au {
		if v == "callback_query" {
			found = true
		}
	}
	if !found {
		t.Fatalf("allowed_updates 应包含 callback_query: %+v", au)
	}
}

// TestAdapterInteractiveCapabilities 回归：telegramAdapter 必须实现全部交互
// 可选接口——漏实现不报编译错误，只会让插件探测静默失败、core 剥掉按钮段
// 退化为文本模式（本次按钮功能上线时真实发生过的回归）。
func TestAdapterInteractiveCapabilities(t *testing.T) {
	a := NewAdapter(nil)
	var src adapter.Adapter = a
	if _, ok := src.(adapter.InteractiveExt); !ok {
		t.Fatal("telegramAdapter 应实现 adapter.InteractiveExt（SupportsKeyboard）")
	}
	if _, ok := src.(adapter.MsgEditorExt); !ok {
		t.Fatal("telegramAdapter 应实现 adapter.MsgEditorExt（EditGroupMsg/EditFriendMsg）")
	}
	if _, ok := src.(adapter.InteractionAnswerer); !ok {
		t.Fatal("telegramAdapter 应实现 adapter.InteractionAnswerer（AnswerInteraction）")
	}
	if !src.(adapter.InteractiveExt).SupportsKeyboard() {
		t.Fatal("Telegram 应声明支持内联按钮")
	}

	// 事件回调收到的包装外观可断言 bot.Interactive / bot.MsgEditor 且支持按钮
	b := adapter.WrapBot(nil, src)
	iv, ok := b.(bot.Interactive)
	if !ok || !iv.SupportsKeyboard() {
		t.Fatal("包装后的 bot 外观应断言 bot.Interactive 且支持按钮")
	}
	if _, ok := b.(bot.MsgEditor); !ok {
		t.Fatal("包装后的 bot 外观应断言 bot.MsgEditor")
	}
}
