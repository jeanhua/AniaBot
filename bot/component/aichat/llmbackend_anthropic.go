package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// anthropicBackend Anthropic Messages API 格式后端。
//
// 与 Chat Completions 的主要差异：
//   - system 提示词走独立的 system 参数而非消息；
//   - max_tokens 必填；
//   - 深度思考（extended thinking）以 thinking 块返回且带签名，tool calling
//     多轮中必须原样回传——为此把思考块序列化到 Message.ThinkingBlocks 持久化；
//   - 开启 thinking 时 temperature/top_p/top_k 不允许下发。
type anthropicBackend struct {
	client anthropic.Client
	model  string
}

func newAnthropicBackend(baseURL, apiKey, model string) *anthropicBackend {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &anthropicBackend{client: anthropic.NewClient(opts...), model: model}
}

// thinkingBlock 持久化到 Message.ThinkingBlocks 的元素：
// 记录 Anthropic 思考块回放所需的最小字段（thinking+signature 或 redacted data）。
type thinkingBlock struct {
	Type      string `json:"type"` // "thinking" | "redacted_thinking"
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
	Data      string `json:"data,omitempty"`
}

// anthropicDefaultMaxTokens Anthropic max_tokens 必填，调用方未配置时的默认上限。
const anthropicDefaultMaxTokens = 8192

func (b *anthropicBackend) generate(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
	// Anthropic 对大 max_tokens 的非流式请求强制要求流式（SDK 直接报
	// "streaming is required"，按 max_tokens/128k 估算超 10 分钟即触发），
	// 因此统一走流式通道内部聚合，对外仍表现为一次性返回。
	resp, usage, _, err := b.generateStream(ctx, messages, opts)
	return resp, usage, err
}

func (b *anthropicBackend) generateStream(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, bool, error) {
	params, err := b.buildParams(messages, opts)
	if err != nil {
		return GenerateResponse{}, TokenUsage{}, false, err
	}

	stream := b.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	acc := newAnthropicStreamAccumulator(opts.OnStreamDelta)
	started := false
	for stream.Next() {
		event := stream.Current()
		acc.Add(event)
		started = true // 任何流事件都算已开始输出
	}
	if err := stream.Err(); err != nil {
		if ctx.Err() != nil {
			return acc.Result(), acc.usage, started, ctx.Err()
		}
		return acc.Result(), acc.usage, started, err
	}

	resp := acc.Result()
	if resp.Content == "" && len(resp.ToolCalls) == 0 {
		return resp, acc.usage, started, fmt.Errorf("no content returned from LLM")
	}
	return resp, acc.usage, started, nil
}

// buildParams 组装 Anthropic 请求参数（消息转换 + 采样/工具/思考配置）。
func (b *anthropicBackend) buildParams(messages []Message, opts ChatOptions) (anthropic.MessageNewParams, error) {
	msgs, system, err := convertAnthropicMessages(messages)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}

	maxTokens := int64(anthropicDefaultMaxTokens)
	if opts.MaxToken != nil {
		maxTokens = int64(*opts.MaxToken)
	}
	if opts.MaxCompletionToken != nil {
		maxTokens = int64(*opts.MaxCompletionToken)
	}
	if maxTokens <= 0 {
		maxTokens = anthropicDefaultMaxTokens
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(b.model),
		Messages:  msgs,
		MaxTokens: maxTokens,
	}
	if len(system) > 0 {
		params.System = system
	}

	if opts.ReasoningEffort != nil {
		budget := thinkingBudgetTokens(*opts.ReasoningEffort)
		// Anthropic 要求 budget_tokens < max_tokens：不足时抬高 max_tokens
		if maxTokens <= budget {
			maxTokens = budget + 4096
			params.MaxTokens = maxTokens
		}
		params.Thinking = anthropic.ThinkingConfigParamUnion{
			OfEnabled: &anthropic.ThinkingConfigEnabledParam{BudgetTokens: budget},
		}
		// 开启 thinking 时 API 不允许 temperature/top_p/top_k，直接不下发
	} else {
		if opts.Temperature != nil {
			params.Temperature = anthropic.Float(*opts.Temperature)
		}
		if opts.TopP != nil {
			params.TopP = anthropic.Float(*opts.TopP)
		}
		if opts.TopK != nil {
			params.TopK = anthropic.Int(int64(*opts.TopK))
		}
	}

	if len(opts.Tools) > 0 {
		tools := make([]anthropic.ToolUnionParam, 0, len(opts.Tools))
		for _, td := range opts.Tools {
			tools = append(tools, convertAnthropicToolDef(td))
		}
		params.Tools = tools
	}
	return params, nil
}

// thinkingBudgetTokens 把统一的 ReasoningEffort 档位映射为 Anthropic thinking 预算。
func thinkingBudgetTokens(effort string) int64 {
	switch effort {
	case "medium":
		return 16384
	case "high":
		return 32768
	default: // low 及未知档位
		return 4096
	}
}

// convertAnthropicMessages 把内部消息模型转换为 Anthropic 消息数组 + system 块。
// tool 结果在 Anthropic 中归属 user 角色，连续的 tool 结果合并进同一条 user 消息。
func convertAnthropicMessages(messages []Message) ([]anthropic.MessageParam, []anthropic.TextBlockParam, error) {
	var result []anthropic.MessageParam
	var system []anthropic.TextBlockParam
	var pendingToolResults []anthropic.ContentBlockParamUnion

	flushToolResults := func() {
		if len(pendingToolResults) > 0 {
			result = append(result, anthropic.NewUserMessage(pendingToolResults...))
			pendingToolResults = nil
		}
	}

	for _, msg := range messages {
		switch msg.Role {
		case RoleSystem:
			if text := ExtractMessageText(msg); text != "" {
				system = append(system, anthropic.TextBlockParam{Text: text})
			}

		case RoleUser:
			flushToolResults()
			blocks := make([]anthropic.ContentBlockParamUnion, 0, len(msg.Parts))
			for _, p := range msg.Parts {
				switch p.Type {
				case ContentPartText:
					if p.Text != "" {
						blocks = append(blocks, anthropic.NewTextBlock(p.Text))
					}
				case ContentPartImageURL:
					blocks = append(blocks, anthropicImageBlock(p.ImageURL))
				}
			}
			if len(blocks) > 0 {
				result = append(result, anthropic.NewUserMessage(blocks...))
			}

		case RoleAssistant:
			flushToolResults()
			blocks, err := convertAnthropicAssistantBlocks(msg)
			if err != nil {
				return nil, nil, err
			}
			if len(blocks) > 0 {
				result = append(result, anthropic.NewAssistantMessage(blocks...))
			}

		case RoleTool:
			content := ""
			if len(msg.Parts) > 0 && msg.Parts[0].Type == ContentPartText {
				content = msg.Parts[0].Text
			}
			pendingToolResults = append(pendingToolResults,
				anthropic.NewToolResultBlock(msg.ToolCallID, content, false))

		default:
			return nil, nil, fmt.Errorf("unknown message role: %s", msg.Role)
		}
	}
	flushToolResults()
	return result, system, nil
}

// convertAnthropicAssistantBlocks 转换 assistant 消息：思考块（必须位于 tool_use
// 之前）→ 文本块 → tool_use 块。
func convertAnthropicAssistantBlocks(msg Message) ([]anthropic.ContentBlockParamUnion, error) {
	var blocks []anthropic.ContentBlockParamUnion

	if len(msg.ThinkingBlocks) > 0 {
		var tbs []thinkingBlock
		if err := json.Unmarshal(msg.ThinkingBlocks, &tbs); err == nil {
			for _, tb := range tbs {
				switch tb.Type {
				case "thinking":
					blocks = append(blocks, anthropic.ContentBlockParamUnion{
						OfThinking: &anthropic.ThinkingBlockParam{Thinking: tb.Thinking, Signature: tb.Signature},
					})
				case "redacted_thinking":
					blocks = append(blocks, anthropic.ContentBlockParamUnion{
						OfRedactedThinking: &anthropic.RedactedThinkingBlockParam{Data: tb.Data},
					})
				}
			}
		}
	}

	if text := ExtractMessageText(msg); text != "" {
		blocks = append(blocks, anthropic.NewTextBlock(text))
	}

	for _, tc := range msg.ToolCalls {
		// Input 为 any：传 json.RawMessage 可原样下发模型生成的参数 JSON
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, json.RawMessage(tc.Arguments), tc.Name))
	}
	return blocks, nil
}

// anthropicImageBlock 构造图片块：data: URI 解析为 base64 源，其余按 URL 源下发。
func anthropicImageBlock(url string) anthropic.ContentBlockParamUnion {
	if mediaType, data, ok := parseImageDataURI(url); ok {
		return anthropic.NewImageBlockBase64(mediaType, data)
	}
	return anthropic.NewImageBlock(anthropic.URLImageSourceParam{URL: url})
}

// parseImageDataURI 解析 data:<mediaType>;base64,<data> 形式的图片 URI。
func parseImageDataURI(uri string) (mediaType, data string, ok bool) {
	if !strings.HasPrefix(uri, "data:") {
		return "", "", false
	}
	rest := uri[len("data:"):]
	idx := strings.Index(rest, ",")
	if idx < 0 {
		return "", "", false
	}
	meta, data := rest[:idx], rest[idx+1:]
	mediaType = strings.SplitN(meta, ";", 2)[0]
	if mediaType == "" || data == "" {
		return "", "", false
	}
	return mediaType, data, true
}

// convertAnthropicToolDef 把内部工具定义转换为 Anthropic 工具参数。
// input_schema 从通用 JSON schema map 提取 properties/required，其余键原样透传。
func convertAnthropicToolDef(td llmtool.ToolDef) anthropic.ToolUnionParam {
	schema := anthropic.ToolInputSchemaParam{}
	extra := map[string]any{}
	for k, v := range td.Function.Parameters {
		switch k {
		case "properties":
			schema.Properties = v
		case "required":
			switch r := v.(type) {
			case []string:
				schema.Required = r
			case []any:
				for _, item := range r {
					if s, ok := item.(string); ok {
						schema.Required = append(schema.Required, s)
					}
				}
			}
		case "type":
			// input_schema 固定为 object，忽略
		default:
			extra[k] = v
		}
	}
	if len(extra) > 0 {
		schema.ExtraFields = extra
	}
	return anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name:        td.Function.Name,
			Description: anthropic.String(td.Function.Description),
			InputSchema: schema,
		},
	}
}

// anthropicTokenUsage 转换 token 用量。Anthropic 不直接返回总量，以 in+out 合计；
// 缓存命中取 cache_read_input_tokens。
func anthropicTokenUsage(u anthropic.Usage) TokenUsage {
	return TokenUsage{
		PromptTokens:     int(u.InputTokens),
		CompletionTokens: int(u.OutputTokens),
		TotalTokens:      int(u.InputTokens + u.OutputTokens),
		CachedTokens:     int(u.CacheReadInputTokens),
	}
}

// anthropicStreamAccumulator Anthropic 流式事件累积器：内容、思考块（含签名）、
// 按 block index 组装的 tool_use（input_json_delta 为参数增量）与 usage。
type anthropicStreamAccumulator struct {
	content       strings.Builder
	reasoning     strings.Builder
	toolCalls     map[int64]*llmtool.ToolCall
	toolOrder     []int64
	thinking      map[int64]*thinkingBlock
	thinkingOrder []int64
	usage         TokenUsage
	onDelta       func(string)
}

func newAnthropicStreamAccumulator(onDelta func(string)) *anthropicStreamAccumulator {
	return &anthropicStreamAccumulator{
		toolCalls: map[int64]*llmtool.ToolCall{},
		thinking:  map[int64]*thinkingBlock{},
		onDelta:   onDelta,
	}
}

func (a *anthropicStreamAccumulator) Add(event anthropic.MessageStreamEventUnion) {
	switch event.Type {
	case "message_start":
		a.usage = anthropicTokenUsage(event.Message.Usage)

	case "content_block_start":
		cb := event.ContentBlock
		switch cb.Type {
		case "tool_use":
			a.toolCalls[event.Index] = &llmtool.ToolCall{ID: cb.ID, Name: cb.Name}
			a.toolOrder = append(a.toolOrder, event.Index)
		case "thinking":
			a.thinking[event.Index] = &thinkingBlock{Type: "thinking", Thinking: cb.Thinking, Signature: cb.Signature}
			a.thinkingOrder = append(a.thinkingOrder, event.Index)
		case "redacted_thinking":
			a.thinking[event.Index] = &thinkingBlock{Type: "redacted_thinking", Data: cb.Data}
			a.thinkingOrder = append(a.thinkingOrder, event.Index)
		}

	case "content_block_delta":
		d := event.Delta
		switch d.Type {
		case "text_delta":
			a.content.WriteString(d.Text)
			if a.onDelta != nil && d.Text != "" {
				a.onDelta(d.Text)
			}
		case "thinking_delta":
			a.reasoning.WriteString(d.Thinking)
			if tb, ok := a.thinking[event.Index]; ok {
				tb.Thinking += d.Thinking
			}
		case "signature_delta":
			if tb, ok := a.thinking[event.Index]; ok {
				tb.Signature += d.Signature
			}
		case "input_json_delta":
			if tc, ok := a.toolCalls[event.Index]; ok {
				tc.Arguments += d.PartialJSON
			}
		}

	case "message_delta":
		// message_delta 只携带 output_tokens 等增量用量，覆盖最终值
		if event.Usage.OutputTokens > 0 {
			a.usage.CompletionTokens = int(event.Usage.OutputTokens)
			a.usage.TotalTokens = a.usage.PromptTokens + a.usage.CompletionTokens
		}
	}
}

func (a *anthropicStreamAccumulator) Result() GenerateResponse {
	resp := GenerateResponse{
		Content:          a.content.String(),
		ReasoningContent: a.reasoning.String(),
	}
	// 按 block index 升序输出（模型的规范顺序），与流到达顺序无关
	order := append([]int64(nil), a.toolOrder...)
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, idx := range order {
		resp.ToolCalls = append(resp.ToolCalls, *a.toolCalls[idx])
	}
	if len(a.thinkingOrder) > 0 {
		tOrder := append([]int64(nil), a.thinkingOrder...)
		sort.Slice(tOrder, func(i, j int) bool { return tOrder[i] < tOrder[j] })
		tbs := make([]thinkingBlock, 0, len(tOrder))
		for _, idx := range tOrder {
			tbs = append(tbs, *a.thinking[idx])
		}
		if raw, err := json.Marshal(tbs); err == nil {
			resp.ThinkingBlocks = raw
		}
	}
	return resp
}
