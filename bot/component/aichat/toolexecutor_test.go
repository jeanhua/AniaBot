package aichat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// fakeToolExecutor 测试用工具执行器：fn 为 nil 时按工具名返回固定文本。
type fakeToolExecutor struct {
	fn func(ctx context.Context, call llmtool.ToolCall, callbacks llmtool.CallBackFuncs) (string, error)
}

func (f *fakeToolExecutor) Execute(ctx context.Context, call llmtool.ToolCall, callbacks llmtool.CallBackFuncs) (string, error) {
	if f.fn != nil {
		return f.fn(ctx, call, callbacks)
	}
	return "result:" + call.Name, nil
}

func (f *fakeToolExecutor) Tools() []llmtool.ToolDef { return nil }

func newTestOrchestrator(exec ToolExecutor) *ToolOrchestrator {
	return NewToolOrchestrator(exec, NewMessageBuilder("test prompt"))
}

// TestExecuteToolCallsParallelPreservesOrder 并行执行下结果消息必须与工具调用顺序一致：
// 即使慢工具先启动、快工具后启动，回填到结果切片的下标仍按工具数组顺序。
func TestExecuteToolCallsParallelPreservesOrder(t *testing.T) {
	exec := &fakeToolExecutor{fn: func(ctx context.Context, call llmtool.ToolCall, _ llmtool.CallBackFuncs) (string, error) {
		if call.Name == "slow" {
			time.Sleep(100 * time.Millisecond) // 慢工具先完成不了，验证结果仍保序
		}
		return "result:" + call.Name, nil
	}}
	o := newTestOrchestrator(exec)

	calls := []llmtool.ToolCall{
		{ID: "call_slow", Name: "slow", Arguments: "{}"},
		{ID: "call_fast", Name: "fast", Arguments: "{}"},
		{ID: "call_third", Name: "third", Arguments: "{}"},
	}
	results, err := o.executeToolCalls(context.Background(), calls, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, want := range []string{"call_slow", "call_fast", "call_third"} {
		if results[i].ToolCallID != want {
			t.Fatalf("result[%d] ToolCallID = %q, want %q", i, results[i].ToolCallID, want)
		}
		if got := ExtractMessageText(results[i]); got != "result:"+calls[i].Name {
			t.Fatalf("result[%d] text = %q, want %q", i, got, "result:"+calls[i].Name)
		}
	}
}

// TestExecuteToolCallsErrorContinues 单个工具失败不中断其余工具，错误转为结果文本。
func TestExecuteToolCallsErrorContinues(t *testing.T) {
	var ran []string
	var mu sync.Mutex
	exec := &fakeToolExecutor{fn: func(ctx context.Context, call llmtool.ToolCall, _ llmtool.CallBackFuncs) (string, error) {
		mu.Lock()
		ran = append(ran, call.Name)
		mu.Unlock()
		if call.Name == "bad" {
			return "", errors.New("boom")
		}
		return "result:" + call.Name, nil
	}}
	o := newTestOrchestrator(exec)

	calls := []llmtool.ToolCall{
		{ID: "c1", Name: "bad", Arguments: "{}"},
		{ID: "c2", Name: "good", Arguments: "{}"},
	}
	results, err := o.executeToolCalls(context.Background(), calls, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if got := ExtractMessageText(results[0]); !strings.Contains(got, "Error executing tool: boom") {
		t.Fatalf("bad tool result = %q, want error text", got)
	}
	if got := ExtractMessageText(results[1]); got != "result:good" {
		t.Fatalf("good tool result = %q", got)
	}
	if len(ran) != 2 {
		t.Fatalf("expected both tools ran, got %v", ran)
	}
}

// TestExecuteToolCallsPanicIsolated 工具 panic 转为错误文本，不传染其他工具与进程。
func TestExecuteToolCallsPanicIsolated(t *testing.T) {
	exec := &fakeToolExecutor{fn: func(ctx context.Context, call llmtool.ToolCall, _ llmtool.CallBackFuncs) (string, error) {
		if call.Name == "panic" {
			panic("tool blew up")
		}
		return "result:" + call.Name, nil
	}}
	o := newTestOrchestrator(exec)

	calls := []llmtool.ToolCall{
		{ID: "c1", Name: "panic", Arguments: "{}"},
		{ID: "c2", Name: "ok", Arguments: "{}"},
	}
	results, err := o.executeToolCalls(context.Background(), calls, llmtool.CallBackFuncs{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := ExtractMessageText(results[0]); !strings.Contains(got, "Error executing tool") || !strings.Contains(got, "tool blew up") {
		t.Fatalf("panic tool result = %q, want error text", got)
	}
	if got := ExtractMessageText(results[1]); got != "result:ok" {
		t.Fatalf("ok tool result = %q", got)
	}
}

// TestExecuteToolCallsContextCancel 上下文取消时返回 ctx.Err()（与串行版本语义一致）。
func TestExecuteToolCallsContextCancel(t *testing.T) {
	exec := &fakeToolExecutor{fn: func(ctx context.Context, call llmtool.ToolCall, _ llmtool.CallBackFuncs) (string, error) {
		<-ctx.Done() // 等待取消
		return "", ctx.Err()
	}}
	o := newTestOrchestrator(exec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 预先取消

	_, err := o.executeToolCalls(ctx, []llmtool.ToolCall{{ID: "c1", Name: "x", Arguments: "{}"}}, llmtool.CallBackFuncs{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

// TestExecuteToolCallsObserverAndCallbacksRace 并行执行下观察者回调全部触发、
// 回调经互斥串行化无数据竞争（配合 -race 运行）。
func TestExecuteToolCallsObserverAndCallbacksRace(t *testing.T) {
	const n = 8
	var cbTexts []string
	exec := &fakeToolExecutor{fn: func(ctx context.Context, call llmtool.ToolCall, cbs llmtool.CallBackFuncs) (string, error) {
		if cbs.SendText != nil {
			if _, err := cbs.SendText("mid:" + call.Name); err != nil {
				return "", err
			}
		}
		return "result:" + call.Name, nil
	}}
	o := newTestOrchestrator(exec)

	var observed int32
	o.SetToolObserver(func(info ToolCallInfo) {
		atomic.AddInt32(&observed, 1)
		if info.Name == "" {
			t.Errorf("observer got empty tool name")
		}
	})

	calls := make([]llmtool.ToolCall, 0, n)
	for i := 0; i < n; i++ {
		calls = append(calls, llmtool.ToolCall{ID: fmt.Sprintf("c%d", i), Name: fmt.Sprintf("t%d", i), Arguments: "{}"})
	}
	cbs := llmtool.CallBackFuncs{SendText: func(s string) (string, error) {
		cbTexts = append(cbTexts, s) // 生产代码已串行化，此处无锁依赖 -race 检出
		return s, nil
	}}

	results, err := o.executeToolCalls(context.Background(), calls, cbs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if int(observed) != n {
		t.Fatalf("observer called %d times, want %d", observed, n)
	}
	if len(cbTexts) != n {
		t.Fatalf("callback called %d times, want %d", len(cbTexts), n)
	}
	for i, r := range results {
		if got := ExtractMessageText(r); got != "result:t"+fmt.Sprint(i) {
			t.Fatalf("result[%d] = %q", i, got)
		}
	}
}
