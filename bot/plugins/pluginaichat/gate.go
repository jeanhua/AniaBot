package pluginaichat

import (
	"context"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/agenthook"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// buildPreToolGate 组装请求级工具门禁（aichat.ChatOptions.PreToolGate），
// 顺序固定：计划模式 → PreToolUse 钩子 → 工具审批。
// 理由：计划模式是内存判断（最便宜）；钩子要跑 shell（有界 10s/个）；审批要等真人
// （最贵，放最后——已被计划模式/钩子否决的工具不应再打扰用户审批）。
// bash 的命令级审批在工具内部更深处（见 functool/bash.go）。
//
// 主会话 / 子代理 / 定时任务三条执行路径共用本门禁：requester 为 message.FromUint64(0)
// 时仅管理员可批（子代理/定时任务路径）。各管理器为 nil 时对应环节自动跳过；
// 实现并发安全（同轮并行工具调用各自过门；审批的会话级串行化由 approvalManager 负责）。
//
// sendPrompt 为审批提示的纯发送闭包（不走工具回调 SendText——子代理/定时任务路径
// 的 SendText 是丢弃桩，且纯发送与进行中的流式消息互不干扰）。
func (p *AIChatPlugin) buildPreToolGate(sKey, agentKind string, requester message.QID, sendPrompt func(text string)) func(context.Context, llmtool.ToolCall) (bool, string) {
	return func(ctx context.Context, call llmtool.ToolCall) (bool, string) {
		// 1. 计划模式：副作用工具直接阻断，AI 只输出计划
		if p.planManager != nil && p.planManager.IsOn(sKey) {
			if _, blocked := planBlockedTools[call.Name]; blocked {
				return true, "【计划模式】当前处于计划模式（/plan on），工具 " + call.Name + " 已被阻止。请只做分析并输出详细实施计划，不要尝试执行，等待用户发送 /plan off 批准。"
			}
		}
		// 2. PreToolUse 钩子：管理员配置的 shell 命令 / 其他插件的 Go 钩子可阻断调用
		if p.hookManager != nil {
			res := p.hookManager.Run(ctx, agenthook.EventPreToolUse, agenthook.Payload{
				SessionKey: sKey,
				AgentKind:  agentKind,
				ToolName:   call.Name,
				ToolInput:  call.Arguments,
			})
			if res.Block {
				reason := res.Reason
				if reason == "" {
					reason = "钩子要求阻断"
				}
				return true, "工具调用被钩子阻止: " + reason
			}
		}
		// 3. 工具级审批：危险工具执行前向会话发送确认消息，请求者或管理员回复授权
		if p.approvalManager != nil && p.approvalManager.needsApproval(call.Name) {
			// request 返回 (allowed, reason)，门禁语义为 (block, result)，注意取反
			allowed, reason := p.approvalManager.request(ctx, sKey, call.Name, summarizeToolArgs(call.Arguments), requester, sendPrompt)
			if !allowed {
				return true, "工具调用未获批准: " + reason
			}
			return false, ""
		}
		return false, ""
	}
}

// summarizeToolArgs 审批提示中的参数摘要：压缩空白并按 rune 截断，
// 避免超长参数刷屏（完整参数可在面板 Query 日志中查看）。
func summarizeToolArgs(args string) string {
	const maxRunes = 300
	compact := strings.Join(strings.Fields(args), " ")
	return querylog.Truncate(compact, maxRunes)
}
