package functool

import (
	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// CreateTools 创建所有工具并注册到执行器
func CreateTools(executer *llmtool.ToolExecuter, searchToken string) {
	executer.Register(NewTimeTool())
	executer.Register(NewWebSearchTool(searchToken))
	executer.Register(NewWebExploreTool(searchToken))
	executer.Register(NewMemeTool())
	executer.Register(NewSendFileTool())
}

// CreateDefaultTools 创建默认的工具执行器并注册所有工具
func CreateDefaultTools(searchToken string) *llmtool.ToolExecuter {
	executer := llmtool.NewToolExecuter()
	CreateTools(executer, searchToken)
	return executer
}
