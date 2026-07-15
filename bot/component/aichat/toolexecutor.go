package aichat

import (
	"context"
	"fmt"
	"os"
	"strconv"

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

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
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
		result, err := o.executor.Execute(ctx, call, callbacks)
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
