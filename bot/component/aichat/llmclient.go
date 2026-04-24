package aichat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type LLMClient struct {
	client openai.Client
	model  string
}

func NewLLMClient(baseURL, apiKey, model string) (*LLMClient, error) {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	return &LLMClient{client: client, model: model}, nil
}

type GenerateResponse struct {
	Content   string
	ToolCalls []llmtool.ToolCall
	// ReasoningContent 保存 API 返回的推理过程内容（如 DeepSeek 的 reasoning_content），
	// 在 tool calling 多轮对话中需要原样传回。
	ReasoningContent string
}

func (c *LLMClient) Generate(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
	apiMessages, err := c.convertMessages(messages)
	if err != nil {
		return GenerateResponse{}, TokenUsage{}, err
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.model),
		Messages: apiMessages,
	}

	c.applyOptions(&params, opts)

	completion, err := c.client.Chat.Completions.New(ctx, params)
	if err != nil {
		if ctx.Err() != nil {
			return GenerateResponse{}, TokenUsage{}, ctx.Err()
		}
		return GenerateResponse{}, TokenUsage{}, fmt.Errorf("LLM generation failed: %w", err)
	}

	if len(completion.Choices) == 0 {
		return GenerateResponse{}, TokenUsage{}, fmt.Errorf("no choices returned from LLM")
	}

	resp, usage := c.parseResponse(completion)
	return resp, usage, nil
}

func (c *LLMClient) GenerateSingle(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	resp, _, err := c.Generate(ctx, messages, opts)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *LLMClient) applyOptions(params *openai.ChatCompletionNewParams, opts ChatOptions) {
	if opts.MaxToken != nil {
		params.MaxTokens = openai.Int(int64(*opts.MaxToken))
	}
	if opts.MaxCompletionToken != nil {
		params.MaxCompletionTokens = openai.Int(int64(*opts.MaxCompletionToken))
	}
	if opts.Temperature != nil {
		params.Temperature = openai.Float(*opts.Temperature)
	}
	if opts.TopP != nil {
		params.TopP = openai.Float(*opts.TopP)
	}
	if opts.ReasoningEffort != nil {
		switch *opts.ReasoningEffort {
		case "low":
			params.ReasoningEffort = shared.ReasoningEffortLow
		case "medium":
			params.ReasoningEffort = shared.ReasoningEffortMedium
		case "high":
			params.ReasoningEffort = shared.ReasoningEffortHigh
		}
	}
	if len(opts.Tools) > 0 {
		apiTools := make([]openai.ChatCompletionToolUnionParam, 0, len(opts.Tools))
		for _, td := range opts.Tools {
			apiTools = append(apiTools, c.convertToolDef(td))
		}
		params.Tools = apiTools
	}
}

func (c *LLMClient) convertMessages(messages []Message) ([]openai.ChatCompletionMessageParamUnion, error) {
	result := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, msg := range messages {
		apiMsg, err := c.convertMessage(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, apiMsg)
	}
	return result, nil
}

func (c *LLMClient) convertMessage(msg Message) (openai.ChatCompletionMessageParamUnion, error) {
	switch msg.Role {
	case RoleSystem:
		text := ""
		if len(msg.Parts) > 0 && msg.Parts[0].Type == ContentPartText {
			text = msg.Parts[0].Text
		}
		return openai.SystemMessage(text), nil

	case RoleUser:
		hasImage := false
		for _, p := range msg.Parts {
			if p.Type == ContentPartImageURL {
				hasImage = true
				break
			}
		}

		if !hasImage {
			text := ""
			if len(msg.Parts) > 0 && msg.Parts[0].Type == ContentPartText {
				text = msg.Parts[0].Text
			}
			return openai.UserMessage(text), nil
		}

		parts := make([]openai.ChatCompletionContentPartUnionParam, 0, len(msg.Parts))
		for _, p := range msg.Parts {
			switch p.Type {
			case ContentPartText:
				parts = append(parts, openai.TextContentPart(p.Text))
			case ContentPartImageURL:
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL: p.ImageURL,
				}))
			}
		}
		return openai.UserMessage(parts), nil

	case RoleAssistant:
		text := ""
		if len(msg.Parts) > 0 && msg.Parts[0].Type == ContentPartText {
			text = msg.Parts[0].Text
		}

		if len(msg.ToolCalls) == 0 {
			msgUnion := openai.AssistantMessage(text)
			if msg.ReasoningContent != "" {
				if asst := msgUnion.OfAssistant; asst != nil {
					asst.SetExtraFields(map[string]any{
						"reasoning_content": msg.ReasoningContent,
					})
				}
			}
			return msgUnion, nil
		}

		// Assistant message with tool calls
		toolCalls := make([]openai.ChatCompletionMessageToolCallUnionParam, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallUnionParam{
				OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
					ID: tc.ID,
					Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
						Name:      tc.Name,
						Arguments: tc.Arguments,
					},
				},
			})
		}

		assistant := openai.ChatCompletionAssistantMessageParam{}
		if text != "" {
			assistant.Content.OfString = openai.String(text)
		}
		assistant.ToolCalls = toolCalls
		if msg.ReasoningContent != "" {
			assistant.SetExtraFields(map[string]any{
				"reasoning_content": msg.ReasoningContent,
			})
		}
		return openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant}, nil

	case RoleTool:
		content := ""
		if len(msg.Parts) > 0 && msg.Parts[0].Type == ContentPartText {
			content = msg.Parts[0].Text
		}
		return openai.ToolMessage(content, msg.ToolCallID), nil

	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unknown message role: %s", msg.Role)
	}
}

func (c *LLMClient) convertToolDef(td llmtool.ToolDef) openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionToolUnionParam{
		OfFunction: &openai.ChatCompletionFunctionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        td.Function.Name,
				Description: openai.String(td.Function.Description),
				Parameters:  td.Function.Parameters,
			},
		},
	}
}

func (c *LLMClient) parseResponse(completion *openai.ChatCompletion) (GenerateResponse, TokenUsage) {
	choice := completion.Choices[0]

	resp := GenerateResponse{
		Content: choice.Message.Content,
	}

	// 从原始响应 JSON 中提取 reasoning_content（DeepSeek 等 API 的推理过程字段）
	if raw := choice.Message.RawJSON(); raw != "" {
		var rawMap map[string]any
		if err := json.Unmarshal([]byte(raw), &rawMap); err == nil {
			if rc, ok := rawMap["reasoning_content"].(string); ok && rc != "" {
				resp.ReasoningContent = rc
			}
		}
	}

	for _, tc := range choice.Message.ToolCalls {
		resp.ToolCalls = append(resp.ToolCalls, llmtool.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	usage := TokenUsage{}
	if completion.Usage.PromptTokens > 0 {
		usage.PromptTokens = int(completion.Usage.PromptTokens)
	}
	if completion.Usage.CompletionTokens > 0 {
		usage.CompletionTokens = int(completion.Usage.CompletionTokens)
	}
	if completion.Usage.TotalTokens > 0 {
		usage.TotalTokens = int(completion.Usage.TotalTokens)
	}

	return resp, usage
}
