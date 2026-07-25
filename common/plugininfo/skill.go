package plugininfo

// SkillInfo 是面板展示的 skill 元信息
type SkillInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    string   `json:"location"` // skill 在 skills 目录下的相对位置（目录名或文件名）
	Refs        []string `json:"refs"`     // 附属 Markdown 文档
	Extras      []string `json:"extras"`   // 其他附带文件（脚本等）
}
