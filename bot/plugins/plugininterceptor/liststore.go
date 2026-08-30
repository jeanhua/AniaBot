package plugininterceptor

import (
	"strings"
	"sync"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// ListStore 名单状态的共享载体：解析后的群/用户名单与模式，支持运行时热重载。
//
// 拆出独立类型的原因：名单原先在 Start 时一次性读进 map，面板改完必须 /reboot
// 才生效；白名单管理插件需要在收到命令后立刻让改动生效，因此把状态与解析集中
// 到这里，由 Reload 原子替换，读取侧（拦截判定）走读锁。
type ListStore struct {
	mu         sync.RWMutex
	enable     bool
	mode       string
	groups     map[message.QID]struct{}
	friends    map[message.QID]struct{}
	groupUsers map[message.QID]map[message.QID]struct{}
}

func NewListStore() *ListStore {
	return &ListStore{
		mode:       modeBlacklist,
		groups:     map[message.QID]struct{}{},
		friends:    map[message.QID]struct{}{},
		groupUsers: map[message.QID]map[message.QID]struct{}{},
	}
}

// ListSnapshot 名单的只读快照（供管理插件展示与判断，不暴露内部 map）
type ListSnapshot struct {
	Enable     bool
	Mode       string
	Groups     []string
	Friends    []string
	GroupUsers []string
}

// Load 用配置值重建名单。非法的「群ID:用户ID」规则交由 onBadRule 上报后跳过；
// 未知模式回落为黑名单（与原行为一致，宁可少拦不可误拦全部）。
func (s *ListStore) Load(enable bool, mode string, groups, friends, groupUsers []string, onBadRule func(rule string)) {
	g := make(map[message.QID]struct{}, len(groups))
	for _, id := range groups {
		if id = strings.TrimSpace(id); id != "" {
			g[message.FromString(id)] = struct{}{}
		}
	}
	f := make(map[message.QID]struct{}, len(friends))
	for _, id := range friends {
		if id = strings.TrimSpace(id); id != "" {
			f[message.FromString(id)] = struct{}{}
		}
	}
	gu := make(map[message.QID]map[message.QID]struct{}, len(groupUsers))
	for _, line := range groupUsers {
		if strings.TrimSpace(line) == "" {
			continue
		}
		group, user, ok := splitGroupUser(line)
		if !ok {
			if onBadRule != nil {
				onBadRule(line)
			}
			continue
		}
		if gu[group] == nil {
			gu[group] = make(map[message.QID]struct{})
		}
		gu[group][user] = struct{}{}
	}
	if mode != modeBlacklist && mode != modeWhitelist {
		mode = modeBlacklist
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.enable, s.mode = enable, mode
	s.groups, s.friends, s.groupUsers = g, f, gu
}

// Enabled 名单功能是否启用（关闭时全部放行）
func (s *ListStore) Enabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enable
}

// Mode 当前名单模式
func (s *ListStore) Mode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mode
}

// IsWhitelist 当前是否为白名单模式
func (s *ListStore) IsWhitelist() bool {
	return s.Mode() == modeWhitelist
}

// AllowGroup 群聊是否放行（不含群内屏蔽成员判定）
func (s *ListStore) AllowGroup(id message.QID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowLocked(id, s.groups)
}

// AllowFriend 用户是否放行（私聊，及黑名单模式下的群内发送者）
func (s *ListStore) AllowFriend(id message.QID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowLocked(id, s.friends)
}

// allowLocked 白名单模式下仅名单内放行，黑名单模式下名单内拦截（调用方须持读锁）
func (s *ListStore) allowLocked(id message.QID, list map[message.QID]struct{}) bool {
	_, inList := list[id]
	if s.mode == modeWhitelist {
		return inList
	}
	return !inList
}

// BlockedInGroup 用户是否被「群内屏蔽成员」规则命中（硬性拦截，不区分名单模式）
func (s *ListStore) BlockedInGroup(group, user message.QID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users, ok := s.groupUsers[group]
	if !ok {
		return false
	}
	_, hit := users[user]
	return hit
}

// Counts 返回三类名单的条数（供日志与状态展示）
func (s *ListStore) Counts() (groups, friends, groupUsers int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.groups), len(s.friends), len(s.groupUsers)
}
