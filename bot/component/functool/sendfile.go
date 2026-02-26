package functool

import (
	"encoding/json"
	"log"

	"github.com/jeanhua/AniaBot/bot/utils"
	"github.com/tmc/langchaingo/llms"
)

type sendFileParam struct {
	Name    string `json:"name" desc:"要发送的文件名(包括后缀)"`
	Content string `json:"content" desc:"文件的内容"`
}

const (
	FILE_TOOL_NAME = "file"
)

func MakeFileTool() llms.Tool {
	return utils.StructToOpenAITool("file", "用于发送文件", sendFileParam{})
}

func TryHandleFileTool(call llms.ToolCall, msgFuncs OptionFuncs) (string, error) {
	log.Println("执行file... 参数:", call.FunctionCall.Arguments)
	var param = sendFileParam{}
	if err := json.Unmarshal([]byte(call.FunctionCall.Arguments), &param); err != nil {
		return "发送失败", err
	}
	if !msgFuncs.SendFile(param.Name, param.Content) {
		return "发送失败", ToolExecuteError
	}
	return "发送成功", nil
}
