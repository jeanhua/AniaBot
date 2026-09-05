package plugininterceptor

// interceptorConfig 请求拦截插件的配置结构体。实现 plugin.ConfigSchemaProvider
// 后，框架启动时自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成。
type interceptorConfig struct {
	Enable bool   `cfg:"plugin.interceptor.enable" label:"启用请求拦截" group:"请求拦截插件" help:"开启后按名单模式放行或屏蔽群聊/好友的 AI 请求" default:"false"`
	Mode   string `cfg:"plugin.interceptor.mode" label:"名单模式" type:"select" options:"blacklist,whitelist" group:"请求拦截插件" help:"blacklist=名单内的群/用户被屏蔽；whitelist=仅名单内的群放行，放行的群对全部成员开放（私聊仍按用户名单）" default:"blacklist"`
	// Platforms 可用平台总开关（多选，默认全选）：仅勾选平台的消息进入名单判定，
	// 未勾选平台的群聊与私聊直接拦截（不触发 AI），与名单模式无关。
	// 默认全选即与旧行为一致；想单独屏蔽某平台（如只保留 QQ）时取消勾选它即可。
	// 黑名单模式下勾选即放行该平台（名单命中的群/用户除外）；
	// 白名单模式下勾选是放行的必要条件，还需群/用户名单命中。
	Platforms []string `cfg:"plugin.interceptor.platforms" label:"可用平台" type:"multiselect" options:"qq,qqofficial,telegram,feishu,discord" group:"请求拦截插件" help:"仅勾选平台的消息进入名单判定，未勾选的平台直接拦截（不触发 AI）。默认全选（与旧行为一致）；单独屏蔽某平台时取消勾选它" default:"qq,qqofficial,telegram,feishu,discord"`
	// 名单留空的语义：blacklist 模式下表示不屏蔽任何会话；
	// whitelist 模式下表示拦截所有群聊（私聊按用户名单，用户名单为空则全部拦截）
	// ID 支持多平台格式：QQ 为 qq: 前缀，其他平台带前缀（如飞书 fs:oc_xxx）
	Groups  []string `cfg:"plugin.interceptor.groups" label:"群 ID 名单" group:"请求拦截插件" help:"每行一个群 ID（QQ 为 qq:群号，其他平台为带前缀的群 ID）"`
	Friends []string `cfg:"plugin.interceptor.friends" label:"用户 ID 名单" group:"请求拦截插件" help:"每行一个用户 ID（QQ 为 qq:QQ号，其他平台为带前缀的用户 ID）。黑名单模式下对私聊及群聊发送者均生效；白名单模式下仅作用于私聊（放行的群对全部成员开放）"`
	// GroupUsers 群内屏蔽成员（硬性拦截，不区分名单模式，优先级最高）：
	// 每行一条"群ID:用户ID"，仅在该群内屏蔽该成员的消息，私聊与其他群不受影响。
	// 用于"放行某个群但禁止群内的某个人"等场景。
	GroupUsers []string `cfg:"plugin.interceptor.group_users" label:"群内屏蔽成员" group:"请求拦截插件" help:"每行一条 群ID:用户ID（如 tg:-100123:tg:98765、qq:123:qq:456），仅在该群内屏蔽该成员，私聊不受影响；不区分名单模式，优先级最高"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成注册与填充，Start 里直接读 p.cfg。
func (p *InterceptorPlugin) ConfigSchema() any {
	return &p.cfg
}
