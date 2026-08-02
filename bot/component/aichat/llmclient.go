package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type LLMClient struct {
	client   openai.Client
	model    string
	retry    *retryConfig
	fallback *LLMClient
}

// retryConfig 应用层重试配置：maxAttempts 为最大尝试次数（<=1 表示不重试），
// baseDelay 为退避基准时长，每次重试等待 base×2^n 加随机抖动。
type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
}

// llmClientConfig 收集 NewLLMClient 的可选参数。
type llmClientConfig struct {
	maxAttempts     int
	baseDelay       time.Duration
	fallbackBaseURL string
	fallbackAPIKey  string
	fallbackModel   string
}

// LLMClientOption 配置 LLMClient 的可选参数（函数选项模式）。
type LLMClientOption func(*llmClientConfig)

// WithRetry 配置应用层重试：429/5xx/网络错误时指数退避重试，最多 maxAttempts 次
// （0 或 1 表示不重试）。注意 openai-go SDK 已内置 408/409/429/5xx 的指数退避重试
// （默认 2 次），本选项补充 SDK 不覆盖的网络错误与长尾故障，并自定义退避节奏；
// 两层叠加下最坏请求次数为 maxAttempts×(1+SDK 重试次数)。
func WithRetry(maxAttempts int, baseDelay time.Duration) LLMClientOption {
	return func(c *llmClientConfig) {
		c.maxAttempts = maxAttempts
		c.baseDelay = baseDelay
	}
}

// WithFallback 配置备用模型：主模型重试耗尽或遇到不可重试错误时，改用备用模型
// 再请求一次（备用客户端内部自带同等重试）。fallbackModel 为空表示不启用。
// baseURL / apiKey 留空时回退到主模型配置。
func WithFallback(baseURL, apiKey, model string) LLMClientOption {
	return func(c *llmClientConfig) {
		c.fallbackBaseURL = baseURL
		c.fallbackAPIKey = apiKey
		c.fallbackModel = model
	}
}

func NewLLMClient(baseURL, apiKey, model string, opts ...LLMClientOption) (*LLMClient, error) {
	cfg := llmClientConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	c := &LLMClient{client: client, model: model}
	if cfg.maxAttempts > 1 {
		c.retry = &retryConfig{maxAttempts: cfg.maxAttempts, baseDelay: cfg.baseDelay}
	}
	if cfg.fallbackModel != "" {
		fbBaseURL, fbAPIKey := cfg.fallbackBaseURL, cfg.fallbackAPIKey
		if fbBaseURL == "" {
			fbBaseURL = baseURL
		}
		if fbAPIKey == "" {
			fbAPIKey = apiKey
		}
		fb, err := NewLLMClient(fbBaseURL, fbAPIKey, cfg.fallbackModel,
			WithRetry(cfg.maxAttempts, cfg.baseDelay)) // 不传 WithFallback 防止递归
		if err != nil {
			return nil, fmt.Errorf("create fallback client: %w", err)
		}
		c.fallback = fb
	}
	return c, nil
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

	// 应用层重试循环：SDK 已内置 408/409/429/5xx 的指数退避重试，本层补充
	// SDK 不重试的网络错误，并叠加自定义退避节奏。所有等待都尊重 ctx 剩余
	// deadline：取消/超时立即返回，绝不拖长请求。
	for attempt := 0; ; attempt++ {
		resp, usage, err := c.generateOnce(ctx, apiMessages, opts)
		if err == nil {
			return resp, usage, nil
		}

		if ctx.Err() != nil {
			// 取消/超时不重试，原样返回（上层据此区分超时与普通错误）
			return GenerateResponse{}, TokenUsage{}, ctx.Err()
		}

		if !retryableLLMError(err) || c.retry == nil || attempt+1 >= c.retry.maxAttempts {
			// 重试耗尽或不可重试：改用备用模型再请求一次（仅一次，备用客户端
			// 内部自带同等重试）。备用模型也失败时返回其错误。
			if c.fallback != nil {
				fbResp, fbUsage, fbErr := c.fallback.Generate(ctx, messages, opts)
				if fbErr == nil {
					return fbResp, fbUsage, nil
				}
				if ctx.Err() != nil {
					return GenerateResponse{}, TokenUsage{}, ctx.Err()
				}
				return GenerateResponse{}, TokenUsage{}, fmt.Errorf("LLM generation failed (fallback): %w", fbErr)
			}
			return GenerateResponse{}, TokenUsage{}, fmt.Errorf("LLM generation failed: %w", err)
		}

		delay := retryDelay(c.retry.baseDelay, attempt)
		select {
		case <-ctx.Done():
			return GenerateResponse{}, TokenUsage{}, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// generateOnce 单次 LLM 请求（不含重试与备用切换）。
func (c *LLMClient) generateOnce(ctx context.Context, apiMessages []openai.ChatCompletionMessageParamUnion, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
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
		return GenerateResponse{}, TokenUsage{}, err
	}

	if len(completion.Choices) == 0 {
		return GenerateResponse{}, TokenUsage{}, fmt.Errorf("no choices returned from LLM")
	}

	resp, usage := c.parseResponse(completion)
	return resp, usage, nil
}

// GenerateStream 流式生成：与 Generate 相同的重试/备用模型语义，
// 唯一差异——已输出首字节后失败不重试、不切换备用（避免重复输出），
// 返回已积累的（部分）内容与错误。opts.OnStreamDelta 为 nil 时退化为一次性。
func (c *LLMClient) GenerateStream(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
	apiMessages, err := c.convertMessages(messages)
	if err != nil {
		return GenerateResponse{}, TokenUsage{}, err
	}

	for attempt := 0; ; attempt++ {
		resp, usage, started, err := c.streamOnce(ctx, apiMessages, opts)
		if err == nil {
			return resp, usage, nil
		}

		if ctx.Err() != nil {
			// 取消/超时不重试，原样返回（上层据此区分超时与普通错误）
			return GenerateResponse{}, TokenUsage{}, ctx.Err()
		}

		// 已输出首字节：不重试不切换备用，避免用户看到重复输出
		if started {
			return resp, usage, fmt.Errorf("LLM stream failed after partial output: %w", err)
		}

		if !retryableLLMError(err) || c.retry == nil || attempt+1 >= c.retry.maxAttempts {
			if c.fallback != nil {
				fbResp, fbUsage, fbErr := c.fallback.GenerateStream(ctx, messages, opts)
				if fbErr == nil {
					return fbResp, fbUsage, nil
				}
				if ctx.Err() != nil {
					return GenerateResponse{}, TokenUsage{}, ctx.Err()
				}
				return GenerateResponse{}, TokenUsage{}, fmt.Errorf("LLM generation failed (fallback): %w", fbErr)
			}
			return GenerateResponse{}, TokenUsage{}, fmt.Errorf("LLM generation failed: %w", err)
		}

		delay := retryDelay(c.retry.baseDelay, attempt)
		select {
		case <-ctx.Done():
			return GenerateResponse{}, TokenUsage{}, ctx.Err()
		case <-time.After(delay):
		}
	}
}

// streamOnce 单次流式请求：累积内容/reasoning/tool calls（按 Index 组装）/usage，
// 返回 started 表示是否已收到任何数据块（首字节前失败可安全重试）。
func (c *LLMClient) streamOnce(ctx context.Context, apiMessages []openai.ChatCompletionMessageParamUnion, opts ChatOptions) (GenerateResponse, TokenUsage, bool, error) {
	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(c.model),
		Messages: apiMessages,
	}
	c.applyOptions(&params, opts)
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)}

	stream := c.client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	acc := newStreamAccumulator()
	started := false
	for stream.Next() {
		chunk := stream.Current()
		acc.Add(chunk)
		// 真实数据块（有 choices 或 usage）才算已开始输出；
		// SDK 对流内 error 事件不产出 chunk（Next 直接返回 false 并置 Err）
		if len(chunk.Choices) > 0 || chunk.Usage.PromptTokens > 0 ||
			chunk.Usage.CompletionTokens > 0 || chunk.Usage.TotalTokens > 0 {
			started = true
		}
		// 增量回调在流读取 goroutine 串行调用
		if opts.OnStreamDelta != nil && len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			opts.OnStreamDelta(chunk.Choices[0].Delta.Content)
		}
	}
	if err := stream.Err(); err != nil {
		if ctx.Err() != nil {
			return acc.Result(), acc.usage, started, ctx.Err()
		}
		return acc.Result(), acc.usage, started, err
	}

	resp := acc.Result()
	if resp.Content == "" && len(resp.ToolCalls) == 0 {
		return resp, acc.usage, started, fmt.Errorf("no choices returned from LLM")
	}
	return resp, acc.usage, started, nil
}

// streamAccumulator 流式块累积器：内容、reasoning_content、按 Index 组装的
// tool calls（首块带 ID/Name，后续块为 Arguments 增量）与末块 usage。
type streamAccumulator struct {
	content   strings.Builder
	reasoning strings.Builder
	toolCalls map[int64]*llmtool.ToolCall
	toolOrder []int64
	usage     TokenUsage
}

func newStreamAccumulator() *streamAccumulator {
	return &streamAccumulator{toolCalls: map[int64]*llmtool.ToolCall{}}
}

func (a *streamAccumulator) Add(chunk openai.ChatCompletionChunk) {
	for _, choice := range chunk.Choices {
		d := choice.Delta
		if d.Content != "" {
			a.content.WriteString(d.Content)
		}
		// reasoning_content 为扩展字段，从块原始 JSON 提取
		if raw := d.RawJSON(); raw != "" {
			var rawMap map[string]any
			if err := json.Unmarshal([]byte(raw), &rawMap); err == nil {
				if rc, ok := rawMap["reasoning_content"].(string); ok && rc != "" {
					a.reasoning.WriteString(rc)
				}
			}
		}
		for _, tc := range d.ToolCalls {
			cur, ok := a.toolCalls[tc.Index]
			if !ok {
				cur = &llmtool.ToolCall{ID: tc.ID, Name: tc.Function.Name}
				a.toolCalls[tc.Index] = cur
				a.toolOrder = append(a.toolOrder, tc.Index)
				continue
			}
			if tc.ID != "" {
				cur.ID = tc.ID
			}
			if tc.Function.Name != "" {
				cur.Name = tc.Function.Name
			}
			if tc.Function.Arguments != "" {
				cur.Arguments += tc.Function.Arguments
			}
		}
	}
	// usage 仅在包含 include_usage 的末块返回（空 Choices）
	if len(chunk.Choices) == 0 {
		if chunk.Usage.PromptTokens > 0 {
			a.usage.PromptTokens = int(chunk.Usage.PromptTokens)
		}
		if chunk.Usage.CompletionTokens > 0 {
			a.usage.CompletionTokens = int(chunk.Usage.CompletionTokens)
		}
		if chunk.Usage.TotalTokens > 0 {
			a.usage.TotalTokens = int(chunk.Usage.TotalTokens)
		}
		a.usage.CachedTokens = extractCachedTokens(&chunk.Usage)
	}
}

func (a *streamAccumulator) Result() GenerateResponse {
	resp := GenerateResponse{
		Content:          a.content.String(),
		ReasoningContent: a.reasoning.String(),
	}
	// 按 Index 升序输出（模型的规范顺序），与流到达顺序无关
	order := append([]int64(nil), a.toolOrder...)
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, idx := range order {
		resp.ToolCalls = append(resp.ToolCalls, *a.toolCalls[idx])
	}
	return resp
}

// retryableLLMError 判断错误是否值得重试：HTTP 429/5xx 可重试；
// SDK 对网络/传输错误不包装、原样返回原始错误，此时也视为可重试。
func retryableLLMError(err error) bool {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	return true
}

// retryDelay 计算第 attempt 次重试的等待时间：base×2^attempt 加随机抖动
// （0~delay），并设 30s 上限避免长时间空等。
func retryDelay(base time.Duration, attempt int) time.Duration {
	delay := base << attempt
	if delay <= 0 {
		delay = base
	}
	if delay > 30*time.Second {
		delay = 30 * time.Second
	}
	if delay > 0 {
		delay += time.Duration(rand.Int63n(int64(delay)))
	}
	return delay
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
	if opts.TopK != nil {
		// top_k 非 OpenAI 标准参数，openai-go SDK 未内置字段，
		// 经 ExtraFields 原样下发（DeepSeek 等兼容 API 支持）
		params.SetExtraFields(map[string]any{"top_k": *opts.TopK})
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
			// 拼接全部文本片段：多片段的用户消息（如回放历史时图片片段被
			// degradeImagesToText 降级为文本标记）只取首段会静默丢内容
			return openai.UserMessage(ExtractMessageText(msg)), nil
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
	usage.CachedTokens = extractCachedTokens(&completion.Usage)

	return resp, usage
}

// extractCachedTokens 提取命中 prompt 缓存的 token 数。优先取 OpenAI 标准字段
// prompt_tokens_details.cached_tokens；DeepSeek 等提供方返回的是扩展字段
// prompt_cache_hit_tokens，SDK 未建模，需从原始 JSON 兜底解析。均无时返回 0。
func extractCachedTokens(usage *openai.CompletionUsage) int {
	if usage.PromptTokensDetails.CachedTokens > 0 {
		return int(usage.PromptTokensDetails.CachedTokens)
	}
	if raw := usage.RawJSON(); raw != "" {
		var rawMap map[string]any
		if err := json.Unmarshal([]byte(raw), &rawMap); err == nil {
			if hit, ok := rawMap["prompt_cache_hit_tokens"].(float64); ok && hit > 0 {
				return int(hit)
			}
		}
	}
	return 0
}
