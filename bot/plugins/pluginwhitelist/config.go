package pluginwhitelist

// 配置键：白名单管理插件自身的开关与拦截强度；名单内容仍存在
// plugin.interceptor.* 下（由本插件读写），避免同一份名单出现两处配置。
const (
	keyEnable     = "plugin.whitelist.enable"
	keyBlockAll   = "plugin.whitelist.block_all"
	keyNotifyDeny = "plugin.whitelist.notify_denied"

	// interceptor 的名单键：本插件通过 ConfigEditor 读写这几个键，
	// 改完同步刷新 interceptor 的共享 ListStore，即时生效。
	keyIntEnable     = "plugin.interceptor.enable"
	keyIntMode       = "plugin.interceptor.mode"
	keyIntGroups     = "plugin.interceptor.groups"
	keyIntFriends    = "plugin.interceptor.friends"
	keyIntGroupUsers = "plugin.interceptor.group_users"
)

// whitelistConfig 白名单管理插件配置。
//
// 名单本身不在这里：群/用户名单与模式沿用 plugin.interceptor.* 那一份，
// 本插件只提供命令管理与热生效，避免两套名单彼此矛盾。
type whitelistConfig struct {
	Enable bool `cfg:"plugin.whitelist.enable" label:"启用白名单管理" group:"白名单管理" help:"启用后管理员可用 /wl 命令增删查名单，改动立即生效（无需 /reboot）；名单内容与模式沿用「请求拦截插件」的配置" default:"true"`
	// BlockAll 决定拦截强度：拦截判定本身由 interceptor 在 AI 之前做，
	// 本插件排在日志之后、其余插件之前，开启后非白名单会话的消息对所有
	// 功能插件都不可见（复读机、防撤回等一并拦住）。
	// 默认关闭：保持与原有行为一致，升级不改变既有部署的拦截范围。
	BlockAll bool `cfg:"plugin.whitelist.block_all" label:"拦住全部插件" group:"白名单管理" help:"开启后非白名单会话的消息不会传给任何功能插件（复读机、防撤回、AI 等全部拦住），仅日志与系统命令可用；默认关闭，只拦 AI 对话（与请求拦截插件原行为一致）" default:"false"`
	// NotifyDenied 是否给被拦截的会话一句提示。默认关闭：白名单场景下
	// 机器人通常应当对陌生群「完全沉默」，回提示反而暴露存在并可能被刷。
	NotifyDenied bool `cfg:"plugin.whitelist.notify_denied" label:"拦截时回提示" group:"白名单管理" help:"被拦截时回一句「未授权」提示。默认关闭——白名单场景下通常希望机器人对未授权会话完全沉默，不暴露存在也不被刷消息" default:"false"`
}

func (p *WhitelistPlugin) ConfigSchema() any {
	return &p.cfg
}
