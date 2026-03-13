package functool

import (
	"log"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
)

// CreateToolsWithSkill 创建支持 Agent Skill 的工具执行器
// skillsDir: skills 目录路径，如 "./skills"（为空则不加载 skill）
func CreateToolsWithSkill(searchToken string, mcpConfigs []*llmtool.MCPConfig, skillsDir string) (*llmtool.ToolExecuter, *llmtool.SkillManager) {
	executer := CreateDefaultTools(searchToken)

	// 注册 MCP 工具
	if len(mcpConfigs) > 0 {
		if err := RegisterMCPFromConfig(executer, mcpConfigs); err != nil {
			log.Printf("注册MCP工具时出错: %v", err)
		}
	}

	// 注册 Skill 工具
	skillManager := llmtool.NewSkillManager()
	if skillsDir != "" {
		if err := skillManager.LoadFromDir(skillsDir); err != nil {
			log.Printf("加载 skill 目录失败 [%s]: %v", skillsDir, err)
		} else {
			metas := skillManager.List()
			log.Printf("已加载 %d 个 skill", len(metas))
			for _, m := range metas {
				log.Printf("  - skill: %s (%s)", m.Name, m.Description)
			}
		}
	}

	// 将 skill_read 工具注册到执行器
	skillManager.RegisterToExecuter(executer)

	return executer, skillManager
}
