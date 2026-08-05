package aichat

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type ChatBot struct {
	llmClient        *LLMClient
	msgBuilder       *MessageBuilder
	toolOrchestrator *ToolOrchestrator
	window           *messageWindow
}

// chatBotConfig 收集 NewChatBot 的可选参数。
type chatBotConfig struct {
	clientOpts       []LLMClientOption // 主对话 LLM 客户端的可选配置（重试/备用模型）
	compressorClient *LLMClient        // 上下文压缩专用客户端；nil 复用主对话客户端
}

// ChatBotOption 配置 ChatBot 的可选参数（函数选项模式）。
type ChatBotOption func(*chatBotConfig)

// WithClientOptions 为主对话 LLM 客户端附加可选配置（如 WithRetry / WithFallback）。
func WithClientOptions(opts ...LLMClientOption) ChatBotOption {
	return func(c *chatBotConfig) {
		c.clientOpts = append(c.clientOpts, opts...)
	}
}

// WithCompressorClient 指定上下文压缩专用 LLM 客户端（可独立配置更便宜的模型）。
// 未设置时压缩复用主对话客户端。
func WithCompressorClient(client *LLMClient) ChatBotOption {
	return func(c *chatBotConfig) {
		c.compressorClient = client
	}
}

func NewChatBot(baseURL, apiKey, model, prompt string, maxContextTokens int, toolExecutor ToolExecutor, historyStore HistoryStore, opts ...ChatBotOption) (*ChatBot, error) {
	cfg := chatBotConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	llmClient, err := NewLLMClient(baseURL, apiKey, model, cfg.clientOpts...)
	if err != nil {
		return nil, err
	}

	msgBuilder := NewMessageBuilder(prompt)
	toolOrchestrator := NewToolOrchestrator(toolExecutor, msgBuilder)

	compressor := NewContextCompressor(prompt)
	window := newMessageWindow(maxContextTokens, llmClient, compressor, historyStore, cfg.compressorClient)

	return &ChatBot{
		llmClient:        llmClient,
		msgBuilder:       msgBuilder,
		toolOrchestrator: toolOrchestrator,
		window:           window,
	}, nil
}

// LoadHistory 从持久化存储回放历史对话，重启后调用以恢复上下文。
// historyStore 未注入时为空操作。
func (b *ChatBot) LoadHistory(ctx context.Context) {
	b.window.load(ctx)
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, callbacks llmtool.CallBackFuncs, opts ChatOptions) (string, TokenUsage, error) {
	// 压缩检查：在构建消息之前，确保上下文不超限
	if err := b.window.MaybeCompress(ctx); err != nil {
		return "", TokenUsage{}, err
	}
	// 本轮若触发了上下文压缩，其 token 消耗并入当次请求统计（只算 token 字段，
	// 不计 Iterations 与 LastPromptTokens——压缩不是工具循环轮次，其 prompt
	// 大小也不代表当前上下文长度）
	compressUsage := b.window.takeCompressUsage()

	messages := b.msgBuilder.BuildChatMessages(userInput, b.window.history())
	// 记录构建完成时的真实长度作为新消息起点：ExecuteWithTools 返回的
	// updatedMessages 以 messages 为前缀，追加多出的部分即本轮新增消息
	builtLen := len(messages)

	response, updatedMessages, usage, err := b.toolOrchestrator.ExecuteWithTools(ctx, b.llmClient, messages, callbacks, opts)
	usage.PromptTokens += compressUsage.PromptTokens
	usage.CompletionTokens += compressUsage.CompletionTokens
	usage.TotalTokens += compressUsage.TotalTokens
	usage.CachedTokens += compressUsage.CachedTokens
	if err != nil {
		return "", usage, fmt.Errorf("chat execution failed: %w", err)
	}

	b.window.RecordUsage(usage)

	if builtLen < len(updatedMessages) {
		b.window.append(updatedMessages[builtLen:]...)
	}

	response = RemoveThinkContent(response)

	return response, usage, nil
}

// GetSingleImageDesc 生成单张图片的描述文本，返回描述与本次调用的 token 用量
// （用量由调用方并入所属请求/会话的统计与配额）。
func (b *ChatBot) GetSingleImageDesc(ctx context.Context, userInput string, imageURL string, opts ChatOptions) (string, TokenUsage, error) {
	messages := b.msgBuilder.BuildVisionMessages(userInput, imageURL)
	return b.llmClient.GenerateSingleWithUsage(ctx, messages, opts)
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	b.window.clear()
	return nil
}

func (b *ChatBot) ClearDynamicTools() int {
	if b.toolOrchestrator != nil && b.toolOrchestrator.executor != nil {
		if session, ok := b.toolOrchestrator.executor.(*llmtool.SessionToolExecutor); ok {
			return session.ClearDynamicMCPTools()
		}
	}
	return 0
}

func (b *ChatBot) SetToolOrchestrator(orchestrator *ToolOrchestrator) {
	b.toolOrchestrator = orchestrator
}

// SetToolObserver 设置工具调用观察者（每次工具执行完成后回调），传 nil 取消。
// 由调用方保证同一 ChatBot 的 Chat 调用串行（插件层按会话加锁）。
func (b *ChatBot) SetToolObserver(fn func(ToolCallInfo)) {
	b.toolOrchestrator.SetToolObserver(fn)
}

func (b *ChatBot) SetSkillManager(manager *llmtool.SkillManager) {
	b.msgBuilder.WithSkillManager(manager)
}

// SetMaxIterations 设置工具调用循环的最大轮数（子代理等场景使用比主对话更小的上限）。
func (b *ChatBot) SetMaxIterations(max int) {
	b.toolOrchestrator.SetMaxIterations(max)
}
