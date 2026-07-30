package functool

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type LoadImagesParams struct{}

type LoadImagesTool struct {
	llmtool.BaseTool[LoadImagesParams]
}

func NewLoadImagesTool() *LoadImagesTool {
	return &LoadImagesTool{
		BaseTool: llmtool.MakeBaseTool(
			"load_images",
			"按需加载用户当前消息及其引用消息中的图片。仅当理解或回答用户问题确实需要查看图片内容时调用；不要在每次出现图片时自动调用。每张图片以 [图片 <hash>] 形式的哈希标识区分，加载结果中的标识与消息里的标记一一对应。",
			LoadImagesParams{},
		),
	}
}

func (t *LoadImagesTool) Execute(_ context.Context, _ any, callbacks llmtool.CallBackFuncs) (string, error) {
	if callbacks.LoadImages == nil {
		return "当前会话不支持加载图片", nil
	}
	result, err := callbacks.LoadImages()
	if err != nil {
		return "", fmt.Errorf("加载图片失败: %w", err)
	}
	return result, nil
}
