package pluginaichat

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jeanhua/AniaBot/bot/component/llmtool"
	"github.com/jeanhua/AniaBot/common/plugininfo"
)

// skill 面板管理接口（实现 adminpanel.SkillSource）。
// 上传/删除后立即从磁盘热重载 skill 注册表，无需重启；
// 若配置了 skills 白名单，未列入白名单的 skill 不会加载。

// 单个 skill 解压后的体积与文件数上限
const (
	skillMaxTotalBytes = 64 << 20 // 64 MiB
	skillMaxFiles      = 500
)

// SkillList 返回当前已加载的 skill 列表、skills 目录与白名单（供 Web 面板展示）。
func (p *AIChatPlugin) SkillList() ([]plugininfo.SkillInfo, string, []string) {
	if p.skillManager == nil {
		return nil, p.skillsDir, p.cfg.Skills
	}
	metas := p.skillManager.List()
	infos := make([]plugininfo.SkillInfo, 0, len(metas))
	for _, meta := range metas {
		info := plugininfo.SkillInfo{
			Name:        meta.Name,
			Description: meta.Description,
		}
		if skill, ok := p.skillManager.Get(meta.Name); ok {
			info.Location = skillLocation(p.skillsDir, skill.Path)
			for name := range skill.References {
				info.Refs = append(info.Refs, name)
			}
			for name := range skill.ExtraFiles {
				info.Extras = append(info.Extras, name)
			}
			sort.Strings(info.Refs)
			sort.Strings(info.Extras)
		}
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos, p.skillsDir, p.cfg.Skills
}

// SkillDelete 按名称删除 skill：从磁盘移除对应目录/文件后热重载注册表。
func (p *AIChatPlugin) SkillDelete(name string) error {
	if p.skillManager == nil {
		return fmt.Errorf("skill 功能未初始化")
	}
	skill, ok := p.skillManager.Get(name)
	if !ok {
		return fmt.Errorf("skill '%s' 不存在", name)
	}

	skillDirAbs, err1 := filepath.Abs(filepath.Dir(skill.Path))
	skillsDirAbs, err2 := filepath.Abs(p.skillsDir)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("解析路径失败")
	}

	var err error
	if skillDirAbs == skillsDirAbs {
		// 单文件模式：skillsDir/SKILL.md，只删除文件本身
		err = os.Remove(skill.Path)
	} else {
		// 目录模式：删除整个 skill 目录
		err = os.RemoveAll(filepath.Dir(skill.Path))
	}
	if err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}

	if err := p.skillManager.Reload(p.skillsDir, p.cfg.Skills); err != nil {
		return fmt.Errorf("已删除，但重载 skill 失败: %w", err)
	}
	p.Logger.Info("skill 已删除", "skill", name)
	return nil
}

// SkillUpload 从 zip 压缩包内容安装 skill 并热重载。
// zip 结构支持两种形式：
//   - 根目录直接包含 SKILL.md（以压缩包文件名作为 skill 目录名）
//   - 单一顶层目录，其中包含 SKILL.md（以顶层目录名作为 skill 目录名）
func (p *AIChatPlugin) SkillUpload(filename string, data []byte) error {
	if p.skillManager == nil {
		return fmt.Errorf("skill 功能未初始化")
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("无法读取压缩包（请上传 zip 格式）: %w", err)
	}
	if len(zr.File) > skillMaxFiles {
		return fmt.Errorf("压缩包内文件过多（上限 %d 个）", skillMaxFiles)
	}

	// 检测目录结构：剥离的顶层前缀与目标目录名
	prefix, dirName, err := detectSkillRoot(zr, filename)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(p.skillsDir, 0o755); err != nil {
		return fmt.Errorf("创建 skills 目录失败: %w", err)
	}

	// 先解压到临时目录，校验通过后再替换目标目录
	tmpDir, err := os.MkdirTemp(p.skillsDir, ".upload-*")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractSkillZip(zr, prefix, tmpDir); err != nil {
		return err
	}

	if err := llmtool.ValidateSkillDir(tmpDir); err != nil {
		return fmt.Errorf("SKILL.md 校验失败: %w", err)
	}

	target := filepath.Join(p.skillsDir, dirName)
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("清理旧版本失败: %w", err)
	}
	if err := os.Rename(tmpDir, target); err != nil {
		return fmt.Errorf("安装失败: %w", err)
	}

	if err := p.skillManager.Reload(p.skillsDir, p.cfg.Skills); err != nil {
		return fmt.Errorf("已安装，但重载 skill 失败: %w", err)
	}
	p.Logger.Info("skill 已上传", "dir", dirName, "file", filename)
	return nil
}

// detectSkillRoot 判断 zip 内 skill 的顶层前缀，返回 (前缀, skill 目录名)。
func detectSkillRoot(zr *zip.Reader, filename string) (string, string, error) {
	hasRootSkill := false
	topDirs := make(map[string]struct{})
	topDirHasSkill := make(map[string]bool)

	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if f.FileInfo().IsDir() || name == "" {
			continue
		}
		parts := strings.Split(name, "/")
		for _, part := range parts {
			if part == ".." {
				return "", "", fmt.Errorf("压缩包包含非法路径: %s", f.Name)
			}
		}
		if len(parts) == 1 {
			if strings.EqualFold(parts[0], "SKILL.md") {
				hasRootSkill = true
			}
		} else {
			top := parts[0]
			topDirs[top] = struct{}{}
			if len(parts) == 2 && strings.EqualFold(parts[1], "SKILL.md") {
				topDirHasSkill[top] = true
			}
		}
	}

	// 形式一：根目录直接包含 SKILL.md
	if hasRootSkill {
		dirName := sanitizeSkillDirName(strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)))
		if dirName == "" {
			dirName = "skill"
		}
		return "", dirName, nil
	}

	// 形式二：单一顶层目录包含 SKILL.md
	if len(topDirs) == 1 {
		for top := range topDirs {
			if topDirHasSkill[top] {
				dirName := sanitizeSkillDirName(top)
				if dirName == "" {
					return "", "", fmt.Errorf("顶层目录名不合法: %s", top)
				}
				return top + "/", dirName, nil
			}
		}
	}

	return "", "", fmt.Errorf("压缩包中未找到 SKILL.md（应位于根目录或单一顶层目录下）")
}

// sanitizeSkillDirName 校验/清洗 skill 目录名（禁止路径分隔符与 ..）。
func sanitizeSkillDirName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return ""
	}
	if strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}

// extractSkillZip 将 zip 中 prefix 前缀下的文件解压到 dstDir，带 zip-slip 与体积防护。
func extractSkillZip(zr *zip.Reader, prefix, dstDir string) error {
	var total int64
	count := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if prefix != "" && !strings.HasPrefix(name, prefix) {
			continue
		}
		rel := strings.TrimPrefix(name, prefix)
		if rel == "" {
			continue
		}
		// 防御：拒绝绝对路径与 ..
		clean := path.Clean(rel)
		if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("压缩包包含非法路径: %s", f.Name)
		}

		total += int64(f.UncompressedSize64)
		if total > skillMaxTotalBytes {
			return fmt.Errorf("解压后体积超过上限（%d MB）", skillMaxTotalBytes>>20)
		}
		count++

		target := filepath.Join(dstDir, filepath.FromSlash(clean))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("创建目录失败: %w", err)
		}
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("读取压缩包文件失败: %w", err)
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode())
		if err != nil {
			rc.Close()
			return fmt.Errorf("写入文件失败: %w", err)
		}
		_, copyErr := io.Copy(out, rc)
		closeErr := out.Close()
		rc.Close()
		if copyErr != nil {
			return fmt.Errorf("解压文件失败: %w", copyErr)
		}
		if closeErr != nil {
			return fmt.Errorf("写入文件失败: %w", closeErr)
		}
	}
	if count == 0 {
		return fmt.Errorf("压缩包内容为空")
	}
	return nil
}

// skillLocation 返回 skill 主文件相对于 skillsDir 的展示路径。
func skillLocation(skillsDir, skillPath string) string {
	rel, err := filepath.Rel(skillsDir, skillPath)
	if err != nil {
		return skillPath
	}
	return filepath.ToSlash(rel)
}
