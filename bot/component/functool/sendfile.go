package functool

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

var errToolExecute = errors.New("function tool执行错误")

// SendFileTool 读取本地文件并发送
type SendFileParams struct {
	Name string `json:"name" desc:"要发送的文件名(包括后缀),用于显示,如 1.txt"`
	Path string `json:"path" desc:"本地文件完整路径,如 /tmp/1.txt"`
}

type SendFileTool struct {
	llmtool.BaseTool[SendFileParams]
}

func NewSendFileTool() *SendFileTool {
	return &SendFileTool{
		BaseTool: llmtool.MakeBaseTool("file", "用于读取本地文件并发送给用户", SendFileParams{}),
	}
}

func (t *SendFileTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*SendFileParams)
	log.Println("执行file... 参数:", p)

	if strings.Contains(p.Path, "config.json") || strings.Contains(p.Path, "config.dev.json") {
		return "禁止发送config文件", errToolExecute
	}

	data, err := os.ReadFile(p.Path)
	if err != nil {
		return fmt.Sprintf("读取文件失败: %v", err), errToolExecute
	}
	_, err = callbacks.SendFile(p.Name, base64.StdEncoding.EncodeToString(data))
	if err != nil {
		return "发送失败", errToolExecute
	}
	return "发送成功", nil
}

// SendURLFileTool 从 URL 发送文件（直接传递 URL，由框架处理）
type SendURLFileParams struct {
	URL string `json:"url" desc:"文件的URL地址"`
}

type SendURLFileTool struct {
	llmtool.BaseTool[SendURLFileParams]
}

func NewSendURLFileTool() *SendURLFileTool {
	return &SendURLFileTool{
		BaseTool: llmtool.MakeBaseTool("urlfile", "用于通过URL发送文件给用户", SendURLFileParams{}),
	}
}

func (t *SendURLFileTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*SendURLFileParams)
	log.Println("执行urlfile... 参数:", p)

	_, err := callbacks.SendFileURL(p.URL)
	if err != nil {
		return "发送失败", errToolExecute
	}
	return "发送成功", nil
}
