package llmtool

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// SkillMeta 是从 SKILL.md frontmatter 中解析的元数据
type SkillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// Skill 代表一个已加载的 skill
type Skill struct {
	Meta       SkillMeta
	Content    string            // SKILL.md 的完整内容（含 frontmatter）
	Path       string            // SKILL.md 文件路径
	DirPath    string            // skill 所在目录路径
	References map[string]string // 引用文档：文件名 -> 内容（如 reference.md）
	ExtraFiles map[string]string // 附带文件：文件名 -> 绝对路径（如 script.sh）
}

// SkillManager 管理所有可用的 Skill
type SkillManager struct {
	mu     sync.RWMutex
	skills map[string]*Skill // key: skill name
}

// NewSkillManager 创建一个新的 SkillManager
func NewSkillManager() *SkillManager {
	return &SkillManager{
		skills: make(map[string]*Skill),
	}
}

// LoadFromDir 从指定目录扫描并加载所有 skill
// 支持两种目录结构：
//   - skillsDir/skill-name/SKILL.md （可包含附属文件如 reference.md、script.sh 等）
//   - skillsDir/SKILL.md （单文件模式）
func (m *SkillManager) LoadFromDir(skillsDir string) error {
	return m.LoadFromDirWithFilter(skillsDir, nil)
}

// LoadFromDirWithFilter 从指定目录加载 skill，只加载 names 中指定的 skill
// names 为空时等同于 LoadFromDir（加载全部）
func (m *SkillManager) LoadFromDirWithFilter(skillsDir string, names []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadFromDirLocked(skillsDir, names)
}

// loadFromDirLocked 是 LoadFromDirWithFilter 的无锁内部实现
func (m *SkillManager) loadFromDirLocked(skillsDir string, names []string) error {
	var filter map[string]struct{}
	if len(names) > 0 {
		filter = make(map[string]struct{}, len(names))
		for _, n := range names {
			filter[n] = struct{}{}
		}
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return fmt.Errorf("读取 skills 目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 子目录模式：加载 SKILL.md 和附属文件
			skillDir := filepath.Join(skillsDir, entry.Name())
			skillPath := filepath.Join(skillDir, "SKILL.md")

			if _, err := os.Stat(skillPath); os.IsNotExist(err) {
				continue
			}

			skill, err := loadSkillFromDir(skillPath, skillDir)
			if err != nil {
				return fmt.Errorf("加载 skill 失败 [%s]: %w", skillPath, err)
			}

			if filter == nil {
				m.skills[skill.Meta.Name] = skill
			} else if _, ok := filter[skill.Meta.Name]; ok {
				m.skills[skill.Meta.Name] = skill
			}
		} else if strings.ToUpper(entry.Name()) == "SKILL.MD" {
			// 单文件模式：skillsDir/SKILL.md
			skillPath := filepath.Join(skillsDir, entry.Name())

			skill, err := loadSkillFromFile(skillPath)
			if err != nil {
				return fmt.Errorf("加载 skill 失败 [%s]: %w", skillPath, err)
			}

			if filter == nil {
				m.skills[skill.Meta.Name] = skill
			} else if _, ok := filter[skill.Meta.Name]; ok {
				m.skills[skill.Meta.Name] = skill
			}
		}
	}

	return nil
}

// Reload 重新从磁盘加载 skill 并原子替换当前注册表（用于面板上传/删除后热更新）
func (m *SkillManager) Reload(skillsDir string, names []string) error {
	tmp := NewSkillManager()
	if err := tmp.loadFromDirLocked(skillsDir, names); err != nil {
		return err
	}
	m.mu.Lock()
	m.skills = tmp.skills
	m.mu.Unlock()
	return nil
}

// Register 手动注册一个 Skill（直接传入路径）
func (m *SkillManager) Register(skillPath string) error {
	skill, err := loadSkillFromFile(skillPath)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.skills[skill.Meta.Name] = skill
	m.mu.Unlock()
	return nil
}

// Get 获取指定名称的 skill
func (m *SkillManager) Get(name string) (*Skill, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.skills[name]
	return s, ok
}

// List 返回所有 skill 的元数据列表
func (m *SkillManager) List() []*SkillMeta {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SkillMeta, 0, len(m.skills))
	for _, s := range m.skills {
		meta := s.Meta // 复制，避免外部修改
		result = append(result, &meta)
	}
	return result
}

// BuildAvailableSkillsPrompt 生成注入 system prompt 的 <skills_registry> 块
func (m *SkillManager) BuildAvailableSkillsPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n<agent_skills_registry>\n")
	sb.WriteString("  <instruction>\n")
	sb.WriteString("    The following skills define your operational logic and constraints.\n")
	sb.WriteString("    BEFORE invoking any tool, you MUST match the intent against these skills first.\n")
	sb.WriteString("  </instruction>\n\n")

	for _, s := range m.skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", s.Meta.Name))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", s.Meta.Description))
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</agent_skills_registry>\n")
	return sb.String()
}

// RegisterToExecuter 将 skill 读取工具注册到 ToolExecuter
// 调用此方法后，Agent 就拥有了读取 SKILL.md 的能力
func (m *SkillManager) RegisterToExecuter(executer *ToolExecuter) {
	executer.Register(NewSkillReadTool(m))
}

// ValidateSkillDir 校验指定目录下的 SKILL.md（含附属文件）能否被正常加载
func ValidateSkillDir(skillDir string) error {
	_, err := loadSkillFromDir(filepath.Join(skillDir, "SKILL.md"), skillDir)
	return err
}

// loadSkillFromFile 从 SKILL.md 文件路径解析 Skill（单文件模式，无附属文件）
func loadSkillFromFile(path string) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)
	meta, err := parseSkillFrontmatter(content)
	if err != nil {
		return nil, fmt.Errorf("解析 frontmatter 失败: %w", err)
	}

	if meta.Name == "" {
		// 回退：使用目录名作为 skill name
		meta.Name = filepath.Base(filepath.Dir(path))
	}

	return &Skill{
		Meta:       *meta,
		Content:    content,
		Path:       path,
		DirPath:    filepath.Dir(path),
		References: make(map[string]string),
		ExtraFiles: make(map[string]string),
	}, nil
}

// loadSkillFromDir 从子目录加载 SKILL.md 和所有附属文件（递归扫描子目录）
func loadSkillFromDir(skillPath, skillDir string) (*Skill, error) {
	skill, err := loadSkillFromFile(skillPath)
	if err != nil {
		return nil, err
	}

	// 递归扫描目录下的附属文件（跳过 SKILL.md 本身）
	err = filepath.Walk(skillDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}

		// 跳过目录本身和 SKILL.md
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(info.Name(), "SKILL.MD") && filepath.Dir(path) == skillDir {
			return nil
		}

		// 计算相对于 skillDir 的路径作为 key
		relPath, err := filepath.Rel(skillDir, path)
		if err != nil {
			return nil
		}
		// 统一使用正斜杠
		relPath = filepath.ToSlash(relPath)

		absPath, _ := filepath.Abs(path)
		if isMarkdownFile(info.Name()) {
			data, err := os.ReadFile(absPath)
			if err != nil {
				log.Println("读取Skill附属md文件失败: ", absPath)
				return nil
			}
			skill.References[relPath] = string(data)
		} else {
			skill.ExtraFiles[relPath] = absPath
		}

		return nil
	})
	if err != nil {
		return skill, nil // 遍历失败仍返回已加载的主文件
	}

	return skill, nil
}

// isMarkdownFile 根据扩展名判断是否为 Markdown 文件
func isMarkdownFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// parseSkillFrontmatter 解析 SKILL.md 中的 YAML frontmatter（--- 块）
func parseSkillFrontmatter(content string) (*SkillMeta, error) {
	// 检查是否以 --- 开头
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return &SkillMeta{}, nil
	}

	// 找到 frontmatter 结束位置
	trimmed := strings.TrimSpace(content)
	// 跳过第一个 ---
	rest := trimmed[3:]
	before, _, ok := strings.Cut(rest, "---")
	if !ok {
		return &SkillMeta{}, nil
	}

	yamlContent := before
	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(yamlContent), &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// ─────────────────────────────────────────────
// SkillReadTool：让 Agent 调用来读取 SKILL.md
// ─────────────────────────────────────────────

// SkillReadParams 是调用 skill_read 工具时的参数
type SkillReadParams struct {
	SkillName string `json:"skill_name" desc:"要读取的 skill 名称，从 available_skills 列表中选择"`
	FileName  string `json:"file_name,omitempty" desc:"可选，skill 目录下的附属文件名（如 reference.md）。不填则返回 SKILL.md 主文件及其附属文件列表"`
}

// SkillReadTool 工具：读取指定 skill 的完整 SKILL.md 内容
type SkillReadTool struct {
	BaseTool[SkillReadParams]
	manager *SkillManager
}

// NewSkillReadTool 创建 SkillReadTool
func NewSkillReadTool(manager *SkillManager) *SkillReadTool {
	return &SkillReadTool{
		BaseTool: MakeBaseTool(
			"skill_read",
			"读取指定 skill 的详细指令内容。当你需要使用某个 skill 时，先调用此工具获取其完整指令，再按照指令执行任务。参数 skill_name 从 <available_skills> 列表中选择。如需读取附属文档（如 reference.md），传入 file_name 参数。skill 中的脚本文件（.sh/.py等）可直接通过 bash 工具执行，无需先读取。",
			SkillReadParams{},
		),
		manager: manager,
	}
}

func (t *SkillReadTool) Execute(ctx context.Context, params any, callbacks CallBackFuncs) (string, error) {
	p := params.(*SkillReadParams)

	if p.SkillName == "" {
		// 没有指定名称时，返回所有可用 skill 列表
		metas := t.manager.List()
		if len(metas) == 0 {
			return "当前没有可用的 skill。", nil
		}
		var sb strings.Builder
		sb.WriteString("可用的 skill 列表：\n")
		for _, m := range metas {
			sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Name, m.Description))
		}
		return sb.String(), nil
	}

	log.Println("[SKILL查询] 正在查询", p.SkillName)
	skill, ok := t.manager.Get(p.SkillName)
	if !ok {
		// 返回可用列表，便于 Agent 纠正
		metas := t.manager.List()
		names := make([]string, 0, len(metas))
		for _, m := range metas {
			names = append(names, m.Name)
		}
		return "", fmt.Errorf("skill '%s' 不存在，可用的 skill: [%s]",
			p.SkillName, strings.Join(names, ", "))
	}

	// 请求特定附属文件
	if p.FileName != "" {
		content, exists := skill.References[p.FileName]
		if !exists {
			available := make([]string, 0, len(skill.References))
			for name := range skill.References {
				available = append(available, name)
			}
			if len(available) == 0 {
				return "", fmt.Errorf("skill '%s' 没有附属文件", p.SkillName)
			}
			return "", fmt.Errorf("skill '%s' 中不存在文件 '%s'，可用文件: [%s]",
				p.SkillName, p.FileName, strings.Join(available, ", "))
		}
		return content, nil
	}

	// 返回 SKILL.md 内容，附带附属文件/脚本列表
	result := skill.Content
	if len(skill.References) > 0 || len(skill.ExtraFiles) > 0 {
		var sb strings.Builder
		sb.WriteString(result)
		sb.WriteString("\n\n---\n\n")
		if len(skill.References) > 0 {
			sb.WriteString("## [reference文档]\n\n")
			sb.WriteString("通过 skill_read 的 file_name 参数按需读取：\n")
			for name := range skill.References {
				sb.WriteString(fmt.Sprintf("- %s\n", name))
			}
		}
		if len(skill.ExtraFiles) > 0 {
			sb.WriteString("\n## [附带文件]\n\n")
			sb.WriteString("无需读取，可直接通过 bash 工具直接执行：\n")
			for name, path := range skill.ExtraFiles {
				sb.WriteString(fmt.Sprintf("- %s → %s\n", name, path))
			}
		}
		result = sb.String()
	}

	return result, nil
}
