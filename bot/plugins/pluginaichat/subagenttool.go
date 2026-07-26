package pluginaichat

import (
	"context"
	"fmt"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

// subagentToolBase 为子代理工具共享插件引用与当前会话信息（创建时绑定）。
type subagentToolBase struct {
	plugin  *AIChatPlugin
	bot     bot.Bot
	id      message.QID
	isGroup bool
}

// newSubagentTools 创建子代理委派工具，注册到当前会话的执行器中。
// 工具描述写入当前会话场景与默认超时，让 AI 明确委派上下文与等待时长。
func newSubagentTools(p *AIChatPlugin, b bot.Bot, id message.QID, isGroup bool) []llmtool.Tool {
	base := subagentToolBase{plugin: p, bot: b, id: id, isGroup: isGroup}
	sessionDesc := "私聊（对方QQ " + id.String() + "）"
	if isGroup {
		sessionDesc = "群聊（群号 " + id.String() + "）"
	}
	desc := "将一个复杂/耗时的子任务委派给一次性子代理执行并等待其完成。子代理以全新上下文运行（看不到当前对话历史），" +
		"拥有与你一致的工具能力（以其实际可用的工具列表为准），执行完毕后仅把最终结果返回给你。" +
		"适用场景：需要多步工具调用的独立子任务（如深度调研、多轮搜索后总结），可避免中间过程占用当前对话上下文。" +
		"当前会话为" + sessionDesc + "，子代理在同一会话场景下工作。" +
		fmt.Sprintf("执行期间你需要同步等待（默认超时 %d 秒，超时后子代理中止、你仍可继续回复），耗时任务建议先在回复中告知用户。", int(p.subagentTimeout().Seconds())) +
		"子代理无法再委派子代理"
	return []llmtool.Tool{
		&subagentRunTool{
			BaseTool:         llmtool.MakeBaseTool("subagent_run", desc, subagentRunParams{}),
			subagentToolBase: base,
		},
	}
}

// ---- subagent_run ----

type subagentRunParams struct {
	Task       string `json:"task" desc:"完整自洽的任务指令：子代理看不到当前对话，必须把背景、目标、期望的输出格式都写清楚"`
	TimeoutSec int    `json:"timeout_sec,omitempty" desc:"本次执行的超时秒数（上限 1800），不填用默认值；实际还会按当前请求的剩余时间预算自动收缩"`
}

type subagentRunTool struct {
	llmtool.BaseTool[subagentRunParams]
	subagentToolBase
}

func (t *subagentRunTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*subagentRunParams)
	task := strings.TrimSpace(p.Task)
	if task == "" {
		return "", fmt.Errorf("task 不能为空")
	}
	return t.plugin.runSubagent(ctx, t.bot, t.id, t.isGroup, task, p.TimeoutSec, callbacks)
}
