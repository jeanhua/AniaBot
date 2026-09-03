package marketplace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// readmeCacheTTL 插件详情 README 的短期磁盘缓存时长：面板反复点开同一插件
// 详情时直接命中缓存，避免每次请求 GitHub（慢且消耗 API 配额）。
const readmeCacheTTL = 10 * time.Minute

// indexCacheFile 市场索引的本地缓存文件名。
const indexCacheFile = "index.json"

// cachedReadme 单插件 README 缓存文件内容。
type cachedReadme struct {
	Content string `json:"content"`
}

// indexCachePath 返回市场索引本地缓存路径。
func (s *Service) indexCachePath() string {
	return filepath.Join(s.cacheDir(), indexCacheFile)
}

// readmeCachePath 返回单插件 README 缓存路径（插件 ID 已通过格式校验，可安全作为文件名）。
func (s *Service) readmeCachePath(id string) string {
	return filepath.Join(s.cacheDir(), "readme", id+".json")
}

// loadCachedReadme 读取未过期的 README 缓存；ok=true 表示命中（内容可能为空串）。
func (s *Service) loadCachedReadme(id string) (content string, ok bool) {
	path := s.readmeCachePath(id)
	st, err := os.Stat(path)
	if err != nil || time.Since(st.ModTime()) > readmeCacheTTL {
		_ = os.Remove(path)
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var c cachedReadme
	if json.Unmarshal(data, &c) != nil {
		_ = os.Remove(path)
		return "", false
	}
	return c.Content, true
}

// saveCachedReadme 写入单插件 README 缓存（临时文件 + 改名，避免读到半截内容）。
func (s *Service) saveCachedReadme(id, content string) {
	path := s.readmeCachePath(id)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, mustJSON(cachedReadme{Content: content}), 0o644); err != nil {
		return
	}
	_ = os.Remove(path)
	_ = os.Rename(tmp, path)
}

// clearReadmeCache 清除全部 README 缓存（索引强制刷新后调用，详情跟随最新版本）。
func (s *Service) clearReadmeCache() {
	_ = os.RemoveAll(filepath.Join(s.cacheDir(), "readme"))
}
