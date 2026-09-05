package plugininterceptor

import (
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/jeanhua/AniaBot/common/model/message"
)

// 规范平台名（与各适配器 Definition.Platform 一致）
const (
	platformQQ         = "qq"
	platformQQOfficial = "qqofficial"
	platformFeishu     = "feishu"
	platformTelegram   = "telegram"
	platformDiscord    = "discord"
)

// allPlatforms 全平台集合：平台名单为 nil（从未配置，如旧版本调用）时视为全选，
// 与面板默认值一致，保证升级兼容。
var allPlatforms = []string{platformQQ, platformQQOfficial, platformTelegram, platformFeishu, platformDiscord}

// ListStore 名单状态的共享载体：解析后的群/用户/平台名单与模式，支持运行时热重载。
//
// 拆出独立类型的原因：名单原先在 Start 时一次性读进 map，面板改完必须 /reboot
// 才生效；白名单管理插件（插件市场 whitelist）需要在收到命令后立刻让改动生效，因此把状态与解析集中
// 到这里，由 Reload 原子替换，读取侧（拦截判定）走读锁。
type ListStore struct {
	mu         sync.RWMutex
	enable     bool
	mode       string
	groups     map[message.QID]struct{}
	friends    map[message.QID]struct{}
	groupUsers map[message.QID]map[message.QID]struct{}
	platforms  map[string]struct{}
}

func NewListStore() *ListStore {
	return &ListStore{
		mode:       modeBlacklist,
		groups:     map[message.QID]struct{}{},
		friends:    map[message.QID]struct{}{},
		groupUsers: map[message.QID]map[message.QID]struct{}{},
		platforms:  map[string]struct{}{},
	}
}

// ListSnapshot 名单的只读快照（供管理插件展示与判断，不暴露内部 map）
type ListSnapshot struct {
	Enable     bool
	Mode       string
	Groups     []string
	Friends    []string
	GroupUsers []string
	Platforms  []string
}

// NormalizePlatformToken 把平台名单项归一化为规范平台名。
// 接受平台名（qq/qqofficial/feishu/telegram/discord）与常用简称（qo/tg/fs/dc），
// 大小写不限，末尾带不带冒号均可（如 tg、TG:、telegram 都返回 telegram）。
// 第二个返回值为 false 表示不是合法的平台 token。
func NormalizePlatformToken(s string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.TrimSuffix(t, ":")
	t = strings.TrimSpace(t)
	switch t {
	case platformQQ:
		return platformQQ, true
	case platformQQOfficial, "qo":
		return platformQQOfficial, true
	case platformFeishu, "fs":
		return platformFeishu, true
	case platformTelegram, "tg":
		return platformTelegram, true
	case platformDiscord, "dc":
		return platformDiscord, true
	default:
		return "", false
	}
}

// inferPlatformFromQID 从框架统一 ID 推断平台：按前缀匹配，裸数字视为 QQ 旧格式。
// 无法识别时返回空串（调用方应透放，不做平台级拦截）。
func inferPlatformFromQID(id message.QID) string {
	s := strings.TrimSpace(string(id))
	if s == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(s, message.QQIDPrefix):
		return platformQQ
	case strings.HasPrefix(s, "qo:"):
		return platformQQOfficial
	case strings.HasPrefix(s, "fs:"):
		return platformFeishu
	case strings.HasPrefix(s, "tg:"):
		return platformTelegram
	case strings.HasPrefix(s, "dc:"):
		return platformDiscord
	}
	if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		return platformQQ
	}
	return ""
}

// PlatformOfMessage 解析消息所属的规范平台名。
// 优先使用适配器填充的 msg.Platform（同样走简称归一化，兼容大小写/简称）；
// 为空或无法识别时退化为从群 ID / 发送者 ID / 用户 ID 的前缀推断，
// 仍无法识别时返回空串（平台判定透放，仅按群/用户名单处理）。
func PlatformOfMessage(msg message.Message) string {
	if p := strings.TrimSpace(msg.Platform); p != "" {
		if norm, ok := NormalizePlatformToken(p); ok {
			return norm
		}
		return ""
	}
	for _, id := range []message.QID{msg.GroupId, msg.Sender.UserId, msg.UserId} {
		if p := inferPlatformFromQID(id); p != "" {
			return p
		}
	}
	return ""
}

// LoadWithPlatforms 用配置值重建全部名单（含平台名单）。
// platforms 为 nil 表示从未配置（兼容旧调用），视为全选；
// 显式空切片表示取消全选（全部平台拦截）。
// 未知的平台名与非法的「群ID:用户ID」规则交由 onBadRule 上报后跳过；
// 未知模式回落为黑名单（宁可少拦不可误拦全部）。
func (s *ListStore) LoadWithPlatforms(enable bool, mode string, groups, friends, groupUsers, platforms []string, onBadRule func(rule string)) {
	if platforms == nil {
		platforms = allPlatforms
	}
	p := make(map[string]struct{}, len(platforms))
	for _, line := range platforms {
		if strings.TrimSpace(line) == "" {
			continue
		}
		norm, ok := NormalizePlatformToken(line)
		if !ok {
			if onBadRule != nil {
				onBadRule(line)
			}
			continue
		}
		p[norm] = struct{}{}
	}
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
	s.groups, s.friends, s.groupUsers, s.platforms = g, f, gu, p
}

// Load 用配置值重建名单（兼容旧签名，供插件市场 whitelist 管理插件调用）。
// 为兼容已存在的平台配置，本方法保留当前平台名单不变，仅重建群/用户/群内规则；
// 需要连带更新平台名单时请使用 LoadWithPlatforms。
func (s *ListStore) Load(enable bool, mode string, groups, friends, groupUsers []string, onBadRule func(rule string)) {
	s.mu.RLock()
	kept := make([]string, 0, len(s.platforms))
	for p := range s.platforms {
		kept = append(kept, p)
	}
	s.mu.RUnlock()
	s.LoadWithPlatforms(enable, mode, groups, friends, groupUsers, kept, onBadRule)
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

// Platforms 返回平台名单的排序副本（供展示）
func (s *ListStore) Platforms() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.platforms))
	for p := range s.platforms {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// PlatformListed 平台是否在平台名单内（接受简称/大小写/冒号，空串返回 false）
func (s *ListStore) PlatformListed(platform string) bool {
	norm, ok := NormalizePlatformToken(platform)
	if !ok {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, hit := s.platforms[norm]
	return hit
}

// PlatformEnabled 平台总开关：该平台是否允许进入名单判定。
// 未知平台（空串）返回 true（透放，仅按群/用户名单处理，兼容自定义适配器）。
func (s *ListStore) PlatformEnabled(platform string) bool {
	norm, ok := NormalizePlatformToken(platform)
	if !ok {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, hit := s.platforms[norm]
	return hit
}

// AllowGroup 群聊是否放行（不含群内屏蔽成员与平台判定）
func (s *ListStore) AllowGroup(id message.QID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.allowLocked(id, s.groups)
}

// AllowFriend 用户是否放行（私聊，及黑名单模式下的群内发送者；不含平台判定）
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

// Counts 返回三类名单的条数（供日志与状态展示；保持旧签名兼容）
func (s *ListStore) Counts() (groups, friends, groupUsers int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.groups), len(s.friends), len(s.groupUsers)
}

// CountsEx 返回含平台名单在内的四类条数（供日志与状态展示）
func (s *ListStore) CountsEx() (groups, friends, groupUsers, platforms int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.groups), len(s.friends), len(s.groupUsers), len(s.platforms)
}

// Snapshot 返回名单的只读快照（供管理插件展示与判断，不暴露内部 map）
func (s *ListStore) Snapshot() ListSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := ListSnapshot{Enable: s.enable, Mode: s.mode}
	for id := range s.groups {
		snap.Groups = append(snap.Groups, string(id))
	}
	for id := range s.friends {
		snap.Friends = append(snap.Friends, string(id))
	}
	for p := range s.platforms {
		snap.Platforms = append(snap.Platforms, p)
	}
	for group, users := range s.groupUsers {
		for user := range users {
			snap.GroupUsers = append(snap.GroupUsers, string(group)+":"+string(user))
		}
	}
	sort.Strings(snap.Groups)
	sort.Strings(snap.Friends)
	sort.Strings(snap.GroupUsers)
	sort.Strings(snap.Platforms)
	return snap
}
