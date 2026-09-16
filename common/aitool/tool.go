// Package aitool 平台专属 AI 工具注入契约与各平台的工具集实现。
//
// 平台适配器可以把平台特有的能力（如 QQ 的 AI 语音、戳一戳、群签到）包装成
// AI Agent 可调用的工具，经 bot 外观的 AITools 方法提供给 AI 对话插件：插件
// 创建会话时对 bot 外观类型断言 Provider，命中则把工具桥接注册进该会话。
// 本包位于 common 层（不依赖 bot/component/llmtool），由 llmtool 侧的
// AdaptProviderTool 完成到 llmtool.Tool 的桥接。
//
// 新增平台工具的方式：在适配器包的 BotWrapper 外观上实现 Provider.AITools，
// 返回按固定顺序构造的工具切片（禁止 map 遍历——工具定义会序列化进每次
// LLM 请求的 tools 字段，顺序抖动会打失上游 prompt 前缀缓存），工具名用
// 平台前缀命名（如 qq_、fs_）避免与其他层工具冲突。
package aitool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// Tool 平台注入 AI Agent 的工具最小契约（与 llmtool.Tool 的方法子集对齐）。
// 适配器侧按此实现即可，无需依赖 llmtool——Go 接口为结构化类型，
// llmtool.AdaptProviderTool 会把它桥接为会话可注册的 llmtool.Tool。
type Tool interface {
	// Name 工具名，全局唯一；必须带平台前缀（如 "qq_send_poke"），
	// 会话层注册时与其他工具同名会被跳过
	Name() string
	// Description 面向 LLM 的工具说明（中文，说明用途、参数含义与限制）
	Description() string
	// Params 参数模板指针（结构体，json + desc 标签），反射引擎据此生成
	// function calling 的 JSON Schema 并解析 LLM 实参；无参数工具返回 &struct{}{}
	Params() any
	// Execute 执行工具；params 为与 Params 同型的指针（已按 LLM 实参填充）。
	// 平台工具不经过 llmtool.CallBackFuncs：执行所需能力在构造时经 Context
	// 绑定（bot 外观 + 会话目标），工具实现直接持有并使用
	Execute(ctx context.Context, params any) (string, error)
}

// MessageSink 历史消息工具的宿主回调：工具拉取到消息后调用（含筛选未命中的
// 消息），宿主借此把消息中的图片登记进请求级图片注册表，load_images 才能
// 按哈希加载历史图片。ctx 为工具执行时的请求上下文（宿主可从中取请求级状态）；
// 注册历史消息工具的宿主未提供回调时该能力退化为纯文本展示。
type MessageSink func(ctx context.Context, msgs []message.Message)

// Context 平台工具的会话绑定信息：AITools 工厂在会话创建时收到，
// 各工具闭包持有，执行时按它定位会话与调用平台能力。
type Context struct {
	// Bot 当前会话的 bot 外观（平台能力包装后，可继续断言 bot.QQ 等可选接口）
	Bot bot.Bot
	// Target 会话对端 ID（群聊为群 ID，私聊为好友 ID），带平台统一前缀；
	// 工具参数省略目标 ID 时用它兜底，其前缀也是推断平台 ID 前缀的依据
	Target message.QID
	// IsGroup 会话是否为群聊
	IsGroup bool
	// OnMessages 历史消息工具的图片登记回调（可选，见 MessageSink），
	// 由宿主插件在构造 Context 时注入；适配器提供方无需关心
	OnMessages MessageSink
}

// Provider 平台 AI 工具注入能力（可选接口，参照 QQExt 的可选能力模式）。
// 平台适配器的 bot 外观包装器（adapter.RegisterBotWrapper 注册）实现它；
// AI 对话插件创建会话时对 bot 外观类型断言探测，命中则注入返回的工具。
type Provider interface {
	// AITools 返回该平台注入会话的工具列表。
	// 实现约定：
	//  1. 切片必须按固定顺序构造（字面量或循环），禁止 map 遍历——
	//     最终序列化顺序由框架按工具名排序兜底，但稳定的构造顺序便于排查；
	//  2. 工具名必须带平台前缀，避免与内置工具/其他平台工具同名；
	//  3. 每次调用返回等价集合（同一平台的会话间工具集一致）。
	AITools(ctx Context) []Tool
}

// SideEffector 副作用声明（可选接口）：会改动平台侧状态的工具（发语音、
// 戳一戳、签到等）实现它并返回 true，计划模式（/plan on）据此阻断；
// 只读工具无需实现。声明随注册收集进插件级的副作用工具名集合，
// 门禁按名字匹配（第三方适配器贡献的工具同样被覆盖）。
type SideEffector interface {
	SideEffect() bool
}

// NormalizeID 规范化 AI 传入的平台 ID 参数：允许 "qq:123"/"lil:123"/纯数字等
// 写法，统一还原为「当前会话平台前缀 + 平台原始 ID」。前缀取自 template
// （会话目标 ID，天然携带正确适配器前缀），避免 OneBot 双协议端（napcat qq: /
// luckylilia lil:）下传错前缀导致适配器剥离失败。输入无法解析为纯数字 ID 时
// 返回错误（QQ 平台 ID 恒为数字；其他平台工具请直接用原文，不经本函数）。
func NormalizeID(input string, template message.QID) (message.QID, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return "", fmt.Errorf("ID 不能为空")
	}
	// 容忍 "qq:12345"/"lil:12345"/"12345" 等写法：取最后一个冒号后的数字部分
	if i := strings.LastIndex(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if _, err := strconv.ParseUint(s, 10, 64); err != nil {
		return "", fmt.Errorf("无法识别的 ID %q（QQ 平台 ID 为纯数字）", input)
	}
	return message.QID(prefixOf(template) + s), nil
}

// prefixOf 推断模板 QID 的平台前缀（如 "qq:"、"lil:"；裸数字返回 ""）。
func prefixOf(q message.QID) string {
	s := string(q)
	if i := strings.Index(s, ":"); i > 0 {
		return s[:i+1]
	}
	return ""
}

// marshalJSON 序列化工具结果；失败时退回 fmt 占位（工具结果不允许空返回）。
func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
