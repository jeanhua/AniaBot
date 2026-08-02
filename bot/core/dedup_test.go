package core

import (
	"context"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

// countingPlugin 统计消息/通知回调次数的最小插件（嵌入 plugin.Meta 获得其余默认实现）。
type countingPlugin struct {
	plugin.Meta
	groupMsgs  atomic.Int64
	friendMsgs atomic.Int64
	emojiLikes atomic.Int64
}

func (p *countingPlugin) OnGroupMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	p.groupMsgs.Add(1)
	return true, nil
}

func (p *countingPlugin) OnFriendMsg(ctx context.Context, b bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	p.friendMsgs.Add(1)
	return true, nil
}

func (p *countingPlugin) OnGroupMsgEmojiLike(ctx context.Context, b bot.Bot, n message.GroupMsgEmojiLikeNotice) error {
	p.emojiLikes.Add(1)
	return nil
}

// fakeKeyerAdapter 带 adapter.EventKeyer 的假适配器（复用 route_test.go 的 fakeAdapter）。
type fakeKeyerAdapter struct {
	fakeAdapter
}

func (f *fakeKeyerAdapter) MessageKey(msg message.Message) (string, bool) {
	if msg.MessageId == "" {
		return "", false
	}
	return "custom:" + msg.MessageId.String(), true
}

func (f *fakeKeyerAdapter) NoticeKey(noticeType string, notice any) (string, bool) {
	if n, ok := notice.(message.GroupMsgEmojiLikeNotice); ok && n.MessageId != "" {
		return "emoji:" + n.MessageId.String(), true
	}
	return "", false
}

// newDedupTestBot 构造一个带内存存储与计数插件的 AniaBot。
func newDedupTestBot(t *testing.T, a adapter.Adapter) (*AniaBot, *countingPlugin) {
	t.Helper()
	bot := &AniaBot{ctx: context.Background()}
	bot.storage = NewAniaMemoryStorage(slog.New(slog.NewTextHandler(io.Discard, nil)))
	cp := &countingPlugin{}
	bot.plugins = []plugin.Plugin{cp}
	bot.adapters = []*adapterEntry{{def: adapter.Definition{Name: "test", Platform: "test", IDPrefix: ""}, adapter: a}}
	return bot, cp
}

// TestMessageDedupFallback 无 EventKeyer 的适配器（NapCat/OneBot）按「平台+MessageId」兜底去重。
func TestMessageDedupFallback(t *testing.T) {
	a, cp := newDedupTestBot(t, &fakeAdapter{name: "test", platform: "test"})
	e := a.adapters[0]

	msg := message.Message{MessageId: message.FromUint64(10086), SelfId: message.FromUint64(1), UserId: message.FromUint64(2)}
	a.onGroupEvent(e, msg)
	a.onGroupEvent(e, msg)
	if cp.groupMsgs.Load() != 1 {
		t.Fatalf("重复 MessageId 应只触发一次插件链, got %d", cp.groupMsgs.Load())
	}

	msg2 := message.Message{MessageId: message.FromUint64(10087), SelfId: message.FromUint64(1), UserId: message.FromUint64(2)}
	a.onGroupEvent(e, msg2)
	if cp.groupMsgs.Load() != 2 {
		t.Fatalf("不同 MessageId 应分别触发, got %d", cp.groupMsgs.Load())
	}
}

// TestMessageDedupEventKeyer 实现 EventKeyer 的适配器（飞书）优先使用其提供的键。
func TestMessageDedupEventKeyer(t *testing.T) {
	a, cp := newDedupTestBot(t, &fakeKeyerAdapter{fakeAdapter{name: "test", platform: "test"}})
	e := a.adapters[0]

	msg := message.Message{MessageId: message.QID("fs:om_123"), SelfId: message.QID("fs:ou_bot"), UserId: message.QID("fs:ou_usr")}
	a.onGroupEvent(e, msg)
	a.onGroupEvent(e, msg)
	if cp.groupMsgs.Load() != 1 {
		t.Fatalf("EventKeyer 键重复投递应只触发一次, got %d", cp.groupMsgs.Load())
	}
}

// TestMessageDedupEmptyID 无 MessageId 的消息不做去重（放行）。
func TestMessageDedupEmptyID(t *testing.T) {
	a, cp := newDedupTestBot(t, &fakeAdapter{name: "test", platform: "test"})
	e := a.adapters[0]

	msg := message.Message{SelfId: message.FromUint64(1), UserId: message.FromUint64(2)}
	a.onGroupEvent(e, msg)
	a.onGroupEvent(e, msg)
	if cp.groupMsgs.Load() != 2 {
		t.Fatalf("无 MessageId 的消息不应去重, got %d", cp.groupMsgs.Load())
	}
}

// TestNoticeDedupWithKey 适配器提供 NoticeKey 时通知去重。
func TestNoticeDedupWithKey(t *testing.T) {
	a, cp := newDedupTestBot(t, &fakeKeyerAdapter{fakeAdapter{name: "test", platform: "test"}})
	e := a.adapters[0]

	notice := message.GroupMsgEmojiLikeNotice{MessageId: message.QID("fs:om_456")}
	a.onGroupMsgEmojiLikeEvent(e, notice)
	a.onGroupMsgEmojiLikeEvent(e, notice)
	if cp.emojiLikes.Load() != 1 {
		t.Fatalf("带键的通知重复投递应只触发一次, got %d", cp.emojiLikes.Load())
	}
}

// TestNoticeNoDedupWithoutKey 无 EventKeyer 的适配器通知不做组合兜底（避免误伤）。
func TestNoticeNoDedupWithoutKey(t *testing.T) {
	a, cp := newDedupTestBot(t, &fakeAdapter{name: "test", platform: "test"})
	e := a.adapters[0]

	notice := message.GroupMsgEmojiLikeNotice{MessageId: message.FromUint64(456)}
	a.onGroupMsgEmojiLikeEvent(e, notice)
	a.onGroupMsgEmojiLikeEvent(e, notice)
	if cp.emojiLikes.Load() != 2 {
		t.Fatalf("无 EventKeyer 的通知不应去重, got %d", cp.emojiLikes.Load())
	}
}

// fakeSelfIDAdapter 带 adapter.SelfIDProvider 的假适配器。
type fakeSelfIDAdapter struct {
	fakeAdapter
	selfID message.QID
}

func (f *fakeSelfIDAdapter) SelfID() message.QID { return f.selfID }

// TestFillSelfID 事件未携带 self_id 时用适配器 SelfIDProvider 兜底填充。
func TestFillSelfID(t *testing.T) {
	a, _ := newDedupTestBot(t, &fakeSelfIDAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, selfID: message.QID("fs:ou_bot")})
	e := a.adapters[0]

	// 空 self_id → 填充
	msg := message.Message{UserId: message.QID("fs:ou_usr")}
	a.fillSelfID(e, &msg)
	if msg.SelfId != "fs:ou_bot" {
		t.Fatalf("空 SelfId 应被适配器兜底填充, got %q", msg.SelfId)
	}

	// 已携带 → 不动
	msg2 := message.Message{SelfId: message.QID("fs:ou_known")}
	a.fillSelfID(e, &msg2)
	if msg2.SelfId != "fs:ou_known" {
		t.Fatalf("非空 SelfId 不应被覆盖, got %q", msg2.SelfId)
	}
}

// segTestAdapter 带 adapter.SegmentSupport 的假适配器。
type segTestAdapter struct {
	fakeAdapter
	supported []string
}

func (s *segTestAdapter) SupportedSegments() []string { return s.supported }

// TestCheckSegmentSupport 对不支持的段类型计数告警（不阻断发送），支持的类型不计数。
func TestCheckSegmentSupport(t *testing.T) {
	a := &AniaBot{}
	ad := &segTestAdapter{fakeAdapter: fakeAdapter{name: "segtest", platform: "segtest"}, supported: []string{"text", "image"}}
	segs := []message.OB11Segment{
		{Type: message.SegmentText, Data: map[string]any{"text": "hi"}},
		{Type: message.SegmentFace, Data: map[string]any{"id": 1}},
	}

	a.checkSegmentSupport(ad, segs)
	key := "segtest|face"
	v, ok := segWarn.Load(key)
	if !ok {
		t.Fatal("不支持的段类型应有计数")
	}
	if n := v.(*atomic.Int64).Load(); n != 1 {
		t.Fatalf("计数应为 1, got %d", n)
	}
	// 支持的类型不计数
	if _, ok := segWarn.Load("segtest|text"); ok {
		t.Fatal("支持的段类型不应计数")
	}

	// 再次发送累计到 2
	a.checkSegmentSupport(ad, segs)
	if n := v.(*atomic.Int64).Load(); n != 2 {
		t.Fatalf("计数应为 2, got %d", n)
	}

	// 未实现 SegmentSupport 的适配器静默跳过
	a.checkSegmentSupport(&fakeAdapter{name: "x", platform: "x"}, segs)
	if _, ok := segWarn.Load("x|face"); ok {
		t.Fatal("未声明支持的适配器不应计数")
	}
}
