package functool

import (
	"context"
	"errors"
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

var errToolExecute = errors.New("function tool执行错误")

type SendFileParams struct {
	Name    string `json:"name" desc:"要发送的文件名(包括后缀)"`
	Content string `json:"content" desc:"文件的内容"`
}

type SendFileTool struct {
	llmtool.BaseTool[SendFileParams]
}

func NewSendFileTool() *SendFileTool {
	return &SendFileTool{
		BaseTool: llmtool.MakeBaseTool("file", "用于发送文件", SendFileParams{}),
	}
}

func (t *SendFileTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*SendFileParams)
	log.Println("执行file... 参数:", p)

	_, err := callbacks.SendFile(p.Name, p.Content)
	if err != nil {
		return "发送失败", errToolExecute
	}
	return "发送成功", nil
}
