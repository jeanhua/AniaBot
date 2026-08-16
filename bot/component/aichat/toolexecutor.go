package aichat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/agenthook"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type ToolExecutor interface {
	Execute(ctx context.Context, call llmtool.ToolCall, callbacks llmtool.CallBackFuncs) (string, error)
	Tools() []llmtool.ToolDef
}

type ToolOrchestrator struct {
	executor      ToolExecutor
	msgBuilder    *MessageBuilder
	maxIterations int
	toolObserver  func(ToolCallInfo)
	// hookRunner/hookBase 钩子执行器与会话身份（SessionKey/AgentKind），
	// 由 ChatBot.SetHookRunner 注入；nil 时 PostToolUse 埋点跳过
	hookRunner HookRunner
	hookBase   agenthook.Payload
}

// ToolCallInfo 一次工具调用的执行记录，供工具调用观察者（SetToolObserver）使用，
// 例如面板 Query 日志记录 bash 等工具的执行详情。
type ToolCallInfo struct {
	Name       string
	Arguments  string
	Result     string
	DurationMs int64
	Err        error
}

func NewToolOrchestrator(executor ToolExecutor, msgBuilder *MessageBuilder) *ToolOrchestrator {
	// 默认上限仅作兜底：主对话/定时任务由插件配置 plugin.ai_chat_bot.max_iterations
	// 在 ChatBot 创建后通过 SetMaxIterations 覆盖
	return &ToolOrchestrator{
		executor:      executor,
		msgBuilder:    msgBuilder,
		maxIterations: 20,
	}
}

func (o *ToolOrchestrator) SetMaxIterations(max int) {
	o.maxIterations = max
}

// SetToolObserver 设置工具调用观察者：每次工具执行完成后回调一次（传 nil 取消）。
// 同一轮多个工具并行执行时回调在互斥锁内串行调用，观察者无需自行加锁。
// 调用方需保证同一 orchestrator 的 ExecuteWithTools 串行执行。
func (o *ToolOrchestrator) SetToolObserver(fn func(ToolCallInfo)) {
	o.toolObserver = fn
}

// SetHookRunner 注入钩子执行器与会话身份（工具调用完成后触发 PostToolUse 钩子），
// 传 nil 取消。由调用方保证同一 orchestrator 的 ExecuteWithTools 串行执行。
func (o *ToolOrchestrator) SetHookRunner(r HookRunner, base agenthook.Payload) {
	o.hookRunner = r
	o.hookBase = base
}

// observe 串行调用工具观察者（存在时）。同轮并行工具共享 obsMu。
func (o *ToolOrchestrator) observe(mu *sync.Mutex, info ToolCallInfo) {
	if o.toolObserver == nil {
		return
	}
	mu.Lock()
	o.toolObserver(info)
	mu.Unlock()
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	// CachedTokens 命中上游 prompt 缓存的 token 数（DeepSeek prompt_cache_hit_tokens /
	// OpenAI prompt_tokens_details.cached_tokens），是多轮工具调用的累加值。
	// 提供方不返回缓存字段时为 0
	CachedTokens int
	// LastPromptTokens 本次请求最后一次 LLM 调用的 prompt token 数，
	// 即当前上下文的真实大小。PromptTokens 是多轮工具调用的累加值，
	// 会远超单次上下文，仅适合计费统计，不能用于压缩判断
	LastPromptTokens int
	// Iterations 本次请求调用 LLM 的轮数（含无工具直出与末轮总结）
	Iterations int
}

func (o *ToolOrchestrator) ExecuteWithTools(
	ctx context.Context,
	llmClient *LLMClient,
	messages []Message,
	callbacks llmtool.CallBackFuncs,
	opts ChatOptions,
) (string, []Message, TokenUsage, error) {
	var totalUsage TokenUsage

	// 流式模式：opts.OnStreamDelta 非空时所有 LLM 调用走 GenerateStream
	useStream := opts.OnStreamDelta != nil
	gen := func(ctx context.Context, msgs []Message, o ChatOptions) (GenerateResponse, TokenUsage, error) {
		if useStream {
			return llmClient.GenerateStream(ctx, msgs, o)
		}
		return llmClient.Generate(ctx, msgs, o)
	}

	if o.executor == nil || len(o.executor.Tools()) == 0 {
		resp, usage, err := gen(ctx, messages, opts)
		if err != nil {
			return "", messages, totalUsage, err
		}
		totalUsage = usage
		totalUsage.LastPromptTokens = usage.PromptTokens
		totalUsage.Iterations = 1
		content := resp.Content
		messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(content, nil, resp.ReasoningContent))
		return content, messages, totalUsage, nil
	}

	for i := 0; i < o.maxIterations; i++ {
		tools := o.executor.Tools()

		opts.Tools = tools
		resp, usage, err := gen(ctx, messages, opts)
		if err != nil {
			return "", messages, totalUsage, err
		}

		totalUsage.PromptTokens += usage.PromptTokens
		totalUsage.CompletionTokens += usage.CompletionTokens
		totalUsage.TotalTokens += usage.TotalTokens
		totalUsage.CachedTokens += usage.CachedTokens
		totalUsage.LastPromptTokens = usage.PromptTokens
		totalUsage.Iterations++

		if len(resp.ToolCalls) == 0 {
			messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(resp.Content, nil, resp.ReasoningContent))
			return resp.Content, messages, totalUsage, nil
		}

		// 工具边界：流式模式下通知调用方结束当前流式消息（下一轮首个增量创建新消息）
		if opts.OnStreamRoundEnd != nil {
			opts.OnStreamRoundEnd()
		}

		messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(resp.Content, resp.ToolCalls, resp.ReasoningContent))

		// 流式模式下内容已通过 OnStreamDelta 增量发出，不再重复发送
		content := RemoveThinkContent(resp.Content)
		if !useStream && callbacks.SendText != nil && len(content) > 0 {
			callbacks.SendText(content)
		}

		toolResults, err := o.executeToolCalls(ctx, resp.ToolCalls, callbacks, opts.PreToolGate)
		if err != nil {
			return "", messages, totalUsage, err
		}
		messages = append(messages, toolResults...)
		if callbacks.TakeLoadedImages != nil {
			if imageURLs := callbacks.TakeLoadedImages(); len(imageURLs) > 0 {
				messages = append(messages, o.msgBuilder.BuildImageContextMessage(imageURLs))
			}
		}

		if i == o.maxIterations-1 {
			messages = append(messages, o.msgBuilder.BuildToolLimitMessage())
			// 最后一轮要求模型直接给出文本回答，故不再附带工具定义；
			// 否则模型可能继续发起工具调用而被静默丢弃（仅取 Content），导致空响应
			finalOpts := opts
			finalOpts.Tools = nil
			finalResp, finalUsage, err := gen(ctx, messages, finalOpts)
			if err != nil {
				return "", messages, totalUsage, err
			}
			totalUsage.PromptTokens += finalUsage.PromptTokens
			totalUsage.CompletionTokens += finalUsage.CompletionTokens
			totalUsage.TotalTokens += finalUsage.TotalTokens
			totalUsage.CachedTokens += finalUsage.CachedTokens
			totalUsage.LastPromptTokens = finalUsage.PromptTokens
			totalUsage.Iterations++
			finalContent := finalResp.Content
			messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(finalContent, nil, finalResp.ReasoningContent))
			return finalContent, messages, totalUsage, nil
		}
	}

	return "", messages, totalUsage, fmt.Errorf("exceeded maximum iterations")
}

func (o *ToolOrchestrator) executeToolCalls(
	ctx context.Context,
	toolCalls []llmtool.ToolCall,
	callbacks llmtool.CallBackFuncs,
	gate func(context.Context, llmtool.ToolCall) (bool, string),
) ([]Message, error) {
	// 并行执行同一轮的多个工具调用：结果切片预分配、每个工具按 index 回填，
	// 保证 tool 结果消息与 assistant 消息中 tool_calls 数组的顺序一一对应
	// （OpenAI API 要求结果消息按工具调用顺序配对）。
	// 工具执行互不依赖，适合并行；回调与观察者涉及共享状态，单独串行化。
	results := make([]Message, len(toolCalls))
	var obsMu sync.Mutex // 观察者回调串行化（观察者可能对共享 slice 追加）
	lockedCbs := o.lockedCallbacks(callbacks)

	var (
		mu     sync.Mutex
		ctxErr error // 上下文取消时记录，等待全部工具收尾后统一返回
	)
	var wg sync.WaitGroup
	for i, call := range toolCalls {
		wg.Add(1)
		go func(i int, call llmtool.ToolCall) {
			defer wg.Done()
			// 单个工具 panic 不传染整个进程：转为错误文本回填给 LLM
			defer func() {
				if r := recover(); r != nil {
					results[i] = o.msgBuilder.BuildToolMessage(call.ID, call.Name,
						fmt.Sprintf("Error executing tool: %v", r))
				}
			}()

			start := time.Now()
			// 请求级工具门禁（计划模式 / PreToolUse 钩子 / 人工审批）：阻断时工具不执行，
			// 门禁文本作为该工具的结果消息回填（循环继续，语义等同工具报错）；
			// 在 goroutine 内调用而非 spawn 前统一调用——审批等待不阻塞同轮其他工具的启动，
			// 门禁内部的会话级串行化（如审批按会话排队）由实现方负责。门禁必须并发安全。
			if gate != nil {
				if block, text := gate(ctx, call); block {
					o.observe(&obsMu, ToolCallInfo{
						Name: call.Name, Arguments: call.Arguments,
						Result: text, DurationMs: time.Since(start).Milliseconds(),
					})
					results[i] = o.msgBuilder.BuildToolMessage(call.ID, call.Name, text)
					return
				}
			}

			result, err := o.executor.Execute(ctx, call, lockedCbs)

			o.observe(&obsMu, ToolCallInfo{
				Name:       call.Name,
				Arguments:  call.Arguments,
				Result:     result,
				DurationMs: time.Since(start).Milliseconds(),
				Err:        err,
			})

			if err != nil {
				if ctx.Err() != nil {
					mu.Lock()
					if ctxErr == nil {
						ctxErr = ctx.Err()
					}
					mu.Unlock()
				}
				result = fmt.Sprintf("Error executing tool: %v", err)
			}
			// PostToolUse 钩子（仅通知，结果被忽略）：结果文本截断后随载荷上报；
			// 被门禁阻断的调用未真正执行工具，不触发本事件
			if o.hookRunner != nil {
				payload := o.hookBase
				payload.ToolName = call.Name
				payload.ToolInput = call.Arguments
				payload.ToolResult = truncateRunes(result, hookToolResultRunes)
				_ = o.hookRunner.Run(ctx, agenthook.EventPostToolUse, payload)
			}
			results[i] = o.msgBuilder.BuildToolMessage(call.ID, call.Name, result)
		}(i, call)
	}
	wg.Wait()

	if ctxErr != nil {
		return nil, ctxErr
	}
	return results, nil
}

// lockedCallbacks 为并行工具执行构造回调代理：所有回调经同一互斥锁串行化。
// 工具回调（QQ 消息发送、图片加载队列等）内部可能修改共享状态
// （如 configureImageCallbacks 的 loadedImages 队列），并行调用存在数据竞争；
// 串行化后消息发送顺序取决于各工具的启动顺序，可能不再等于工具调用顺序，
// 但不影响工具结果回填 LLM 的顺序（由 results 下标保证）。
func (o *ToolOrchestrator) lockedCallbacks(callbacks llmtool.CallBackFuncs) llmtool.CallBackFuncs {
	var mu sync.Mutex
	strWrap := func(fn func(string) (string, error)) func(string) (string, error) {
		if fn == nil {
			return nil
		}
		return func(s string) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			return fn(s)
		}
	}
	return llmtool.CallBackFuncs{
		SendText:          strWrap(callbacks.SendText),
		SendImage:         strWrap(callbacks.SendImage),
		SendFile:          str2Wrap(callbacks.SendFile, &mu),
		GetMsgHistory:     wrap2(callbacks.GetMsgHistory, &mu),
		GetPrivateFileURL: strWrap(callbacks.GetPrivateFileURL),
		LoadImages:        wrap0(callbacks.LoadImages, &mu),
		TakeLoadedImages:  wrap0s(callbacks.TakeLoadedImages, &mu),
		LoadLocalImage:    strWrap(callbacks.LoadLocalImage),
		// RequestApproval 刻意透传不加锁：审批会阻塞等待真人回复（默认 120s），
		// 进互斥锁会卡死同轮并行工具的 SendText 等回调；并发安全由实现方
		// （approvalManager 的会话级锁）负责。
		RequestApproval: callbacks.RequestApproval,
	}
}

// str2Wrap 为 func(string, string) (string, error) 签名的回调套互斥锁。
func str2Wrap(fn func(string, string) (string, error), mu *sync.Mutex) func(string, string) (string, error) {
	if fn == nil {
		return nil
	}
	return func(a, b string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		return fn(a, b)
	}
}

// 以下 wrap* 辅助为不同签名的回调套互斥锁；nil 回调原样保留。
func wrap2(fn func(int, int) (string, error), mu *sync.Mutex) func(int, int) (string, error) {
	if fn == nil {
		return nil
	}
	return func(a, b int) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		return fn(a, b)
	}
}

func wrap0(fn func() (string, error), mu *sync.Mutex) func() (string, error) {
	if fn == nil {
		return nil
	}
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		return fn()
	}
}

func wrap0s(fn func() []string, mu *sync.Mutex) func() []string {
	if fn == nil {
		return nil
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return fn()
	}
}
