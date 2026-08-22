package functool

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// LoadImagesParams load_images 工具参数。
type LoadImagesParams struct {
	// Hashes 要加载的图片哈希列表，取自消息文本中的 [图片 <hash> url:<url>] 标记，
	// 例如 ["a1b2c3d4","e5f6a7b8"]；当前消息、get_msg_history 历史记录与合并转发
	// 中展示的图片哈希均可加载。
	Hashes []string `json:"hashes" desc:"要加载的图片哈希列表，取自消息中的 [图片 <hash> url:<url>] 标记"`
}

type LoadImagesTool struct {
	llmtool.BaseTool[LoadImagesParams]
}

func NewLoadImagesTool() *LoadImagesTool {
	return &LoadImagesTool{
		BaseTool: llmtool.MakeBaseTool(
			"load_images",
			"按需加载指定哈希的图片查看内容。消息中的图片以 [图片 <hash> url:<url>] 标识（当前消息、get_msg_history 历史记录、合并转发里的图片都带该标识）。仅当理解或回答用户问题确实需要查看图片内容时调用，通过 hashes 参数传入需要查看的图片哈希，每次只加载需要的那几张，不要一次性全部加载。",
			LoadImagesParams{},
		),
	}
}

func (t *LoadImagesTool) Execute(_ context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*LoadImagesParams)
	if callbacks.LoadImages == nil {
		return "当前会话不支持加载图片", nil
	}
	result, err := callbacks.LoadImages(p.Hashes)
	if err != nil {
		return "", fmt.Errorf("加载图片失败: %w", err)
	}
	return result, nil
}
