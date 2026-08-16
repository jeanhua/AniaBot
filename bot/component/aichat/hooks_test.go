package aichat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeanhua/AniaBot/bot/component/agenthook"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// fakeHookRunner 测试用钩子执行器
type fakeHookRunner struct {
	run func(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result
}

func (f *fakeHookRunner) Run(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result {
	return f.run(ctx, ev, p)
}

// TestChatUserPromptSubmitBlock UserPromptSubmit 钩子阻断：返回 PromptBlockedError，
// 且不发起任何 LLM 请求。
func TestChatUserPromptSubmitBlock(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		fakeChatHandler(w, r)
	}))
	defer srv.Close()

	bot, err := NewChatBot(srv.URL, "test-key", "test-model", "系统提示词", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewChatBot: %v", err)
	}
	bot.SetHookRunner(&fakeHookRunner{run: func(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result {
		if ev == agenthook.EventUserPromptSubmit {
			if p.Prompt != "你好" {
				t.Errorf("载荷 Prompt = %q, want 用户输入", p.Prompt)
			}
			return agenthook.Result{Block: true, Reason: "内容违规"}
		}
		return agenthook.Result{}
	}}, "g:1", agenthook.AgentKindMain)

	_, _, err = bot.Chat(context.Background(), "你好", llmtool.CallBackFuncs{}, ChatOptions{})
	var blocked *PromptBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("应返回 PromptBlockedError, got %v", err)
	}
	if blocked.Reason != "内容违规" {
		t.Fatalf("Reason = %q", blocked.Reason)
	}
	if calls != 0 {
		t.Fatalf("被阻断的请求不应调用 LLM, calls=%d", calls)
	}
}

// TestChatUserPromptSubmitContext 钩子产出的上下文尾部注入到用户消息前（system 保持
// 字节稳定，不破前缀缓存）。
func TestChatUserPromptSubmitContext(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bs, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(bs))
		fakeChatHandler(w, r)
	}))
	defer srv.Close()

	bot, err := NewChatBot(srv.URL, "test-key", "test-model", "系统提示词", 0, nil, nil)
	if err != nil {
		t.Fatalf("NewChatBot: %v", err)
	}
	bot.SetHookRunner(&fakeHookRunner{run: func(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result {
		if ev == agenthook.EventUserPromptSubmit {
			return agenthook.Result{Context: "【钩子上下文】"}
		}
		return agenthook.Result{}
	}}, "g:1", agenthook.AgentKindMain)

	if _, _, err := bot.Chat(context.Background(), "你好", llmtool.CallBackFuncs{}, ChatOptions{}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(bodies) != 1 {
		t.Fatalf("expected 1 request, got %d", len(bodies))
	}
	body := bodies[0]
	ctxIdx := strings.Index(body, "【钩子上下文】")
	inputIdx := strings.Index(body, "你好")
	if ctxIdx < 0 || inputIdx < 0 || ctxIdx > inputIdx {
		t.Fatalf("注入上下文应在用户输入之前: %s", body)
	}
	if strings.Count(body, "系统提示词") != 1 {
		t.Fatalf("system 应保持原样且仅出现一次: %s", body)
	}
}

// TestPreCompactHook 上下文压缩即将发生时触发 PreCompact 钩子（仅通知）。
func TestPreCompactHook(t *testing.T) {
	// llmClient 需非 nil（否则 MaybeCompress 前置守卫直接返回）；压缩函数是桩，不会真正调用
	w := newMessageWindow(100, newTestClient("http://unused.local"), func(ctx context.Context, client *LLMClient, oldMsgs []Message) ([]Message, TokenUsage, error) {
		return []Message{TextMessage(RoleUser, "[对话摘要]")}, TokenUsage{}, nil
	}, nil)
	w.messages = []Message{TextMessage(RoleUser, "旧消息")}
	w.lastPromptTokens = 90 // 超过 80% 阈值

	var fired []agenthook.Event
	w.setHookRunner(&fakeHookRunner{run: func(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result {
		fired = append(fired, ev)
		return agenthook.Result{}
	}}, agenthook.Payload{SessionKey: "g:1", AgentKind: agenthook.AgentKindMain})

	if err := w.MaybeCompress(context.Background()); err != nil {
		t.Fatalf("MaybeCompress: %v", err)
	}
	if len(fired) != 1 || fired[0] != agenthook.EventPreCompact {
		t.Fatalf("压缩前应触发一次 PreCompact, fired=%v", fired)
	}
	if len(w.messages) != 1 || ExtractMessageText(w.messages[0]) != "[对话摘要]" {
		t.Fatalf("压缩应照常进行, messages=%v", w.messages)
	}
}
