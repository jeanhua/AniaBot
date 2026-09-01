package pluginaichat

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
)

// Prompt 覆盖管理器：按 files.prompt_json（面板「扩展配置」页编辑）为群聊/好友
// 覆盖系统提示词。5s TTL 重读配置中心，raw 变化才重新解析——面板保存后秒级
// 热生效，不再需要重启 Bot；消息路径上的读取有界。并发安全。
type promptOverrideManager struct {
	editor    plugin.ConfigEditor
	configKey string
	logger    *slog.Logger

	mu        sync.Mutex
	groups    map[message.QID]string
	friends   map[message.QID]string
	lastRaw   string
	lastCheck time.Time
}

func newPromptOverrideManager(editor plugin.ConfigEditor, configKey string, logger *slog.Logger) *promptOverrideManager {
	return &promptOverrideManager{
		editor:    editor,
		configKey: configKey,
		logger:    logger,
		groups:    map[message.QID]string{},
		friends:   map[message.QID]string{},
	}
}

// loadRaw 用给定原文初始化/覆盖内存快照（Start 时用 viper 快照兜底，
// 之后以配置中心 TTL 重读为准）。解析失败时保留原快照并记日志。
func (m *promptOverrideManager) loadRaw(raw string) {
	groups, friends, err := parsePromptOverrides(raw)
	if err != nil {
		m.logger.Warn("解析 Prompt 覆盖配置失败，沿用旧配置", "error", err.Error())
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups, m.friends = groups, friends
	m.lastRaw = strings.TrimSpace(raw)
	m.lastCheck = time.Now()
}

// get 按会话取覆盖提示词；每次都过 TTL 重读（面板热生效）。
func (m *promptOverrideManager) get(id message.QID, isGroup bool) (string, bool) {
	m.maybeReload()
	m.mu.Lock()
	defer m.mu.Unlock()
	if isGroup {
		p, ok := m.groups[id]
		return p, ok
	}
	p, ok := m.friends[id]
	return p, ok
}

// count 返回当前群聊/好友覆盖条数（日志用）。
func (m *promptOverrideManager) count() (groups, friends int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.groups), len(m.friends)
}

func (m *promptOverrideManager) maybeReload() {
	m.mu.Lock()
	if time.Since(m.lastCheck) < promptOverrideReloadTTL {
		m.mu.Unlock()
		return
	}
	m.lastCheck = time.Now()
	m.mu.Unlock()

	if m.editor == nil {
		return
	}
	v, ok := m.editor.Get(m.configKey)
	if !ok {
		return
	}
	raw, _ := v.(string)
	raw = strings.TrimSpace(raw)

	m.mu.Lock()
	defer m.mu.Unlock()
	if raw == m.lastRaw {
		return // 内容没变不重新解析
	}
	groups, friends, err := parsePromptOverrides(raw)
	if err != nil {
		m.logger.Warn("解析 Prompt 覆盖配置失败，沿用旧配置", "error", err.Error())
		return
	}
	m.lastRaw = raw
	m.groups, m.friends = groups, friends
}

// promptOverrideReloadTTL 配置中心重读间隔：面板保存后数秒内热生效。
const promptOverrideReloadTTL = 5 * time.Second

// promptOverrideConfig files.prompt_json 的 JSON schema。
type promptOverrideConfig struct {
	Groups  map[string]string `json:"groups"`
	Friends map[string]string `json:"friends"`
}

// parsePromptOverrides 解析并校验覆盖配置；JSON 非法时整体拒绝（避免面板写错一半生效）。
// 返回按 message.QID 规范化的群聊/好友覆盖表。
func parsePromptOverrides(raw string) (groups, friends map[message.QID]string, err error) {
	groups = map[message.QID]string{}
	friends = map[message.QID]string{}
	if strings.TrimSpace(raw) == "" {
		return groups, friends, nil
	}
	var fileCfg promptOverrideConfig
	if err := json.Unmarshal([]byte(raw), &fileCfg); err != nil {
		return nil, nil, fmt.Errorf("JSON 解析失败: %w", err)
	}
	for k, v := range fileCfg.Groups {
		groups[message.FromString(k)] = v
	}
	for k, v := range fileCfg.Friends {
		friends[message.FromString(k)] = v
	}
	return groups, friends, nil
}
