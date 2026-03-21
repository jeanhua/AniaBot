package functool

import (
	"context"
	"fmt"
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

type MsgHistoryParams struct {
	Count      int `json:"count" jsonschema:"description=要获取的历史消息数量，默认10条"`
	MessageSeq int `json:"message_seq,omitempty"  jsonschema:"description=翻页游标，填写某条消息的message_seq，即可获取该条消息之前的历史记录，不填默认从最新消息开始"`
}

type MsgHistoryTool struct {
	llmtool.BaseTool[MsgHistoryParams]
}

func NewMsgHistoryTool() *MsgHistoryTool {
	return &MsgHistoryTool{
		BaseTool: llmtool.MakeBaseTool("get_msg_history", "获取当前会话（群聊或好友）的历史消息记录。返回结果中每条消息包含message_seq字段，若需要查看更早的消息，将第一条消息的message_seq作为下次调用的message_seq参数传入即可翻页", MsgHistoryParams{}),
	}
}

func (t *MsgHistoryTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	p := params.(*MsgHistoryParams)
	count := p.Count
	if count <= 0 {
		count = 10
	}
	log.Printf("执行get_msg_history, count=%d, seq=%d", count, p.MessageSeq)

	if callbacks.GetMsgHistory == nil {
		return "", fmt.Errorf("获取历史消息功能不可用")
	}
	return callbacks.GetMsgHistory(count, p.MessageSeq)
}
