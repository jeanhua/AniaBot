package component

import (
	"context"

	"github.com/tmc/langchaingo/llms"
)

type LLM interface {
	GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error)
}

type OpenAILLM struct {
	model llms.Model
}

func NewOpenAILLM(model llms.Model) *OpenAILLM {
	return &OpenAILLM{
		model: model,
	}
}

func (l *OpenAILLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	return l.model.GenerateContent(ctx, messages, opts...)
}
