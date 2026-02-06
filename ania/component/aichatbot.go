package component

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeanhua/AniaBot/ania/component/functool"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/memory"
)

type ChatBot struct {
	prompt      string
	llm         llms.Model
	memory      *memory.ConversationWindowBuffer
	searchToken string
}

func NewChatBot(baseURL, apiKey, model, prompt string, windowSize int, searchToken string) (*ChatBot, error) {
	llm, err := openai.New(
		openai.WithToken(apiKey),
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, err
	}

	mem := memory.NewConversationWindowBuffer(
		windowSize,
		memory.WithReturnMessages(true),
	)

	return &ChatBot{
		prompt:      prompt,
		llm:         llm,
		memory:      mem,
		searchToken: searchToken,
	}, nil
}

func (b *ChatBot) Chat(ctx context.Context, userInput string, sendMsgFunc func(string) bool, opt ...llms.CallOption) (string, error) {
	variables, err := b.memory.LoadMemoryVariables(ctx, map[string]any{})
	if err != nil {
		return "", err
	}

	var messages []llms.MessageContent

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeSystem, b.prompt))

	if historyList, ok := variables["history"].([]llms.ChatMessage); ok {
		for _, msg := range historyList {
			messages = append(messages, llms.TextParts(msg.GetType(), msg.GetContent()))
		}
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, userInput))

	callopt := append(opt,
		llms.WithTools(functool.MakeJinaTool()),
		llms.WithTools(functool.MakeTimeTool()),
	)

	maxIterations := 5
	for i := 0; i < maxIterations; i++ {
		completion, err := b.llm.GenerateContent(ctx, messages, callopt...)
		if err != nil {
			return "", err
		}

		if len(completion.Choices) == 0 {
			return "", fmt.Errorf("no choices returned from LLM")
		}

		choice := completion.Choices[0]

		if len(choice.ToolCalls) == 0 {
			respText := choice.Content
			err = b.memory.SaveContext(ctx,
				map[string]any{"prompt": userInput},
				map[string]any{"response": respText},
			)
			return respText, err
		}

		aiMsg := llms.MessageContent{
			Role: llms.ChatMessageTypeAI,
		}
		if choice.Content != "" {
			sendMsgFunc(choice.Content)
			aiMsg.Parts = append(aiMsg.Parts, llms.TextPart(choice.Content))
		}
		for _, call := range choice.ToolCalls {
			aiMsg.Parts = append(aiMsg.Parts, call)
		}
		messages = append(messages, aiMsg)

		for _, call := range choice.ToolCalls {
			var callResult string
			var err error
			switch call.FunctionCall.Name {
			case functool.JINA_TOOL_SEARCH_NAME, functool.JINA_TOOL_EXPLORE_NAME:
				callResult, err = functool.TryHanleJina(ctx, b.searchToken, call)
			case functool.TIME_TOOL_NAME:
				callResult, err = functool.TryHanleTimeCall(call)
			default:
				err = errors.New("tool not exist")
			}
			if err != nil {
				callResult = fmt.Sprintf("Error executing tool: %v", err)
			}
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: call.ID,
						Name:       call.FunctionCall.Name,
						Content:    callResult,
					},
				},
			})
		}

		if i == maxIterations-1 {
			messages = append(messages, llms.TextParts(
				llms.ChatMessageTypeSystem,
				fmt.Sprintf("You have reached the maximum number of tool calls (%d). Please provide a final response based on the information you have gathered so far. Do not make any more tool calls.", maxIterations),
			))

			finalCompletion, err := b.llm.GenerateContent(ctx, messages)
			if err != nil {
				return "", err
			}

			if len(finalCompletion.Choices) == 0 {
				return "", fmt.Errorf("no choices returned from final LLM call")
			}

			respText := finalCompletion.Choices[0].Content
			err = b.memory.SaveContext(ctx,
				map[string]any{"prompt": userInput},
				map[string]any{"response": respText},
			)
			return respText, err
		}
	}

	return "", fmt.Errorf("unexpected error: exceeded maximum iterations")
}

func (b *ChatBot) ChatWithImage(ctx context.Context, userInput string, imageUrl string, opt ...llms.CallOption) (string, error) {
	parts := []llms.ContentPart{
		llms.TextPart(userInput),
		llms.ImageURLPart(imageUrl),
	}
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextPart(b.prompt)},
		},
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: parts,
		},
	}

	completion, err := b.llm.GenerateContent(ctx, messages, opt...)
	if err != nil {
		return "", err
	}
	return completion.Choices[0].Content, nil
}

func (b *ChatBot) ClearHistory(ctx context.Context) error {
	return b.memory.Clear(ctx)
}
