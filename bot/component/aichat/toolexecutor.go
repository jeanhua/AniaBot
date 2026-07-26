package aichat

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

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
	maxIterationsStr := os.Getenv("MAX_ITERATIONS")
	maxIterations := 20
	if maxIterationsStr != "" {
		if it, err := strconv.Atoi(maxIterationsStr); err == nil {
			maxIterations = it
		}
	}
	return &ToolOrchestrator{
		executor:      executor,
		msgBuilder:    msgBuilder,
		maxIterations: maxIterations,
	}
}

func (o *ToolOrchestrator) SetMaxIterations(max int) {
	o.maxIterations = max
}

// SetToolObserver 设置工具调用观察者：每次工具执行完成后回调一次（与执行同 goroutine，
// 同步调用），传 nil 取消。调用方需保证同一 orchestrator 的 ExecuteWithTools 串行执行。
func (o *ToolOrchestrator) SetToolObserver(fn func(ToolCallInfo)) {
	o.toolObserver = fn
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
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

	if o.executor == nil || len(o.executor.Tools()) == 0 {
		resp, usage, err := llmClient.Generate(ctx, messages, opts)
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
		resp, usage, err := llmClient.Generate(ctx, messages, opts)
		if err != nil {
			return "", messages, totalUsage, err
		}

		totalUsage.PromptTokens += usage.PromptTokens
		totalUsage.CompletionTokens += usage.CompletionTokens
		totalUsage.TotalTokens += usage.TotalTokens
		totalUsage.LastPromptTokens = usage.PromptTokens
		totalUsage.Iterations++

		if len(resp.ToolCalls) == 0 {
			messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(resp.Content, nil, resp.ReasoningContent))
			return resp.Content, messages, totalUsage, nil
		}

		messages = append(messages, o.msgBuilder.BuildAIMessageWithReasoning(resp.Content, resp.ToolCalls, resp.ReasoningContent))

		content := removeThinkContent(resp.Content)
		if callbacks.SendText != nil && len(content) > 0 {
			callbacks.SendText(content)
		}

		toolResults, err := o.executeToolCalls(ctx, resp.ToolCalls, callbacks)
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
			finalResp, finalUsage, err := llmClient.Generate(ctx, messages, finalOpts)
			if err != nil {
				return "", messages, totalUsage, err
			}
			totalUsage.PromptTokens += finalUsage.PromptTokens
			totalUsage.CompletionTokens += finalUsage.CompletionTokens
			totalUsage.TotalTokens += finalUsage.TotalTokens
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
) ([]Message, error) {
	results := make([]Message, 0, len(toolCalls))

	for _, call := range toolCalls {
		start := time.Now()
		result, err := o.executor.Execute(ctx, call, callbacks)
		if o.toolObserver != nil {
			o.toolObserver(ToolCallInfo{
				Name:       call.Name,
				Arguments:  call.Arguments,
				Result:     result,
				DurationMs: time.Since(start).Milliseconds(),
				Err:        err,
			})
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			result = fmt.Sprintf("Error executing tool: %v", err)
		}
		results = append(results, o.msgBuilder.BuildToolMessage(call.ID, call.Name, result))
	}

	return results, nil
}
