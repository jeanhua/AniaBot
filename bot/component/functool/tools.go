package functool

import (
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// createTools 创建所有工具并注册到执行器
func createTools(executer *llmtool.ToolExecuter, searchToken string) {
	executer.Register(NewTimeTool())
	executer.Register(NewWebSearchTool(searchToken))
	executer.Register(NewWebExploreTool(searchToken))
	executer.Register(NewMemeTool())
	executer.Register(NewSendFileTool())
}

// CreateDefaultTools 创建默认的工具执行器并注册所有工具
func CreateDefaultTools(searchToken string) *llmtool.ToolExecuter {
	executer := llmtool.NewToolExecuter()
	createTools(executer, searchToken)
	return executer
}

// RegisterMCPFromConfig 从配置注册MCP工具到执行器（使用工具发现模式，避免上下文爆炸）
func RegisterMCPFromConfig(executer *llmtool.ToolExecuter, configs []*llmtool.MCPConfig) error {
	for _, config := range configs {
		log.Printf("正在连接MCP服务器: %s (command=%s)", config.Name, config.Command)

		// 使用工具发现模式注册（推荐）
		if err := executer.RegisterMCPWithConfigDiscovery(config); err != nil {
			log.Printf("注册MCP服务器 %s 失败: %v", config.Name, err)
			continue
		}

		log.Printf("成功注册MCP服务器: %s (工具发现模式)", config.Name)
	}
	return nil
}

// RegisterMCPFromConfigTraditional 从配置注册MCP工具到执行器（传统模式，会导致上下文爆炸）
// 仅在工具数量很少（<5个）时使用
func RegisterMCPFromConfigTraditional(executer *llmtool.ToolExecuter, configs []*llmtool.MCPConfig) error {
	for _, config := range configs {
		log.Printf("正在连接MCP服务器: %s (command=%s)", config.Name, config.Command)

		// 传统方式：一次性注册所有工具
		if err := executer.RegisterMCPWithConfig(config); err != nil {
			log.Printf("注册MCP服务器 %s 失败: %v", config.Name, err)
			continue
		}

		log.Printf("成功注册MCP服务器: %s (传统模式)", config.Name)
	}
	return nil
}

// CreateToolsWithMCP 创建工具执行器并注册本地工具和MCP工具
func CreateToolsWithMCP(searchToken string, mcpConfigs []*llmtool.MCPConfig) *llmtool.ToolExecuter {
	executer := CreateDefaultTools(searchToken)

	if len(mcpConfigs) > 0 {
		if err := RegisterMCPFromConfig(executer, mcpConfigs); err != nil {
			log.Printf("注册MCP工具时出错: %v", err)
		}
	}

	return executer
}
