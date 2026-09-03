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

// SharedStore 名单的进程内共享实例：拦截判定与白名单管理插件读写同一份状态，
// 管理插件改完调用 Load 即时生效，无需 /reboot。
// 在 NewPlugin 中赋值；白名单管理插件通过 Store() 获取。
var sharedStore = NewListStore()

// Store 返回共享的名单存储，供白名单管理插件读写。
func Store() *ListStore { return sharedStore }

// InterceptorPlugin 请求拦截插件：位于日志插件与 AI 对话插件之间，
// 按白名单/黑名单模式放行或屏蔽某些群聊、好友的消息（返回 false
// 终止传播，后续插件——主要是 AI 对话插件——不再收到该消息）。
//
// 名单状态存放在共享的 ListStore 中：面板改完由白名单管理插件热重载，
// 也可由 /wl 命令即时增删。
type InterceptorPlugin struct {
	plugin.Meta
	// cfg 插件配置，由框架在 Start 前自动填充（见 ConfigSchema）
	cfg   interceptorConfig
	store *ListStore
}

func NewPlugin() *InterceptorPlugin {
	p := &InterceptorPlugin{store: sharedStore}
	p.Name = "请求拦截插件"
	p.HelpWords = "按白名单/黑名单模式放行或屏蔽指定群聊、好友的 AI 请求，支持屏蔽群内指定成员，请在 Web 控制面板配置"
	p.AdminOnly = true
	p.Author = "jeanhua"
	p.Version = "1.2.0"
	// 在普通插件（复读机等）之后、AI 对话插件之前执行：
	// 被拦截的会话仍可使用其他功能插件，仅 AI 请求被屏蔽
	p.Order = plugin.LevelLog + 100
	p.ShowFor = plugininfo.ShowForNone
	return p
}

func (p *InterceptorPlugin) Start(ctx context.Context, cfg *viper.Viper) error {
	p.store.Load(p.cfg.Enable, p.cfg.Mode, p.cfg.Groups, p.cfg.Friends, p.cfg.GroupUsers,
		func(rule string) { p.Logger.Warn("忽略非法的群内屏蔽成员规则", "rule", rule) })

	if !p.cfg.Enable {
		p.Logger.Info("请求拦截插件已加载（未启用拦截，放行全部消息）")
		return nil
	}
	groups, friends, groupUsers := p.store.Counts()
	p.Logger.Info("请求拦截插件初始化完成",
		"mode", p.store.Mode(),
		"groups", groups,
		"friends", friends,
		"groupUsers", groupUsers)
	return nil
}

func (p *InterceptorPlugin) OnGroupMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.store.Enabled() {
		return true, nil
	}
	if !p.store.AllowGroup(msg.GroupId) {
		p.Logger.Info("拦截群聊消息", "mode", p.store.Mode(), "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	if p.store.BlockedInGroup(msg.GroupId, msg.Sender.UserId) {
		p.Logger.Info("拦截群内屏蔽成员消息", "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	if p.store.IsWhitelist() {
		// 白名单模式下，被放行的群对全体成员开放（群内屏蔽成员规则除外），
		// 无需逐成员加入用户名单；用户名单此时仅作用于私聊
		return true, nil
	}
	if !p.store.AllowFriend(msg.Sender.UserId) {
		p.Logger.Info("拦截群内成员消息", "mode", p.store.Mode(), "groupId", msg.GroupId, "userId", msg.Sender.UserId)
		return false, nil
	}
	return true, nil
}

// idPrefixes 已知的平台 ID 前缀（QQ 为 qq:，其余平台为各自前缀）。
// 用于解析"群ID:用户ID"规则时确定群段边界：群段带前缀时第一个冒号属于前缀，
// 边界在第二个冒号处；否则边界在第一个冒号处。
var idPrefixes = []string{message.QQIDPrefix, "qo:", "tg:", "fs:", "dc:"}

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
	return message.FromString(g), message.FromString(u), true
}

func (p *InterceptorPlugin) OnFriendMsg(ctx context.Context, bot bot.Bot, cmd command.Command, msg message.Message) (bool, error) {
	if !p.store.Enabled() {
		return true, nil
	}
	if !p.store.AllowFriend(msg.Sender.UserId) {
		p.Logger.Info("拦截好友消息", "mode", p.store.Mode(), "userId", msg.Sender.UserId)
		return false, nil
	}
	return true, nil
}
