package plugininterceptor

import (
	"context"
	"strings"

	"github.com/jeanhua/AniaBot/common/bot"
	"github.com/jeanhua/AniaBot/common/model/command"
	"github.com/jeanhua/AniaBot/common/model/message"
	"github.com/jeanhua/AniaBot/common/plugin"
	"github.com/jeanhua/AniaBot/common/plugininfo"
	"github.com/spf13/viper"
)

// 名单模式
const (
	modeBlacklist = "blacklist" // 黑名单：名单内的会话被拦截
	modeWhitelist = "whitelist" // 白名单：仅名单内的会话放行
)

// InterceptorPlugin 请求拦截插件：位于日志插件与 AI 对话插件之间，
// 按白名单/黑名单模式放行或屏蔽某些群聊、好友的消息（返回 false
// 终止传播，后续插件——主要是 AI 对话插件——不再收到该消息）。
type InterceptorPlugin struct {
	plugin.Meta
	// cfg 插件配置，由框架在 Start 前自动填充（见 ConfigSchema）
	cfg interceptorConfig

	groups  map[message.QID]struct{}
	friends map[message.QID]struct{}
	// groupUsers 群内屏蔽成员：groupID -> 被屏蔽的 userID 集合。
	// 硬性拦截（不区分名单模式，优先级最高），仅作用于群聊。
	groupUsers map[message.QID]map[message.QID]struct{}
}

func NewPlugin() *InterceptorPlugin {
	p := &InterceptorPlugin{}
	p.Name = "请求拦截插件"
	p.HelpWords = "按白名单/黑名单模式放行或屏蔽指定群聊、好友的 AI 请求，支持屏蔽群内指定成员，请在 Web 控制面板配置"
	p.AdminOnly = true
	p.Author = "jeanhua"
	p.Version = "1.1.0"
	// 在普通插件（复读机、防撤回等）之后、AI 对话插件之前执行：
	// 被拦截的会话仍可使用其他功能插件，仅 AI 请求被屏蔽
	p.Order = plugin.LevelLog + 100
	p.ShowFor = plugininfo.ShowForNone
	return p
}

func (p *InterceptorPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	if !p.cfg.Enable {
		p.Logger.Info("请求拦截插件已加载（未启用拦截，放行全部消息）")
		return nil
	}

	p.groups = make(map[message.QID]struct{}, len(p.cfg.Groups))
	for _, id := range p.cfg.Groups {
		p.groups[message.FromString(strings.TrimSpace(id))] = struct{}{}
	}
	p.friends = make(map[message.QID]struct{}, len(p.cfg.Friends))
	for _, id := range p.cfg.Friends {
		p.friends[message.FromString(strings.TrimSpace(id))] = struct{}{}
	}
	p.groupUsers = make(map[message.QID]map[message.QID]struct{}, len(p.cfg.GroupUsers))
	for _, line := range p.cfg.GroupUsers {
		group, user, ok := splitGroupUser(line)
		if !ok {
			p.Logger.Warn("忽略非法的群内屏蔽成员规则", "rule", line)
			continue
		}
		if p.groupUsers[group] == nil {
			p.groupUsers[group] = make(map[message.QID]struct{})
		}
		p.groupUsers[group][user] = struct{}{}
	}

	if p.cfg.Mode != modeBlacklist && p.cfg.Mode != modeWhitelist {
		p.Logger.Warn("未知的名单模式，按黑名单模式处理", "mode", p.cfg.Mode)
		p.cfg.Mode = modeBlacklist
	}

	p.Logger.Info("请求拦截插件初始化完成",
		"mode", p.cfg.Mode,
		"groups", len(p.groups),
		"friends", len(p.friends),
		"groupUsers", len(p.groupUsers))
	return nil
}

// allow 判断指定会话是否放行。whitelist 模式下仅名单内放行，
// blacklist 模式下名单内拦截。
func (p *InterceptorPlugin) allow(id message.QID, list map[message.QID]struct{}) bool {
	_, inList := list[id]
	if p.cfg.Mode == modeWhitelist {
		return inList
	}
	return !inList
}

func (p *InterceptorPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.cfg.Enable {
		return true, nil
	}
	if !p.allow(msg.GroupId, p.groups) {
		p.Logger.Info("拦截群聊消息", "mode", p.cfg.Mode, "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	if p.blockedInGroup(msg.GroupId, msg.Sender.UserId) {
		p.Logger.Info("拦截群内屏蔽成员消息", "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	if p.cfg.Mode == modeWhitelist {
		// 白名单模式下，被放行的群对全体成员开放（群内屏蔽成员规则除外），
		// 无需逐成员加入用户名单；用户名单此时仅作用于私聊
		return true, nil
	}
	if !p.allow(msg.Sender.UserId, p.friends) {
		p.Logger.Info("拦截群内成员消息", "mode", p.cfg.Mode, "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	return true, nil
}

// blockedInGroup 判断用户是否被"群内屏蔽成员"规则命中。
// 该规则为硬性拦截：不区分名单模式，仅作用于群聊（私聊不受影响）。
func (p *InterceptorPlugin) blockedInGroup(group, user message.QID) bool {
	users, ok := p.groupUsers[group]
	if !ok {
		return false
	}
	_, hit := users[user]
	return hit
}

// idPrefixes 已知的平台 ID 前缀（QQ 为纯数字无前缀）。
// 用于解析"群ID:用户ID"规则时确定群段边界：群段带前缀时第一个冒号属于前缀，
// 边界在第二个冒号处；否则边界在第一个冒号处。
var idPrefixes = []string{"tg:", "fs:"}

// splitGroupUser 解析一行"群ID:用户ID"规则，返回群 ID 与用户 ID。
func splitGroupUser(line string) (group, user message.QID, ok bool) {
	line = strings.TrimSpace(line)
	first := strings.Index(line, ":")
	if first < 0 {
		return "", "", false
	}
	boundary := first
	for _, prefix := range idPrefixes {
		if strings.HasPrefix(line, prefix) {
			rest := line[len(prefix):]
			next := strings.Index(rest, ":")
			if next < 0 {
				return "", "", false
			}
			boundary = len(prefix) + next
			break
		}
	}
	g, u := strings.TrimSpace(line[:boundary]), strings.TrimSpace(line[boundary+1:])
	if g == "" || u == "" {
		return "", "", false
	}
	return message.QID(g), message.QID(u), true
}

func (p *InterceptorPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.cfg.Enable {
		return true, nil
	}
	if !p.allow(msg.Sender.UserId, p.friends) {
		p.Logger.Info("拦截好友消息", "mode", p.cfg.Mode, "userId", msg.Sender.UserId)
		return false, nil
	}
	return true, nil
}
