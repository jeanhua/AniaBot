package pluginaichat

import (
	"context"
	"errors"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/querylog"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// Query 日志：记录每次 AI 回复（群 @ / 私聊触发）从收到消息到最终响应的完整过程，
// 持久化到 PersistentStorage（querylog: 命名空间），供 Web 面板「Query 日志」页展示。

// initQueryLogger 按配置初始化 Query 日志记录器；未启用时保持 nil（埋点自动跳过）。
func (p *AIChatPlugin) initQueryLogger() {
	if !p.cfg.QueryLog.Enable {
		p.Logger.Info("Query 日志功能未启用（plugin.ai_chat_bot.query_log.enable=false）")
		return
	}
	maxEntries := p.cfg.QueryLog.MaxEntries
	if maxEntries <= 0 {
		maxEntries = 200
	}
	p.queryLogger = querylog.New(p.PersistentStorage.Clone("querylog:"), maxEntries, p.Logger.WithGroup("querylog"))
	// 重启前未正常收尾的执行中记录（如等待工具审批时进程退出）统一标记为中断，
	// 避免面板一直显示「执行中」；内存中的审批/会话状态已随进程消失，无法恢复
	if n := p.queryLogger.MarkRunningInterrupted(); n > 0 {
		p.Logger.Info("已将重启前遗留的执行中 Query 日志标记为中断", "count", n)
	}
}

// QueryLogRecent 按条件查询 Query 日志（新在前），实现 adminpanel.QueryLogSource。
func (p *AIChatPlugin) QueryLogRecent(f querylog.Filter) []querylog.Entry {
	if p.queryLogger == nil {
		return nil
	}
	return p.queryLogger.Query(f)
}

// queryRecorder 一次 Query 的日志记录上下文。processChatBatch 在会话锁内串行执行，
// 且工具观测回调与 Chat 同 goroutine，因此 toolCalls 无需额外加锁。
type queryRecorder struct {
	entry          querylog.Entry
	toolCalls      []querylog.ToolCallRecord
	toolCallsTotal int // 工具调用总数（含超出上限被丢弃的）
	start          time.Time
	key            string // 会话 key（takeExtraUsage 用）
}

// beginQuery 开始记录一次 Query：写入 running 状态的日志并挂载工具调用观察者。
// 观察者把每次工具调用增量落盘（明细 + 已耗时），面板执行中即可见进度。
// 日志功能未启用时返回 nil，调用方直接跳过。
func (p *AIChatPlugin) beginQuery(chat *aichat.ChatBot, id message.QID, isGroup bool, batch []message.Message, query string) *queryRecorder {
	if p.queryLogger == nil {
		return nil
	}
	chatType := "friend"
	if isGroup {
		chatType = "group"
	}
	senders := make([]string, 0, len(batch))
	seen := make(map[message.QID]struct{}, len(batch))
	for i := range batch {
		uid := batch[i].Sender.UserId
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		senders = append(senders, uid.String())
	}
	r := &queryRecorder{start: time.Now(), key: sessionKey(id, isGroup)}
	r.entry = p.queryLogger.Record(querylog.Entry{
		ChatType: chatType,
		TargetID: id.String(),
		Senders:  senders,
		Query:    querylog.Truncate(query, querylog.MaxQueryRunes),
		Status:   querylog.StatusRunning,
	})
	chat.SetToolObserver(func(info aichat.ToolCallInfo) {
		p.onToolCall(r, info)
	})
	return r
}

// onToolCall 工具调用观察者回调：累计明细并即时增量落盘——面板按 running 状态
// 轮询时能看到执行中的工具调用进度与已耗时，无需等 finishQuery 才可见中间流程。
// 观察者由 orchestrator 串行调用（无需额外加锁），Logger.Update 内部有互斥。
func (p *AIChatPlugin) onToolCall(r *queryRecorder, info aichat.ToolCallInfo) {
	r.toolCallsTotal++
	rec := querylog.ToolCallRecord{
		Name:       info.Name,
		Arguments:  querylog.Truncate(info.Arguments, querylog.MaxArgsRunes),
		Result:     querylog.Truncate(info.Result, querylog.MaxResultRunes),
		DurationMs: info.DurationMs,
	}
	if info.Err != nil {
		rec.Error = info.Err.Error()
	}
	if len(r.toolCalls) < querylog.MaxToolCallRecords {
		r.toolCalls = append(r.toolCalls, rec) // 明细最多保留 MaxToolCallRecords 条，总数仍计入 ToolCallsTotal
	}
	p.queryLogger.Update(r.entry.ID, func(e *querylog.Entry) {
		// 复制 slice 再存入，避免与后续 append 共享底层数组
		e.ToolCalls = append([]querylog.ToolCallRecord(nil), r.toolCalls...)
		e.ToolCallsTotal = r.toolCallsTotal
		e.DurationMs = time.Since(r.start).Milliseconds()
	})
}

// finishQuery 结束记录：回填状态、耗时、token 用量、工具调用明细与最终回复/错误。
func (p *AIChatPlugin) finishQuery(r *queryRecorder, chat *aichat.ChatBot, usage aichat.TokenUsage, reply string, err error) {
	if r == nil {
		return
	}
	chat.SetToolObserver(nil)
	status := querylog.StatusSuccess
	errText := ""
	if err != nil {
		errText = err.Error()
		switch {
		case errors.Is(err, context.Canceled):
			status = querylog.StatusStopped
		case errors.Is(err, context.DeadlineExceeded):
			status = querylog.StatusTimeout
		default:
			status = querylog.StatusError
		}
	}
	toolCalls := r.toolCalls
	// 并入主请求循环之外派生的消耗（异步子代理 / team_run 成员 / 备用图片识别，
	// 见 usageacc.go），使统计反映会话的完整成本（配额在各派生调用点单独累加）
	extra := p.takeExtraUsage(r.key)
	p.queryLogger.Update(r.entry.ID, func(e *querylog.Entry) {
		e.Status = status
		e.DurationMs = time.Since(r.start).Milliseconds()
		e.Iterations = usage.Iterations + extra.Iterations
		e.PromptTokens = usage.PromptTokens + extra.PromptTokens
		e.CompletionTokens = usage.CompletionTokens + extra.CompletionTokens
		e.TotalTokens = usage.TotalTokens + extra.TotalTokens
		e.CachedTokens = usage.CachedTokens + extra.CachedTokens
		e.ToolCalls = toolCalls
		e.ToolCallsTotal = r.toolCallsTotal
		e.Reply = querylog.Truncate(reply, querylog.MaxReplyRunes)
		e.Error = errText
	})
}
