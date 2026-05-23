package llmtool

import "context"

type Tool interface {
	Name() string
	Description() string
	Params() any
	Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error)
}

type CallBackFuncs struct {
	SendText      func(text string) (string, error)
	SendFileURL   func(url string) (string, error)
	SendFile      func(name, bs64content string) (string, error)
	GetMsgHistory func(count int, message_seq int) (string, error)
}
