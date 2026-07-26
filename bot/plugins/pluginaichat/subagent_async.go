package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// asyncSubagentEntry 记录一个正在运行的异步子代理
type asyncSubagentEntry struct {
	id        string
	cancel    context.CancelFunc
	task      string
	startTime time.Time
}

// asyncSubagentGroup 单个会话的异步子代理集合（支持多子代理并发）
type asyncSubagentGroup struct {
	mu      sync.Mutex
	entries []asyncSubagentEntry
}

func (g *asyncSubagentGroup) add(id string, cancel context.CancelFunc, task string) {
	g.mu.Lock()
	g.entries = append(g.entries, asyncSubagentEntry{
		id:        id,
		cancel:    cancel,
		task:      task,
		startTime: time.Now(),
	})
	g.mu.Unlock()
}

func (g *asyncSubagentGroup) remove(id string) {
	g.mu.Lock()
	filtered := make([]asyncSubagentEntry, 0, len(g.entries))
	for _, e := range g.entries {
		if e.id != id {
			filtered = append(filtered, e)
		}
	}
	g.entries = filtered
	g.mu.Unlock()
}

func (g *asyncSubagentGroup) cancelAll() {
	g.mu.Lock()
	for _, e := range g.entries {
		e.cancel()
	}
	g.entries = nil
	g.mu.Unlock()
}

func (g *asyncSubagentGroup) cancelOne(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, e := range g.entries {
		if e.id == id {
			e.cancel()
			return true
		}
	}
	return false
}

// snapshot 返回当前运行中子代理的快照（副本）
func (g *asyncSubagentGroup) snapshot() []asyncSubagentEntry {
	g.mu.Lock()
	defer g.mu.Unlock()
	cp := make([]asyncSubagentEntry, len(g.entries))
	copy(cp, g.entries)
	return cp
}

// getAsyncGroup 获取或创建会话的异步子代理组
func (p *AIChatPlugin) getAsyncGroup(id message.QID, isGroup bool) *asyncSubagentGroup {
	key := sessionKey(id, isGroup)
	v, _ := p.asyncSubagents.LoadOrStore(key, &asyncSubagentGroup{})
	return v.(*asyncSubagentGroup)
}

// cancelAsyncSubagents 取消指定会话所有运行中的异步子代理
func (p *AIChatPlugin) cancelAsyncSubagents(id message.QID, isGroup bool) {
	key := sessionKey(id, isGroup)
	if v, ok := p.asyncSubagents.LoadAndDelete(key); ok {
		v.(*asyncSubagentGroup).cancelAll()
	}
}

// listRunningSubagents 返回当前会话运行中的异步子代理列表（供 subagent_list 工具使用）
func (p *AIChatPlugin) listRunningSubagents(id message.QID, isGroup bool) []asyncSubagentEntry {
	g := p.getAsyncGroup(id, isGroup)
	return g.snapshot()
}

// cancelSubagentByID 按 ID 取消单个运行中的异步子代理（供 subagent_cancel 工具使用）
func (p *AIChatPlugin) cancelSubagentByID(id message.QID, isGroup bool, subagentID string) bool {
	g := p.getAsyncGroup(id, isGroup)
	return g.cancelOne(subagentID)
}

// launchAsyncSubagent 在后台 goroutine 中启动子代理，立即返回占位文本。
//
// ctx 为主请求上下文：仅用于检查剩余时间预算是否 ≥ subagentParentReserve。
// 子代理的实际执行使用独立的 context.WithCancel(context.Background()) ——
// 无 deadline，让 runSubagent 内部的 resolveSubagentTimeout + WithTimeout 自行管理
// 超时；/stop 通过 asyncSubagentGroup 注册的 cancel 取消。
func (p *AIChatPlugin) launchAsyncSubagent(
	ctx context.Context,
	b bot.Bot,
	id message.QID,
	isGroup bool,
	task string,
	timeoutSec int,
	parentCbs llmtool.CallBackFuncs,
) string {
	// 检查父请求剩余预算
	if _, err := resolveSubagentTimeout(p.subagentTimeout(), timeoutSec, ctx); err != nil {
		return fmt.Sprintf("无法启动子代理: %v", err)
	}

	launchID := fmt.Sprintf("%d", time.Now().UnixNano())

	// 无 deadline 的 cancel context：runSubagent 内部自行通过 resolveSubagentTimeout
	// 创建带 timeout 的子 context，外部 cancel（/stop）不影响超时计算，且
	// asyncCtx.Err() == nil 让 runSubagent 能正确区分「子代理自身超时」和「外部取消」
	asyncCtx, asyncCancel := context.WithCancel(context.Background())

	p.getAsyncGroup(id, isGroup).add(launchID, asyncCancel, task)

	b.Go("subagent:"+sessionKey(id, isGroup)+":"+launchID, func() {
		defer p.getAsyncGroup(id, isGroup).remove(launchID)
		result, runErr := p.runSubagent(asyncCtx, b, id, isGroup, task, timeoutSec, parentCbs)
		p.onSubagentComplete(b, id, isGroup, task, result, runErr)
	})

	return fmt.Sprintf("✅ 子代理已启动（ID: %s），正在后台执行任务\n任务内容: %s",
		launchID, task)
}

// onSubagentComplete 子代理完成后的回调（在子代理 goroutine 中执行）。
//
// 1. 格式化结果文本
// 2. 构造合成消息并入队 pending 队列
// 3. 若会话空闲则自动触发 AI 处理
func (p *AIChatPlugin) onSubagentComplete(b bot.Bot, id message.QID, isGroup bool, task string, result string, err error) {
	var text string
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "超时"):
			text = fmt.Sprintf("【子代理执行超时】\n任务: %s\n%s", task, err.Error())
		case errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "canceled"):
			p.Logger.Info("异步子代理已被取消", "id", id, "is_group", isGroup, "task", task)
			return // 被 /stop 取消，不注入结果
		default:
			text = fmt.Sprintf("【子代理执行失败】\n任务: %s\n错误: %s", task, err.Error())
		}
	} else {
		text = fmt.Sprintf("【子代理执行完成】\n任务: %s\n\n%s", task, result)
	}

	// 构造合成消息
	syntheticMsg := message.Message{
		Sender: message.MessageSender{
			Nickname: "子代理",
			UserId:   message.FromUint64(0),
		},
		Message: []message.OB11Segment{
			{Type: message.SegmentText, Data: map[string]any{"text": text}},
		},
	}

	// 入队
	first, ok := p.enqueuePending(id, isGroup, syntheticMsg)
	if !ok {
		p.Logger.Warn("异步子代理结果入队失败（队列已满）", "id", id, "is_group", isGroup)
		return
	}

	if first {
		p.Logger.Info("异步子代理结果已入队（队列首条）", "id", id, "is_group", isGroup, "task", task)
	} else {
		p.Logger.Info("异步子代理结果已入队", "id", id, "is_group", isGroup, "task", task)
	}

	// 尝试触发处理：若会话空闲（锁可用）则立即处理排队消息
	p.tryProcessPending(b, id, isGroup)
}

// tryProcessPending 尝试获取会话锁并处理排队消息。
// 若锁不可用（会话正在响应中），排队消息会在当前响应结束后的 drain 循环中被处理。
func (p *AIChatPlugin) tryProcessPending(b bot.Bot, id message.QID, isGroup bool) {
	if !p.tryLock(id, isGroup) {
		return
	}
	defer p.unLock(id, isGroup)

	chat := p.getChat(b, id, isGroup, p.getPromptForID(id, isGroup))
	if chat == nil {
		p.drainPending(id, isGroup)
		p.Logger.Error("tryProcessPending: 无法获取 ChatBot", "id", id, "is_group", isGroup)
		return
	}

	chatCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	p.setActiveContext(id, isGroup, cancel)
	defer p.clearActiveContext(id, isGroup)

	batch := p.drainPending(id, isGroup)
	for len(batch) > 0 {
		if !p.processChatBatch(chatCtx, b, id, isGroup, chat, batch) {
			p.drainPending(id, isGroup)
			return
		}
		batch = p.drainPending(id, isGroup)
	}
}
