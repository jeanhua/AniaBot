package aitool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// fakeHistoryBot 测试桩：实现历史消息工具所需的基础接口（GetGroupMsgHistory /
// GetFriendMsgHistory），按预设页返回并记录调用参数。
type fakeHistoryBot struct {
	bot.Bot
	pages    [][]message.Message // 每页按调用次序返回（模拟最新在前的分页）
	calls    []int               // 每次调用的 (count, seq) 记录，下标与 pages 对应
	lastSeqs []int
	group    bool // true=群接口，false=好友接口
	fail     bool // true=模拟平台不支持（接口返回 false）
}

func (f *fakeHistoryBot) GetGroupMsgHistory(groupId message.QID, count int, seq int) (*[]message.Message, bool) {
	if !f.group || f.fail {
		return nil, false
	}
	return f.page(count, seq)
}

func (f *fakeHistoryBot) GetFriendMsgHistory(userId message.QID, count int, seq int) (*[]message.Message, bool) {
	if f.group || f.fail {
		return nil, false
	}
	return f.page(count, seq)
}

func (f *fakeHistoryBot) page(count int, seq int) (*[]message.Message, bool) {
	idx := len(f.calls)
	f.calls = append(f.calls, count)
	f.lastSeqs = append(f.lastSeqs, seq)
	if idx >= len(f.pages) {
		return &[]message.Message{}, true
	}
	list := append([]message.Message(nil), f.pages[idx]...)
	return &list, true
}

func historyCtx(b bot.Bot, sink MessageSink) Context {
	return Context{Bot: b, Target: message.QID("qq:888"), IsGroup: true, OnMessages: sink}
}

// mkMsg 构造带发送者与游标的消息。
func mkMsg(seq int, sender string) message.Message {
	return message.Message{MessageSeq: seq, Sender: message.MessageSender{UserId: message.QID(sender)}}
}

func callHistory(t *testing.T, tool Tool, args string) string {
	t.Helper()
	out, err := tool.Execute(context.Background(), paramsOf(t, tool, args))
	if err != nil {
		t.Fatalf("执行失败: %v", err)
	}
	return out
}

// paramsOf 模拟框架的参数解析路径：JSON 实参 → Params() 同型结构体指针。
func paramsOf(t *testing.T, tool Tool, args string) any {
	t.Helper()
	p := tool.Params()
	if args == "" {
		return p
	}
	if err := json.Unmarshal([]byte(args), p); err != nil {
		t.Fatalf("参数解析失败: %v", err)
	}
	return p
}

func TestHistoryNoFilter(t *testing.T) {
	fake := &fakeHistoryBot{group: true, pages: [][]message.Message{
		{mkMsg(105, "qq:1"), mkMsg(104, "qq:2"), mkMsg(103, "qq:1")},
	}}
	tool := NewMsgHistoryTool(historyCtx(fake, nil), "qq_get_msg_history")

	out := callHistory(t, tool, `{"count":3}`)
	if !strings.Contains(out, "[message_seq:105]") || !strings.Contains(out, "[message_seq:103]") {
		t.Errorf("输出缺少消息游标:\n%s", out)
	}
	if fake.lastSeqs[0] != 0 {
		t.Errorf("首次调用应从最新开始（seq=0）, got %d", fake.lastSeqs[0])
	}
}

func TestHistoryFilterByUser(t *testing.T) {
	// 两页数据：第一页只有 1 条命中，第二页 2 条；凑够 3 条即止
	fake := &fakeHistoryBot{group: true, pages: [][]message.Message{
		{mkMsg(60, "qq:1"), mkMsg(59, "qq:2"), mkMsg(58, "qq:2"), mkMsg(57, "qq:2")},
		{mkMsg(56, "qq:1"), mkMsg(55, "qq:1"), mkMsg(54, "qq:2"), mkMsg(53, "qq:2"), mkMsg(52, "qq:2")},
	}}
	setTestPageSize(t, 4)
	tool := NewMsgHistoryTool(historyCtx(fake, nil), "qq_get_msg_history")

	out := callHistory(t, tool, `{"count":3,"user_id":"1"}`)
	if !strings.Contains(out, "已回溯最近 9 条消息") {
		t.Errorf("应注明回溯总量:\n%s", out)
	}
	if !strings.Contains(out, "3 条来自 qq:1") {
		t.Errorf("应注明命中数:\n%s", out)
	}
	if strings.Count(out, "[message_seq:") != 3 {
		t.Errorf("应只输出命中的 3 条:\n%s", out)
	}
	if !strings.Contains(out, "[message_seq:56]") || !strings.Contains(out, "[message_seq:60]") {
		t.Errorf("命中消息缺失:\n%s", out)
	}
	// 第二次调用游标应为第一页最后一条的 seq（57）
	if fake.lastSeqs[1] != 57 {
		t.Errorf("翻页游标应为 57, got %d", fake.lastSeqs[1])
	}
	// 凑够即止：不应有"继续回溯"提示
	if strings.Contains(out, "继续向前筛选") {
		t.Errorf("已凑够命中数时不应提示继续翻页:\n%s", out)
	}
}

func TestHistoryFilterNotFound(t *testing.T) {
	fake := &fakeHistoryBot{group: true, pages: [][]message.Message{
		{mkMsg(5, "qq:2"), mkMsg(4, "qq:2")},
	}}
	tool := NewMsgHistoryTool(historyCtx(fake, nil), "qq_get_msg_history")

	out := callHistory(t, tool, `{"user_id":"qq:999"}`)
	if !strings.Contains(out, "没有找到 qq:999 的发言") {
		t.Errorf("未命中应如实提示:\n%s", out)
	}
	if !strings.Contains(out, "已回溯最近 2 条") {
		t.Errorf("应注明回溯总量:\n%s", out)
	}
}

// TestHistoryFilterCrossPrefix 筛选 ID 容忍 qq:/lil:/纯数字写法，统一按会话前缀匹配。
func TestHistoryFilterCrossPrefix(t *testing.T) {
	pages := [][]message.Message{{mkMsg(5, "qq:2"), mkMsg(4, "qq:1")}}
	for _, input := range []string{`{"user_id":"lil:2"}`, `{"user_id":"qq:2"}`, `{"user_id":"2"}`} {
		// 每次调用用全新桩：仅验证 ID 归一化，与分页状态无关
		fake := &fakeHistoryBot{group: true, pages: pages}
		tool := NewMsgHistoryTool(historyCtx(fake, nil), "qq_get_msg_history")
		out := callHistory(t, tool, input)
		if !strings.Contains(out, "1 条来自 qq:2") {
			t.Errorf("输入 %s 应命中 qq:2:\n%s", input, out)
		}
	}
}

// TestHistoryPaginationStopConditions 游标不前进 / 返回不足一页时应停止翻页，
// 防止对不支持分页的平台（内存缓存兜底）无限回溯。
func TestHistoryPaginationStopConditions(t *testing.T) {
	// 平台忽略游标：第二页返回与第一页相同的内容（游标不前进 → 停止）
	stub := &fakeHistoryBot{group: true, pages: [][]message.Message{
		{mkMsg(3, "qq:2"), mkMsg(2, "qq:2"), mkMsg(1, "qq:1")},
		{mkMsg(3, "qq:2"), mkMsg(2, "qq:2"), mkMsg(1, "qq:1")},
	}}
	setTestPageSize(t, 3)
	tool := NewMsgHistoryTool(historyCtx(stub, nil), "qq_get_msg_history")
	out := callHistory(t, tool, `{"user_id":"2"}`)
	if !strings.Contains(out, "已回溯最近 6 条") {
		t.Errorf("游标不前进时应停止翻页并汇总已回溯量:\n%s", out)
	}
	if len(stub.calls) != 2 {
		t.Errorf("游标不前进时应立即停止翻页, calls=%d", len(stub.calls))
	}
	if strings.Contains(out, "继续向前筛选") {
		t.Errorf("已到最早消息时不应提示继续翻页:\n%s", out)
	}

	// 末页不足 page size：视为已到最早消息，不再翻页
	fake := &fakeHistoryBot{group: true, pages: [][]message.Message{
		{mkMsg(2, "qq:2"), mkMsg(1, "qq:1")},
	}}
	setTestPageSize(t, 50)
	tool2 := NewMsgHistoryTool(historyCtx(fake, nil), "qq_get_msg_history")
	out2 := callHistory(t, tool2, `{"user_id":"1"}`)
	if !strings.Contains(out2, "[message_seq:1]") {
		t.Errorf("末页命中应返回:\n%s", out2)
	}
	if len(fake.calls) != 1 {
		t.Errorf("不足一页时应停止翻页, calls=%d", len(fake.calls))
	}
}

// setTestPageSize 临时缩小筛选分页大小（页大小常量对真实平台为 50，
// 测试用小页模拟多轮回溯），测试结束后恢复。
func setTestPageSize(t *testing.T, n int) {
	t.Helper()
	old := historyPageSize
	historyPageSize = n
	t.Cleanup(func() { historyPageSize = old })
}

// TestHistoryImageSink 拉取到的消息应回调 OnMessages（宿主借此登记图片）。
func TestHistoryImageSink(t *testing.T) {
	msgs := []message.Message{mkMsg(5, "qq:1")}
	var sunk [][]message.Message
	fake := &fakeHistoryBot{group: true, pages: [][]message.Message{msgs}}
	ctx := historyCtx(fake, func(_ context.Context, m []message.Message) {
		sunk = append(sunk, m)
	})
	tool := NewMsgHistoryTool(ctx, "qq_get_msg_history")

	callHistory(t, tool, "")
	if len(sunk) != 1 || len(sunk[0]) != 1 {
		t.Fatalf("OnMessages 应收到拉取的消息, got %+v", sunk)
	}
}

// TestHistoryPrivateAndUnsupported 私聊会话走好友接口；平台不支持时如实报错。
func TestHistoryPrivateAndUnsupported(t *testing.T) {
	fake := &fakeHistoryBot{pages: [][]message.Message{{mkMsg(5, "qq:9")}}}
	ctx := Context{Bot: fake, Target: message.QID("qq:123"), IsGroup: false}
	tool := NewMsgHistoryTool(ctx, "qq_get_msg_history")
	out := callHistory(t, tool, "")
	if !strings.Contains(out, "[message_seq:5]") {
		t.Errorf("私聊应走好友接口并返回消息:\n%s", out)
	}

	unsupported := &fakeHistoryBot{group: true, fail: true} // 群会话 + 群接口失败
	tool2 := NewMsgHistoryTool(Context{Bot: unsupported, Target: "qq:1", IsGroup: true}, "qq_get_msg_history")
	if _, err := tool2.Execute(context.Background(), paramsOf(t, tool2, "")); err == nil {
		t.Fatal("平台不支持历史查询时应报错")
	}
}
