package aichat

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type LLMClient struct {
	llm llms.Model
}

func NewLLMClient(baseURL, apiKey, model string) (*LLMClient, error) {
	customClient := &http.Client{
		Transport: &extraBodyTransport{
			base: http.DefaultTransport,
			extraBody: map[string]any{
				"reasoning_split": true,
			},
		},
	}

	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
		openai.WithHTTPClient(customClient),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	return &LLMClient{llm: llm}, nil
}

func (c *LLMClient) Generate(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (*llms.ContentResponse, error) {
	resp, err := c.llm.GenerateContent(ctx, messages, opts...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices returned from LLM")
	}

	return resp, nil
}

func (c *LLMClient) GenerateSingle(ctx context.Context, messages []llms.MessageContent, opts ...llms.CallOption) (string, error) {
	resp, err := c.Generate(ctx, messages, opts...)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Content, nil
}

func (c *LLMClient) Model() llms.Model {
	return c.llm
}
