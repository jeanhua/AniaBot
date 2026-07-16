package functool

import (
	"context"
	"fmt"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// LocalImageConfig local_image 工具配置
type LocalImageConfig struct {
	// Enable 是否启用本地图片读取工具。默认关闭：该工具可读取宿主机上任意可读
	// 图片文件并交给 LLM 查看，LLM（含被注入提示词的情形）可能借此获取敏感
	// 截图等内容，故需显式开启（与 file/bash 工具一致采用 opt-in）。
	Enable bool `json:"enable" mapstructure:"enable"`
}

// LoadLocalImageParams local_image 工具参数
type LoadLocalImageParams struct {
	Path string `json:"path" desc:"本地图片文件的完整路径,如 /tmp/cat.png"`
}

type LoadLocalImageTool struct {
	llmtool.BaseTool[LoadLocalImageParams]
}

func NewLoadLocalImageTool() *LoadLocalImageTool {
	return &LoadLocalImageTool{
		BaseTool: llmtool.MakeBaseTool(
			"local_image",
			"读取宿主机本地图片文件供查看。仅当用户明确要求查看某张本地图片（并给出路径）时调用；图片将在下一轮上下文中提供，请直接查看后回答。",
			LoadLocalImageParams{},
		),
	}
}

func (t *LoadLocalImageTool) Execute(_ context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*LoadLocalImageParams)
	if callbacks.LoadLocalImage == nil {
		return "当前会话不支持读取本地图片", nil
	}
	result, err := callbacks.LoadLocalImage(p.Path)
	if err != nil {
		return "", fmt.Errorf("读取本地图片失败: %w", err)
	}
	return result, nil
}
