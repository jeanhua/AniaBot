package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/openai/openai-go/v3"
)

// LLMClient LLM 客户端外壳：持有具体 API 格式后端（llmBackend），
// 统一提供应用层重试与备用模型切换语义。
type LLMClient struct {
	backend  llmBackend
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
	apiFormat       string
	fallbackBaseURL string
	fallbackAPIKey  string
	fallbackModel   string
	fallbackFormat  string
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

// WithAPIFormat 指定 LLM API 格式（APIFormatChatCompletions / APIFormatResponses /
// APIFormatAnthropic），空值等同 chat_completions。
func WithAPIFormat(format string) LLMClientOption {
	return func(c *llmClientConfig) {
		c.apiFormat = format
	}
}

// WithFallback 配置备用模型：主模型重试耗尽或遇到不可重试错误时，改用备用模型
// 再请求一次（备用客户端内部自带同等重试）。fallbackModel 为空表示不启用。
// baseURL / apiKey / format 留空时回退到主模型配置。
func WithFallback(baseURL, apiKey, model, format string) LLMClientOption {
	return func(c *llmClientConfig) {
		c.fallbackBaseURL = baseURL
		c.fallbackAPIKey = apiKey
		c.fallbackModel = model
		c.fallbackFormat = format
	}
}

func NewLLMClient(baseURL, apiKey, model string, opts ...LLMClientOption) (*LLMClient, error) {
	cfg := llmClientConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	backend, err := newLLMBackend(cfg.apiFormat, baseURL, apiKey, model)
	if err != nil {
		return nil, err
	}
	c := &LLMClient{backend: backend, model: model}
	if cfg.maxAttempts > 1 {
		c.retry = &retryConfig{maxAttempts: cfg.maxAttempts, baseDelay: cfg.baseDelay}
	}
	if cfg.fallbackModel != "" {
		fbBaseURL, fbAPIKey, fbFormat := cfg.fallbackBaseURL, cfg.fallbackAPIKey, cfg.fallbackFormat
		if fbBaseURL == "" {
			fbBaseURL = baseURL
		}
		if fbAPIKey == "" {
			fbAPIKey = apiKey
		}
		fb, err := NewLLMClient(fbBaseURL, fbAPIKey, cfg.fallbackModel,
			WithRetry(cfg.maxAttempts, cfg.baseDelay),
			WithAPIFormat(fbFormat)) // 不传 WithFallback 防止递归
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
	// ThinkingBlocks 保存 Anthropic 格式的思考块原始信息（JSON 数组，元素见
	// thinkingBlock），tool calling 多轮中 Anthropic 要求原样回传；仅 anthropic
	// 格式写入。
	ThinkingBlocks json.RawMessage
}

func (c *LLMClient) Generate(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
	// 应用层重试循环：SDK 已内置 408/409/429/5xx 的指数退避重试，本层补充
	// SDK 不重试的网络错误，并叠加自定义退避节奏。所有等待都尊重 ctx 剩余
	// deadline：取消/超时立即返回，绝不拖长请求。
	for attempt := 0; ; attempt++ {
		resp, usage, err := c.backend.generate(ctx, messages, opts)
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

// GenerateStream 流式生成：与 Generate 相同的重试/备用模型语义，
// 唯一差异——已输出首字节后失败不重试、不切换备用（避免重复输出），
// 返回已积累的（部分）内容与错误。opts.OnStreamDelta 为 nil 时退化为一次性。
func (c *LLMClient) GenerateStream(ctx context.Context, messages []Message, opts ChatOptions) (GenerateResponse, TokenUsage, error) {
	for attempt := 0; ; attempt++ {
		resp, usage, started, err := c.backend.generateStream(ctx, messages, opts)
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

// retryableLLMError 判断错误是否值得重试：HTTP 429/5xx 可重试；
// SDK 对网络/传输错误不包装、原样返回原始错误，此时也视为可重试。
// 同时识别 openai-go 与 anthropic-sdk-go 两种 SDK 的错误类型。
func retryableLLMError(err error) bool {
	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	var antErr *anthropic.Error
	if errors.As(err, &antErr) {
		return antErr.StatusCode == 429 || antErr.StatusCode >= 500
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

// GenerateSingleWithUsage 单次生成（不带工具），返回内容与 token 用量。
// 供调用方把压缩器、图片描述等辅助 LLM 调用的消耗计入统计。
func (c *LLMClient) GenerateSingleWithUsage(ctx context.Context, messages []Message, opts ChatOptions) (string, TokenUsage, error) {
	resp, usage, err := c.Generate(ctx, messages, opts)
	if err != nil {
		return "", usage, err
	}
	return resp.Content, usage, nil
}

func (c *LLMClient) GenerateSingle(ctx context.Context, messages []Message, opts ChatOptions) (string, error) {
	content, _, err := c.GenerateSingleWithUsage(ctx, messages, opts)
	return content, err
}
