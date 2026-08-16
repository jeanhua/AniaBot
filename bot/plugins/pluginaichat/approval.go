package pluginaichat

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/oplog"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// 工具审批：危险工具（默认 file/config_set，bash 另有命令级审批）执行前向会话
// 发送确认消息，由请求发送者或机器人管理员回复「允许/拒绝」授权；超时自动拒绝。

// approvalVerdict 审批结论（含操作者，供审计）
type approvalVerdict struct {
	allow bool
	by    message.QID
}

// approvalRequest 一次待批请求（每会话同时只有一个）
type approvalRequest struct {
	tool      string
	summary   string
	requester message.QID // message.FromUint64(0) = 仅管理员可批（子代理/定时任务路径）
	resultCh  chan approvalVerdict
}

// approvalManager 工具审批管理器。并发安全。
type approvalManager struct {
	tools   map[string]struct{} // 需审批的工具名（工具级门）
	timeout time.Duration
	admin   message.QID
	logger  *slog.Logger

	mu      sync.Mutex
	pending map[string]*approvalRequest // sessionKey → 当前待批
	locks   sync.Map                    // sessionKey → *sync.Mutex：同会话多个审批串行（并行工具调用逐个提示，不并发刷屏）
}

func newApprovalManager(tools []string, timeoutSec int, admin message.QID, logger *slog.Logger) *approvalManager {
	set := make(map[string]struct{}, len(tools))
	for _, t := range tools {
		if t = strings.TrimSpace(t); t != "" {
			set[t] = struct{}{}
		}
	}
	// 限幅：过短会来不及回复，过长会吃光消息处理预算（bot.msg_event_timeout_sec）
	timeoutSec = min(max(timeoutSec, 10), 240)
	return &approvalManager{
		tools:   set,
		timeout: time.Duration(timeoutSec) * time.Second,
		admin:   admin,
		logger:  logger,
		pending: make(map[string]*approvalRequest),
	}
}

func (m *approvalManager) needsApproval(tool string) bool {
	_, ok := m.tools[tool]
	return ok
}

// request 发起一次审批：发送提示消息后阻塞等待回复/取消/超时。
// 返回 (allowed, reason)：reason 在拒绝时说明原因（拒绝/超时/取消）。
// 会话级互斥锁保证同会话并行工具调用的多个审批逐个提示。
// 注意：本方法可能阻塞至超时（默认 120s），调用方不得持锁调用。
func (m *approvalManager) request(ctx context.Context, sKey, tool, summary string, requester message.QID, sendPrompt func(text string)) (bool, string) {
	lockAny, _ := m.locks.LoadOrStore(sKey, &sync.Mutex{})
	lock := lockAny.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	req := &approvalRequest{tool: tool, summary: summary, requester: requester, resultCh: make(chan approvalVerdict, 1)}
	m.mu.Lock()
	m.pending[sKey] = req
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		delete(m.pending, sKey)
		m.mu.Unlock()
	}()

	sendPrompt(m.formatPrompt(tool, summary, requester))
	m.logger.Info("发起工具审批", "session", sKey, "tool", tool, "requester", requester)

	select {
	case v := <-req.resultCh:
		if v.allow {
			m.audit(sKey, tool, summary, "允许", v.by)
			return true, ""
		}
		m.audit(sKey, tool, summary, "拒绝", v.by)
		return false, "用户拒绝了本次操作"
	case <-ctx.Done():
		// /stop 取消请求（同一 chatCtx）：不记审计（未形成决定）
		return false, "审批等待已取消（请求被停止）"
	case <-time.After(m.timeout):
		m.audit(sKey, tool, summary, "超时自动拒绝", message.FromUint64(0))
		return false, fmt.Sprintf("审批超时（%d 秒无回复），已自动拒绝", int(m.timeout.Seconds()))
	}
}

// tryHandleReply 尝试把一条新消息当作审批回复处理。返回 true 表示已消费
// （调用方应停止后续插件传播与正常聊天流程）。
// 权限：仅请求发送者或机器人管理员可批；requester 为 0（子代理/定时任务路径）时
// 仅管理员。非审批内容/无权发送者的消息返回 false，走正常流程。
func (m *approvalManager) tryHandleReply(id message.QID, isGroup bool, sender message.QID, text string) bool {
	sKey := sessionKey(id, isGroup)
	m.mu.Lock()
	req, ok := m.pending[sKey]
	m.mu.Unlock()
	if !ok {
		return false // 快速路径：无待批请求，正常消息零干扰
	}
	if sender != req.requester && sender != m.admin {
		return false
	}
	allow, ok := parseApprovalReply(text)
	if !ok {
		return false
	}
	// resultCh 带缓冲：即使等待方恰好超时退出，发送也不会阻塞
	req.resultCh <- approvalVerdict{allow: allow, by: sender}
	m.logger.Info("审批回复", "session", sKey, "tool", req.tool, "allow", allow, "by", sender)
	return true
}

// formatPrompt 审批提示消息文本（纯文本路径发送；群聊中 requester 非 0 时
// 文本注明其 ID——发送闭包是 @ 无关的纯发送，避免与流式消息/丢弃桩冲突）。
func (m *approvalManager) formatPrompt(tool, summary string, requester message.QID) string {
	var sb strings.Builder
	sb.WriteString("【工具审批】AI 请求执行工具：" + tool)
	if summary != "" {
		sb.WriteString("\n参数摘要：" + summary)
	}
	if requester == message.FromUint64(0) {
		fmt.Fprintf(&sb, "\n请管理员在 %d 秒内回复「允许」或「拒绝」（超时自动拒绝）", int(m.timeout.Seconds()))
	} else {
		fmt.Fprintf(&sb, "\n请消息发送者（%s）或管理员在 %d 秒内回复「允许」或「拒绝」（超时自动拒绝）",
			requester.String(), int(m.timeout.Seconds()))
	}
	return sb.String()
}

func (m *approvalManager) audit(sKey, tool, summary, verdict string, by message.QID) {
	detail := fmt.Sprintf("工具 %s %s（会话 %s）", tool, verdict, sKey)
	if by != message.FromUint64(0) {
		detail += "，操作者 " + by.String()
	}
	oplog.Record(oplog.CategoryAI, "tool_approval", detail)
}

// parseApprovalReply 解析审批回复：ok=false 表示不是审批回复。
func parseApprovalReply(text string) (allow, ok bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "允许", "同意", "批准", "allow", "approve", "yes", "y":
		return true, true
	case "拒绝", "deny", "reject", "no", "n":
		return false, true
	}
	return false, false
}
