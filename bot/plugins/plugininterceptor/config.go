package plugininterceptor

// interceptorConfig 请求拦截插件的配置结构体。实现 plugin.ConfigSchemaProvider
// 后，框架启动时自动注册字段（面板渲染 + 默认值补齐），并在 Start 前填充完成。
type interceptorConfig struct {
	Enable bool   `cfg:"plugin.interceptor.enable" label:"启用请求拦截" group:"请求拦截插件" help:"开启后按名单模式放行或屏蔽群聊/好友的 AI 请求" default:"false"`
	Mode   string `cfg:"plugin.interceptor.mode" label:"名单模式" type:"select" options:"blacklist,whitelist" group:"请求拦截插件" help:"blacklist=名单内的群/用户被屏蔽；whitelist=仅名单内的群放行，放行的群对全部成员开放（私聊仍按用户名单）" default:"blacklist"`
	// 名单留空的语义：blacklist 模式下表示不屏蔽任何会话；
	// whitelist 模式下表示拦截所有群聊（私聊按用户名单，用户名单为空则全部拦截）
	// ID 支持多平台格式：QQ 为纯数字，其他平台带前缀（如飞书 fs:oc_xxx）
	Groups  []string `cfg:"plugin.interceptor.groups" label:"群 ID 名单" group:"请求拦截插件" help:"每行一个群 ID（QQ 为群号，其他平台为带前缀的群 ID）"`
	Friends []string `cfg:"plugin.interceptor.friends" label:"用户 ID 名单" group:"请求拦截插件" help:"每行一个用户 ID（QQ 为 QQ 号，其他平台为带前缀的用户 ID）。黑名单模式下对私聊及群聊发送者均生效；白名单模式下仅作用于私聊（放行的群对全部成员开放）"`
	// GroupUsers 群内屏蔽成员（硬性拦截，不区分名单模式，优先级最高）：
	// 每行一条"群ID:用户ID"，仅在该群内屏蔽该成员的消息，私聊与其他群不受影响。
	// 用于"放行某个群但禁止群内的某个人"等场景。
	GroupUsers []string `cfg:"plugin.interceptor.group_users" label:"群内屏蔽成员" group:"请求拦截插件" help:"每行一条 群ID:用户ID（如 tg:-100123:tg:98765，QQ 为 群号:QQ号），仅在该群内屏蔽该成员，私聊不受影响；不区分名单模式，优先级最高"`
}

// ConfigSchema 实现 plugin.ConfigSchemaProvider：返回配置结构体指针，
// 框架在 Start 前自动完成注册与填充，Start 里直接读 p.cfg。
func (p *InterceptorPlugin) ConfigSchema() any {
	return &p.cfg
}
