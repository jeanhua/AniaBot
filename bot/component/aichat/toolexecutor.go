package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/tmc/langchaingo/llms"
)

type ToolExecutor interface {
	Execute(ctx context.Context, call llms.ToolCall, callbacks llmtool.CallBackFuncs) (string, error)
	Tools() []llms.Tool
}

type ToolOrchestrator struct {
	executor      ToolExecutor
	msgBuilder    *MessageBuilder
	maxIterations int
}

func NewToolOrchestrator(executor ToolExecutor, msgBuilder *MessageBuilder) *ToolOrchestrator {
	return &ToolOrchestrator{
		executor:      executor,
		msgBuilder:    msgBuilder,
		maxIterations: 10,
	}
}

func (o *ToolOrchestrator) SetMaxIterations(max int) {
	o.maxIterations = max
}

// TokenUsage 记录本次请求的 token 消耗
type TokenUsage struct {
	PromptTokens     int // 发给模型的输入 token 数，包括 system prompt、历史消息、本次用户消息
	CompletionTokens int // CompletionTokens：模型生成的输出 token 数
	TotalTokens      int // TotalTokens：两者之和，也就是本次请求实际计费的 token 总量
}

func extractTokenUsage(resp *llms.ContentResponse) TokenUsage {
	if len(resp.Choices) == 0 {
		return TokenUsage{}
	}
	info := resp.Choices[0].GenerationInfo
	get := func(key string) int {
		if v, ok := info[key]; ok {
			switch n := v.(type) {
			case int:
				return n
			case int64:
				return int(n)
			case float64:
				return int(n)
			}
		}
		return 0
	}
	return TokenUsage{
		PromptTokens:     get("PromptTokens"),
		CompletionTokens: get("CompletionTokens"),
		TotalTokens:      get("TotalTokens"),
	}
}

func (o *ToolOrchestrator) ExecuteWithTools(
	ctx context.Context,
	llmClient *LLMClient,
	messages []llms.MessageContent,
	callbacks llmtool.CallBackFuncs,
	opts ...llms.CallOption,
) (string, []llms.MessageContent, TokenUsage, error) {
	var totalUsage TokenUsage

	if o.executor == nil || len(o.executor.Tools()) == 0 {
		resp, err := llmClient.Generate(ctx, messages, opts...)
		if err != nil {
			return "", messages, totalUsage, err
		}
		totalUsage = extractTokenUsage(resp)
		content := resp.Choices[0].Content
		messages = append(messages, o.msgBuilder.BuildAIMessage(content, nil))
		return content, messages, totalUsage, nil
	}

	for i := 0; i < o.maxIterations; i++ {
		// 每次迭代重新获取工具列表，支持动态加载（如 MCP 工具加载后立即生效）
		callOpts := append(opts, llms.WithTools(o.executor.Tools()))
		resp, err := llmClient.Generate(ctx, messages, callOpts...)
		if err != nil {
			return "", messages, totalUsage, err
		}

		u := extractTokenUsage(resp)
		totalUsage.PromptTokens += u.PromptTokens
		totalUsage.CompletionTokens += u.CompletionTokens
		totalUsage.TotalTokens += u.TotalTokens

		choice := resp.Choices[0]

		// 没有工具调用，返回最终响应
		if len(choice.ToolCalls) == 0 {
			messages = append(messages, o.msgBuilder.BuildAIMessage(choice.Content, nil))
			return choice.Content, messages, totalUsage, nil
		}

		// 保存 AI 消息（包含工具调用）
		messages = append(messages, o.msgBuilder.BuildAIMessage(choice.Content, choice.ToolCalls))

		// 发送 AI 的文本内容（如果有）
		if callbacks.SendText != nil && choice.Content != "" {
			callbacks.SendText(choice.Content)
		}

		// 执行工具调用
		toolResults, err := o.executeToolCalls(ctx, choice.ToolCalls, callbacks)
		if err != nil {
			return "", messages, totalUsage, err
		}
		messages = append(messages, toolResults...)

		// 达到最大迭代次数，强制生成最终响应
		if i == o.maxIterations-1 {
			messages = append(messages, o.msgBuilder.BuildToolLimitMessage())
			finalResp, err := llmClient.Generate(ctx, messages)
			if err != nil {
				return "", messages, totalUsage, err
			}
			if len(finalResp.Choices) == 0 {
				return "", messages, totalUsage, fmt.Errorf("no choices returned from final LLM call")
			}
			fu := extractTokenUsage(finalResp)
			totalUsage.PromptTokens += fu.PromptTokens
			totalUsage.CompletionTokens += fu.CompletionTokens
			totalUsage.TotalTokens += fu.TotalTokens
			finalContent := finalResp.Choices[0].Content
			messages = append(messages, o.msgBuilder.BuildAIMessage(finalContent, nil))
			return finalContent, messages, totalUsage, nil
		}
	}

	return "", messages, totalUsage, fmt.Errorf("exceeded maximum iterations")
}

func (o *ToolOrchestrator) executeToolCalls(
	ctx context.Context,
	toolCalls []llms.ToolCall,
	callbacks llmtool.CallBackFuncs,
) ([]llms.MessageContent, error) {
	results := make([]llms.MessageContent, 0, len(toolCalls))

	for _, call := range toolCalls {
		result, err := o.executor.Execute(ctx, call, callbacks)
		if err != nil {
			result = fmt.Sprintf("Error executing tool: %v", err)
		}
		results = append(results, o.msgBuilder.BuildToolMessage(call.ID, call.FunctionCall.Name, result))
	}

	return results, nil
}
