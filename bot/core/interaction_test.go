package core

import (
	"context"
	"testing"

	"github.com/jeanhua/AniaBot/common/adapter"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugin"
)

// interactionPlugin 实现 plugin.InteractionHandler 的测试插件。
type interactionPlugin struct {
	plugin.Meta
	called bool
	got    *message.InteractionEvent
}

func (ip *interactionPlugin) OnInteraction(ctx context.Context, b bot.Bot, ev *message.InteractionEvent) error {
	ip.called = true
	ip.got = ev
	ev.AnswerText = "done"
	return nil
}

// interactionAdapter 支持按钮交互与应答的测试适配器（keyboardSupport 控制
// SupportsKeyboard，用于剥离逻辑测试）。
type interactionAdapter struct {
	fakeAdapter
	keyboardSupport bool
	answered        [][2]string // [callbackId, text]
	sent            []message.OB11Segment
}

func (ia *interactionAdapter) SupportsKeyboard() bool { return ia.keyboardSupport }

func (ia *interactionAdapter) AnswerInteraction(callbackId, text string) bool {
	ia.answered = append(ia.answered, [2]string{callbackId, text})
	return true
}

func (ia *interactionAdapter) SendGroupMsg(id message.QID, chain msgchain.GroupChain) (message.QID, bool) {
	ia.sent = chain.GetGroupMsg()
	return "1", true
}

// newInteractionBot 构造挂载了插件与适配器的 AniaBot。
func newInteractionBot(t *testing.T, a adapter.Adapter, plugins ...plugin.Plugin) *AniaBot {
	t.Helper()
	ania := NewAniaBot()
	ania.adapters = []*adapterEntry{
		{def: adapter.Definition{Name: "test", Platform: "test", IDPrefix: "ts:"}, adapter: a},
	}
	ania.plugins = plugins
	return ania
}

// TestOnInteractionRouting 回调数据「插件名:载荷」按前缀路由到对应插件，
// 载荷剥离前缀后传入，插件返回后以 AnswerText 应答平台。
func TestOnInteractionRouting(t *testing.T) {
	ip := &interactionPlugin{}
	ip.Name = "音乐点歌"
	ia := &interactionAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, keyboardSupport: true}
	ania := newInteractionBot(t, ia, ip)

	ania.onInteraction(ania.adapters[0], message.InteractionEvent{
		CallbackId: "cb1", Data: "音乐点歌:pg:2",
	})
	if !ip.called {
		t.Fatal("同名插件应收到回调")
	}
	if ip.got == nil || ip.got.Data != "pg:2" || ip.got.CallbackId != "cb1" {
		t.Fatalf("载荷应剥离插件前缀: %+v", ip.got)
	}
	if len(ia.answered) != 1 || ia.answered[0] != [2]string{"cb1", "done"} {
		t.Fatalf("应以插件填写的 AnswerText 应答: %+v", ia.answered)
	}
}

// TestOnInteractionUnowned 无主回调（插件不存在/插件未实现处理器/数据无前缀）
// 不投递任何插件，但统一静默应答消除客户端等待。
func TestOnInteractionUnowned(t *testing.T) {
	ip := &interactionPlugin{}
	ip.Name = "音乐点歌"
	ia := &interactionAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, keyboardSupport: true}
	ania := newInteractionBot(t, ia, ip)

	cases := []struct {
		name string
		data string
	}{
		{"插件不存在", "别的插件:pg:2"},
		{"插件未实现处理器", "无处理器插件:pg:2"},
		{"数据无前缀", "pg:2"},
	}
	for _, c := range cases {
		ip.called = false
		ania.onInteraction(ania.adapters[0], message.InteractionEvent{CallbackId: "cb2", Data: c.data})
		if ip.called {
			t.Fatalf("%s: 不应投递插件", c.name)
		}
		if len(ia.answered) != 1 || ia.answered[0] != [2]string{"cb2", ""} {
			t.Fatalf("%s: 无主回调应静默应答: %+v", c.name, ia.answered)
		}
		ia.answered = nil
	}

	// callbackId 为空（平台无应答概念）不触发应答
	ania.onInteraction(ania.adapters[0], message.InteractionEvent{Data: "音乐点歌:pg:2"})
	if len(ia.answered) != 0 {
		t.Fatalf("callbackId 为空不应应答: %+v", ia.answered)
	}
}

// TestStripUnsupportedKeyboard 平台不支持内联按钮时出站剥离 keyboard 段，
// 支持时原样透传。
func TestStripUnsupportedKeyboard(t *testing.T) {
	chain := msgchain.Builder().Group().Text("列表").
		Keyboard(msgchain.Row(msgchain.Button("下一页", "m:pg:2"))).Build()

	// 不支持的平台：剥离
	ia := &interactionAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, keyboardSupport: false}
	ania := newInteractionBot(t, ia)
	if _, ok := ania.SendGroupMsg("ts:1", chain); !ok {
		t.Fatal("发送应成功")
	}
	if len(ia.sent) != 1 || ia.sent[0].Type == message.SegmentKeyboard {
		t.Fatalf("keyboard 段应被剥离: %+v", ia.sent)
	}

	// 支持的平台：透传
	ia2 := &interactionAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, keyboardSupport: true}
	ania2 := newInteractionBot(t, ia2)
	if _, ok := ania2.SendGroupMsg("ts:1", chain); !ok {
		t.Fatal("发送应成功")
	}
	if len(ia2.sent) != 2 {
		t.Fatalf("keyboard 段应透传给支持的平台: %+v", ia2.sent)
	}

	// 不支持平台 + 纯文本链：无剥离告警路径，原样发送
	ia3 := &interactionAdapter{fakeAdapter: fakeAdapter{name: "test", platform: "test"}, keyboardSupport: false}
	ania3 := newInteractionBot(t, ia3)
	plain := msgchain.Builder().Group().Text("纯文本").Build()
	if _, ok := ania3.SendGroupMsg("ts:1", plain); !ok {
		t.Fatal("纯文本发送应成功")
	}
	if len(ia3.sent) != 1 {
		t.Fatalf("纯文本链不应被修改: %+v", ia3.sent)
	}
}

// TestMetaCallbackData 插件回调数据打包：插件名前缀 + 载荷（端到端路由
// 行为由 TestOnInteractionRouting 覆盖）。
func TestMetaCallbackData(t *testing.T) {
	var p plugin.Meta
	p.Name = "音乐点歌"
	if got := p.CallbackData("pg:2"); got != "音乐点歌:pg:2" {
		t.Fatalf("打包结果不符: %q", got)
	}
}

// 保证 interactionAdapter/interactionPlugin 实现对应接口（编译期断言）。
var (
	_ adapter.InteractiveExt      = (*interactionAdapter)(nil)
	_ adapter.InteractionAnswerer = (*interactionAdapter)(nil)
	_ plugin.InteractionHandler   = (*interactionPlugin)(nil)
	_ plugin.Plugin               = (*interactionPlugin)(nil)
)
