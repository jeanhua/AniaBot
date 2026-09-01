package marketplace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// InstalledPlugin 已安装插件记录。
type InstalledPlugin struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	InstalledAt string `json:"installed_at"`
}

// InstalledManifest 本地已安装插件清单（plugin_dir/manifest.json）。
type InstalledManifest struct {
	Plugins []InstalledPlugin `json:"plugins"`
}

// manifestStore 已安装清单的读写（磁盘 JSON，持久卷），带内存缓存与备份。
type manifestStore struct {
	mu      sync.Mutex
	path    string
	plugins []InstalledPlugin
	loaded  bool
}

func newManifestStore(pluginDir string) *manifestStore {
	return &manifestStore{path: filepath.Join(pluginDir, "manifest.json")}
}

func (m *manifestStore) load() ([]InstalledPlugin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return m.plugins, nil
	}
	m.plugins = nil
	if data, err := os.ReadFile(m.path); err == nil {
		var im InstalledManifest
		if err := json.Unmarshal(data, &im); err != nil {
			return nil, fmt.Errorf("解析已安装插件清单失败 %s: %w", m.path, err)
		}
		m.plugins = im.Plugins
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	m.loaded = true
	return m.plugins, nil
}

// save 写入清单，并保留一份 manifest.json.bak 供回滚恢复。
func (m *manifestStore) save(plugins []InstalledPlugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	_ = os.Remove(m.path + ".bak")
	_ = os.Rename(m.path, m.path+".bak") // 首次写入时不存在，忽略
	data, err := json.MarshalIndent(InstalledManifest{Plugins: plugins}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return err
	}
	m.plugins = plugins
	m.loaded = true
	return nil
}

// restoreBackup 从 manifest.json.bak 恢复清单（回滚时调用）。
func (m *manifestStore) restoreBackup() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(m.path + ".bak")
	if err != nil {
		return fmt.Errorf("没有可回滚的插件清单备份: %w", err)
	}
	var im InstalledManifest
	if err := json.Unmarshal(data, &im); err != nil {
		return err
	}
	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return err
	}
	m.plugins = im.Plugins
	m.loaded = true
	return nil
}

func (m *manifestStore) find(id string) (InstalledPlugin, bool) {
	plugins, err := m.load()
	if err != nil {
		return InstalledPlugin{}, false
	}
	for _, p := range plugins {
		if p.ID == id {
			return p, true
		}
	}
	return InstalledPlugin{}, false
}

func (m *manifestStore) set(p InstalledPlugin) error {
	plugins, err := m.load()
	if err != nil {
		plugins = nil
	}
	replaced := false
	for i := range plugins {
		if plugins[i].ID == p.ID {
			plugins[i] = p
			replaced = true
			break
		}
	}
	if !replaced {
		plugins = append(plugins, p)
	}
	return m.save(plugins)
}

func (m *manifestStore) remove(id string) error {
	plugins, err := m.load()
	if err != nil {
		return err
	}
	out := plugins[:0]
	for _, p := range plugins {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return m.save(out)
}

func (m *manifestStore) all() []InstalledPlugin {
	plugins, _ := m.load()
	return plugins
}

// nowStamp 安装时间戳（固定宽度 UTC，文本序 = 时间序）。
func nowStamp() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
