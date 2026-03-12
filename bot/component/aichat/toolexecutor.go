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
		maxIterations: 20,
	}
}

func (o *ToolOrchestrator) SetMaxIterations(max int) {
	o.maxIterations = max
}

func (o *ToolOrchestrator) ExecuteWithTools(
	ctx context.Context,
	llmClient *LLMClient,
	messages []llms.MessageContent,
	callbacks llmtool.CallBackFuncs,
	opts ...llms.CallOption,
) (string, []llms.MessageContent, error) {
	if o.executor == nil || len(o.executor.Tools()) == 0 {
		resp, err := llmClient.Generate(ctx, messages, opts...)
		if err != nil {
			return "", messages, err
		}
		return resp.Choices[0].Content, messages, nil
	}

	callOpts := append(opts, llms.WithTools(o.executor.Tools()))

	for i := 0; i < o.maxIterations; i++ {
		resp, err := llmClient.Generate(ctx, messages, callOpts...)
		if err != nil {
			return "", messages, err
		}

		choice := resp.Choices[0]

		if len(choice.ToolCalls) == 0 {
			return choice.Content, messages, nil
		}

		messages = append(messages, o.msgBuilder.BuildAIMessage(choice.Content, choice.ToolCalls))

		if callbacks.SendText != nil && choice.Content != "" {
			callbacks.SendText(choice.Content)
		}

		toolResults, err := o.executeToolCalls(ctx, choice.ToolCalls, callbacks)
		if err != nil {
			return "", messages, err
		}
		messages = append(messages, toolResults...)

		if i == o.maxIterations-1 {
			messages = append(messages, o.msgBuilder.BuildToolLimitMessage())
			finalResp, err := llmClient.Generate(ctx, messages)
			if err != nil {
				return "", messages, err
			}
			if len(finalResp.Choices) == 0 {
				return "", messages, fmt.Errorf("no choices returned from final LLM call")
			}
			return finalResp.Choices[0].Content, messages, nil
		}
	}

	return "", messages, fmt.Errorf("exceeded maximum iterations")
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
