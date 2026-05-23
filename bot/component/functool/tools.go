package functool

import (
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// CreateDefaultTools 创建默认的工具执行器并注册所有内置工具
func CreateDefaultTools(searchToken string, bashConfig BashConfig) (*llmtool.ToolExecuter, error) {
	executer := llmtool.NewToolExecuter()
	executer.Register(NewTimeTool())
	executer.Register(NewWebSearchTool(searchToken))
	executer.Register(NewWebExploreTool(searchToken))
	executer.Register(NewMemeTool())
	executer.Register(NewSendFileTool())
	executer.Register(NewSendURLFileTool())
	executer.Register(NewMsgHistoryTool())
	if bashConfig.Enable {
		bashTool, err := NewBashTool(bashConfig)
		if err != nil {
			return nil, err
		}
		executer.Register(bashTool)
	}
	return executer, nil
}

// CreateToolsWithMCP 创建工具执行器，注册内置工具和 MCP 工具（工具发现模式）
func CreateToolsWithMCP(searchToken string, mcpConfigs []*llmtool.MCPConfig, bashConfig BashConfig) (*llmtool.ToolExecuter, error) {
	executer, err := CreateDefaultTools(searchToken, bashConfig)
	if err != nil {
		return nil, err
	}
	for _, config := range mcpConfigs {
		log.Printf("正在连接MCP服务器: %s (command=%s)", config.Name, config.Command)
		if err := executer.RegisterMCPWithConfigDiscovery(config); err != nil {
			log.Printf("注册MCP服务器 %s 失败: %v", config.Name, err)
			continue
		}
		log.Printf("成功注册MCP服务器: %s", config.Name)
	}
	return executer, nil
}

// CreateToolsWithSkill 创建工具执行器，注册内置工具、MCP 工具和 Skill 工具
// skillsDir 为空时跳过 skill 加载
// skills 非空时只加载指定名称的 skill，为空时加载全部
func CreateToolsWithSkill(searchToken string, mcpConfigs []*llmtool.MCPConfig, skillsDir string, bashConfig BashConfig, skills []string) (*llmtool.ToolExecuter, *llmtool.SkillManager, error) {
	executer, err := CreateToolsWithMCP(searchToken, mcpConfigs, bashConfig)
	if err != nil {
		return nil, nil, err
	}

	skillManager := llmtool.NewSkillManager()
	if skillsDir != "" {
		if err := skillManager.LoadFromDirWithFilter(skillsDir, skills); err != nil {
			log.Printf("加载 skill 目录失败 [%s]: %v", skillsDir, err)
		} else {
			for _, m := range skillManager.List() {
				log.Printf("已加载 skill: %s (%s)", m.Name, m.Description)
			}
		}
	}
	skillManager.RegisterToExecuter(executer)

	return executer, skillManager, nil
}
