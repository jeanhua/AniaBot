package pluginaichat

import (
	"context"
	"log/slog"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// embedder 封装 OpenAI 兼容的 embeddings 客户端（复用 openai-go/v3 的
// EmbeddingService，与 LLMClient 同一套 SDK）。
//
// 所有调用失败时静默返回 nil，由 knowledgeManager 退回纯关键词检索，
// 不阻塞文档写入与检索主流程。provider 若未实现 /v1/embeddings（如老版
// DeepSeek），自动退化为关键词检索，功能不致整体失效。
type embedder struct {
	client openai.Client
	model  string
	logger *slog.Logger
}

// newEmbedder 创建语义向量计算器；baseURL/apiKey/model 任一为空返回 nil。
func newEmbedder(baseURL, apiKey, model string, logger *slog.Logger) *embedder {
	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	return &embedder{
		client: openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(baseURL)),
		model:  model,
		logger: logger,
	}
}

// EmbedMany 批量计算文本向量，与输入顺序一一对应；出错返回 nil。
func (e *embedder) EmbedMany(ctx context.Context, texts []string) [][]float32 {
	if e == nil || len(texts) == 0 {
		return nil
	}
	resp, err := e.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Model: openai.EmbeddingModel(e.model),
		Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: texts},
	})
	if err != nil {
		if e.logger != nil {
			e.logger.Warn("embedding 调用失败，退回关键词检索", "error", err)
		}
		return nil
	}
	out := make([][]float32, 0, len(resp.Data))
	for _, d := range resp.Data {
		vec := make([]float32, 0, len(d.Embedding))
		for _, v := range d.Embedding {
			vec = append(vec, float32(v))
		}
		out = append(out, vec)
	}
	return out
}

// EmbedOne 计算单个文本向量；出错返回 nil。
func (e *embedder) EmbedOne(ctx context.Context, text string) []float32 {
	vs := e.EmbedMany(ctx, []string{text})
	if len(vs) == 0 {
		return nil
	}
	return vs[0]
}
