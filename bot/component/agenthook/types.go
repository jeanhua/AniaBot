// Package agenthook 提供 AI 代理生命周期钩子：在会话事件（工具调用前后、用户输入、
// 会话开始/结束、上下文压缩等）上执行管理员配置的 shell 命令（Claude Code 语义：
// stdin 传 JSON 载荷，退出码 0 通过 / 2 阻断 / 其他为非阻断错误），同时支持其他
// 插件以 Go 接口形式注册进程内钩子。
//
// 仅 PreToolUse / UserPromptSubmit 的阻断（Result.Block）会被引擎采纳，
// 其余事件一律按通知处理（Block 被忽略）。
package agenthook

import "context"

// Event 钩子事件（与 Claude Code 语义对齐）
type Event string

const (
	// EventSessionStart 会话（重）创建时触发：首次发言或会话被淘汰后重建。
	// Result.Context 非空时会在下一轮注入到用户消息前（只消费一次）。
	EventSessionStart Event = "SessionStart"
	// EventUserPromptSubmit 每轮用户输入进入对话前触发；
	// 可阻断（Block+Reason 告知用户）或注入上下文（Context 拼到用户消息前）。
	EventUserPromptSubmit Event = "UserPromptSubmit"
	// EventPreToolUse 每次工具调用前触发（Payload.ToolName/ToolInput 有效）；
	// 唯一可阻断工具执行的事件：Block 时工具不执行，Reason 作为工具结果回填给 AI。
	EventPreToolUse Event = "PreToolUse"
	// EventPostToolUse 每次工具执行完成后触发（ToolResult 为截断后的结果），仅通知。
	EventPostToolUse Event = "PostToolUse"
	// EventStop 一次完整响应结束时触发（Prompt 为截断后的最终回复），仅通知。
	EventStop Event = "Stop"
	// EventSubagentStop 子代理执行结束时触发（Prompt 为任务描述），仅通知。
	EventSubagentStop Event = "SubagentStop"
	// EventPreCompact 上下文压缩即将发生时触发，仅通知（压缩是对话存续的必要环节，不允许阻断）。
	EventPreCompact Event = "PreCompact"
)

// 代理身份标识（Payload.AgentKind）
const (
	AgentKindMain     = "main"     // 主会话
	AgentKindSubagent = "subagent" // 子代理（含团队成员）
	AgentKindClock    = "clock"    // 定时任务触发的一次性会话
)

// valid 事件名是否合法（配置校验用）
func (e Event) valid() bool {
	switch e {
	case EventSessionStart, EventUserPromptSubmit, EventPreToolUse,
		EventPostToolUse, EventStop, EventSubagentStop, EventPreCompact:
		return true
	}
	return false
}

// Payload 钩子载荷：同时是 shell 钩子的 stdin JSON（snake_case 字段名向 Claude Code 看齐）
type Payload struct {
	Event      Event  `json:"hook_event_name"`
	SessionKey string `json:"session_id"`           // g:<id> / f:<id>，clock 为其目标会话
	AgentKind  string `json:"agent_kind"`           // main | subagent | clock
	ToolName   string `json:"tool_name,omitempty"`  // 工具事件：工具名
	ToolInput  string `json:"tool_input,omitempty"` // 工具事件：原始 JSON 参数
	// ToolResult PostToolUse：截断后的工具结果（≤1000 runes）
	ToolResult string `json:"tool_result,omitempty"`
	// Prompt UserPromptSubmit 为用户输入；Stop 为截断后的最终回复；SubagentStop 为任务描述
	Prompt string `json:"prompt,omitempty"`
}

// Result 钩子聚合结果。Block 仅对 PreToolUse / UserPromptSubmit 生效；
// Context 由调用点尾部注入（拼到用户消息文本前，不改 system prompt）。
type Result struct {
	Block   bool   // true 时 Reason 作为阻断原因（工具结果 / 提示用户）
	Reason  string // 阻断原因
	Context string // 附加上下文
	Err     error  // 非阻断性错误（仅记日志）
}

// ShellHookSpec 单个 shell 钩子配置
type ShellHookSpec struct {
	// Matcher 工具名正则（仅工具事件有意义；空 = 全部）。
	// 非工具事件的 ToolName 为空串，带 matcher 的钩子自然不匹配。
	Matcher string `json:"matcher"`
	// Command shell 命令，stdin 接收 Payload JSON
	Command string `json:"command"`
	// TimeoutSec 单钩子超时（秒）；默认 10，上限 60
	TimeoutSec int `json:"timeout_sec,omitempty"`
}

// FileConfig 是 files.hooks_json 的顶层结构
type FileConfig struct {
	Hooks map[Event][]ShellHookSpec `json:"hooks"`
}

// Handler Go 钩子接口：插件开发者实现它接收全部钩子事件。
// 由 core 启动时按「可选接口 + 类型断言」收集（同管理面板 source 收集模式），
// 注入给实现 HandlerRegistry 的插件。
type Handler interface {
	OnAgentHook(ctx context.Context, ev Event, p Payload) Result
}

// HandlerRegistry 由需要接收 Go 钩子的插件实现（AI 对话插件）。
type HandlerRegistry interface {
	SetGoHookHandlers(handlers []Handler)
}

// ConfigStore 配置中心读能力（结构化复制 plugin.ConfigEditor 的 Get，
// 与 functool.ConfigStore 同例，避免 component → common/plugin 依赖）。
type ConfigStore interface {
	Get(key string) (any, bool)
}
