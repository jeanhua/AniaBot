package pluginaichat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/jeanhua/AniaBot/bot/component/aichat"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/message"
)

const (
	// subagentMaxTimeoutSec 子代理单次执行的超时上限（秒），防止 AI 传入离谱的 timeout_sec
	subagentMaxTimeoutSec = 1800
	// subagentParentReserve 为主请求预留的收尾时间。框架对单次消息处理有总预算
	// （core.MsgEventTimeout=5min），子代理超时必须早于该预算触发，超时才能作为
	// 工具结果优雅返回、让主 AI 用剩余时间完成最终回复；否则父 deadline 先触发，
	// 整个主请求会以「请求超时」中止
	subagentParentReserve = 30 * time.Second
)

// defaultSubagentPrompt 子代理的系统提示词：子代理是主 AI 委派任务的一次性工作者，
// 无历史对话上下文，最终文本结果返回给主 AI 而非直接发给用户。
// 可用能力不在此枚举——具体工具以会话实际注册的工具列表为准（部分功能受配置门控）。
const defaultSubagentPrompt = `你是一个子代理，由主 AI 委派执行一个具体任务。

## 工作方式
- 你没有历史对话上下文，任务描述中包含了完成任务所需的全部信息
- 你可以使用提供的工具完成任务，具体以实际可用的工具列表为准
- 你的最终文本回复会作为执行结果返回给主 AI，请输出完整、可直接使用的结果

## 注意
- 只输出与任务相关的内容，不要寒暄
- 你执行过程中的中间消息不会发送给任何人`

// subagentTimeout 子代理默认超时：配置缺失/非法时兜底 300 秒；
// 先做 int 限幅再乘 time.Second，防止超大配置值 int64 溢出为负 duration
func (p *AIChatPlugin) subagentTimeout() time.Duration {
	sec := p.cfg.Subagent.TimeoutSec
	if sec <= 0 {
		sec = 300
	}
	if sec > subagentMaxTimeoutSec {
		sec = subagentMaxTimeoutSec
	}
	return time.Duration(sec) * time.Second
}

func (p *AIChatPlugin) subagentMaxIterations() int {
	if p.cfg.Subagent.MaxIterations <= 0 {
		return 10
	}
	return p.cfg.Subagent.MaxIterations
}

func (p *AIChatPlugin) subagentMaxResultLen() int {
	if p.cfg.Subagent.MaxResultLen <= 0 {
		return 4000
	}
	return p.cfg.Subagent.MaxResultLen
}

// resolveSubagentTimeout 计算子代理单次执行的实际超时：
//  1. timeoutSec>0 时覆盖默认值（先限幅 int 再乘 time.Second，防溢出绕过上限）；
//  2. 父上下文带 deadline 时（框架消息处理总预算），再为主请求预留
//     subagentParentReserve 收尾时间压缩超时——保证子代理超时先触发，以工具结果
//     优雅返回；剩余预算不足时直接报错，不启动子代理。
func resolveSubagentTimeout(defaultTimeout time.Duration, timeoutSec int, parentCtx context.Context) (time.Duration, error) {
	timeout := defaultTimeout
	if timeoutSec > 0 {
		if timeoutSec > subagentMaxTimeoutSec {
			timeoutSec = subagentMaxTimeoutSec
		}
		timeout = time.Duration(timeoutSec) * time.Second
	}
	if deadline, ok := parentCtx.Deadline(); ok {
		budget := time.Until(deadline) - subagentParentReserve
		if budget <= 0 {
			return 0, fmt.Errorf("当前请求剩余时间不足，无法启动子代理")
		}
		if timeout > budget {
			timeout = budget
		}
	}
	return timeout, nil
}

// runSubagent 同步执行一次子代理任务（在主 AI 的工具调用内等待完成）。
//
// 参照 clock 的 executeTask 模式构建全新的一次性 ChatBot：nil historyStore（不持久化、
// 执行后丢弃）、独立的 SessionToolExecutor（动态 MCP 工具互不影响）。与主会话的区别：
// 不注册 subagent 工具自身（防止递归委派），中间轮文本丢弃不发给用户，图片加载状态
// 独立（不与主会话互踩，见 makeSubagentCallbacks）。
//
// ctx 为主会话的请求上下文：/stop 取消主请求时子代理一并取消；timeoutSec<=0 用默认超时。
func (p *AIChatPlugin) runSubagent(ctx context.Context, b bot.Bot, id message.QID, isGroup bool, task string, timeoutSec int, parentCbs llmtool.CallBackFuncs) (string, error) {
	timeout, err := resolveSubagentTimeout(p.subagentTimeout(), timeoutSec, ctx)
	if err != nil {
		return "", err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 一次性会话：注册与主会话一致的会话级工具（clock/memory），但不注册 subagent 自身
	sessionExecutor := p.toolExecutor.NewSessionExecutor()
	p.registerScopedTools(sessionExecutor, id, isGroup)

	prompt := defaultSubagentPrompt + p.buildScenePrompt(b, id, isGroup)
	chat, err := aichat.NewChatBot(
		p.cfg.BaseURL, p.cfg.APIKey, p.cfg.Model,
		prompt, p.cfg.MaxContextTokens, sessionExecutor, nil,
	)
	if err != nil {
		return "", fmt.Errorf("创建子代理失败: %w", err)
	}
	if p.skillManager != nil {
		chat.SetSkillManager(p.skillManager)
	}
	chat.SetMaxIterations(p.subagentMaxIterations())

	logger := p.Logger.WithGroup("subagent")
	cbs := p.makeSubagentCallbacks(runCtx, parentCbs, logger)

	logger.Info("子代理开始执行", "id", id, "is_group", isGroup, "timeout", timeout, "task", task)
	start := time.Now()
	resp, usage, err := chat.Chat(runCtx, "【子代理任务】\n"+task, cbs, p.buildChatOptions())
	duration := time.Since(start)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil:
			logger.Warn("子代理执行超时", "id", id, "is_group", isGroup, "timeout", timeout)
			return "", fmt.Errorf("子代理执行超时（%s），已中止", timeout)
		case ctx.Err() != nil:
			// 主请求被 /stop 取消或父上下文结束，原样返回让上层按取消处理
			return "", ctx.Err()
		default:
			logger.Warn("子代理执行失败", "id", id, "is_group", isGroup, "error", err.Error())
			return "", err
		}
	}

	result, truncated := truncateSubagentResult(resp, p.subagentMaxResultLen())
	logger.Info("子代理执行完成", "id", id, "is_group", isGroup,
		"duration", duration, "iterations", usage.Iterations, "tokens", usage.TotalTokens, "truncated", truncated)

	meta := fmt.Sprintf("【子代理执行完成】耗时 %.1fs · LLM 轮数 %d · token %d",
		duration.Seconds(), usage.Iterations, usage.TotalTokens)
	if truncated {
		meta += " · 结果过长已截断"
	}
	if result == "" {
		return meta + "\n（子代理没有返回内容）", nil
	}
	return meta + "\n" + result, nil
}

// truncateSubagentResult 按字符数（rune）截断子代理结果，防止超长结果污染主对话上下文。
// maxRunes<=0 表示不截断。返回截断后的文本与是否发生了截断。
func truncateSubagentResult(s string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return s, false
	}
	total := utf8.RuneCountInString(s)
	if total <= maxRunes {
		return s, false
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + fmt.Sprintf("\n…（结果过长已截断，原文共 %d 字符）", total), true
}

// makeSubagentCallbacks 构造子代理的工具回调。
//
// 发送类能力（SendImage/SendFile）与只读能力（GetMsgHistory/GetPrivateFileURL）
// 透传主会话回调——子代理与主 AI 处于同一会话场景。SendText 替换为丢弃中间轮文本
// （记日志）：子代理的结论通过最终结果返回，不直接打扰用户。
//
// 图片加载三回调（LoadImages/TakeLoadedImages/LoadLocalImage）不透传：它们闭包捕获的
// 是主请求级的 loadedImages/loaded 状态（见 utils.go configureImageCallbacks），透传
// 会让主/子两个工具循环排干同一图片队列（主 AI 加载的图片被偷进子代理上下文，或
// 子代理毒化 loaded 标志导致主 AI 无法再加载用户图片）。参照 clock 的
// makeClockCallback 模式，为子代理建立独立的图片状态。
func (p *AIChatPlugin) makeSubagentCallbacks(ctx context.Context, parent llmtool.CallBackFuncs, logger *slog.Logger) llmtool.CallBackFuncs {
	var loadedImages []string
	cbs := llmtool.CallBackFuncs{
		SendText: func(s string) (string, error) {
			logger.Info("子代理丢弃中间轮文本（未发送）", "text", s)
			return "已记录，未发送", nil
		},
		SendImage:         parent.SendImage,
		SendFile:          parent.SendFile,
		GetMsgHistory:     parent.GetMsgHistory,
		GetPrivateFileURL: parent.GetPrivateFileURL,
		// 用户消息图片的加载状态属于主请求；子代理确需查看时在结果中说明，由主 AI 自行加载
		LoadImages: func() (string, error) {
			return "子代理无法直接加载用户消息中的图片；如确需查看，请在最终结果中说明，由主 AI 自行调用 load_images", nil
		},
		TakeLoadedImages: func() []string {
			imgs := loadedImages
			loadedImages = nil
			return imgs
		},
	}
	// 本地图片读取使用子代理独立的图片队列（主模型多模态或配置了备用识别模型时可用）
	if p.cfg.Multimodal || p.ocrModel != nil {
		cbs.LoadLocalImage = func(path string) (string, error) {
			return p.loadLocalImageInto(ctx, path, &loadedImages), nil
		}
	}
	return cbs
}
