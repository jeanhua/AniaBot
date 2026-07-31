package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/component/tasklog"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/msgchain"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/jeanhua/AniaBot/common/storage"
	"github.com/robfig/cron/v3"
)

// 触发对象类型
const (
	clockTargetGroup  = "group"
	clockTargetFriend = "friend"
)

const (
	clockTaskPrefix = "task:"
	clockIndexKey   = "index"
	clockSeqKey     = "seq"
)

// ClockTask AI 定时任务。
//
// 与框架的 robfig/cron 静态任务（StartCron 注册）不同，ClockTask 由 AI / 用户动态
// 创建、持久化到 PersistentStorage，由 clockManager 拥有的独立 *cron.Cron 实例调度。
// 触发时以全新的一次性上下文（无历史持久化）执行，结束后丢弃，支持完整工具调用流程。
type ClockTask struct {
	ID         string      `json:"id"`
	Cron       string      `json:"cron"`        // cron 表达式（5 字段或 @every 等）
	Title      string      `json:"title"`       // 任务标题
	Content    string      `json:"content"`     // 任务内容，触发时作为对话内容发送给 AI
	TargetType string      `json:"target_type"` // group / friend
	TargetID   string      `json:"target_id"`   // 群号或 QQ 号
	Enabled    bool        `json:"enabled"`
	RunOnce    bool        `json:"run_once"`    // 单次任务：触发执行完成后自动销毁
	TimeoutSec int         `json:"timeout_sec"` // 单次执行超时秒数，<=0 用默认值
	CreatedBy  message.QID `json:"created_by"`  // 创建者 QQ（用于群聊 @ 提醒），0 表示无
	Note       string      `json:"note,omitempty"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
	LastRunAt  time.Time   `json:"last_run_at,omitempty"`
	NextRunAt  time.Time   `json:"next_run_at,omitempty"`
}

// ClockUpdateFields 定时任务可更新字段，指针类型表示「仅当提供时才更新」。
type ClockUpdateFields struct {
	Cron       *string
	Title      *string
	Content    *string
	TargetType *string
	TargetID   *string
	Enabled    *bool
	RunOnce    *bool
	TimeoutSec *int
	Note       *string
}

// clockManager AI 定时任务调度器：持久化 + 调度 + 触发执行 + 执行日志。
//
// 拥有独立的 *cron.Cron 实例（不与框架的 cron 共用），从而与 StartCron 注册的
// 静态任务明确区分。所有状态变更在 mu 保护下落盘，保证重启不丢失。
type clockManager struct {
	plugin *AIChatPlugin
	store  storage.PersistentStorage
	log    *tasklog.Logger
	logger *slog.Logger

	defaultTimeout time.Duration

	mu          sync.Mutex
	tasks       map[string]*ClockTask
	entries     map[string]cron.EntryID
	seq         uint64
	cron        *cron.Cron
	cronStarted bool
	bot         bot.Bot
}

func newClockManager(p *AIChatPlugin, defaultTimeout time.Duration, maxLog int) *clockManager {
	m := &clockManager{
		plugin:         p,
		store:          p.PersistentStorage.Clone("clock:"),
		log:            tasklog.New(p.PersistentStorage.Clone("clocklog:"), maxLog, p.Logger.WithGroup("tasklog")),
		logger:         p.Logger.WithGroup("clock"),
		defaultTimeout: defaultTimeout,
		tasks:          map[string]*ClockTask{},
		entries:        map[string]cron.EntryID{},
		// SkipIfStillRunning：上一次执行未结束时跳过一次触发，
		// 防止短周期任务（如 @every 30s）执行超时后并发叠加
		// （重复推送消息、重复消耗 API 额度）
		cron: cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger))),
	}
	m.loadAll()
	return m
}

// recentLogs 返回最近的执行日志（新在前），供 /clock 命令查询。
func (m *clockManager) recentLogs(limit int) []tasklog.Entry {
	return m.log.Recent(limit)
}

// TaskLogQuery 按条件查询 AI 定时任务执行日志（clock 未启用时返回 nil），
// 实现 adminpanel.TaskLogSource，供 Web 控制面板「任务日志」页使用。
func (p *AIChatPlugin) TaskLogQuery(f tasklog.Filter) []tasklog.Entry {
	if p.clockManager == nil {
		return nil
	}
	return p.clockManager.log.Query(f)
}

// taskInfos 返回所有任务的面板展示信息（供 Web 控制面板）。
func (m *clockManager) taskInfos() []plugininfo.ClockTaskInfo {
	tasks := m.List()
	out := make([]plugininfo.ClockTaskInfo, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, plugininfo.ClockTaskInfo{
			ID:         t.ID,
			Title:      t.Title,
			Content:    t.Content,
			Note:       t.Note,
			Cron:       t.Cron,
			TargetType: t.TargetType,
			TargetID:   t.TargetID,
			Enabled:    t.Enabled,
			RunOnce:    t.RunOnce,
			TimeoutSec: t.TimeoutSec,
			CreatedBy:  t.CreatedBy.Uint64(),
			CreatedAt:  t.CreatedAt,
			LastRunAt:  t.LastRunAt,
			NextRunAt:  t.NextRunAt,
		})
	}
	return out
}

// ClockTasks 供 Web 控制面板查询 AI 定时任务列表（clock 未启用时返回 nil）。
func (p *AIChatPlugin) ClockTasks() []plugininfo.ClockTaskInfo {
	if p.clockManager == nil {
		return nil
	}
	return p.clockManager.taskInfos()
}

// CreateClockTask 供 Web 控制面板新建定时任务，返回新任务 ID。
func (p *AIChatPlugin) CreateClockTask(c plugininfo.ClockTaskCreate) (string, error) {
	if p.clockManager == nil {
		return "", fmt.Errorf("定时任务功能未启用")
	}
	return p.clockManager.Add(&ClockTask{
		Cron:       c.Cron,
		Title:      c.Title,
		Content:    c.Content,
		TargetType: c.TargetType,
		TargetID:   c.TargetID,
		Enabled:    c.Enabled,
		RunOnce:    c.RunOnce,
		TimeoutSec: c.TimeoutSec,
		Note:       c.Note,
	})
}

// UpdateClockTask 供 Web 控制面板编辑定时任务（仅更新提供的字段）。
func (p *AIChatPlugin) UpdateClockTask(id string, f plugininfo.ClockTaskUpdate) error {
	if p.clockManager == nil {
		return fmt.Errorf("定时任务功能未启用")
	}
	_, err := p.clockManager.Update(id, ClockUpdateFields{
		Cron:       f.Cron,
		Title:      f.Title,
		Content:    f.Content,
		Note:       f.Note,
		TimeoutSec: f.TimeoutSec,
		Enabled:    f.Enabled,
	})
	return err
}

// DeleteClockTask 供 Web 控制面板删除定时任务。
func (p *AIChatPlugin) DeleteClockTask(id string) error {
	if p.clockManager == nil {
		return fmt.Errorf("定时任务功能未启用")
	}
	if !p.clockManager.Delete(id) {
		return fmt.Errorf("定时任务不存在: %s", id)
	}
	return nil
}

// Start 在 Bot 就绪后启动调度器并调度所有已加载任务。由 Awake 调用。
func (m *clockManager) Start(b bot.Bot) {
	m.mu.Lock()
	m.bot = b
	m.cron.Start()
	m.cronStarted = true
	for _, t := range m.tasks {
		m.scheduleLocked(t)
	}
	count := len(m.tasks)
	m.mu.Unlock()
	m.logger.Info("AI定时任务调度器已启动", "tasks", count)
}

// ---- 持久化 ----

func (m *clockManager) loadAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var ids []string
	if ok := m.store.Get(context.Background(), clockIndexKey, &ids); !ok {
		// 无索引时回退扫描 task: 前缀，兼容直接写入的情况
		keys, err := m.store.Keys(context.Background(), clockTaskPrefix)
		if err == nil {
			for _, k := range keys {
				ids = append(ids, strings.TrimPrefix(k, clockTaskPrefix))
			}
		}
	}
	for _, id := range ids {
		var t ClockTask
		if ok := m.store.Get(context.Background(), clockTaskPrefix+id, &t); ok {
			if t.ID == "" {
				t.ID = id
			}
			m.tasks[id] = &t
		}
	}
	if s, ok := m.store.GetString(context.Background(), clockSeqKey); ok {
		if n, err := strconv.ParseUint(s, 10, 64); err == nil {
			m.seq = n
		}
	}
}

func (m *clockManager) nextID() string {
	m.seq++
	m.store.SetString(context.Background(), clockSeqKey, strconv.FormatUint(m.seq, 10))
	return strconv.FormatUint(m.seq, 36)
}

func (m *clockManager) persistTaskLocked(t *ClockTask) {
	m.store.Set(context.Background(), clockTaskPrefix+t.ID, t)
}

func (m *clockManager) persistIndexLocked() {
	ids := make([]string, 0, len(m.tasks))
	for id := range m.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	m.store.Set(context.Background(), clockIndexKey, ids)
}

// ---- 增删改查 ----

// Add 校验并新增定时任务，返回新任务 ID。
func (m *clockManager) Add(t *ClockTask) (string, error) {
	if t == nil {
		return "", fmt.Errorf("任务不能为空")
	}
	if _, err := cron.ParseStandard(t.Cron); err != nil {
		return "", fmt.Errorf("cron 表达式无效: %w", err)
	}
	if strings.TrimSpace(t.Content) == "" {
		return "", fmt.Errorf("任务内容不能为空")
	}
	if t.TargetType != clockTargetGroup && t.TargetType != clockTargetFriend {
		return "", fmt.Errorf("触发对象类型必须为 group 或 friend")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	t.ID = m.nextID()
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now
	m.tasks[t.ID] = t
	m.persistTaskLocked(t)
	m.persistIndexLocked()
	if m.cronStarted {
		m.scheduleLocked(t)
	}
	return t.ID, nil
}

// Update 按字段更新任务并重新调度。
func (m *clockManager) Update(id string, f ClockUpdateFields) (*ClockTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cur, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("定时任务不存在: %s", id)
	}
	nt := *cur
	if f.Cron != nil {
		if _, err := cron.ParseStandard(*f.Cron); err != nil {
			return nil, fmt.Errorf("cron 表达式无效: %w", err)
		}
		nt.Cron = *f.Cron
	}
	if f.Title != nil {
		nt.Title = *f.Title
	}
	if f.Content != nil {
		if strings.TrimSpace(*f.Content) == "" {
			return nil, fmt.Errorf("任务内容不能为空")
		}
		nt.Content = *f.Content
	}
	if f.TargetType != nil {
		if *f.TargetType != clockTargetGroup && *f.TargetType != clockTargetFriend {
			return nil, fmt.Errorf("触发对象类型必须为 group 或 friend")
		}
		nt.TargetType = *f.TargetType
	}
	if f.TargetID != nil {
		nt.TargetID = *f.TargetID
	}
	if f.Enabled != nil {
		nt.Enabled = *f.Enabled
	}
	if f.RunOnce != nil {
		nt.RunOnce = *f.RunOnce
	}
	if f.TimeoutSec != nil {
		nt.TimeoutSec = *f.TimeoutSec
	}
	if f.Note != nil {
		nt.Note = *f.Note
	}
	nt.UpdatedAt = time.Now()
	m.tasks[id] = &nt
	m.persistTaskLocked(&nt)
	if m.cronStarted {
		m.unscheduleLocked(id)
		m.scheduleLocked(&nt)
	}
	return &nt, nil
}

// Delete 删除任务，返回是否曾存在。
func (m *clockManager) Delete(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[id]; !ok {
		return false
	}
	delete(m.tasks, id)
	m.store.Del(context.Background(), clockTaskPrefix+id)
	m.persistIndexLocked()
	if m.cronStarted {
		m.unscheduleLocked(id)
	}
	return true
}

// Get 返回任务的副本。
func (m *clockManager) Get(id string) (*ClockTask, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tasks[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// List 返回所有任务的副本，按创建时间升序。
func (m *clockManager) List() []*ClockTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*ClockTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		cp := *t
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

// ListByTarget 返回指定触发对象的任务列表。
func (m *clockManager) ListByTarget(targetType string, targetID string) []*ClockTask {
	all := m.List()
	out := make([]*ClockTask, 0)
	for _, t := range all {
		if t.TargetType == targetType && t.TargetID == targetID {
			out = append(out, t)
		}
	}
	return out
}

// ---- 调度 ----

func (m *clockManager) scheduleLocked(t *ClockTask) {
	if !t.Enabled {
		return
	}
	taskID := t.ID
	entryID, err := m.cron.AddFunc(t.Cron, func() {
		// 重新读取最新状态，任务可能已被更新 / 禁用 / 删除
		m.mu.Lock()
		cur := m.tasks[taskID]
		m.mu.Unlock()
		if cur == nil || !cur.Enabled {
			return
		}
		m.runTask(cur)
		// 单次任务：触发执行完成后自动销毁（无论成功/超时/失败均已走完 runTask）
		if cur.RunOnce {
			m.destroyOneShot(taskID)
		}
	})
	if err != nil {
		m.logger.Error("注册定时任务失败", "task", t.ID, "cron", t.Cron, "error", err)
		return
	}
	m.entries[t.ID] = entryID
	if entry := m.cron.Entry(entryID); entry.Valid() {
		t.NextRunAt = entry.Next
		m.persistTaskLocked(t)
	}
}

func (m *clockManager) unscheduleLocked(id string) {
	if entryID, ok := m.entries[id]; ok {
		m.cron.Remove(entryID)
		delete(m.entries, id)
	}
}

// destroyOneShot 销毁已触发的单次任务：从内存与持久化中移除并取消调度。
// 在 cron 回调中于 runTask 之后调用——runTask 已把执行投递到独立 goroutine，
// 此处仅清理调度状态，不影响进行中的执行（任务结构体不会被原地修改）。
func (m *clockManager) destroyOneShot(id string) {
	if m.Delete(id) {
		m.logger.Info("单次定时任务执行后已自动销毁", "task", id)
	}
}

// RunNow 立即触发一次任务（手动执行，不影响 cron 调度）。任务不存在时返回 false。
func (m *clockManager) RunNow(id string) bool {
	t, ok := m.Get(id)
	if !ok {
		return false
	}
	m.runTask(t)
	return true
}

// ---- 触发执行 ----

// taskRecorder 一次定时任务执行的工具调用收集器。工具观测回调与 Chat 同 goroutine
// 串行执行，因此无需加锁。参考 querylog.queryRecorder。
type taskRecorder struct {
	toolCalls      []tasklog.ToolCallRecord
	toolCallsTotal int // 工具调用总数（含超出上限被丢弃的）
}

// observe 工具调用观察回调：追加一条执行记录，明细最多保留 MaxToolCallRecords 条。
func (r *taskRecorder) observe(info aichat.ToolCallInfo) {
	r.toolCallsTotal++
	if len(r.toolCalls) >= tasklog.MaxToolCallRecords {
		return
	}
	rec := tasklog.ToolCallRecord{
		Name:       info.Name,
		Arguments:  tasklog.Truncate(info.Arguments, tasklog.MaxArgsRunes),
		Result:     tasklog.Truncate(info.Result, tasklog.MaxResultRunes),
		DurationMs: info.DurationMs,
	}
	if info.Err != nil {
		rec.Error = info.Err.Error()
	}
	r.toolCalls = append(r.toolCalls, rec)
}

// runTask 在独立的可恢复 goroutine 中执行一次任务。
func (m *clockManager) runTask(task *ClockTask) {
	if m.bot == nil {
		m.logger.Warn("定时任务触发但 bot 尚未就绪，跳过", "task", task.ID, "title", task.Title)
		return
	}
	m.bot.Go("clock:"+task.ID, func() {
		start := time.Now()
		logEntry := m.log.Record(tasklog.Entry{
			TaskID:         task.ID,
			TaskTitle:      task.Title,
			TargetType:     task.TargetType,
			TargetID:       task.TargetID,
			TriggerTime:    start,
			TriggerContent: tasklog.Truncate(m.buildTriggerPrompt(task), tasklog.MaxContentRunes),
			Status:         tasklog.StatusRunning,
		})

		// 记录 LastRunAt
		m.mu.Lock()
		if cur, ok := m.tasks[task.ID]; ok {
			cur.LastRunAt = start
			m.persistTaskLocked(cur)
		}
		m.mu.Unlock()

		finished := false
		defer func() {
			if !finished {
				// panic 或未正常返回：兜底标记为 error
				m.log.Update(logEntry.ID, func(e *tasklog.Entry) {
					e.Status = tasklog.StatusError
					e.DurationMs = time.Since(start).Milliseconds()
					e.Error = "未正常完成（可能发生 panic）"
					e.FinishedAt = time.Now()
				})
				m.logger.Error("定时任务未正常完成", "task", task.ID, "title", task.Title)
			}
		}()

		timeout := m.defaultTimeout
		if task.TimeoutSec > 0 {
			timeout = time.Duration(task.TimeoutSec) * time.Second
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		rec := &taskRecorder{}
		resp, usage, runErr := m.executeTask(ctx, task, rec)
		duration := time.Since(start)

		// fillExecution 回填执行过程明细（LLM 轮数 / 工具调用 / 最终回复 / token 用量）
		fillExecution := func(e *tasklog.Entry) {
			e.DurationMs = duration.Milliseconds()
			e.Iterations = usage.Iterations
			e.ToolCalls = rec.toolCalls
			e.ToolCallsTotal = rec.toolCallsTotal
			e.Reply = tasklog.Truncate(resp, tasklog.MaxReplyRunes)
			e.PromptTokens = usage.PromptTokens
			e.CompletionTokens = usage.CompletionTokens
			e.TotalTokens = usage.TotalTokens
			e.CachedTokens = usage.CachedTokens
			e.FinishedAt = time.Now()
		}

		if runErr != nil {
			status := tasklog.StatusError
			errMsg := runErr.Error()
			if errors.Is(runErr, context.DeadlineExceeded) {
				status = tasklog.StatusTimeout
			}
			m.log.Update(logEntry.ID, func(e *tasklog.Entry) {
				e.Status = status
				e.Error = errMsg
				fillExecution(e)
			})
			finished = true
			m.logger.Warn("定时任务执行结束（非成功）", "task", task.ID, "title", task.Title,
				"status", status, "error", errMsg, "duration", duration)
			if status == tasklog.StatusTimeout {
				m.sendText(task, "⏰ 定时任务「"+task.Title+"」执行超时（"+timeout.String()+"），已中止")
			}
			return
		}

		m.log.Update(logEntry.ID, func(e *tasklog.Entry) {
			e.Status = tasklog.StatusSuccess
			fillExecution(e)
		})
		finished = true
		m.logger.Info("定时任务执行成功", "task", task.ID, "title", task.Title,
			"duration", duration, "tokens", usage.TotalTokens)
		_ = resp // 最终回复已在 executeTask 内发送
	})
}

// executeTask 构建一次性上下文执行任务并返回最终回复与 token 用量。
// rec 非空时挂载工具调用观察者，执行过程中的工具明细追加到 rec。
func (m *clockManager) executeTask(ctx context.Context, task *ClockTask, rec *taskRecorder) (string, aichat.TokenUsage, error) {
	p := m.plugin
	isGroup := task.TargetType == clockTargetGroup
	targetQID, _ := strconv.ParseUint(task.TargetID, 10, 64)
	// 注入对话场景（群聊/私聊），触发时 AI 同样清楚自己面对的场景
	prompt := p.getPromptForID(message.FromUint64(targetQID), isGroup) + p.buildScenePrompt(m.bot, message.FromUint64(targetQID), isGroup)

	// 每次触发独立的 SessionToolExecutor（动态 MCP 工具互不影响）；
	// historyStore 传 nil → 全新一次性上下文，不持久化、执行后丢弃
	sessionExecutor := p.toolExecutor.NewSessionExecutor()
	// 注册定时任务专用子代理工具（受 subagent.enable 门控）：子代理在后台异步
	// 执行，任务收尾时统一等待全部完成并把结果回喂给任务 AI——只有汇总后的
	// 最终回复才会推送给目标
	var subagents *clockSubagentSet
	if p.cfg.Subagent.Enable {
		subagents = newClockSubagentSet()
		for _, tool := range newClockSubagentTools(p, m.bot, task, subagents) {
			sessionExecutor.RegisterSession(tool)
		}
		// 兜底：任意路径返回前取消仍运行中的子代理（正常路径 drainClockSubagents 已处理）
		defer subagents.cancelPending()
	}
	chat, err := aichat.NewChatBot(
		p.cfg.BaseURL, p.cfg.APIKey, p.cfg.Model,
		prompt, p.cfg.MaxContextTokens, sessionExecutor, nil,
	)
	if err != nil {
		return "", aichat.TokenUsage{}, fmt.Errorf("创建对话失败: %w", err)
	}
	chat.SetMaxIterations(p.mainMaxIterations())
	if p.skillManager != nil {
		chat.SetSkillManager(p.skillManager)
	}
	if rec != nil {
		chat.SetToolObserver(rec.observe)
	}

	cbs := m.makeClockCallback(ctx, task)
	resp, usage, err := chat.Chat(ctx, m.buildTriggerPrompt(task), cbs, p.buildChatOptions())
	if err != nil {
		return "", usage, err
	}
	// 若任务 AI 委派了异步子代理：等待全部子代理返回，把结果回喂给 AI 合成
	// 最终回复——只有这最后一轮输出才会推送，子代理返回前的中间回复不推送
	if subagents != nil && subagents.hasPending() {
		resp, usage = m.drainClockSubagents(ctx, task, chat, cbs, subagents, resp, usage)
	}
	// 只发送最终文本回复到触发对象；多轮工具过程中的中间轮文本由
	// makeClockCallback 的 SendText 丢弃（仅记日志），避免触发对象收到
	// 中途碎片的非预期消息。工具主动调用的图片/文件仍正常发送。
	if strings.TrimSpace(resp) != "" {
		m.sendText(task, resp)
	}
	return resp, usage, nil
}

// buildTriggerPrompt 构造触发时发送给 AI 的内容：【定时任务】标题\n 内容。
func (m *clockManager) buildTriggerPrompt(task *ClockTask) string {
	var sb strings.Builder
	sb.WriteString("【定时任务】")
	if strings.TrimSpace(task.Title) != "" {
		sb.WriteString(task.Title)
	}
	sb.WriteString("\n")
	sb.WriteString(task.Content)
	if strings.TrimSpace(task.Note) != "" {
		sb.WriteString("\n（备注：")
		sb.WriteString(task.Note)
		sb.WriteString("）")
	}
	return sb.String()
}

// makeClockCallback 构造触发时面向目标对象（群 / 好友）的工具回调。
func (m *clockManager) makeClockCallback(ctx context.Context, task *ClockTask) llmtool.CallBackFuncs {
	b := m.bot
	targetIDStr := task.TargetID
	targetID, _ := strconv.ParseUint(targetIDStr, 10, 64)
	isGroup := task.TargetType == clockTargetGroup
	logger := m.logger
	var loadedImages []string

	qid := message.FromUint64(targetID)
	cbs := llmtool.CallBackFuncs{
		// SendText 是多轮工具循环中模型中间轮文本的自动回执通道。
		// 定时任务不自动回显——数字人要主动发消息必须显式调用 send_message 工具；
		// 这里仅记录日志，丢弃中间轮文本，避免触发对象收到非预期的多条消息。
		SendText: func(s string) (string, error) {
			logger.Info("定时任务丢弃中间轮文本（未主动发送）", "task", task.ID, "text", s)
			return "已记录，未发送", nil
		},
		SendImage: func(bs64 string) (string, error) {
			ok := m.sendImage(task, bs64)
			if !ok {
				return "", fmt.Errorf("发送失败")
			}
			logger.Info("定时任务发送图片", "task", task.ID, "target", targetIDStr)
			return "发送成功", nil
		},
		SendFile: func(name, bs64 string) (string, error) {
			ok := m.sendFile(task, name, bs64)
			if !ok {
				return "", fmt.Errorf("发送失败")
			}
			logger.Info("定时任务发送文件", "task", task.ID, "target", targetIDStr, "file", name)
			return "发送成功", nil
		},
		GetMsgHistory: func(count, messageSeq int) (string, error) {
			if isGroup {
				msgs, ok := b.GetGroupMsgHistory(qid, count, messageSeq)
				if !ok || msgs == nil {
					return "", fmt.Errorf("获取历史消息失败")
				}
				return formatHistoryText(msgs, b), nil
			}
			msgs, ok := b.GetFriendMsgHistory(qid, count, messageSeq)
			if !ok || msgs == nil {
				return "", fmt.Errorf("获取历史消息失败")
			}
			return formatHistoryText(msgs, b), nil
		},
		GetPrivateFileURL: func(fileId string) (string, error) {
			if isGroup {
				return "", fmt.Errorf("当前为群聊定时任务，无法获取私聊文件")
			}
			url, ok := b.GetPrivateFileURL(qid, fileId)
			if !ok {
				return "", fmt.Errorf("获取私聊文件URL失败")
			}
			return url, nil
		},
		LoadImages: func() (string, error) {
			return "当前为定时任务触发，无消息图片可加载", nil
		},
		TakeLoadedImages: func() []string {
			imgs := loadedImages
			loadedImages = nil
			return imgs
		},
	}

	// 复用插件本地图片读取逻辑（主模型多模态或配置了备用识别模型时可用）
	if m.plugin.cfg.Multimodal || m.plugin.ocrModel != nil {
		p := m.plugin
		cbs.LoadLocalImage = func(path string) (string, error) {
			return p.loadLocalImageInto(ctx, path, &loadedImages), nil
		}
	}
	return cbs
}

// formatHistoryText 将历史消息格式化为纯文本（与 msgcallback.go 的格式一致）。
func formatHistoryText(msgs *[]message.Message, b bot.Bot) string {
	var sb strings.Builder
	for _, msg := range *msgs {
		sb.WriteString(fmt.Sprintf("[message_seq:%d]\n", msg.MessageSeq))
		sb.WriteString(msg.FriendlyText(true,
			message.WithGetMsgFunc(b.GetMsgDetail),
			message.WithGetForwardMsgFunc(b.GetForwardMsg),
		))
		sb.WriteString("\n")
	}
	return sb.String()
}

// ---- 目标消息发送 ----

func (m *clockManager) sendText(task *ClockTask, text string) bool {
	if m.bot == nil {
		return false
	}
	targetID, _ := strconv.ParseUint(task.TargetID, 10, 64)
	if task.TargetType == clockTargetGroup {
		builder := msgchain.Builder().Group()
		if task.CreatedBy != "" {
			builder.Mention(task.CreatedBy)
			builder.Text(" " + text)
		} else {
			builder.Text(text)
		}
		_, ok := m.bot.SendGroupMsg(message.FromUint64(targetID), builder.Build())
		return ok
	}
	builder := msgchain.Builder().Friend()
	builder.Text(text)
	_, ok := m.bot.SendFriendMsg(message.FromUint64(targetID), builder.Build())
	return ok
}

func (m *clockManager) parseTargetID(task *ClockTask) message.QID {
	n, _ := strconv.ParseUint(task.TargetID, 10, 64)
	return message.FromUint64(n)
}

func (m *clockManager) sendImage(task *ClockTask, bs64 string) bool {
	if m.bot == nil {
		return false
	}
	qid := m.parseTargetID(task)
	if task.TargetType == clockTargetGroup {
		builder := msgchain.Builder().Group()
		builder.ImageBase64(bs64)
		_, ok := m.bot.SendGroupMsg(qid, builder.Build())
		return ok
	}
	builder := msgchain.Builder().Friend()
	builder.ImageBase64(bs64)
	_, ok := m.bot.SendFriendMsg(qid, builder.Build())
	return ok
}

func (m *clockManager) sendFile(task *ClockTask, name, bs64 string) bool {
	if m.bot == nil {
		return false
	}
	qid := m.parseTargetID(task)
	if task.TargetType == clockTargetGroup {
		builder := msgchain.Builder().Group()
		builder.FileBase64(name, bs64)
		_, ok := m.bot.SendGroupMsg(qid, builder.Build())
		return ok
	}
	builder := msgchain.Builder().Friend()
	builder.FileBase64(name, bs64)
	_, ok := m.bot.SendFriendMsg(qid, builder.Build())
	return ok
}
