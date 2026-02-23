package functool

import (
	"log"

	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/tmc/langchaingo/llms"
)

type timeParam struct{}

const (
	TIME_TOOL_NAME = "time"
)

func MakeTimeTool() []llms.Tool {
	return []llms.Tool{
		utils.StructToOpenAITool("time", "用于获取当前时间", timeParam{}),
	}
}

func TryHandleTimeCall(call llms.ToolCall) (string, error) {
	log.Println("执行time...")
	return utils.GetFormattedTime(), nil
}
