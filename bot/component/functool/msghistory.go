package functool

import (
	"context"
	"fmt"
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type MsgHistoryParams struct {
	Count int `json:"count" jsonschema:"description=要获取的历史消息数量,default=10"`
}

type MsgHistoryTool struct {
	llmtool.BaseTool[MsgHistoryParams]
}

func NewMsgHistoryTool() *MsgHistoryTool {
	return &MsgHistoryTool{
		BaseTool: llmtool.MakeBaseTool("get_msg_history", "获取当前会话（群聊或好友）的历史消息记录", MsgHistoryParams{}),
	}
}

func (t *MsgHistoryTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*MsgHistoryParams)
	count := p.Count
	if count <= 0 {
		count = 10
	}
	log.Printf("执行get_msg_history, count=%d", count)

	if callbacks.GetMsgHistory == nil {
		return "", fmt.Errorf("获取历史消息功能不可用")
	}
	return callbacks.GetMsgHistory(count)
}
