package functool

import (
	"context"
	"fmt"
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/bot/utils"
)

type TimeParams struct{}

type TimeTool struct {
	llmtool.BaseTool[TimeParams]
}

func NewTimeTool() *TimeTool {
	return &TimeTool{
		BaseTool: llmtool.MakeBaseTool("time", "用于获取当前时间", TimeParams{}),
	}
}

func (t *TimeTool) Execute(ctx context.Context, params any, callbacks llmtool.CallBackFuncs) (string, error) {
	log.Println("执行time...")
	currentTime := utils.GetFormattedTime()
	return fmt.Sprintf("当前时间是: %s", currentTime), nil
}
