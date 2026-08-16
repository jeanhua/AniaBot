package aichat

import (
	"context"

	"github.com/jeanhua/AniaBot/bot/component/agenthook"
)

// HookRunner 钩子执行器窄接口：由插件层实现并注入（agenthook.Manager），
// aichat 引擎本身不执行任何 shell 命令，只做事件触发与结果采纳。
// 未注入（nil）时所有钩子埋点自动跳过。
type HookRunner interface {
	Run(ctx context.Context, ev agenthook.Event, p agenthook.Payload) agenthook.Result
}

// PromptBlockedError UserPromptSubmit 钩子阻断用户输入时由 Chat 返回；
// 插件层应把 Reason 告知用户后按正常流程继续（不算请求失败，不丢弃排队消息）。
type PromptBlockedError struct {
	Reason string
}

func (e *PromptBlockedError) Error() string {
	return "prompt blocked by hook: " + e.Reason
}

// hookToolResultRunes PostToolUse 载荷中工具结果的截断长度
const hookToolResultRunes = 1000

// truncateRunes 按 rune 截断，避免切在多字节字符中间产生非法 UTF-8
func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
