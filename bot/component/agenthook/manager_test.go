package agenthook

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeConfigStore 测试用配置中心读能力
type fakeConfigStore struct {
	raw      string
	getCalls atomic.Int32
}

func (f *fakeConfigStore) Get(key string) (any, bool) {
	f.getCalls.Add(1)
	return f.raw, true
}

// recordHandler 记录收到的钩子事件的 Go 钩子
type recordHandler struct {
	events  []Event
	results []Result
	next    Result
}

func (h *recordHandler) OnAgentHook(ctx context.Context, ev Event, p Payload) Result {
	h.events = append(h.events, ev)
	h.results = append(h.results, Result{})
	return h.next
}

func TestManagerDisabledShortCircuits(t *testing.T) {
	withFakeExec(t)
	store := &fakeConfigStore{raw: `{"hooks":{"PreToolUse":[{"command":"exit2:stderr=危险"}]}}`}
	m := NewManager(store, "files.hooks_json", nil)
	// 未 SetEnabled：默认关闭，Run 直接短路，不读配置不执行命令
	res := m.Run(context.Background(), EventPreToolUse, Payload{ToolName: "bash"})
	if res.Block || res.Context != "" || res.Err != nil {
		t.Fatalf("关闭时应返回零值, got %+v", res)
	}
	if store.getCalls.Load() != 0 {
		t.Fatalf("关闭时不应读取配置中心, getCalls=%d", store.getCalls.Load())
	}
}

func TestManagerShellHookBlock(t *testing.T) {
	withFakeExec(t)
	store := &fakeConfigStore{raw: `{"hooks":{"PreToolUse":[
		{"matcher":"^bash$","command":"exit2:stderr=危险命令"},
		{"command":"exit0:stdout=通用上下文"}
	],"Stop":[
		{"matcher":"^bash$","command":"exit2:stderr=不该触发"},
		{"command":"exit0:stdout=通用上下文"}
	]}}`}
	m := NewManager(store, "files.hooks_json", nil)
	m.SetEnabled(true)
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// bash 命中第一条（阻断）→ 短路，第二条不执行
	res := m.Run(context.Background(), EventPreToolUse, Payload{ToolName: "bash"})
	if !res.Block || res.Reason != "危险命令" {
		t.Fatalf("bash 应被阻断, got %+v", res)
	}
	// time 不命中 matcher（跳过第一条），第二条通过并产出 Context
	res = m.Run(context.Background(), EventPreToolUse, Payload{ToolName: "time"})
	if res.Block || res.Context != "通用上下文" {
		t.Fatalf("time 应通过并携带上下文, got %+v", res)
	}
	// 非工具事件：带 matcher 的钩子不匹配，空 matcher 的执行
	res = m.Run(context.Background(), EventStop, Payload{})
	if res.Block || res.Context != "通用上下文" {
		t.Fatalf("Stop 仅空 matcher 生效, got %+v", res)
	}
}

func TestManagerGoHandlerOrderingAndShortCircuit(t *testing.T) {
	withFakeExec(t)
	store := &fakeConfigStore{raw: `{"hooks":{"PreToolUse":[{"command":"exit0:stdout=shell-ctx"}]}}`}
	m := NewManager(store, "files.hooks_json", nil)
	m.SetEnabled(true)
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}

	// Go 钩子先于 shell 执行，Context 按来源拼接
	goHook := &recordHandler{next: Result{Context: "go-ctx"}}
	m.SetGoHandlers([]Handler{goHook})
	res := m.Run(context.Background(), EventPreToolUse, Payload{ToolName: "bash"})
	if res.Context != "go-ctx\nshell-ctx" {
		t.Fatalf("Context 拼接顺序错误: %q", res.Context)
	}

	// Go 钩子阻断 → shell 钩子不再执行（短路）
	blocking := &recordHandler{next: Result{Block: true, Reason: "go 拒绝"}}
	m.SetGoHandlers([]Handler{blocking})
	res = m.Run(context.Background(), EventPreToolUse, Payload{ToolName: "bash"})
	if !res.Block || res.Reason != "go 拒绝" {
		t.Fatalf("Go 钩子应短路阻断, got %+v", res)
	}
}

func TestManagerGoHookPanicIsolated(t *testing.T) {
	m := NewManager(nil, "files.hooks_json", nil)
	m.SetEnabled(true)
	m.SetGoHandlers([]Handler{panicHandler{}})
	res := m.Run(context.Background(), EventStop, Payload{})
	if res.Block {
		t.Fatalf("panic 不应产生阻断")
	}
}

type panicHandler struct{}

func (panicHandler) OnAgentHook(ctx context.Context, ev Event, p Payload) Result { panic("boom") }

func TestManagerHotReload(t *testing.T) {
	withFakeExec(t)
	store := &fakeConfigStore{raw: ""}
	m := NewManager(store, "files.hooks_json", nil)
	m.SetEnabled(true)
	if err := m.Reload(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	callsAfterFirst := store.getCalls.Load()

	// TTL 内再次 Run：不重新读取配置
	m.Run(context.Background(), EventStop, Payload{})
	if store.getCalls.Load() != callsAfterFirst {
		t.Fatalf("TTL 内不应重读配置中心")
	}

	// 越过 TTL + 内容变化：重新加载并生效
	store.raw = `{"hooks":{"Stop":[{"command":"exit0:stdout=新上下文"}]}}`
	m.lastCheck = time.Now().Add(-time.Minute)
	res := m.Run(context.Background(), EventStop, Payload{})
	if res.Context != "新上下文" {
		t.Fatalf("热加载后应生效新配置, got %+v", res)
	}

	// 配置损坏：沿用旧快照且报错不阻断
	store.raw = `{bad json`
	m.lastCheck = time.Now().Add(-time.Minute)
	res = m.Run(context.Background(), EventStop, Payload{})
	if res.Context != "新上下文" {
		t.Fatalf("损坏配置应沿用旧快照, got %+v", res)
	}
}

func TestManagerInvalidEventConfig(t *testing.T) {
	store := &fakeConfigStore{raw: `{"hooks":{"Typo":[{"command":"echo hi"}]}}`}
	m := NewManager(store, "files.hooks_json", nil)
	m.SetEnabled(true)
	if err := m.Reload(); err == nil || !strings.Contains(err.Error(), "未知钩子事件") {
		t.Fatalf("非法事件名应报错, got %v", err)
	}
}
