package functool

import (
	"context"
	"fmt"
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// PrivateFileParams 获取私聊文件URL的参数
type PrivateFileParams struct {
	FileID string `json:"file_id" desc:"私聊文件的file_id，从收到的文件消息中获取"`
}

// PrivateFileTool 获取私聊文件URL的工具
type PrivateFileTool struct {
	llmtool.BaseTool[PrivateFileParams]
}

func NewPrivateFileTool() *PrivateFileTool {
	return &PrivateFileTool{
		BaseTool: llmtool.MakeBaseTool("get_private_file_url", "获取私聊文件的下载URL。当用户在私聊中发送文件时，文件消息中不包含URL字段，需使用此工具通过file_id获取可下载的文件链接", PrivateFileParams{}),
	}
}

func (t *PrivateFileTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*PrivateFileParams)
	log.Println("执行get_private_file_url... file_id:", p.FileID)

	if callbacks.GetPrivateFileURL == nil {
		return "", fmt.Errorf("获取私聊文件URL功能不可用")
	}
	return callbacks.GetPrivateFileURL(p.FileID)
}
