package agenthook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// reloadTTL 配置热加载节流：距上次读取超过该时长才重新从配置中心拉取，
// raw 文本未变化时跳过正则编译——面板编辑秒级生效，且 PreToolUse 高频路径上
// 的存储读取有界。
const reloadTTL = 5 * time.Second

// Manager 钩子管理器：聚合 shell 钩子（files.hooks_json 配置）与其他插件注册的
// Go 钩子，向 AI 引擎提供统一的 Run 入口。并发安全（快照读 + 无状态执行）。
// 未启用（enabled=false）时 Run 直接短路返回零值。
type Manager struct {
	logger    *slog.Logger
	editor    ConfigStore // nil = 仅 Go 钩子（无配置中心）
	configKey string
	runner    *shellRunner

	enabled atomic.Bool

	// snapshot 编译后的钩子快照（构建后不可变，整体替换）
	snapshot atomic.Value // map[Event][]compiledHook
	// goHandlers 其他插件注册的 Go 钩子（启动时由 core 收集注入）
	goHandlers atomic.Value // []Handler

	reloadMu  sync.Mutex // 热加载串行化
	lastRaw   string
	lastCheck time.Time
}

func NewManager(editor ConfigStore, configKey string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	m := &Manager{
		editor:    editor,
		configKey: configKey,
		logger:    logger,
		runner:    newShellRunner(),
	}
	m.snapshot.Store(map[Event][]compiledHook{})
	m.goHandlers.Store([]Handler(nil))
	return m
}

// SetEnabled 设置功能开关（来自 plugin.ai_chat_bot.hooks.enable，重启生效）
func (m *Manager) SetEnabled(on bool) { m.enabled.Store(on) }

// Enabled 报告功能开关状态
func (m *Manager) Enabled() bool { return m.enabled.Load() }

// SetGoHandlers 注入其他插件注册的 Go 钩子（实现 HandlerRegistry 的存储侧）
func (m *Manager) SetGoHandlers(h []Handler) { m.goHandlers.Store(h) }

// Reload 立即从配置中心重新读取并编译钩子配置（插件 Start 时调用一次；
// 之后 Run 内部按 TTL 惰性热加载）。解析失败保留旧快照。
func (m *Manager) Reload() error {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	return m.reloadLocked()
}

// maybeReload 按 TTL 节流地惰性热加载；失败沿用旧配置。
func (m *Manager) maybeReload() {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	if time.Since(m.lastCheck) < reloadTTL {
		return
	}
	if err := m.reloadLocked(); err != nil {
		m.logger.Warn("热加载钩子配置失败，沿用旧配置", "error", err.Error())
	}
}

func (m *Manager) reloadLocked() error {
	m.lastCheck = time.Now()
	if m.editor == nil {
		return nil
	}
	raw := ""
	if v, ok := m.editor.Get(m.configKey); ok {
		raw, _ = v.(string)
	}
	if raw == m.lastRaw {
		return nil
	}
	cfg := &FileConfig{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), cfg); err != nil {
			// 记住 raw 避免每 5s 重复报错；管理员修正后内容变化才会重新加载
			m.lastRaw = raw
			return fmt.Errorf("解析钩子配置 %s 失败: %w", m.configKey, err)
		}
	}
	compiled, err := compileHooks(cfg)
	if err != nil {
		m.lastRaw = raw
		return err
	}
	m.snapshot.Store(compiled)
	m.lastRaw = raw
	count := 0
	for _, hooks := range compiled {
		count += len(hooks)
	}
	m.logger.Info("钩子配置已加载", "events", len(compiled), "hooks", count)
	return nil
}

// Run 执行某事件的全部钩子（实现 aichat.HookRunner）。
// 顺序：Go 钩子（进程内、廉价）优先，shell 钩子按配置顺序逐个执行。
// 任一钩子返回 Block 即短路返回；多个 Context 按来源拼接（\n 分隔）；
// 非阻断错误只记日志，绝不影响主流程。
func (m *Manager) Run(ctx context.Context, ev Event, p Payload) Result {
	if !m.enabled.Load() {
		return Result{}
	}
	m.maybeReload()
	p.Event = ev

	var contexts []string
	join := func() string { return strings.Join(contexts, "\n") }

	if handlers, ok := m.goHandlers.Load().([]Handler); ok {
		for _, h := range handlers {
			res := safeGoHook(h, ctx, ev, p)
			if res.Context != "" {
				contexts = append(contexts, res.Context)
			}
			if res.Err != nil {
				m.logger.Warn("Go 钩子执行出错", "event", ev, "error", res.Err.Error())
			}
			if res.Block {
				return Result{Block: true, Reason: res.Reason, Context: join()}
			}
		}
	}

	hooks, _ := m.snapshot.Load().(map[Event][]compiledHook)
	for _, hook := range hooks[ev] {
		if !hook.matches(p.ToolName) {
			continue
		}
		res := m.runner.run(ctx, hook, p)
		if res.Context != "" {
			contexts = append(contexts, res.Context)
		}
		if res.Err != nil {
			m.logger.Warn("shell 钩子执行出错", "event", ev, "command", hook.spec.Command, "error", res.Err.Error())
		}
		if res.Block {
			m.logger.Info("钩子阻断", "event", ev, "tool", p.ToolName, "command", hook.spec.Command, "reason", res.Reason)
			return Result{Block: true, Reason: res.Reason, Context: join()}
		}
	}

	return Result{Context: join()}
}

// safeGoHook 单个 Go 钩子的 panic 隔离：插件钩子的 panic 不传染主流程
func safeGoHook(h Handler, ctx context.Context, ev Event, p Payload) (res Result) {
	defer func() {
		if r := recover(); r != nil {
			res = Result{Err: fmt.Errorf("Go 钩子 panic: %v", r)}
		}
	}()
	return h.OnAgentHook(ctx, ev, p)
}
